package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gyeon/backend/internal/customers"
)

// OrdersImportRequest mirrors ImportRequest but is scoped to /wc/v3/orders.
// Only upsert-style behaviour is supported — replace would cascade-delete
// real local data (post-import storefront orders, manual entries) and
// orphan addresses snapshotted by other rows.
type OrdersImportRequest struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
	Limit    int    `json:"limit"`
	// Status filters WC orders fetched from /wc/v3/orders by their WC-side
	// status. Accepted: "any" (default, empty also treated as any),
	// "pending", "processing", "on-hold", "completed", "cancelled",
	// "refunded", "failed". Unknown values are coerced to "any" by the
	// handler via normalizeWCOrderStatus.
	Status string `json:"status"`
	// Year scopes the import to orders created in that calendar year (site
	// timezone, as accepted by WC's after/before params). 0 or unset =
	// no year filter (all years).
	Year int `json:"year"`
}

// normalizeWCOrderStatus coerces a user-supplied WC status string into a
// safe value to forward to the WC REST API. Empty / unknown → "any".
func normalizeWCOrderStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "any":
		return "any"
	case "pending", "processing", "on-hold", "completed", "cancelled", "refunded", "failed", "collected":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return "any"
	}
}

// OrdersProgressUpdate is streamed once per processed order.
type OrdersProgressUpdate struct {
	TotalOrders        int      `json:"total_orders"`
	ProcessedOrders    int      `json:"processed_orders"`
	ImportedOrders     int      `json:"imported_orders"`     // newly inserted
	UpdatedOrders      int      `json:"updated_orders"`      // matched by wc_order_id, updated in place
	ImportedLineItems  int      `json:"imported_line_items"` // line items inserted in this run (linked + unlinked)
	UnlinkedLineItems  int      `json:"unlinked_line_items"` // line items whose product/variant could not be resolved locally — kept as snapshot, variant_id NULL
	ImportedShipments  int      `json:"imported_shipments"`  // orders that carried an already-created ShipAny waybill, written to the shipments table
	SkippedOrders      int      `json:"skipped_orders"`      // status not in the import map (trash/draft/etc.)
	Failed             int      `json:"failed"`
	CurrentOrder       string   `json:"current_order,omitempty"`
	Done               bool     `json:"done"`
	Errors             []string `json:"errors"`
}

// mapWCOrderStatus translates a WooCommerce status to the local
// order_status enum. Returns ("", false) for statuses that should be
// skipped entirely (trash, draft, auto-draft, …).
func mapWCOrderStatus(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "pending", "on-hold":
		return "pending", true
	case "processing":
		return "paid", true
	case "completed":
		return "delivered", true
	case "collected":
		// Custom WC status used by the source store to mark
		// customer-picked-up orders. Local equivalent is "shipped"
		// (order has left our hands).
		return "shipped", true
	case "cancelled", "failed":
		return "cancelled", true
	case "refunded":
		return "refunded", true
	default:
		return "", false
	}
}

// OrderTotal returns the WC store's total order count via X-WP-Total. 0 on
// any error — the test endpoint already validated connectivity.
func (s *Service) OrderTotal(req OrdersImportRequest) int {
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).fetchOrderTotal(req.Status, req.Year)
}

// RunOrdersStreaming pages through /wc/v3/orders, upserts each order +
// addresses + line items, and emits a CustomersProgressUpdate-shaped
// progress frame per order. Final frame has Done = true.
func (s *Service) RunOrdersStreaming(ctx context.Context, req OrdersImportRequest, send func(OrdersProgressUpdate)) {
	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := OrdersProgressUpdate{Errors: []string{}}

	p.TotalOrders = wc.fetchOrderTotal(req.Status, req.Year)
	if req.Limit > 0 && (p.TotalOrders == 0 || p.TotalOrders > req.Limit) {
		p.TotalOrders = req.Limit
	}
	send(p)

	prefix := s.readSetting(ctx, "order_number_prefix")
	if prefix == "" {
		prefix = "ORD"
	}

pages:
	for page := 1; ; page++ {
		batch, err := wc.fetchOrders(page, req.Status, req.Year)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch orders page %d: %v", page, err))
			break
		}
		if len(batch) == 0 {
			break
		}
		for _, o := range batch {
			if req.Limit > 0 && p.ProcessedOrders >= req.Limit {
				break pages
			}

			localStatus, ok := mapWCOrderStatus(o.Status)
			if !ok {
				p.SkippedOrders++
				p.Errors = append(p.Errors, fmt.Sprintf("order #%s skipped: unsupported status %q", o.Number, o.Status))
				p.ProcessedOrders++
				send(p)
				continue
			}

			p.CurrentOrder = "#" + o.Number
			send(p)

			if err := s.upsertOrder(ctx, o, localStatus, prefix, &p); err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("order #%s: %v", o.Number, err))
				p.Failed++
			}
			p.ProcessedOrders++
			p.CurrentOrder = ""
			send(p)
		}
	}

	p.Done = true
	send(p)
}

