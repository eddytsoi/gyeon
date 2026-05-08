package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
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
}

// OrdersProgressUpdate is streamed once per processed order.
type OrdersProgressUpdate struct {
	TotalOrders        int      `json:"total_orders"`
	ProcessedOrders    int      `json:"processed_orders"`
	ImportedOrders     int      `json:"imported_orders"`     // newly inserted
	UpdatedOrders      int      `json:"updated_orders"`      // matched by wc_order_id, updated in place
	ImportedLineItems  int      `json:"imported_line_items"` // line items inserted in this run (linked + unlinked)
	UnlinkedLineItems  int      `json:"unlinked_line_items"` // line items whose product/variant could not be resolved locally — kept as snapshot, variant_id NULL
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
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).fetchOrderTotal()
}

// RunOrdersStreaming pages through /wc/v3/orders, upserts each order +
// addresses + line items, and emits a CustomersProgressUpdate-shaped
// progress frame per order. Final frame has Done = true.
func (s *Service) RunOrdersStreaming(ctx context.Context, req OrdersImportRequest, send func(OrdersProgressUpdate)) {
	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := OrdersProgressUpdate{Errors: []string{}}

	p.TotalOrders = wc.fetchOrderTotal()
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
		batch, err := wc.fetchOrders(page)
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

	createdAt := parseWCTime(o.DateCreated)

	shipAddrID, err := snapshotOrderAddress(ctx, tx, customerID, o)
	if err != nil {
		return fmt.Errorf("snapshot address: %w", err)
	}

	notes := nullableString(o.CustomerNote)

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
				created_at          = $11
			 WHERE id = $1`,
			orderID, customerID, status, shipAddrID,
			subtotal, shippingFee, discount, tax, total,
			notes, createdAt,
		); err != nil {
			return fmt.Errorf("update order: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE order_id=$1`, orderID); err != nil {
			return fmt.Errorf("clear line items: %w", err)
		}
		p.UpdatedOrders++
	} else {
		var number int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO orders (
				wc_order_id, customer_id, status, shipping_address_id,
				subtotal, shipping_fee, discount_amount, tax_amount, total,
				notes, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, number`,
			o.ID, customerID, status, shipAddrID,
			subtotal, shippingFee, discount, tax, total,
			notes, createdAt,
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

	for _, li := range o.LineItems {
		if li.Quantity <= 0 {
			continue // refund / 0-qty pseudo-rows
		}
		variantID, linked := s.resolveVariant(ctx, tx, li)
		sku := strings.TrimSpace(li.SKU)
		if sku == "" {
			sku = fmt.Sprintf("WC-%d", li.ProductID)
		}
		unitPrice := parseDecimal(li.Price)
		lineTotal := parseDecimal(li.Total)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (
				order_id, variant_id, product_name, variant_sku, variant_attrs,
				unit_price, quantity, line_total
			) VALUES ($1, $2, $3, $4, NULL, $5, $6, $7)`,
			orderID, variantID,
			li.Name, sku,
			unitPrice, li.Quantity, lineTotal,
		); err != nil {
			return fmt.Errorf("insert line item: %w", err)
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
func snapshotOrderAddress(ctx context.Context, tx *sql.Tx, customerID *string, o wcOrder) (*string, error) {
	var src wcCustomerAddress
	switch {
	case hasAddress(o.Shipping):
		src = o.Shipping
	case hasAddress(o.Billing.wcCustomerAddress):
		src = o.Billing.wcCustomerAddress
	default:
		return nil, nil
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

	var id string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, FALSE)
		RETURNING id`,
		customerID, first, last,
		nullableString(src.Phone),
		strings.TrimSpace(src.Address1),
		nullableString(src.Address2),
		strings.TrimSpace(src.City),
		nullableString(src.State),
		strings.TrimSpace(src.Postcode),
		country,
	).Scan(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

func parseDecimal(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
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
