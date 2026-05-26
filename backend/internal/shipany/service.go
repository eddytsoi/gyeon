package shipany

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/settings"
)

// Service orchestrates ShipAny calls + persistence + order-status sync.
type Service struct {
	client    *HTTPClient
	settings  *settings.Service
	db        *sql.DB
	orderSvc  *orders.OrderService
}

func NewService(client *HTTPClient, settings *settings.Service, db *sql.DB, orderSvc *orders.OrderService) *Service {
	return &Service{client: client, settings: settings, db: db, orderSvc: orderSvc}
}

// Enabled reports the master toggle. Settings UI flips this; storefront
// + admin both gate features behind it.
func (s *Service) Enabled(ctx context.Context) bool {
	return s.read(ctx, "shipany_enabled") == "true"
}

// Configured returns true when the toggle is on AND credentials are present.
// Used by the storefront to decide whether to call /quote at all.
func (s *Service) Configured(ctx context.Context) bool {
	return s.Enabled(ctx) &&
		s.read(ctx, "shipany_user_id") != "" &&
		s.read(ctx, "shipany_api_key") != ""
}

// ── Quote ──────────────────────────────────────────────────────────────

type CartLine struct {
	WeightGrams int
	Quantity    int
	LengthMM    int
	WidthMM     int
	HeightMM    int
}

// QuoteForCart builds a parcel from the cart's variant weights (or the
// configured fallback) and asks ShipAny for available rate options.
// Destination is the customer-supplied checkout address.
func (s *Service) QuoteForCart(ctx context.Context, dest Address, lines []CartLine, declaredValueHKD float64) ([]RateOption, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}
	totalWeight := 0
	maxL, maxW, maxH := 0, 0, 0
	for _, ln := range lines {
		w := ln.WeightGrams
		if w <= 0 {
			w = s.defaultWeight(ctx)
		}
		totalWeight += w * ln.Quantity
		if ln.LengthMM > maxL {
			maxL = ln.LengthMM
		}
		if ln.WidthMM > maxW {
			maxW = ln.WidthMM
		}
		if ln.HeightMM > maxH {
			maxH = ln.HeightMM
		}
	}
	if totalWeight <= 0 {
		totalWeight = s.defaultWeight(ctx)
	}

	dest.Country = strings.ToUpper(strings.TrimSpace(dest.Country))
	if dest.Country == "" {
		dest.Country = "HK"
	}
	return s.client.Quote(ctx, QuoteRequest{
		Origin:      s.originAddress(ctx),
		Destination: dest,
		Parcel: Parcel{
			WeightGrams: totalWeight,
			ValueHKD:    declaredValueHKD,
			LengthCM:    float64(maxL) / 10,
			WidthCM:     float64(maxW) / 10,
			HeightCM:    float64(maxH) / 10,
		},
	})
}

// ── Pickup points ──────────────────────────────────────────────────────

func (s *Service) PickupPoints(ctx context.Context, carrier, district string) ([]PickupPoint, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}
	return s.client.ListPickupPoints(ctx, carrier, district)
}

// ListCouriers fetches the merchant's enabled couriers for the admin
// settings dropdown. Returns ErrNotConfigured when credentials are blank.
func (s *Service) ListCouriers(ctx context.Context) ([]Courier, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}
	return s.client.ListCouriers(ctx)
}