// firstShippingMethod returns the first non-empty shipping method title from a
// WC order's shipping_lines (most orders have exactly one). Empty when the
// order has no shipping line — e.g. local pickup or free shipping.
func firstShippingMethod(o wcOrder) string {
	for _, sl := range o.ShippingLines {
		if t := strings.TrimSpace(sl.MethodTitle); t != "" {
			return t
		}
	}
	return ""
}

// upsertOrder runs the full upsert for one WC order in a single tx:
// resolve / create customer → snapshot shipping address → INSERT or UPDATE
// the order row → wipe & re-insert line items. Re-imports of the same WC
// order overwrite top-level fields and line items in place but never
// touch addresses on already-imported customers (that policy lives in the
// customers import path).
func (s *Service) upsertOrder(ctx context.Context, o wcOrder, status, prefix string, p *OrdersProgressUpdate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Already-created ShipAny waybill carried in the WC order meta (if any).
	// When present we write a shipments row below and — only when the order
	// would otherwise land as 已付款 (paid) — bump it to 處理中 (processing),
	// mirroring the native paid → processing advance that creating a shipment
	// performs. Orders already further along (已發貨/已送達/已取消) keep their
	// status and just get the waybill attached.
	wb := parseShipanyWaybill(o.MetaData)
	effStatus := status
	if wb != nil && status == "paid" {
		effStatus = "processing"
	}

	customerID, err := s.resolveCustomer(ctx, tx, o)
	if err != nil {
		return fmt.Errorf("resolve customer: %w", err)
	}

	// Build totals. WC line item.subtotal is pre-discount, ex-tax — that
	// matches the local schema's "subtotal" column. discount_total /
	// shipping_total / total_tax / total come straight off the order.
	var subtotal float64
	for _, li := range o.LineItems {
		subtotal += parseDecimal(li.Subtotal)
	}
	shippingFee := parseDecimal(o.ShippingTotal)
	discount := parseDecimal(o.DiscountTotal)
	tax := parseDecimal(o.TotalTax)
	total := parseDecimal(o.Total)

	// Prefer date_created_gmt (UTC) over date_created (site timezone, naive).
	// The source WC store runs in UTC-8 / HKT, so parsing the site-time value
	// as UTC would drift created_at by 8 hours.
	createdAt := parseWCTime(firstNonEmpty(o.DateCreatedGMT, o.DateCreated))

	shipAddrID, shipFields, err := snapshotOrderAddress(ctx, tx, customerID, o)
	if err != nil {
		return fmt.Errorf("snapshot address: %w", err)
	}
	shipArgs := shipSnapshotArgs(shipFields)

	notes := nullableString(o.CustomerNote)

	custEmail := nullableString(strings.ToLower(o.Billing.Email))
	custPhone := nullableString(o.Billing.Phone)
	custName := nullableString(strings.TrimSpace(o.Billing.FirstName) + " " + strings.TrimSpace(o.Billing.LastName))

	// WC shipping method title (e.g. 「順豐速運」) — NULL when the order has no
	// shipping line (pickup / free). Surfaces in the admin 按物流 breakdown.
	shippingMethod := nullableString(firstShippingMethod(o))

	// Payment snapshot. Prefer the human-readable gateway title over the slug
	// for orders.payment_method (the admin renders it raw). payment_method is
	// VARCHAR(50) — rune-cap (not byte-cap) so a multibyte title isn't split.
	pm := firstNonEmpty(o.PaymentMethodTitle, o.PaymentMethod)
	if r := []rune(pm); len(r) > 50 {
		pm = string(r[:50])
	}
	paymentMethod := nullableString(pm)
	txnID := nullableString(strings.TrimSpace(o.TransactionID))

	// payment_status / paid_at only when WC actually recorded a payment. The
	// non-empty guard matters: parseWCTime returns NOW() on empty input, which
	// would stamp unpaid orders as paid-now and drift on every re-import.
	var paymentStatus *string
	var paidAt *time.Time
	if raw := firstNonEmpty(o.DatePaidGMT, o.DatePaid); strings.TrimSpace(raw) != "" {
		s := "succeeded" // matches the vocabulary native Stripe orders use
		paymentStatus = &s
		t := parseWCTime(raw)
		paidAt = &t
	}

	// Look up existing.
	var orderID string
	existed := false
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE wc_order_id=$1`, o.ID).Scan(&orderID)
	if err == nil {
		existed = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup order: %w", err)
	}

	if existed {
		updateArgs := append(append([]any{
			orderID, customerID, effStatus, shipAddrID,
			subtotal, shippingFee, discount, tax, total,
			notes, createdAt,
			custEmail, custPhone, custName,
		}, shipArgs...), shippingMethod, paymentMethod, paymentStatus, paidAt, txnID)
		// Re-imports refresh the ship_* snapshot when the WC address changed,
		// mirroring how customer_* are refreshed above. Payment columns are
		// refreshed too so re-import backfills already-imported orders in place.
		if _, err := tx.ExecContext(ctx, `
			UPDATE orders SET
				customer_id         = $2,
				status              = $3,
				shipping_address_id = $4,
				subtotal            = $5,
				shipping_fee        = $6,
				discount_amount     = $7,
				tax_amount          = $8,
				total               = $9,
				notes               = $10,
				created_at          = $11,
				customer_email      = $12,
				customer_phone      = $13,
				customer_name       = $14,
				ship_first_name     = $15,
				ship_last_name      = $16,
				ship_phone          = $17,
				ship_line1          = $18,
				ship_line2          = $19,
				ship_city           = $20,
				ship_state          = $21,
				ship_postal_code    = $22,
				ship_country        = $23,
				shipping_method     = $24,
				payment_method      = $25,
				payment_status      = $26,
				paid_at             = $27,
				transaction_id      = $28
			 WHERE id = $1`,
			updateArgs...,
		); err != nil {
			return fmt.Errorf("update order: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE order_id=$1`, orderID); err != nil {
			return fmt.Errorf("clear line items: %w", err)
		}
		p.UpdatedOrders++
	} else {
		var number int64
		insertArgs := append(append([]any{
			o.ID, customerID, effStatus, shipAddrID,
			subtotal, shippingFee, discount, tax, total,
			notes, createdAt,
			custEmail, custPhone, custName,
		}, shipArgs...), shippingMethod, paymentMethod, paymentStatus, paidAt, txnID)
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO orders (
				wc_order_id, customer_id, status, shipping_address_id,
				subtotal, shipping_fee, discount_amount, tax_amount, total,
				notes, created_at,
				customer_email, customer_phone, customer_name,
				ship_first_name, ship_last_name, ship_phone, ship_line1, ship_line2,
				ship_city, ship_state, ship_postal_code, ship_country,
				shipping_method, payment_method, payment_status, paid_at, transaction_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			          $15, $16, $17, $18, $19, $20, $21, $22, $23, $24,
			          $25, $26, $27, $28)
			RETURNING id, number`,
			insertArgs...,
		).Scan(&orderID, &number); err != nil {
			return fmt.Errorf("insert order: %w", err)
		}
		orderNumber := fmt.Sprintf("%s-%04d", prefix, number)
		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET order_number=$2 WHERE id=$1`,
			orderID, orderNumber); err != nil {
			return fmt.Errorf("set order_number: %w", err)
		}
		p.ImportedOrders++
	}

	// Attach an already-created ShipAny waybill (tracking number, label, courier)
	// when the WC order carried one. Runs in the same tx so the shipment row and
	// the order's 處理中 status commit atomically.
	if wb != nil {
		if err := upsertImportedShipment(ctx, tx, orderID, wb); err != nil {
			return fmt.Errorf("import shipment: %w", err)
		}
		p.ImportedShipments++
	}

	// Two-pass insert so WC Product Bundles parent ↔ child links survive
	// the import: pass 1 writes parents + standalones and remembers their
	// Gyeon UUIDs keyed by WC's _bundle_cart_key; pass 2 writes children
	// with parent_item_id resolved through that map. WC sometimes emits
	// children before parent, so we can't do this in one pass.
	parentGyeonID := make(map[string]string) // _bundle_cart_key → order_items.id
	type childRow struct {
		li        wcLineItem
		bundledBy string
	}
	var children []childRow

	for _, li := range o.LineItems {
		if li.Quantity <= 0 {
			continue // refund / 0-qty pseudo-rows
		}
		cartKey, bundledBy := li.bundleKeys()
		if bundledBy != "" {
			children = append(children, childRow{li: li, bundledBy: bundledBy})
			continue
		}
		variantID, linked := s.resolveVariant(ctx, tx, li)
		sku := strings.TrimSpace(li.SKU)
		if sku == "" {
			sku = fmt.Sprintf("WC-%d", li.ProductID)
		}
		unitPrice := parseDecimal(string(li.Price))
		lineTotal := parseDecimal(li.Total)
		var insertedID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO order_items (
				order_id, variant_id, product_name, variant_sku, variant_attrs,
				unit_price, quantity, line_total
			) VALUES ($1, $2, $3, $4, NULL, $5, $6, $7)
			RETURNING id`,
			orderID, variantID,
			li.Name, sku,
			unitPrice, li.Quantity, lineTotal,
		).Scan(&insertedID); err != nil {
			return fmt.Errorf("insert line item: %w", err)
		}
		if cartKey != "" {
			parentGyeonID[cartKey] = insertedID
		}
		p.ImportedLineItems++
		if !linked {
			p.UnlinkedLineItems++
		}
	}

	for _, c := range children {
		variantID, linked := s.resolveVariant(ctx, tx, c.li)
		sku := strings.TrimSpace(c.li.SKU)
		if sku == "" {
			sku = fmt.Sprintf("WC-%d", c.li.ProductID)
		}
		unitPrice := parseDecimal(string(c.li.Price))
		lineTotal := parseDecimal(c.li.Total)
		var parentID sql.NullString
		if pid, ok := parentGyeonID[c.bundledBy]; ok {
			parentID = sql.NullString{String: pid, Valid: true}
		} else {
			// Broken WC data — child references a parent we never saw.
			// Insert standalone so we don't drop the line, surface a note.
			p.Errors = append(p.Errors,
				fmt.Sprintf("order #%s: orphan bundle child (bundled_by=%q) inserted standalone", o.Number, c.bundledBy))
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (
				order_id, variant_id, product_name, variant_sku, variant_attrs,
				unit_price, quantity, line_total, parent_item_id
			) VALUES ($1, $2, $3, $4, NULL, $5, $6, $7, $8)`,
			orderID, variantID,
			c.li.Name, sku,
			unitPrice, c.li.Quantity, lineTotal, parentID,
		); err != nil {
			return fmt.Errorf("insert bundle child line item: %w", err)
		}
		p.ImportedLineItems++
		if !linked {
			p.UnlinkedLineItems++
		}
	}

	return tx.Commit()
}

