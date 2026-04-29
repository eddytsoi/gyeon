package shipany

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
}

// QuoteForCart builds a parcel from the cart's variant weights (or the
// configured fallback) and asks ShipAny for available rate options.
// Destination is the customer-supplied checkout address.
func (s *Service) QuoteForCart(ctx context.Context, dest Address, lines []CartLine, declaredValueHKD float64) ([]RateOption, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}
	totalWeight := 0
	for _, ln := range lines {
		w := ln.WeightGrams
		if w <= 0 {
			w = s.defaultWeight(ctx)
		}
		totalWeight += w * ln.Quantity
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
		Parcel:      Parcel{WeightGrams: totalWeight, ValueHKD: declaredValueHKD},
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

	parcel := Parcel{
		WeightGrams: s.defaultWeight(ctx),
		ValueHKD:    order.Subtotal,
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

	created, err := s.client.CreateShipment(ctx, CreateShipmentRequest{
		Carrier:       carrier,
		Service:       service,
		OrderRef:      fmt.Sprintf("ORD-%d", order.Number),
		Origin:        s.originAddress(ctx),
		Destination:   dest,
		Parcel:        parcel,
		PickupPointID: pickupID,
		CustomerNote:  customerNote,
	})
	if err != nil {
		return nil, err
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

// HandleTrackingEvent normalises ShipAny event strings to the local shipment
// status taxonomy and advances the order status when the transition is allowed.
// Idempotent: only flips status forward.
func (s *Service) HandleTrackingEvent(ctx context.Context, evt TrackingEvent, raw []byte) error {
	if evt.TrackingNumber == "" && evt.ShipmentID == "" {
		return errors.New("event has neither tracking_number nor shipment_id")
	}

	var sh DBShipment
	var err error
	switch {
	case evt.TrackingNumber != "":
		err = s.db.QueryRowContext(ctx,
			`SELECT id, order_id, status FROM shipments WHERE tracking_number=$1`,
			evt.TrackingNumber).Scan(&sh.ID, &sh.OrderID, &sh.Status)
	default:
		err = s.db.QueryRowContext(ctx,
			`SELECT id, order_id, status FROM shipments WHERE shipany_shipment_id=$1`,
			evt.ShipmentID).Scan(&sh.ID, &sh.OrderID, &sh.Status)
	}
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("shipany webhook: no shipment matches event %+v", evt)
		return nil
	}
	if err != nil {
		return err
	}

	nextStatus := normalizeEventStatus(evt.Event)
	advanceOrder := nextStatus == "in_transit" || nextStatus == "delivered"

	// Always persist the raw payload for forensics.
	_, err = s.db.ExecContext(ctx,
		`UPDATE shipments
		   SET status = CASE WHEN $2 = '' THEN status ELSE $2 END,
		       last_tracking_event = $3,
		       tracking_number = COALESCE(NULLIF($4,''), tracking_number)
		 WHERE id = $1`,
		sh.ID, nextStatus, json.RawMessage(raw), evt.TrackingNumber)
	if err != nil {
		return err
	}

	if !advanceOrder {
		return nil
	}
	order, err := s.orderSvc.GetByID(ctx, sh.OrderID)
	if err != nil {
		return err
	}
	target := orderStatusFor(nextStatus)
	if target == "" {
		return nil
	}
	if order.Status == target {
		return nil // already there, idempotent
	}
	_, err = s.orderSvc.UpdateStatus(ctx, sh.OrderID, orders.UpdateStatusRequest{
		Status: target,
		Note:   strPtr("ShipAny tracking event: " + evt.Event),
	})
	if err != nil {
		// Not all transitions are valid (e.g. delivered while still pending).
		// Swallow — the shipment status is the source of truth and will be
		// surfaced in the admin UI; manual reconciliation can fix the order.
		log.Printf("shipany webhook: cannot advance order %s to %s: %v", sh.OrderID, target, err)
	}
	return nil
}

// ── Helpers ────────────────────────────────────────────────────────────

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
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
		Line1:      s.read(ctx, "shipany_origin_line1"),
		Line2:      s.read(ctx, "shipany_origin_line2"),
		District:   s.read(ctx, "shipany_origin_district"),
		City:       s.read(ctx, "shipany_origin_city"),
		PostalCode: s.read(ctx, "shipany_origin_postal"),
		Country:    "HK",
	}
}

// normalizeEventStatus maps ShipAny event strings (which we have to confirm
// at integration time) to our small internal taxonomy. Unknown events return
// "" so the caller leaves status unchanged.
func normalizeEventStatus(eventName string) string {
	switch strings.ToLower(strings.ReplaceAll(eventName, ".", "_")) {
	case "shipment_created", "created":
		return "created"
	case "pickup_requested":
		return "pickup_requested"
	case "shipment_in_transit", "in_transit", "picked_up", "shipment_picked_up":
		return "in_transit"
	case "shipment_delivered", "delivered":
		return "delivered"
	case "shipment_exception", "exception", "failed", "shipment_failed":
		return "exception"
	}
	return ""
}

// orderStatusFor returns the order status to advance to when a tracking
// event of this normalized type fires. Empty = don't touch the order.
func orderStatusFor(shipStatus string) orders.OrderStatus {
	switch shipStatus {
	case "in_transit":
		return orders.StatusShipped
	case "delivered":
		return orders.StatusDelivered
	}
	return ""
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

func strPtr(s string) *string { return &s }
