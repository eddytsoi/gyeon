package wcshim

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/respond"
)

// wcOrderUpdate is the minimal slice of the WC Orders REST schema we read.
// ShipAny sends a small payload — `status` plus a few `meta_data` entries
// carrying the shipment details. We deliberately tolerate unknown fields.
type wcOrderUpdate struct {
	Status   string   `json:"status"`
	MetaData []wcMeta `json:"meta_data"`
}

// wcMeta mirrors WC's `meta_data[]` shape. Value is intentionally raw —
// WC sends scalars and objects in the same slot, and ShipAny's tracking
// payload is an object.
type wcMeta struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// shipanyTrackingMeta is the structure that ShipAny / the WC plugin
// stores under the `_pr_shipment_shipany_label_tracking` order meta key.
// See: source/shipany/includes/class-shipany-wc-order-ecs-asia.php
// (the plugin writes the same shape it receives back from ShipAny).
type shipanyTrackingMeta struct {
	ShipmentID     string `json:"shipment_id"`
	TrackingNumber string `json:"tracking_number"`
	TrackingURL    string `json:"tracking_url"`
	LabelURL       string `json:"label_url"`
}

// normalizeStatus canonicalises a ShipAny status string so matching is
// robust to formatting drift. ShipAny's real vocabulary is space-separated
// Title Case — "Collected By Courier", "Order Delivered" — see the plugin's
// ShipanyHelper::get_all_order_status(). The previous implementation matched
// underscore_case ("collected_by_courier"), which never matched the spaced
// form ShipAny actually sends, so every status fell through to "unmapped"
// and no order ever advanced. We now lowercase and collapse every run of
// underscores/whitespace to a single space, so spaced, underscored and
// odd-cased variants all match.
func normalizeStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	pendingSep := false
	for _, r := range s {
		if r == '_' || unicode.IsSpace(r) {
			pendingSep = b.Len() > 0
			continue
		}
		if pendingSep {
			b.WriteByte(' ')
			pendingSep = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mapStatus translates ShipAny's shipment status into our internal order
// status enum. Returns "" (no advance) for anything we don't recognise —
// the caller still echoes a 200 so ShipAny doesn't retry, but the order
// stays where it is. Unrecognised non-empty statuses are logged "unmapped".
//
// Gyeon's transition graph is paid→processing→shipped→delivered
// (orders.allowedTransitions). The paid→processing advance is handled when
// the shipment is created (shipany.QueueHandler), so here we only map the
// in-transit milestones (→ shipped) and the terminal ones (→ delivered).
// Several ShipAny statuses collapse onto each milestone; the first to
// arrive advances the order and any later one logs a benign "cannot
// transition" and is ignored. Pre-pickup states (e.g. "Order Created",
// "Pickup Request Sent") are intentionally left unmapped.
func mapStatus(s string) orders.OrderStatus {
	switch normalizeStatus(s) {
	case "collected by courier",
		"in transit", "shipping",
		"arrived transit point", "departed transit point",
		"delivery in progress", "ready for delivery",
		"out for delivery", "delivering to convenience store":
		return orders.StatusShipped
	case "order delivered", "order completed",
		"collected by customer",
		"delivered to locker", "delivered to service point":
		return orders.StatusDelivered
	}
	return ""
}

// noteForStatus is the human-readable note attached to the status
// transition. Shows up in the order timeline and order_status_history.
func noteForStatus(s string, target orders.OrderStatus) string {
	return "ShipAny: " + s + " (→ " + string(target) + ")"
}

// resolveOrderID maps the {id} in ShipAny's callback path to Gyeon's
// internal order UUID. ShipAny echoes whatever external reference it stored
// at shipment-create time. Gyeon sends it both ext_order_id (the UUID) and
// ext_order_ref (the human order number, e.g. "ORD-5117"), and legacy
// imports also carry a numeric WooCommerce id — so we accept all three:
//
//  1. a Gyeon UUID            → orders.id
//  2. a numeric WC order id   → orders.wc_order_id
//  3. the human order number  → orders.order_number   (ext_order_ref)
//
// Returns ErrOrderNotFound if nothing matches.
func resolveOrderID(ctx context.Context, db *sql.DB, pathID string) (string, error) {
	if _, err := uuid.Parse(pathID); err == nil {
		var id string
		err := db.QueryRowContext(ctx, `SELECT id FROM orders WHERE id = $1`, pathID).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		// A well-formed UUID that isn't an order id — fall through.
	}
	if wcID, convErr := strconv.Atoi(pathID); convErr == nil {
		var id string
		err := db.QueryRowContext(ctx,
			`SELECT id FROM orders WHERE wc_order_id = $1`, wcID).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	// Final fallback: ShipAny may echo ext_order_ref, which Gyeon set to the
	// human order number.
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE order_number = $1`, pathID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", orders.ErrOrderNotFound
	}
	return id, err
}

// extractTracking walks meta_data[] for the WC plugin's tracking blob.
// Returns nil if the key is absent or unparseable.
func extractTracking(meta []wcMeta) *shipanyTrackingMeta {
	for _, m := range meta {
		if m.Key != "_pr_shipment_shipany_label_tracking" {
			continue
		}
		var t shipanyTrackingMeta
		if err := json.Unmarshal(m.Value, &t); err != nil {
			// WC sometimes stores the value as a serialized string; try
			// unwrapping one level of string before giving up.
			var s string
			if jerr := json.Unmarshal(m.Value, &s); jerr == nil {
				if err2 := json.Unmarshal([]byte(s), &t); err2 == nil {
					return &t
				}
			}
			return nil
		}
		return &t
	}
	return nil
}

// upsertShipmentTracking writes the tracking blob onto the order's
// existing shipment row, keyed by shipany_shipment_id (which is UNIQUE)
// when present, falling back to order_id. If no shipment exists yet
// this is a no-op (Gyeon admin creates shipments via the existing
// shipany.CreateForOrder path; a status-update arriving before that
// is logged but not synthesised into a stub row).
func upsertShipmentTracking(ctx context.Context, db *sql.DB, orderID string, t *shipanyTrackingMeta) error {
	if t.ShipmentID != "" {
		res, err := db.ExecContext(ctx,
			`UPDATE shipments
			    SET tracking_number = COALESCE(NULLIF($2,''), tracking_number),
			        tracking_url    = COALESCE(NULLIF($3,''), tracking_url),
			        label_url       = COALESCE(NULLIF($4,''), label_url)
			  WHERE shipany_shipment_id = $1`,
			t.ShipmentID, t.TrackingNumber, t.TrackingURL, t.LabelURL)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return nil
		}
	}
	// Fall back to matching by order_id — covers older shipments that
	// pre-date the shipment_id ever being persisted.
	_, err := db.ExecContext(ctx,
		`UPDATE shipments
		    SET tracking_number    = COALESCE(NULLIF($2,''), tracking_number),
		        tracking_url       = COALESCE(NULLIF($3,''), tracking_url),
		        label_url          = COALESCE(NULLIF($4,''), label_url),
		        shipany_shipment_id = COALESCE(NULLIF($5,''), shipany_shipment_id)
		  WHERE order_id = $1`,
		orderID, t.TrackingNumber, t.TrackingURL, t.LabelURL, t.ShipmentID)
	return err
}

// updateOrderHandler is the only WC endpoint we expose:
//
//	PUT /wp-json/wc/v3/orders/{id}
//
// It advances Gyeon's order status and (optionally) refreshes the
// tracking blob on the related shipment row. Always replies 200 with
// a minimal echo so ShipAny doesn't retry on transient internal errors.
func (h *Handler) updateOrderHandler(w http.ResponseWriter, r *http.Request) {
	pathID := chi.URLParam(r, "id")

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		log.Printf("wcshim: read body for order %s: %v", pathID, err)
		respond.JSON(w, http.StatusOK, map[string]any{"id": pathID})
		return
	}

	// TEMP diagnostic: ShipAny's notification payload shape is not documented
	// in the plugin source (its server builds the PUT). Log the raw body so we
	// can confirm where ShipAny actually puts the shipment status. Downgrade to
	// debug / remove once the status-update path is confirmed working in prod.
	log.Printf("wcshim: inbound PUT order=%s from=%s body=%s",
		pathID, r.RemoteAddr, truncate(string(raw), 2048))

	var body wcOrderUpdate
	if err := json.Unmarshal(raw, &body); err != nil {
		log.Printf("wcshim: decode body for order %s: %v body=%q",
			pathID, err, truncate(string(raw), 256))
		respond.JSON(w, http.StatusOK, map[string]any{"id": pathID})
		return
	}

	// TEMP diagnostic: show what we parsed out of the WC schema so a mismatch
	// (empty status, or the status sitting in an unexpected meta key) is obvious.
	metaKeys := make([]string, 0, len(body.MetaData))
	for _, m := range body.MetaData {
		metaKeys = append(metaKeys, m.Key)
	}
	log.Printf("wcshim: parsed order=%s status=%q meta_keys=%v", pathID, body.Status, metaKeys)

	orderID, err := resolveOrderID(r.Context(), h.db, pathID)
	if err != nil {
		log.Printf("wcshim: unknown order %q (status=%q)", pathID, body.Status)
		respond.JSON(w, http.StatusOK, map[string]any{"id": pathID})
		return
	}

	// 1. Advance order status if mapped.
	if target := mapStatus(body.Status); target != "" {
		note := noteForStatus(body.Status, target)
		if _, err := h.orderSvc.UpdateStatus(r.Context(), orderID,
			orders.UpdateStatusRequest{Status: target, Note: &note}); err != nil {
			// Most likely an invalid forward transition (e.g. delivered
			// while still pending because we missed an event). Log only —
			// the shipment row is the source of truth for ops.
			log.Printf("wcshim: advance order %s to %s: %v", orderID, target, err)
		}
	} else if body.Status != "" {
		log.Printf("wcshim: unmapped ShipAny status %q for order %s", body.Status, orderID)
	}

	// 2. Refresh shipment tracking columns when ShipAny sent the meta.
	if t := extractTracking(body.MetaData); t != nil {
		if err := upsertShipmentTracking(r.Context(), h.db, orderID, t); err != nil {
			log.Printf("wcshim: update shipment tracking for order %s: %v", orderID, err)
		}
	}

	// 3. Minimal WC-shaped echo. ShipAny doesn't appear to consume the
	//    response body, but returning something parseable is polite.
	respond.JSON(w, http.StatusOK, map[string]any{
		"id":        pathID,
		"status":    body.Status,
		"meta_data": body.MetaData,
	})
}

// truncate is a local debug helper so error logs don't dump megabyte payloads.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
