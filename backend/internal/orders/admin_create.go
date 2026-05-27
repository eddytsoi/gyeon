package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/pricing"
	"gyeon/backend/internal/shop"
)

// AdminCreateRequest is the payload accepted by POST /admin/orders. It
// resembles CheckoutRequest but takes line items directly (no cart) and
// targets an admin workflow: payment is deferred by default, the carrier
// is locked to SF Express via site settings, and an optional payment-link
// email can be triggered for the customer to complete checkout out-of-band.
type AdminCreateRequest struct {
	// Customer — either CustomerID (existing customer) or CustomerInfo
	// (guest path; UpsertGuest will find-or-create a customers row by email).
	CustomerID   *string       `json:"customer_id,omitempty"`
	CustomerInfo *CustomerInfo `json:"customer_info,omitempty"`

	// Line items — variant id + quantity. Bundle parents are expanded
	// server-side; do NOT include bundle component variants here.
	Items []AdminCreateItem `json:"items"`

	// Shipping address — either an existing one or a new one to insert.
	ShippingAddressID *string               `json:"shipping_address_id,omitempty"`
	ShippingAddress   *ShippingAddressInput `json:"shipping_address,omitempty"`
	SaveAddress       bool                  `json:"save_address,omitempty"`

	// Optional manual shipping-fee override. When nil, defaults to 0 — the
	// HK SF Express flow is 到付 (customer pays courier directly) or absorbed
	// by the merchant under the free-shipping threshold. Admin can set a
	// positive number when they're billing the customer for shipping
	// directly.
	ShippingFeeOverride *float64 `json:"shipping_fee_override,omitempty"`

	CouponCode *string `json:"coupon_code,omitempty"`

	// Customer-visible notes that end up on the order detail / receipt.
	Notes *string `json:"notes,omitempty"`

	// Initial order status. Must be one of: pending | processing | cancelled.
	// Defaults to pending when blank.
	InitialStatus OrderStatus `json:"initial_status,omitempty"`

	// EmailPaymentLink, when true and the customer has an email on file,
	// triggers a Stripe PaymentIntent + a transactional email containing the
	// magic /pay link so the customer can finish checkout in their browser.
	EmailPaymentLink bool `json:"email_payment_link,omitempty"`
}

// AdminCreateItem is one line in the admin-built order. Quantity must be
// >= 1; the backend looks up the variant's current price, name, SKU, and
// product kind.
type AdminCreateItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

var (
	ErrAdminCreateNoItems        = errors.New("at least one item is required")
	ErrAdminCreateInvalidStatus  = errors.New("initial_status must be one of: pending, processing, cancelled")
	ErrAdminCreateInsufficientStock = errors.New("insufficient stock for one or more items")
	ErrAdminCreateVariantNotFound   = errors.New("variant not found")
)

// validInitialStatuses are the order statuses an admin is allowed to set
// when creating an order. shipped / delivered are excluded because they
// imply downstream side effects (shipment records, low-stock notification
// history) that this entry point isn't equipped to set up.
var validInitialStatuses = map[OrderStatus]bool{
	StatusPending:    true,
	StatusProcessing: true,
	StatusCancelled:  true,
}

