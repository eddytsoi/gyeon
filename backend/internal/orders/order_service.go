package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/pricing"
)

type OrderStatus string

const (
	StatusPending    OrderStatus = "pending"
	StatusPaid       OrderStatus = "paid"
	StatusProcessing OrderStatus = "processing"
	StatusShipped    OrderStatus = "shipped"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
	StatusRefunded   OrderStatus = "refunded"
)

type Order struct {
	ID                string      `json:"id"`
	CustomerID        *string     `json:"customer_id,omitempty"`
	Status            OrderStatus `json:"status"`
	ShippingAddressID *string     `json:"shipping_address_id,omitempty"`
	Subtotal          float64     `json:"subtotal"`
	ShippingFee       float64     `json:"shipping_fee"`
	DiscountAmount    float64     `json:"discount_amount"`
	Total             float64     `json:"total"`
	Notes             *string     `json:"notes,omitempty"`
	CustomerEmail     *string     `json:"customer_email,omitempty"`
	CustomerPhone     *string     `json:"customer_phone,omitempty"`
	CustomerName      *string     `json:"customer_name,omitempty"`
	PaymentIntentID   *string     `json:"payment_intent_id,omitempty"`
	PaymentStatus     *string     `json:"payment_status,omitempty"`
	PaymentMethod     *string     `json:"payment_method,omitempty"`
	Items             []OrderItem `json:"items,omitempty"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
}

type OrderItem struct {
	ID           string                 `json:"id"`
	OrderID      string                 `json:"order_id"`
	VariantID    *string                `json:"variant_id,omitempty"`
	ProductName  string                 `json:"product_name"`
	VariantSKU   string                 `json:"variant_sku"`
	VariantAttrs map[string]interface{} `json:"variant_attrs,omitempty"`
	UnitPrice    float64                `json:"unit_price"`
	Quantity     int                    `json:"quantity"`
	LineTotal    float64                `json:"line_total"`
}

type CustomerInfo struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type ShippingAddressInput struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type CheckoutRequest struct {
	CartID            string                `json:"cart_id"`
	CustomerID        *string               `json:"customer_id"`
	CustomerInfo      *CustomerInfo         `json:"customer_info,omitempty"`
	ShippingAddressID *string               `json:"shipping_address_id,omitempty"`
	ShippingAddress   *ShippingAddressInput `json:"shipping_address,omitempty"`
	SaveAddress       bool                  `json:"save_address,omitempty"`
	ShippingFee       float64               `json:"shipping_fee"`
	CouponCode        *string               `json:"coupon_code"`
	Notes             *string               `json:"notes"`
}

type CheckoutResult struct {
	Order          *Order `json:"order"`
	ClientSecret   string `json:"client_secret"`
	PublishableKey string `json:"publishable_key"`
	Mode           string `json:"mode"`
}

type UpdateStatusRequest struct {
	Status OrderStatus `json:"status"`
	Note   *string     `json:"note"`
}

var ErrEmptyCart = errors.New("cart is empty")
var ErrCustomerInfoRequired = errors.New("customer_info is required for guest checkout")
var ErrShippingRequired = errors.New("shipping_address or shipping_address_id is required")
var ErrOrderNotFound = errors.New("order not found")

// valid forward transitions
var allowedTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:    {StatusPaid, StatusCancelled},
	StatusPaid:       {StatusProcessing, StatusRefunded},
	StatusProcessing: {StatusShipped, StatusCancelled},
	StatusShipped:    {StatusDelivered},
	StatusDelivered:  {StatusRefunded},
	StatusCancelled:  {},
	StatusRefunded:   {},
}

type OrderService struct {
	db          *sql.DB
	cartSvc     *CartService
	pricingSvc  *pricing.Service
	customerSvc *customers.Service
	paymentSvc  *payment.Service
	emailSvc    *email.Service
}

func NewOrderService(
	db *sql.DB,
	cartSvc *CartService,
	pricingSvc *pricing.Service,
	customerSvc *customers.Service,
	paymentSvc *payment.Service,
	emailSvc *email.Service,
) *OrderService {
	return &OrderService{
		db:          db,
		cartSvc:     cartSvc,
		pricingSvc:  pricingSvc,
		customerSvc: customerSvc,
		paymentSvc:  paymentSvc,
		emailSvc:    emailSvc,
	}
}

func (s *OrderService) Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	cart, err := s.cartSvc.GetByID(ctx, req.CartID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, ErrEmptyCart
	}

	// Resolve customer: existing logged-in id, or upsert guest by email.
	customerID := req.CustomerID
	customerEmail := ""
	customerPhone := ""
	customerName := ""

	if customerID != nil && *customerID != "" {
		c, err := s.customerSvc.GetByID(ctx, *customerID)
		if err == nil {
			customerEmail = c.Email
			customerName = c.FirstName + " " + c.LastName
			if c.Phone != nil {
				customerPhone = *c.Phone
			}
		}
		// Form-supplied customer_info overrides for this order's snapshot
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

	// Resolve shipping address: existing id, or insert new
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

		// Split name back into first/last for the address row
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

	// Fetch line item info (outside the transaction so we can compute discounts first)
	type lineItem struct {
		variantID   string
		productID   string
		categoryID  *string
		productName string
		sku         string
		price       float64
		quantity    int
	}

	var lines []lineItem
	var subtotal float64
	for _, item := range cart.Items {
		var li lineItem
		li.variantID = item.VariantID
		li.quantity = item.Quantity

		err := s.db.QueryRowContext(ctx,
			`SELECT pv.sku, pv.price, pv.product_id, p.category_id, p.name
			 FROM product_variants pv
			 JOIN products p ON p.id = pv.product_id
			 WHERE pv.id = $1`, item.VariantID).
			Scan(&li.sku, &li.price, &li.productID, &li.categoryID, &li.productName)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("variant %s not found", item.VariantID)
		}
		if err != nil {
			return nil, err
		}

		subtotal += li.price * float64(li.quantity)
		lines = append(lines, li)
	}

	// Compute discounts before opening the transaction
	var discountResult pricing.DiscountResult
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
		discountResult, err = s.pricingSvc.ComputeDiscount(ctx, pricingItems, subtotal, req.CouponCode)
		if err != nil {
			return nil, err
		}
	}

	discountAmount := discountResult.TotalDiscount
	total := subtotal - discountAmount + req.ShippingFee
	if total < 0 {
		total = 0
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Decrement stock atomically
	for _, item := range cart.Items {
		res, err := tx.ExecContext(ctx,
			`UPDATE product_variants SET stock_qty = stock_qty - $2
			 WHERE id = $1 AND stock_qty >= $2`,
			item.VariantID, item.Quantity)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, fmt.Errorf("insufficient stock for variant %s", item.VariantID)
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

	var order Order
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes, customer_email, customer_phone, customer_name, payment_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'requires_payment_method')
		 RETURNING id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes,
		           customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		           created_at, updated_at`,
		customerID, shippingAddressID, subtotal, req.ShippingFee, discountAmount, total, req.Notes,
		emailPtr, phonePtr, namePtr).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Insert order items
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
	}

	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`, order.ID, StatusPending)

	if discountResult.CouponID != nil {
		if err := pricing.IncrementCouponUsage(ctx, tx, *discountResult.CouponID); err != nil {
			return nil, err
		}
	}

	tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, req.CartID)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Create Stripe PaymentIntent (outside the order tx — Stripe is the source of truth for the PI;
	// if this fails after the order is committed, the order can be retried/cancelled separately).
	result := &CheckoutResult{Order: &order}
	if s.paymentSvc != nil {
		intent, err := s.paymentSvc.CreatePaymentIntent(ctx, payment.CreateIntentParams{
			AmountCents: int64(total*100 + 0.5), // round to nearest cent
			Currency:    "hkd",
			OrderID:     order.ID,
			Email:       customerEmail,
		})
		if err != nil {
			log.Printf("create payment intent for order %s: %v", order.ID, err)
			return nil, fmt.Errorf("payment setup failed: %w", err)
		}

		_, err = s.db.ExecContext(ctx,
			`UPDATE orders SET payment_intent_id=$2 WHERE id=$1`, order.ID, intent.ID)
		if err != nil {
			log.Printf("persist payment_intent_id on order %s: %v", order.ID, err)
		}
		order.PaymentIntentID = &intent.ID
		result.ClientSecret = intent.ClientSecret
		result.PublishableKey = s.paymentSvc.PublishableKey(ctx)
		result.Mode = s.paymentSvc.Mode(ctx)
	}

	return result, nil
}

// MarkPaidByPaymentIntent flips a pending order to `paid` and triggers the
// confirmation email. Called from the Stripe webhook on payment_intent.succeeded.
func (s *OrderService) MarkPaidByPaymentIntent(ctx context.Context, paymentIntentID string) error {
	var orderID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE payment_intent_id=$1`, paymentIntentID).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("webhook: no order found for payment_intent %s", paymentIntentID)
		return nil
	}
	if err != nil {
		return err
	}

	// Update payment_status (idempotent) and try to flip order status.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE orders SET payment_status='succeeded' WHERE id=$1`, orderID)

	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == StatusPending {
		_, err := s.UpdateStatus(ctx, orderID, UpdateStatusRequest{Status: StatusPaid})
		if err != nil {
			log.Printf("webhook: update status for order %s: %v", orderID, err)
		}
		// Send confirmation email (best-effort)
		s.sendConfirmationEmail(ctx, order)
	}
	return nil
}

