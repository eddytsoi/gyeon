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

// mapStatus translates ShipAny's status string into our internal order
// status enum. Returns "" (no advance) for anything we don't recognise
// — the caller still echoes a 200 so ShipAny doesn't retry, but the
// order stays where it is.
//
// Real ShipAny event names (per merchant confirmation):
//   - Collected_By_Courier        → shipped
//   - Order_Delivered             → delivered
//   - Order_Completed             → delivered
//
// Comparison is case-insensitive in case ShipAny normalises differently
// between events.
func mapStatus(s string) orders.OrderStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "collected_by_courier":
		return orders.StatusShipped
	case "order_delivered", "order_completed":
		return orders.StatusDelivered
	}
	return ""
}

// noteForStatus is the human-readable note attached to the status
// transition. Shows up in the order timeline and order_status_history.
func noteForStatus(s string, target orders.OrderStatus) string {
	return "ShipAny: " + s + " (→ " + string(target) + ")"
}

// resolveOrderID accepts either a Gyeon UUID (native orders) or a
// WooCommerce numeric ID (imported orders). Returns Gyeon's internal
// UUID, or an empty string + ErrOrderNotFound if no row matches.
func resolveOrderID(ctx context.Context, db *sql.DB, pathID string) (string, error) {
	if _, err := uuid.Parse(pathID); err == nil {
		// Probably a native Gyeon UUID — confirm it exists.
		var id string
		err := db.QueryRowContext(ctx, `SELECT id FROM orders WHERE id = $1`, pathID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return "", orders.ErrOrderNotFound
		}
		return id, err
	}
	// Try numeric WC order id (legacy imports).
	wcID, convErr := strconv.Atoi(pathID)
	if convErr != nil {
		return "", orders.ErrOrderNotFound
	}
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE wc_order_id = $1`, wcID).Scan(&id)
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

	var body wcOrderUpdate
	if err := json.Unmarshal(raw, &body); err != nil {
		log.Printf("wcshim: decode body for order %s: %v body=%q",
			pathID, err, truncate(string(raw), 256))
		respond.JSON(w, http.StatusOK, map[string]any{"id": pathID})
		return
	}

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