// resolveCustomer returns a *string for orders.customer_id. Resolution
// order: WC customer ID → billing email (existing row) → billing email
// (insert unattached row) → NULL (no customer ref at all).
func (s *Service) resolveCustomer(ctx context.Context, tx *sql.Tx, o wcOrder) (*string, error) {
	if o.CustomerID > 0 {
		var id string
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM customers WHERE wc_customer_id=$1`, o.CustomerID).Scan(&id)
		if err == nil {
			return &id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	email := strings.TrimSpace(strings.ToLower(o.Billing.Email))
	if email == "" {
		// Guest checkout with no email — leave customer_id NULL.
		return nil, nil
	}

	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE email=$1`, email).Scan(&id)
	if err == nil {
		// Backfill wc_customer_id when the order references a numeric WC user.
		if o.CustomerID > 0 {
			_, _ = tx.ExecContext(ctx,
				`UPDATE customers SET wc_customer_id=$2 WHERE id=$1 AND wc_customer_id IS NULL`,
				id, o.CustomerID)
		}
		return &id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Insert a new (un-passworded) customer for this order. Mirrors the
	// guest-checkout path in customers.UpsertGuest.
	first := strings.TrimSpace(o.Billing.FirstName)
	last := strings.TrimSpace(o.Billing.LastName)
	if first == "" && last == "" {
		first = "Customer"
	}
	var wcCustID sql.NullInt32
	if o.CustomerID > 0 {
		wcCustID = sql.NullInt32{Int32: int32(o.CustomerID), Valid: true}
	}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name, phone, wc_customer_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		email, first, last, nullableString(o.Billing.Phone), wcCustID,
	).Scan(&id); err != nil {
		return nil, fmt.Errorf("insert order customer: %w", err)
	}
	return &id, nil
}