func (s *OrderService) sendConfirmationEmail(ctx context.Context, order *Order) {
	if s.emailSvc == nil || order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}

	// Look up shipping address (best-effort)
	var line1, line2, city, state, postal, country string
	if order.ShippingAddressID != nil {
		s.db.QueryRowContext(ctx,
			`SELECT line1, COALESCE(line2,''), city, COALESCE(state,''), postal_code, country
			 FROM addresses WHERE id=$1`, *order.ShippingAddressID).
			Scan(&line1, &line2, &city, &state, &postal, &country)
	}

	items := make([]email.OrderEmailItem, len(order.Items))
	for i, it := range order.Items {
		items[i] = email.OrderEmailItem{
			Name:      it.ProductName,
			SKU:       it.VariantSKU,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			LineTotal: it.LineTotal,
		}
	}

	setupURL := ""
	if order.CustomerID != nil && s.customerSvc != nil {
		// Generate a setup-password token only if the customer has no password yet.
		var pwHash sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT password_hash FROM customers WHERE id=$1`, *order.CustomerID).Scan(&pwHash)
		if err == nil && (!pwHash.Valid || pwHash.String == "") {
			token, err := s.customerSvc.CreateSetupToken(ctx, *order.CustomerID)
			if err == nil {
				base := s.emailSvc.PublicBaseURL(ctx)
				setupURL = fmt.Sprintf("%s/account/setup-password?token=%s", base, token)
			} else {
				log.Printf("create setup token for customer %s: %v", *order.CustomerID, err)
			}
		}
	}

	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}

	err := s.emailSvc.SendOrderConfirmation(ctx, email.OrderEmailParams{
		OrderID:         order.ID,
		CustomerName:    name,
		CustomerEmail:   *order.CustomerEmail,
		Items:           items,
		Subtotal:        order.Subtotal,
		ShippingFee:     order.ShippingFee,
		DiscountAmount:  order.DiscountAmount,
		Total:           order.Total,
		Currency:        "HKD",
		ShippingLine1:   line1,
		ShippingLine2:   line2,
		ShippingCity:    city,
		ShippingPostal:  postal,
		ShippingCountry: country,
		SetupURL:        setupURL,
	})
	if err != nil {
		log.Printf("send order confirmation email for order %s: %v", order.ID, err)
	}
}

func splitName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.SplitN(full, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func (s *OrderService) GetByID(ctx context.Context, id string) (*Order, error) {
	var order Order
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes,
		        customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		        created_at, updated_at
		 FROM orders WHERE id = $1`, id).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.CreatedAt, &order.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total
		 FROM order_items WHERE order_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OrderItem
		rows.Scan(&item.ID, &item.OrderID, &item.VariantID, &item.ProductName,
			&item.VariantSKU, &item.UnitPrice, &item.Quantity, &item.LineTotal)
		order.Items = append(order.Items, item)
	}
	return &order, nil
}

func (s *OrderService) List(ctx context.Context, limit, offset int) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, customer_id, status, subtotal, shipping_fee, discount_amount, total,
		        customer_email, customer_phone, customer_name, payment_intent_id, payment_status,
		        created_at, updated_at
		 FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.Subtotal,
			&o.ShippingFee, &o.DiscountAmount, &o.Total,
			&o.CustomerEmail, &o.CustomerPhone, &o.CustomerName,
			&o.PaymentIntentID, &o.PaymentStatus,
			&o.CreatedAt, &o.UpdatedAt)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *OrderService) UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*Order, error) {
	var current OrderStatus
	err := s.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = $1`, id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	allowed := false
	for _, next := range allowedTransitions[current] {
		if next == req.Status {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("cannot transition from %s to %s", current, req.Status)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var order Order
	err = tx.QueryRowContext(ctx,
		`UPDATE orders SET status = $2 WHERE id = $1
		 RETURNING id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes,
		           customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		           created_at, updated_at`,
		id, req.Status).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status, note) VALUES ($1, $2, $3)`,
		id, req.Status, req.Note)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &order, nil
}