// AdminCreate builds a new order from an admin-provided spec. Reuses the
// same pricing/tax/free-shipping/stock primitives as the customer-facing
// Checkout flow so totals are computed identically.
func (s *OrderService) AdminCreate(ctx context.Context, req AdminCreateRequest) (*Order, error) {
	// --- Validate ---------------------------------------------------------
	if len(req.Items) == 0 {
		return nil, ErrAdminCreateNoItems
	}
	for _, it := range req.Items {
		if it.VariantID == "" || it.Quantity <= 0 {
			return nil, ErrAdminCreateNoItems
		}
	}

	status := req.InitialStatus
	if status == "" {
		status = StatusPending
	}
	if !validInitialStatuses[status] {
		return nil, ErrAdminCreateInvalidStatus
	}

	// --- Resolve customer (mirrors Checkout) ------------------------------
	customerID := req.CustomerID
	customerEmail := ""
	customerPhone := ""
	customerName := ""
	customerRole := customers.RoleCustomer
	// Matches order_service: only confirmed-existing customers are non-guest.
	// Admin-created orders for new/guest customers fall through to UpsertGuest
	// below and remain isGuest=true for promotion eligibility.
	isGuest := true

	if customerID != nil && *customerID != "" {
		c, err := s.customerSvc.GetByID(ctx, *customerID)
		if err == nil {
			customerEmail = c.Email
			customerName = strings.TrimSpace(c.FirstName + " " + c.LastName)
			if c.Phone != nil {
				customerPhone = *c.Phone
			}
			customerRole = customers.NormalizeRole(c.Role)
			isGuest = false
		}
		if req.CustomerInfo != nil {
			if req.CustomerInfo.Email != "" {
				customerEmail = req.CustomerInfo.Email
			}
			if req.CustomerInfo.Phone != "" {
				customerPhone = req.CustomerInfo.Phone
			}
			if req.CustomerInfo.FirstName != "" || req.CustomerInfo.LastName != "" {
				customerName = strings.TrimSpace(req.CustomerInfo.FirstName + " " + req.CustomerInfo.LastName)
			}
		}
	} else {
		if req.CustomerInfo == nil || req.CustomerInfo.Email == "" {
			return nil, ErrCustomerInfoRequired
		}
		var phonePtr *string
		if req.CustomerInfo.Phone != "" {
			p := req.CustomerInfo.Phone
			phonePtr = &p
		}
		c, _, err := s.customerSvc.UpsertGuest(ctx,
			req.CustomerInfo.Email,
			req.CustomerInfo.FirstName,
			req.CustomerInfo.LastName,
			phonePtr,
		)
		if err != nil {
			return nil, fmt.Errorf("upsert guest: %w", err)
		}
		customerID = &c.ID
		customerEmail = c.Email
		customerPhone = req.CustomerInfo.Phone
		customerName = strings.TrimSpace(req.CustomerInfo.FirstName + " " + req.CustomerInfo.LastName)
	}

	// --- Resolve shipping address (mirrors Checkout) ----------------------
	shippingAddressID := req.ShippingAddressID
	if (shippingAddressID == nil || *shippingAddressID == "") && req.ShippingAddress != nil {
		country := req.ShippingAddress.Country
		if country == "" {
			country = "HK"
		}
		var line2, state *string
		if req.ShippingAddress.Line2 != "" {
			v := req.ShippingAddress.Line2
			line2 = &v
		}
		if req.ShippingAddress.State != "" {
			v := req.ShippingAddress.State
			state = &v
		}
		var phonePtr *string
		if customerPhone != "" {
			p := customerPhone
			phonePtr = &p
		}
		first, last := splitName(customerName)

		var addrID string
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			customerID, first, last, phonePtr,
			req.ShippingAddress.Line1, line2,
			req.ShippingAddress.City, state,
			req.ShippingAddress.PostalCode, country).Scan(&addrID)
		if err != nil {
			return nil, fmt.Errorf("insert address: %w", err)
		}
		shippingAddressID = &addrID
	}
	if shippingAddressID == nil || *shippingAddressID == "" {
		return nil, ErrShippingRequired
	}

	// --- Load line item info (variants + bundle components) ---------------
	type bundleComponent struct {
		variantID   string
		productName string
		sku         string
		price       float64
		quantity    int
	}
	type lineItem struct {
		variantID   string
		productID   string
		categoryID  *string
		productName string
		sku         string
		price       float64
		quantity    int
		kind        string
		components  []bundleComponent
	}

	var lines []lineItem
	var subtotal float64
	for _, it := range req.Items {
		var li lineItem
		li.variantID = it.VariantID
		li.quantity = it.Quantity

		var variantName sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT pv.sku, pv.price, pv.product_id, p.category_id, p.name, pv.name, p.kind
			 FROM product_variants pv
			 JOIN products p ON p.id = pv.product_id
			 WHERE pv.id = $1`, it.VariantID).
			Scan(&li.sku, &li.price, &li.productID, &li.categoryID, &li.productName, &variantName, &li.kind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrAdminCreateVariantNotFound, it.VariantID)
		}
		if err != nil {
			return nil, err
		}
		if li.kind != "bundle" {
			li.productName = shop.ProductDisplayName(li.productName, variantName.String)
		}

		if li.kind == "bundle" {
			compRows, err := s.db.QueryContext(ctx,
				`SELECT bi.component_variant_id, p.name, pv.name, pv.sku, pv.price, bi.quantity
				 FROM bundle_items bi
				 JOIN product_variants pv ON pv.id = bi.component_variant_id
				 JOIN products p ON p.id = pv.product_id
				 WHERE bi.bundle_product_id = $1
				 ORDER BY bi.sort_order ASC`, li.productID)
			if err != nil {
				return nil, err
			}
			for compRows.Next() {
				var bc bundleComponent
				var compVariantName sql.NullString
				var compQty int
				if err := compRows.Scan(&bc.variantID, &bc.productName, &compVariantName, &bc.sku, &bc.price, &compQty); err != nil {
					compRows.Close()
					return nil, err
				}
				bc.productName = shop.ProductDisplayName(bc.productName, compVariantName.String)
				bc.quantity = compQty * it.Quantity
				li.components = append(li.components, bc)
			}
			compRows.Close()
			if err := compRows.Err(); err != nil {
				return nil, err
			}
		}

		subtotal += li.price * float64(li.quantity)
		lines = append(lines, li)
	}

	// --- Compute discount / tax / shipping (same primitives as Checkout) --
	var discountResult pricing.DiscountResult
	var err error
	if s.pricingSvc != nil {
		pricingItems := make([]pricing.LineItem, len(lines))
		for i, li := range lines {
			pricingItems[i] = pricing.LineItem{
				VariantID:  li.variantID,
				ProductID:  li.productID,
				CategoryID: li.categoryID,
				Price:      li.price,
				Quantity:   li.quantity,
			}
		}
		discountResult, err = s.pricingSvc.ComputeDiscount(ctx, pricingItems, subtotal, req.CouponCode, customerRole, isGuest)
		if err != nil {
			return nil, err
		}
	}

	discountAmount := discountResult.TotalDiscount
	taxableAmount := subtotal - discountAmount
	if taxableAmount < 0 {
		taxableAmount = 0
	}

	var taxAmount float64
	if s.taxSvc != nil {
		taxRes := s.taxSvc.Calculate(ctx, taxableAmount)
		taxAmount = taxRes.TaxAmount
		if !taxRes.Inclusive {
			taxableAmount += taxAmount
		}
	}

	shippingFree := s.shippingFreeFor(ctx, customerRole, subtotal-discountAmount)
	shippingFee := 0.0
	if req.ShippingFeeOverride != nil {
		shippingFee = *req.ShippingFeeOverride
		if shippingFee < 0 {
			shippingFee = 0
		}
	}
	// Free shipping wins over any non-zero override: an admin who ticks
	// "custom fee" but enters a positive value on a free-shipping-qualifying
	// order should still see HK$0 — matches the customer-facing Checkout.
	if shippingFree {
		shippingFee = 0
	}

	total := taxableAmount + shippingFee
	if total < 0 {
		total = 0
	}

	// --- Persist (transactional, mirrors Checkout) ------------------------
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	type stockDec struct {
		variantID string
		quantity  int
		before    int
		after     int
	}
	var stockDecs []stockDec
	deductOne := func(variantID string, qty int) error {
		var before, after int
		err := tx.QueryRowContext(ctx,
			`UPDATE product_variants SET stock_qty = stock_qty - $2
			 WHERE id = $1 AND stock_qty >= $2
			 RETURNING stock_qty`, variantID, qty).Scan(&after)
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: variant %s", ErrAdminCreateInsufficientStock, variantID)
		}
		if err != nil {
			return err
		}
		before = after + qty
		stockDecs = append(stockDecs, stockDec{variantID, qty, before, after})
		return nil
	}
	for _, li := range lines {
		if li.kind == "bundle" {
			for _, bc := range li.components {
				if err := deductOne(bc.variantID, bc.quantity); err != nil {
					return nil, err
				}
			}
		} else {
			if err := deductOne(li.variantID, li.quantity); err != nil {
				return nil, err
			}
		}
	}

	var emailPtr, phonePtr, namePtr *string
	if customerEmail != "" {
		emailPtr = &customerEmail
	}
	if customerPhone != "" {
		phonePtr = &customerPhone
	}
	if customerName != "" {
		namePtr = &customerName
	}

	// SF Express defaults from site settings; NULL when shipany is disabled
	// (matches legacy Checkout behavior).
	shipanyOn, defaultCarrier, defaultService, err := s.resolveDefaultShipping(ctx)
	if err != nil {
		return nil, err
	}
	var carrierPtr, servicePtr *string
	if shipanyOn {
		carrierPtr = &defaultCarrier
		servicePtr = &defaultService
	}

	// Admin-created orders default to payment_status = 'unpaid'; admin
	// flips it later via the order detail page (or via the EmailPaymentLink
	// flow below, which switches to 'requires_payment_method').
	paymentStatus := "unpaid"
	if req.EmailPaymentLink {
		paymentStatus = "requires_payment_method"
	}

	var order Order
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, shipping_address_id, status, subtotal, shipping_fee, shipping_free, discount_amount, tax_amount, total, notes,
		                     customer_email, customer_phone, customer_name, payment_status,
		                     selected_carrier, selected_service, pickup_point_id, pickup_point_label)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 RETURNING id, number, customer_id, status, shipping_address_id, subtotal, shipping_fee, shipping_free, discount_amount, tax_amount, total, notes,
		           customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		           selected_carrier, selected_service, pickup_point_id, pickup_point_label,
		           created_at, updated_at`,
		customerID, shippingAddressID, status, subtotal, shippingFee, shippingFree, discountAmount, taxAmount, total, req.Notes,
		emailPtr, phonePtr, namePtr, paymentStatus,
		carrierPtr, servicePtr, nil, nil).
		Scan(&order.ID, &order.Number, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.ShippingFree, &order.DiscountAmount, &order.TaxAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.SelectedCarrier, &order.SelectedService, &order.PickupPointID, &order.PickupPointLabel,
			&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	order.OrderNumber = fmt.Sprintf("%s-%04d", s.orderNumberPrefix(ctx), order.Number)
	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET order_number = $2 WHERE id = $1`,
		order.ID, order.OrderNumber); err != nil {
		return nil, err
	}

	for _, li := range lines {
		lineTotal := li.price * float64(li.quantity)
		var item OrderItem
		err := tx.QueryRowContext(ctx,
			`INSERT INTO order_items (order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id, order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total`,
			order.ID, li.variantID, li.productName, li.sku, li.price, li.quantity, lineTotal).
			Scan(&item.ID, &item.OrderID, &item.VariantID, &item.ProductName,
				&item.VariantSKU, &item.UnitPrice, &item.Quantity, &item.LineTotal)
		if err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)

		for _, bc := range li.components {
			var child OrderItem
			childTotal := bc.price * float64(bc.quantity)
			cerr := tx.QueryRowContext(ctx,
				`INSERT INTO order_items (order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total, parent_item_id)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				 RETURNING id, order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total`,
				order.ID, bc.variantID, bc.productName, bc.sku, bc.price, bc.quantity, childTotal, item.ID).
				Scan(&child.ID, &child.OrderID, &child.VariantID, &child.ProductName,
					&child.VariantSKU, &child.UnitPrice, &child.Quantity, &child.LineTotal)
			if cerr != nil {
				return nil, cerr
			}
			parentID := item.ID
			child.ParentItemID = &parentID
			order.Items = append(order.Items, child)
		}
	}

	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`, order.ID, status)

	statusCopy := status
	_ = CreateSystemNoticeTx(ctx, tx, order.ID, &statusCopy, "Order created by admin")

	if discountResult.CouponID != nil {
		if err := pricing.IncrementCouponUsage(ctx, tx, *discountResult.CouponID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Inventory history + low-stock alerts (best-effort, outside the tx).
	if len(stockDecs) > 0 {
		decs := make([]lowStockDec, len(stockDecs))
		for i, d := range stockDecs {
			decs[i] = lowStockDec{VariantID: d.variantID, Quantity: d.quantity}
		}
		go s.checkLowStockCrossings(context.Background(), decs)

		orderIDStr := order.ID
		for _, d := range stockDecs {
			s.recordInventoryHistory(ctx, d.variantID, d.before, d.after, "order.admin_create", &orderIDStr)
		}
	}

	if s.onCreated != nil {
		go s.onCreated(context.Background(), &order)
	}

	// Optional Stripe payment-link email. Skipped silently when no Stripe is
	// configured or no email is on file — admin still sees the created order
	// and can re-trigger payment later.
	if req.EmailPaymentLink && s.paymentSvc != nil && customerEmail != "" && s.emailSvc != nil {
		intent, ierr := s.paymentSvc.CreatePaymentIntent(ctx, payment.CreateIntentParams{
			AmountCents: int64(total*100 + 0.5),
			Currency:    "hkd",
			OrderID:     order.ID,
			Email:       customerEmail,
		})
		if ierr == nil {
			if _, uerr := s.db.ExecContext(ctx,
				`UPDATE orders SET payment_intent_id=$2 WHERE id=$1`, order.ID, intent.ID); uerr == nil {
				order.PaymentIntentID = &intent.ID
			}
			s.sendPaymentLinkEmail(ctx, &order, intent.ClientSecret)
		}
	}

	return &order, nil
}