// resolveVariant maps a WC line item to a local product_variants.id when
// possible. Returns (id, true) when matched; (NULL, false) when the line
// references a product that hasn't been imported. Match order:
//  1. variation_id → product_variants.wc_variation_id
//  2. product_id   → product_variants whose product has wc_product_id and
//     wc_variation_id IS NULL (the simple-product fallback variant).
func (s *Service) resolveVariant(ctx context.Context, tx *sql.Tx, li wcLineItem) (sql.NullString, bool) {
	if li.VariationID > 0 {
		var id string
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM product_variants WHERE wc_variation_id=$1`, li.VariationID).Scan(&id)
		if err == nil {
			return sql.NullString{String: id, Valid: true}, true
		}
	}
	if li.ProductID > 0 {
		var id string
		err := tx.QueryRowContext(ctx, `
			SELECT pv.id
			  FROM product_variants pv
			  JOIN products p ON p.id = pv.product_id
			 WHERE p.wc_product_id = $1 AND pv.wc_variation_id IS NULL
			 LIMIT 1`, li.ProductID).Scan(&id)
		if err == nil {
			return sql.NullString{String: id, Valid: true}, true
		}
	}
	return sql.NullString{Valid: false}, false
}

// snapshotOrderAddress inserts a fresh address row each time an order is
// upserted. Prefers shipping over billing; falls back to billing when
// shipping is empty (some WC stores omit shipping for digital goods).
// Returns NULL when neither has a usable line1/city/postcode.
// Also returns the resolved AddressFields so the caller can freeze them onto
// the order's ship_* snapshot columns. Returns (nil, nil, nil) when neither
// shipping nor billing has a usable address.
func snapshotOrderAddress(ctx context.Context, tx *sql.Tx, customerID *string, o wcOrder) (*string, *customers.AddressFields, error) {
	var src wcCustomerAddress
	switch {
	case hasAddress(o.Shipping):
		src = o.Shipping
	case hasAddress(o.Billing.wcCustomerAddress):
		src = o.Billing.wcCustomerAddress
	default:
		return nil, nil, nil
	}

	first := strings.TrimSpace(src.FirstName)
	last := strings.TrimSpace(src.LastName)
	if first == "" && last == "" {
		first = strings.TrimSpace(o.Billing.FirstName)
		last = strings.TrimSpace(o.Billing.LastName)
	}
	if first == "" && last == "" {
		first = "Customer"
	}

	country := strings.ToUpper(strings.TrimSpace(src.Country))
	if len(country) != 2 {
		country = "HK"
	}

	fields := customers.AddressFields{
		FirstName:  first,
		LastName:   last,
		Phone:      nullableString(src.Phone),
		Line1:      strings.TrimSpace(src.Address1),
		Line2:      nullableString(src.Address2),
		City:       strings.TrimSpace(src.City),
		State:      nullableString(src.State),
		PostalCode: strings.TrimSpace(src.Postcode),
		Country:    country,
	}

	// Dedup-on-write: reuse the customer's existing matching address instead of
	// snapshotting a fresh row per order. This is the core fix for duplicate
	// addresses in 我的帳戶 > 地址, and makes re-imports idempotent. Guest orders
	// (customerID == nil) still insert an unshared snapshot.
	id, err := customers.FindOrCreateAddress(ctx, tx, customerID, fields, false)
	if err != nil {
		return nil, nil, err
	}
	return &id, &fields, nil
}

// shipSnapshotArgs returns the 9 ship_* column values (in column order) for an
// order INSERT/UPDATE from the resolved address fields, or 9 NULLs when the
// order has no usable address (guest / digital-only).
func shipSnapshotArgs(f *customers.AddressFields) []any {
	if f == nil {
		return []any{nil, nil, nil, nil, nil, nil, nil, nil, nil}
	}
	return []any{f.FirstName, f.LastName, f.Phone, f.Line1, f.Line2, f.City, f.State, f.PostalCode, f.Country}
}

func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// parseWCTime accepts WC's date_created in either RFC3339 (when set on
// modern installs) or naive "2006-01-02T15:04:05" (older / GMT-stripped).
// Falls back to current time if parsing fails — the order still gets
// recorded but its placement in history will be off.
func parseWCTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now()
}

// readSetting is a thin wrapper around settingsSvc.Get that swallows the
// not-found / error case to an empty string. Used by the orders import
// for non-critical lookups (order_number_prefix etc.) where missing rows
// just fall back to a default.
func (s *Service) readSetting(ctx context.Context, key string) string {
	st, err := s.settingsSvc.Get(ctx, key)
	if err != nil || st == nil {
		return ""
	}
	return st.Value
}
