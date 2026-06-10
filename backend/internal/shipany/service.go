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
	client   *HTTPClient
	settings *settings.Service
	db       *sql.DB
	orderSvc *orders.OrderService
	notices  *orders.NoticeService
}

func NewService(client *HTTPClient, settings *settings.Service, db *sql.DB, orderSvc *orders.OrderService, notices *orders.NoticeService) *Service {
	return &Service{client: client, settings: settings, db: db, orderSvc: orderSvc, notices: notices}
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

	dest := destAddressFromOrder(order)

	customerNote := ""
	if order.Notes != nil {
		customerNote = *order.Notes
	}

	// paid_by_rcvr (SF freight-collect) follows the frozen shipping_free flag
	// the order service stamped at checkout — which is already role-aware
	// (installer vs default threshold). Consuming the frozen flag avoids
	// re-deriving the decision from live settings (which would diverge from
	// the receipt if admins change the threshold post-checkout) and keeps
	// the installer-specific behaviour correct without re-loading the
	// customer's role here.
	paidByReceiver := !order.ShippingFree

	items := s.orderItemsForShipment(ctx, orderID, order)

	// Cross-border (e.g. HK→Macau) needs an SF *international* plan (e.g.
	// "SF Standard Express - HKMOTW"); the order's stored domestic default
	// ("SF Express") is rejected by SF for non-HK destinations. Resolve a
	// valid plan + locked quote price live from ShipAny. Domestic orders skip
	// this entirely and ship with the stored default + no quot_uid (unchanged).
	storedCarrier, storedService := carrier, service
	quotUID := ""
	courType := ""
	feeHKD := 0.0
	if !strings.EqualFold(strings.TrimSpace(dest.Country), "HK") {
		opts, qerr := s.client.Quote(ctx, QuoteRequest{
			Origin:      s.originAddress(ctx),
			Destination: dest,
			Parcel:      parcel,
		})
		if qerr != nil {
			return nil, fmt.Errorf("shipany cross-border quote (order %s, dest %s): %w", orderID, dest.Country, qerr)
		}
		opt := pickCrossBorderOption(opts, carrier)
		if opt == nil {
			return nil, fmt.Errorf("shipany: no service available to %s for order %s", dest.Country, orderID)
		}
		carrier, service, quotUID, courType, feeHKD = opt.Carrier, opt.Service, opt.QuotUID, opt.CourType, opt.FeeHKD
	}

	created, err := s.client.CreateShipment(ctx, CreateShipmentRequest{
		Carrier:        carrier,
		Service:        service,
		CourType:       courType,
		QuotUID:        quotUID,
		FeeHKD:         feeHKD,
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
	// override for a legacy order, or when the cross-border quote resolved a
	// different plan than the stored domestic default — otherwise the order
	// page would keep showing the "no carrier selected" UI (override case) or
	// the stale domestic service (cross-border case) even though we shipped on
	// the resolved plan.
	if override != nil || carrier != storedCarrier || service != storedService {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE orders SET selected_carrier = $1, selected_service = $2 WHERE id = $3`,
			carrier, service, orderID); err != nil {
			log.Printf("shipany createForOrder: persist resolved carrier/service on %s: %v", orderID, err)
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

// PostTrackingNotice posts a clickable SF Express tracking link to the customer
// on the order timeline. Reads the latest shipment and gates on the waybill
// prefix (SF…) rather than the carrier UID — more reliable than matching
// ShipAny courier labels. Best-effort: failures are logged, never returned.
// Fired when the order transitions 處理中 → 已發貨 so the customer learns the
// waybill number only once the order is actually marked shipped.
func (s *Service) PostTrackingNotice(ctx context.Context, orderID string) {
	if s.notices == nil {
		return
	}
	sh, err := s.GetByOrderID(ctx, orderID)
	if err != nil {
		log.Printf("shipany: load shipment for tracking notice on order %s: %v", orderID, err)
		return
	}
	if sh == nil || sh.TrackingNumber == nil || *sh.TrackingNumber == "" {
		return
	}
	tn := *sh.TrackingNumber
	if !strings.HasPrefix(strings.ToUpper(tn), "SF") {
		return
	}
	// Stay idempotent: skip if an automated message already mentions this
	// waybill for the order. Guards against duplicate triggers — e.g. repeat
	// Collected_By_Courier webhooks, or a redeploy that moved when this fires
	// straddling an in-flight order (the v0.9.298 cause of double messages).
	if mentioned, err := s.notices.AutoCustomerMessageMentions(ctx, orderID, tn); err != nil {
		log.Printf("shipany: check existing tracking notice for order %s: %v", orderID, err)
	} else if mentioned {
		return
	}
	body := fmt.Sprintf("SF Express 運單號碼 [%s](%s) 點擊可查看運單詳情", tn, sfExpressTrackingURL(tn))
	if _, err := s.notices.CreateAutoCustomerMessage(ctx, orderID, body); err != nil {
		log.Printf("shipany: post tracking notice for order %s: %v", orderID, err)
	}
}

// sfExpressTrackingURL builds the customer-facing SF Express waybill page URL.
func sfExpressTrackingURL(trackingNumber string) string {
	return "https://hk.sf-express.com/hk/tc/waybill/waybill-detail/" + trackingNumber
}

// destAddressFromOrder maps an order's frozen shipping snapshot to the ShipAny
// receiver Address. Shared by CreateForOrder and UpdateAddressForOrder so the
// rcvr_ctc sent on an address edit is byte-for-byte consistent with what the
// original create emitted (notably the HK convention of mirroring city into
// district). Caller must ensure order.ShippingAddress is non-nil.
func destAddressFromOrder(order *orders.Order) Address {
	// WooCommerce-imported orders (and any order whose shipping snapshot lacks a
	// phone) freeze an empty ship_phone — WC's shipping address has no phone field,
	// so the contact number only lives on customer_phone (carried over from
	// billing). Fall back to it so the waybill always has a recipient phone for
	// the courier; ShipAny rejects a shipment with no rcvr_ctc number.
	phone := ptrToString(order.ShippingAddress.Phone)
	if strings.TrimSpace(phone) == "" {
		phone = ptrToString(order.CustomerPhone)
	}
	return Address{
		Name:       strings.TrimSpace(order.ShippingAddress.FirstName + " " + order.ShippingAddress.LastName),
		Phone:      phone,
		Line1:      order.ShippingAddress.Line1,
		Line2:      ptrToString(order.ShippingAddress.Line2),
		District:   order.ShippingAddress.City, // HK uses district in the city field
		City:       order.ShippingAddress.City,
		PostalCode: order.ShippingAddress.PostalCode,
		Country:    order.ShippingAddress.Country,
	}
}

// UpdateAddressForOrder syncs an order's (already-persisted) shipping address to
// its existing ShipAny shipment. No-op when the order has no shipment yet.
// Mirrors the WC plugin's post-save hook: before pickup the waybill is
// regenerated with the corrected address; after pickup only ShipAny's record is
// updated. Fired from the orders module's onShippingAddressChanged callback.
func (s *Service) UpdateAddressForOrder(ctx context.Context, orderID string) (*DBShipment, error) {
	if !s.Configured(ctx) {
		return nil, ErrNotConfigured
	}
	sh, err := s.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if sh == nil {
		// No waybill — nothing to sync. The snapshot edit stands on its own.
		return nil, nil
	}
	// A delivered parcel can't change destination; skip the remote call.
	if sh.Status == "delivered" {
		return sh, nil
	}

	order, err := s.orderSvc.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.ShippingAddress == nil {
		return nil, fmt.Errorf("order %s has no shipping address", orderID)
	}

	updated, err := s.client.UpdateShipmentAddress(ctx, sh.ShipanyShipmentID, destAddressFromOrder(order))
	if err != nil {
		return nil, err
	}

	// Persist any label/tracking that regeneration may have changed.
	var trackingNumber, trackingURL, labelURL any
	if updated.TrackingNumber != "" {
		trackingNumber = updated.TrackingNumber
	}
	if updated.TrackingURL != "" {
		trackingURL = updated.TrackingURL
	}
	if updated.LabelURL != "" {
		labelURL = updated.LabelURL
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE shipments
		    SET tracking_number = COALESCE($2, tracking_number),
		        tracking_url    = COALESCE($3, tracking_url),
		        label_url       = COALESCE($4, label_url),
		        updated_at      = now()
		  WHERE id = $1`,
		sh.ID, trackingNumber, trackingURL, labelURL); err != nil {
		return nil, fmt.Errorf("persist updated shipment %s: %w", sh.ID, err)
	}

	final, err := s.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Best-effort internal audit note carrying the refreshed waybill details
	// (incl. the regenerated label download link) — same shape as auto-create
	// so staff get the new label without leaving the order page. System role —
	// not shown to the customer.
	if s.notices != nil && final != nil {
		body := shipmentNoticeBody("送貨地址已更新並已同步至物流運單", final)
		if _, err := s.notices.CreateSystemNotice(ctx, orderID, body); err != nil {
			log.Printf("shipany: post address-update notice for order %s: %v", orderID, err)
		}
	}

	return final, nil
}

// shipmentNoticeBody renders an order-timeline note carrying a shipment's
// waybill details: tracking number, courier, fee, and the downloadable label
// PDF link. Shared by the auto-create job and the address-update sync so both
// surface the same fields (and the frontend auto-links the label URL).
func shipmentNoticeBody(header string, sh *DBShipment) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	if sh.TrackingNumber != nil && *sh.TrackingNumber != "" {
		fmt.Fprintf(&b, "運單號碼：%s\n", *sh.TrackingNumber)
	}
	if sh.Carrier != "" {
		if sh.Service != "" {
			fmt.Fprintf(&b, "貨運公司：%s（%s）\n", sh.Carrier, sh.Service)
		} else {
			fmt.Fprintf(&b, "貨運公司：%s\n", sh.Carrier)
		}
	}
	if sh.FeeHKD > 0 {
		fmt.Fprintf(&b, "運費：HKD %.2f\n", sh.FeeHKD)
	}
	if sh.LabelURL != nil && *sh.LabelURL != "" {
		fmt.Fprintf(&b, "標籤：%s\n", *sh.LabelURL)
	}
	return strings.TrimRight(b.String(), "\n")
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
// Bundle component rows (parent_item_id != null) are emitted as nested rows
// beneath their parent so the ShipAny packing slip lists what's physically
// inside each bundle (items + quantities), mirroring the order detail page.
// The bundle parent line carries the value/qty; each component is a follow-up
// row with a "套裝內含" descr and zero unit price (included in the bundle).
// ShipAny does not validate items[] totals against the authoritative
// parcel-level weight/value, so these extra rows are display-only.
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

	// dimsFor resolves a variant's weight (g) and dimensions (cm), falling back
	// to the default weight when the variant carries none. Shared by parent and
	// component rows.
	dimsFor := func(variantID *string) (w int, l, wd, h float64) {
		w = fallback
		if variantID != nil {
			if d, ok := dimMap[*variantID]; ok {
				if d.w > 0 {
					w = d.w
				}
				l = float64(d.l) / 10
				wd = float64(d.wd) / 10
				h = float64(d.h) / 10
			}
		}
		return
	}

	// Group bundle component rows under their parent and collect top-level rows
	// in order — independent of how GetByID happens to sort the items.
	childrenByParent := map[string][]orders.OrderItem{}
	topLevel := make([]orders.OrderItem, 0, len(order.Items))
	for _, it := range order.Items {
		if it.ParentItemID != nil && *it.ParentItemID != "" {
			childrenByParent[*it.ParentItemID] = append(childrenByParent[*it.ParentItemID], it)
			continue
		}
		topLevel = append(topLevel, it)
	}

	out := make([]ShipanyItem, 0, len(order.Items))
	for _, it := range topLevel {
		w, l, wd, h := dimsFor(it.VariantID)
		children := childrenByParent[it.ID]
		parent := ShipanyItem{
			SKU:          coalesceStr(it.VariantSKU, "ITEM"),
			Name:         coalesceStr(it.ProductName, "Item"),
			UnitPriceHKD: it.UnitPrice,
			Quantity:     it.Quantity,
			WeightGrams:  w,
			LengthCM:     l,
			WidthCM:      wd,
			HeightCM:     h,
		}
		if len(children) > 0 {
			parent.Descr = "套裝" // bundle header
		}
		out = append(out, parent)

		// Nest each bundle component beneath the bundle line so the packing
		// slip shows its contents. Quantity is already scaled by the bundle
		// quantity at checkout; price is zeroed (included in the bundle).
		for _, child := range children {
			cw, cl, cwd, ch := dimsFor(child.VariantID)
			out = append(out, ShipanyItem{
				SKU:         coalesceStr(child.VariantSKU, "ITEM"),
				Name:        "└ " + coalesceStr(child.ProductName, "Item"),
				Descr:       "套裝內含",
				Quantity:    child.Quantity,
				WeightGrams: cw,
				LengthCM:    cl,
				WidthCM:     cwd,
				HeightCM:    ch,
			})
		}
	}
	return out
}

func coalesceStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// pickCrossBorderOption chooses a quoted RateOption for a cross-border shipment:
// the cheapest plan from the preferred courier (the order's configured carrier),
// falling back to the cheapest plan overall. ShipAny only returns plans valid for
// the destination, so any returned option is shippable. Returns nil on empty input.
func pickCrossBorderOption(opts []RateOption, preferredCarrier string) *RateOption {
	var best *RateOption
	for i := range opts {
		o := &opts[i]
		if preferredCarrier != "" && o.Carrier != preferredCarrier {
			continue
		}
		if best == nil || o.FeeHKD < best.FeeHKD {
			best = o
		}
	}
	if best != nil {
		return best
	}
	// No preferred-carrier match — fall back to the cheapest plan overall.
	for i := range opts {
		if best == nil || opts[i].FeeHKD < best.FeeHKD {
			best = &opts[i]
		}
	}
	return best
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
