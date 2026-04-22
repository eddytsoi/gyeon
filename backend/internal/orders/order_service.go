package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

type CheckoutRequest struct {
	CartID            string  `json:"cart_id"`
	CustomerID        *string `json:"customer_id"`
	ShippingAddressID *string `json:"shipping_address_id"`
	ShippingFee       float64 `json:"shipping_fee"`
	Notes             *string `json:"notes"`
}

type UpdateStatusRequest struct {
	Status OrderStatus `json:"status"`
	Note   *string     `json:"note"`
}

var ErrEmptyCart = errors.New("cart is empty")
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
	db      *sql.DB
	cartSvc *CartService
}

func NewOrderService(db *sql.DB, cartSvc *CartService) *OrderService {
	return &OrderService{db: db, cartSvc: cartSvc}
}

func (s *OrderService) Checkout(ctx context.Context, req CheckoutRequest) (*Order, error) {
	cart, err := s.cartSvc.GetByID(ctx, req.CartID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, ErrEmptyCart
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Fetch variant prices and decrement stock atomically
	type lineItem struct {
		variantID   string
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

		err := tx.QueryRowContext(ctx,
			`UPDATE product_variants
			 SET stock_qty = stock_qty - $2
			 WHERE id = $1 AND stock_qty >= $2
			 RETURNING sku, price`,
			item.VariantID, item.Quantity).
			Scan(&li.sku, &li.price)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("insufficient stock for variant %s", item.VariantID)
		}
		if err != nil {
			return nil, err
		}

		// fetch product name
		tx.QueryRowContext(ctx,
			`SELECT p.name FROM products p
			 JOIN product_variants v ON v.product_id = p.id
			 WHERE v.id = $1`, item.VariantID).Scan(&li.productName)

		subtotal += li.price * float64(li.quantity)
		lines = append(lines, li)
	}

	total := subtotal + req.ShippingFee

	// Create order
	var order Order
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, shipping_address_id, subtotal, shipping_fee, total, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes, created_at, updated_at`,
		req.CustomerID, req.ShippingAddressID, subtotal, req.ShippingFee, total, req.Notes).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CreatedAt, &order.UpdatedAt)
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

	// Record initial status in history
	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`, order.ID, StatusPending)

	// Clear cart
	tx.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, req.CartID)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *OrderService) GetByID(ctx context.Context, id string) (*Order, error) {
	var order Order
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes, created_at, updated_at
		 FROM orders WHERE id = $1`, id).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CreatedAt, &order.UpdatedAt)
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
		`SELECT id, customer_id, status, subtotal, shipping_fee, discount_amount, total, created_at, updated_at
		 FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.Subtotal,
			&o.ShippingFee, &o.DiscountAmount, &o.Total, &o.CreatedAt, &o.UpdatedAt)
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
		 RETURNING id, customer_id, status, shipping_address_id, subtotal, shipping_fee, discount_amount, total, notes, created_at, updated_at`,
		id, req.Status).
		Scan(&order.ID, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.DiscountAmount, &order.Total,
			&order.Notes, &order.CreatedAt, &order.UpdatedAt)
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