// ShippingDefault resolves the admin-configured default courier + service
// to display labels. Looks up uids in site_settings then resolves names via
// the same /couriers/ feed admin uses. Returns configured=false when either
// uid is blank or when ShipAny can't be reached — the storefront then shows
// "not configured" and blocks checkout, the same UX as a real misconfigure.
type ShippingDefaultResolved struct {
	Configured  bool   `json:"configured"`
	CourierUID  string `json:"courier_uid,omitempty"`
	CourierName string `json:"courier_name,omitempty"`
	ServiceUID  string `json:"service_uid,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

func (s *Service) ShippingDefault(ctx context.Context) ShippingDefaultResolved {
	courierUID := strings.TrimSpace(s.read(ctx, "shipany_default_courier"))
	serviceUID := strings.TrimSpace(s.read(ctx, "shipany_default_service"))
	if courierUID == "" || serviceUID == "" {
		return ShippingDefaultResolved{Configured: false}
	}
	if !s.Configured(ctx) {
		return ShippingDefaultResolved{Configured: false}
	}
	couriers, err := s.client.ListCouriers(ctx)
	if err != nil {
		log.Printf("shipany shipping-default list couriers: %v", err)
		return ShippingDefaultResolved{Configured: false}
	}
	for _, c := range couriers {
		if c.UID != courierUID {
			continue
		}
		// Service plan names aren't carried by ListCouriers — the UID is the
		// human label too (e.g. "sf_standard"). Surface the uid as the name so
		// the storefront has something readable; admin can rename the service
		// in their own dropdown if needed later.
		serviceName := serviceUID
		for _, p := range c.SvcPlans {
			if p.CourSvcPl == serviceUID {
				serviceName = p.CourSvcPl
				break
			}
		}
		return ShippingDefaultResolved{
			Configured:  true,
			CourierUID:  c.UID,
			CourierName: c.Name,
			ServiceUID:  serviceUID,
			ServiceName: serviceName,
		}
	}
	// Configured uid no longer exists in the merchant's couriers list — treat
	// as unconfigured so admin sees the same "not configured" notice and can fix it.
	return ShippingDefaultResolved{Configured: false}
}

// ── Shipments ──────────────────────────────────────────────────────────

type DBShipment struct {
	ID                string  `json:"id"`
	OrderID           string  `json:"order_id"`
	ShipanyShipmentID string  `json:"shipany_shipment_id"`
	TrackingNumber    *string `json:"tracking_number,omitempty"`
	TrackingURL       *string `json:"tracking_url,omitempty"`
	LabelURL          *string `json:"label_url,omitempty"`
	Carrier           string  `json:"carrier"`
	Service           string  `json:"service"`
	FeeHKD            float64 `json:"fee_hkd"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

var ErrShipmentExists = errors.New("shipment already exists for this order")
var ErrCarrierNotSelected = errors.New("order has no selected_carrier — pick one explicitly")
var ErrShipmentNotFound = errors.New("shipment not found")

// CreateForOrder pulls the order + shipping address, asks ShipAny to create
// a waybill, and persists the result. Idempotent at the order level — a
// second call returns ErrShipmentExists.
func (s *Service) CreateForOrder(ctx context.Context, orderID string, override *RateOption) (*DBShipment, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}

	// Reject if a shipment already exists.
	if existing, _ := s.GetByOrderID(ctx, orderID); existing != nil {
		return nil, ErrShipmentExists
	}

	order, err := s.orderSvc.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.ShippingAddress == nil {
		return nil, fmt.Errorf("order %s has no shipping address", orderID)
	}

	carrier := stringOrEmpty(getOrderCarrier(ctx, s.db, orderID))
	service := stringOrEmpty(getOrderService(ctx, s.db, orderID))
	pickupID, _ := getOrderPickupID(ctx, s.db, orderID)
	if override != nil {
		carrier = override.Carrier
		service = override.Service
	}
	if carrier == "" || service == "" {
		return nil, ErrCarrierNotSelected
	}

	lenMM, widMM, hgtMM := s.orderDimensionsMM(ctx, orderID)
	parcel := Parcel{
		WeightGrams: s.orderWeightGrams(ctx, orderID),
		ValueHKD:    order.Subtotal,
		LengthCM:    float64(lenMM) / 10,
		WidthCM:     float64(widMM) / 10,
		HeightCM:    float64(hgtMM) / 10,
	}

	dest := Address{
		Name:       strings.TrimSpace(order.ShippingAddress.FirstName + " " + order.ShippingAddress.LastName),
		Phone:      ptrToString(order.ShippingAddress.Phone),
		Line1:      order.ShippingAddress.Line1,
		Line2:      ptrToString(order.ShippingAddress.Line2),
		District:   order.ShippingAddress.City, // HK uses district in the city field
		City:       order.ShippingAddress.City,
		PostalCode: order.ShippingAddress.PostalCode,
		Country:    order.ShippingAddress.Country,
	}

	customerNote := ""
	if order.Notes != nil {
		customerNote = *order.Notes
	}

	// paid_by_rcvr (SF freight-collect) is computed per order:
	//   • threshold off → always recipient-pays (matches "順豐速運（到付）" everywhere)
	//   • threshold on + subtotal ≥ threshold → merchant absorbs shipping
	//   • threshold on + subtotal < threshold → recipient still pays
	// The old admin shipany_paid_by_receiver toggle is now UI-display-only; this
	// decision overrides it.
	thresholdEnabled := s.read(ctx, "free_shipping_threshold_enabled") == "true"
	threshold, _ := strconv.ParseFloat(s.read(ctx, "free_shipping_threshold_hkd"), 64)
	paidByReceiver := true
	if thresholdEnabled && threshold > 0 && order.Subtotal >= threshold {
		paidByReceiver = false
	}

	items := s.orderItemsForShipment(ctx, orderID, order)

	created, err := s.client.CreateShipment(ctx, CreateShipmentRequest{
		Carrier:        carrier,
		Service:        service,
		OrderRef:       order.OrderNumber,
		ExtOrderID:     orderID,
		Origin:         s.originAddress(ctx),
		Destination:    dest,
		Parcel:         parcel,
		Items:          items,
		PickupPointID:  pickupID,
		CustomerNote:   customerNote,
		PaidByReceiver: paidByReceiver,
	})
	if err != nil {
		// Recovery: a prior attempt succeeded at ShipAny (returned 201 with a
		// uid) but the subsequent local INSERT below failed — leaving a
		// remote order with no local row. Every retry then hits ShipAny's
		// duplicate-ext_order_id check and gets 403. Detect that case from
		// the error envelope, fetch the orphaned remote order, and persist
		// it as if the create had just succeeded.
		if uid := extractExistingShipanyUID(err); uid != "" {
			recovered, rerr := s.client.FetchOrder(ctx, uid)
			if rerr != nil {
				return nil, fmt.Errorf("%w (recovery FetchOrder %s failed: %v)", err, uid, rerr)
			}
			log.Printf("shipany createForOrder: recovered orphan ShipAny order %s for local order %s", uid, orderID)
			created = recovered
		} else {
			return nil, err
		}
	}

	// Backfill carrier/service onto the order when the operator supplied an
	// override for a legacy order — otherwise the page would keep showing
	// the "no carrier selected" UI even though we already shipped.
	if override != nil {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE orders SET selected_carrier = $1, selected_service = $2 WHERE id = $3`,
			carrier, service, orderID); err != nil {
			log.Printf("shipany createForOrder: persist override on %s: %v", orderID, err)
		}
	}

	row := &DBShipment{
		OrderID:           orderID,
		ShipanyShipmentID: created.ID,
		Carrier:           carrier,
		Service:           service,
		FeeHKD:            created.FeeHKD,
		Status:            "created",
	}
	if created.TrackingNumber != "" {
		v := created.TrackingNumber
		row.TrackingNumber = &v
	}
	if created.TrackingURL != "" {
		v := created.TrackingURL
		row.TrackingURL = &v
	}
	if created.LabelURL != "" {
		v := created.LabelURL
		row.LabelURL = &v
	}

	err = s.db.QueryRowContext(ctx,
		`INSERT INTO shipments
		   (order_id, shipany_shipment_id, tracking_number, tracking_url, label_url,
		    carrier, service, fee_hkd, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, created_at, updated_at`,
		row.OrderID, row.ShipanyShipmentID, row.TrackingNumber, row.TrackingURL, row.LabelURL,
		row.Carrier, row.Service, row.FeeHKD, row.Status).
		Scan(&row.ID, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert shipment: %w", err)
	}
	return row, nil
}

// RequestPickup tells ShipAny to schedule courier collection for the
// shipment, and bumps local status to pickup_requested.
func (s *Service) RequestPickup(ctx context.Context, orderID string) (*DBShipment, error) {
	sh, err := s.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if sh == nil {
		return nil, ErrShipmentNotFound
	}
	if err := s.client.RequestPickup(ctx, sh.ShipanyShipmentID); err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE shipments SET status='pickup_requested' WHERE id=$1 AND status='created'`, sh.ID)
	if err != nil {
		return nil, err
	}
	return s.GetByOrderID(ctx, orderID)
}

func (s *Service) GetByOrderID(ctx context.Context, orderID string) (*DBShipment, error) {
	var sh DBShipment
	err := s.db.QueryRowContext(ctx,
		`SELECT id, order_id, shipany_shipment_id, tracking_number, tracking_url, label_url,
		        carrier, service, fee_hkd, status, created_at, updated_at
		 FROM shipments WHERE order_id=$1
		 ORDER BY created_at DESC LIMIT 1`, orderID).
		Scan(&sh.ID, &sh.OrderID, &sh.ShipanyShipmentID, &sh.TrackingNumber, &sh.TrackingURL, &sh.LabelURL,
			&sh.Carrier, &sh.Service, &sh.FeeHKD, &sh.Status, &sh.CreatedAt, &sh.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sh, nil
}

// ── Helpers ────────────────────────────────────────────────────────────

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}

// orderWeightGrams sums per-variant weight × quantity for an order's
// items. Items missing variant weight fall back to the configured
// default; if no items have weight, the whole parcel falls back too.
func (s *Service) orderWeightGrams(ctx context.Context, orderID string) int {
	rows, err := s.db.QueryContext(ctx,
		`SELECT oi.quantity, pv.weight_grams
		   FROM order_items oi
		   LEFT JOIN product_variants pv ON pv.id = oi.variant_id
		  WHERE oi.order_id = $1`, orderID)
	if err != nil {
		return s.defaultWeight(ctx)
	}
	defer rows.Close()

	fallback := s.defaultWeight(ctx)
	total := 0
	for rows.Next() {
		var qty int
		var w sql.NullInt64
		if err := rows.Scan(&qty, &w); err != nil {
			continue
		}
		grams := fallback
		if w.Valid && w.Int64 > 0 {
			grams = int(w.Int64)
		}
		total += grams * qty
	}
	if total <= 0 {
		return fallback
	}
	return total
}

// orderDimensionsMM returns the max per-axis dimensions (mm) across all order
// items. Items with null dimensions contribute zero. Zero values are omitted
// by the Parcel JSON omitempty tag so ShipAny falls back to weight-only quoting.
func (s *Service) orderDimensionsMM(ctx context.Context, orderID string) (lenMM, widMM, hgtMM int) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pv.length_mm, pv.width_mm, pv.height_mm
		   FROM order_items oi
		   LEFT JOIN product_variants pv ON pv.id = oi.variant_id
		  WHERE oi.order_id = $1`, orderID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var l, w, h sql.NullInt64
		if err := rows.Scan(&l, &w, &h); err != nil {
			continue
		}
		if l.Valid && int(l.Int64) > lenMM {
			lenMM = int(l.Int64)
		}
		if w.Valid && int(w.Int64) > widMM {
			widMM = int(w.Int64)
		}
		if h.Valid && int(h.Int64) > hgtMM {
			hgtMM = int(h.Int64)
		}
	}
	return
}

func (s *Service) defaultWeight(ctx context.Context) int {
	v := s.read(ctx, "shipany_default_weight_grams")
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 500
	}
	return n
}

func (s *Service) originAddress(ctx context.Context) Address {
	return Address{
		Name:       s.read(ctx, "shipany_origin_name"),
		Phone:      s.read(ctx, "shipany_origin_phone"),
		Email:      s.read(ctx, "shipany_origin_email"),
		Line1:      s.read(ctx, "shipany_origin_line1"),
		Line2:      s.read(ctx, "shipany_origin_line2"),
		District:   s.read(ctx, "shipany_origin_district"),
		City:       s.read(ctx, "shipany_origin_city"),
		PostalCode: s.read(ctx, "shipany_origin_postal"),
		Country:    "HK",
		AddrType:   s.read(ctx, "shipany_origin_addr_type"),
	}
}

// orderItemsForShipment builds the per-line shipany items slice from the
// order's items, attaching per-variant weight/dimensions when available.
// Bundle component rows (parent_item_id != null) are skipped to avoid
// double-counting — the parent line already represents the saleable unit.
func (s *Service) orderItemsForShipment(ctx context.Context, orderID string, order *orders.Order) []ShipanyItem {
	type dims struct{ w, l, wd, h int } // weight_g + length/width/height_mm
	dimMap := map[string]dims{}
	rows, err := s.db.QueryContext(ctx,
		`SELECT pv.id, pv.weight_grams, pv.length_mm, pv.width_mm, pv.height_mm
		   FROM product_variants pv
		  WHERE pv.id IN (SELECT variant_id FROM order_items WHERE order_id = $1 AND variant_id IS NOT NULL)`,
		orderID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			var w, l, wd, h sql.NullInt64
			if err := rows.Scan(&id, &w, &l, &wd, &h); err != nil {
				continue
			}
			dimMap[id] = dims{
				w:  int(w.Int64),
				l:  int(l.Int64),
				wd: int(wd.Int64),
				h:  int(h.Int64),
			}
		}
	}

	fallback := s.defaultWeight(ctx)
	out := make([]ShipanyItem, 0, len(order.Items))
	for _, it := range order.Items {
		if it.ParentItemID != nil && *it.ParentItemID != "" {
			continue // bundle child — skip
		}
		w := fallback
		var l, wd, h float64
		if it.VariantID != nil {
			if d, ok := dimMap[*it.VariantID]; ok {
				if d.w > 0 {
					w = d.w
				}
				l = float64(d.l) / 10
				wd = float64(d.wd) / 10
				h = float64(d.h) / 10
			}
		}
		out = append(out, ShipanyItem{
			SKU:          coalesceStr(it.VariantSKU, "ITEM"),
			Name:         coalesceStr(it.ProductName, "Item"),
			UnitPriceHKD: it.UnitPrice,
			Quantity:     it.Quantity,
			WeightGrams:  w,
			LengthCM:     l,
			WidthCM:      wd,
			HeightCM:     h,
		})
	}
	return out
}

func coalesceStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func getOrderCarrier(ctx context.Context, db *sql.DB, orderID string) (sql.NullString, error) {
	var v sql.NullString
	err := db.QueryRowContext(ctx, `SELECT selected_carrier FROM orders WHERE id=$1`, orderID).Scan(&v)
	return v, err
}

func getOrderService(ctx context.Context, db *sql.DB, orderID string) (sql.NullString, error) {
	var v sql.NullString
	err := db.QueryRowContext(ctx, `SELECT selected_service FROM orders WHERE id=$1`, orderID).Scan(&v)
	return v, err
}

func getOrderPickupID(ctx context.Context, db *sql.DB, orderID string) (string, error) {
	var v sql.NullString
	err := db.QueryRowContext(ctx, `SELECT pickup_point_id FROM orders WHERE id=$1`, orderID).Scan(&v)
	if err != nil {
		return "", err
	}
	if v.Valid {
		return v.String, nil
	}
	return "", nil
}

func stringOrEmpty(v sql.NullString, _ error) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// shipanyExistingUIDPattern matches the result.details line ShipAny returns
// when ext_order_id collides with an existing order:
//
//	"Order creation failed as order(uid: <uuid>, ref: <ref>) already exists."
//
// Captured group 1 is the remote order uid we use to recover.
var shipanyExistingUIDPattern = regexp.MustCompile(`order\(uid:\s*([0-9a-fA-F-]{36})`)

// extractExistingShipanyUID pulls the existing remote order uid out of a
// CreateShipment error whose ShipAny envelope says the order already exists.
// Returns "" if the error isn't an APIError or the message doesn't match.
func extractExistingShipanyUID(err error) string {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	if apiErr.Status != 403 && apiErr.Code != 403 {
		return ""
	}
	for _, d := range apiErr.Details {
		if m := shipanyExistingUIDPattern.FindStringSubmatch(d); len(m) == 2 {
			return m[1]
		}
	}
	// Fallback: also scan Raw — defensive in case ShipAny re-shapes the envelope.
	if m := shipanyExistingUIDPattern.FindStringSubmatch(apiErr.Raw); len(m) == 2 {
		return m[1]
	}
	return ""
}
