package orders

import (
	"context"
	"database/sql"
	"errors"
)

type Cart struct {
	ID           string     `json:"id"`
	CustomerID   *string    `json:"customer_id,omitempty"`
	SessionToken *string    `json:"session_token,omitempty"`
	Items        []CartItem `json:"items"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`
}

type CartItem struct {
	ID          string  `json:"id"`
	CartID      string  `json:"cart_id"`
	VariantID   string  `json:"variant_id"`
	Quantity    int     `json:"quantity"`
	AddedAt     string  `json:"added_at"`
	ProductName string  `json:"product_name"`
	ProductSlug string  `json:"product_slug"`
	SKU         string  `json:"sku"`
	Price       float64 `json:"price"`
	ImageURL    *string `json:"image_url,omitempty"`
}

type AddItemRequest struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateItemRequest struct {
	Quantity int `json:"quantity"`
}

var ErrInsufficientStock = errors.New("insufficient stock")
var ErrCartNotFound = errors.New("cart not found")

type CartService struct {
	db *sql.DB
}

func NewCartService(db *sql.DB) *CartService {
	return &CartService{db: db}
}

func (s *CartService) GetOrCreate(ctx context.Context, sessionToken string, customerID *string) (*Cart, error) {
	var cart Cart
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, session_token, created_at, updated_at
		 FROM carts WHERE session_token = $1 AND expires_at > NOW()`, sessionToken).
		Scan(&cart.ID, &cart.CustomerID, &cart.SessionToken, &cart.CreatedAt, &cart.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRowContext(ctx,
			`INSERT INTO carts (customer_id, session_token)
			 VALUES ($1, $2)
			 RETURNING id, customer_id, session_token, created_at, updated_at`,
			customerID, sessionToken).
			Scan(&cart.ID, &cart.CustomerID, &cart.SessionToken, &cart.CreatedAt, &cart.UpdatedAt)
	}
	if err != nil {
		return nil, err
	}

	items, err := s.listItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items
	return &cart, nil
}

func (s *CartService) GetByID(ctx context.Context, id string) (*Cart, error) {
	var cart Cart
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, session_token, created_at, updated_at
		 FROM carts WHERE id = $1`, id).
		Scan(&cart.ID, &cart.CustomerID, &cart.SessionToken, &cart.CreatedAt, &cart.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCartNotFound
	}
	if err != nil {
		return nil, err
	}

	items, err := s.listItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items
	return &cart, nil
}

func (s *CartService) listItems(ctx context.Context, cartID string) ([]CartItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ci.id, ci.cart_id, ci.variant_id, ci.quantity, ci.added_at,
		        p.name, p.slug, pv.sku, pv.price,
		        COALESCE(vmf.url, vi.url, pmf.url, pi.url) AS image_url
		 FROM cart_items ci
		 JOIN product_variants pv ON pv.id = ci.variant_id
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN product_images vi ON vi.variant_id = ci.variant_id
		 LEFT JOIN media_files vmf ON vmf.id = vi.media_file_id
		 LEFT JOIN product_images pi
		     ON pi.product_id = pv.product_id AND pi.is_primary = TRUE
		 LEFT JOIN media_files pmf ON pmf.id = pi.media_file_id
		 WHERE ci.cart_id = $1
		 ORDER BY ci.added_at ASC`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.CartID, &item.VariantID, &item.Quantity, &item.AddedAt,
			&item.ProductName, &item.ProductSlug, &item.SKU, &item.Price, &item.ImageURL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *CartService) AddItem(ctx context.Context, cartID string, req AddItemRequest) (*CartItem, error) {
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// check stock
	var stock int
	err := s.db.QueryRowContext(ctx,
		`SELECT stock_qty FROM product_variants WHERE id = $1 AND is_active = TRUE`, req.VariantID).
		Scan(&stock)
	if err != nil {
		return nil, err
	}
	if stock < req.Quantity {
		return nil, ErrInsufficientStock
	}

	var item CartItem
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO cart_items (cart_id, variant_id, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (cart_id, variant_id)
		 DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
		 RETURNING id, cart_id, variant_id, quantity, added_at`,
		cartID, req.VariantID, req.Quantity).
		Scan(&item.ID, &item.CartID, &item.VariantID, &item.Quantity, &item.AddedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *CartService) UpdateItem(ctx context.Context, cartID, itemID string, req UpdateItemRequest) (*CartItem, error) {
	if req.Quantity <= 0 {
		return nil, s.RemoveItem(ctx, cartID, itemID)
	}

	var item CartItem
	err := s.db.QueryRowContext(ctx,
		`UPDATE cart_items SET quantity = $3
		 WHERE id = $1 AND cart_id = $2
		 RETURNING id, cart_id, variant_id, quantity, added_at`,
		itemID, cartID, req.Quantity).
		Scan(&item.ID, &item.CartID, &item.VariantID, &item.Quantity, &item.AddedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *CartService) RemoveItem(ctx context.Context, cartID, itemID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cart_items WHERE id = $1 AND cart_id = $2`, itemID, cartID)
	return err
}

func (s *CartService) Clear(ctx context.Context, cartID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID)
	return err
}
