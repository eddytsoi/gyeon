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
	VariantName *string `json:"variant_name,omitempty"`
	Price       float64 `json:"price"`
	WeightGrams *int    `json:"weight_grams,omitempty"`
	LengthMM   *int    `json:"length_mm,omitempty"`
	WidthMM    *int    `json:"width_mm,omitempty"`
	HeightMM   *int    `json:"height_mm,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
	// Kind is the product type for this line ("simple" | "bundle"). The
	// frontend uses it to decide whether to render the Children block.
	Kind     string           `json:"kind,omitempty"`
	Children []CartItemChild  `json:"children,omitempty"`
}

// CartItemChild is a component of a bundle line item, hydrated for display
// on cart / checkout / abandoned-cart-email. No price field — bundles price
// is set on the bundle itself; children show only what's inside.
type CartItemChild struct {
	ProductName string  `json:"product_name"`
	ProductSlug string  `json:"product_slug"`
	SKU         string  `json:"sku"`
	VariantName *string `json:"variant_name,omitempty"`
	Quantity    int     `json:"quantity"`
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
		        p.name, p.slug, pv.sku, pv.name, pv.price, pv.weight_grams,
		        pv.length_mm, pv.width_mm, pv.height_mm,
		        COALESCE(
		            CASE WHEN vmf.mime_type LIKE 'video/%' THEN vmf.thumbnail_url END,
		            vmf.webp_url, vmf.url, vi.url,
		            CASE WHEN pmf.mime_type LIKE 'video/%' THEN pmf.thumbnail_url END,
		            pmf.webp_url, pmf.url, pi.url
		        ) AS image_url,
		        p.kind, p.id
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

	items := []CartItem{}
	// Track parent productID per item index so we can hydrate bundle
	// children in one follow-up batch (one query per bundle item — keeps
	// the SQL simple and there are typically few bundle lines per cart).
	type bundleRef struct {
		idx       int
		productID string
		parentQty int
	}
	var bundleRefs []bundleRef
	for rows.Next() {
		var item CartItem
		var productID string
		if err := rows.Scan(&item.ID, &item.CartID, &item.VariantID, &item.Quantity, &item.AddedAt,
			&item.ProductName, &item.ProductSlug, &item.SKU, &item.VariantName, &item.Price, &item.WeightGrams,
			&item.LengthMM, &item.WidthMM, &item.HeightMM, &item.ImageURL, &item.Kind, &productID); err != nil {
			return nil, err
		}
		if item.Kind == "bundle" {
			bundleRefs = append(bundleRefs, bundleRef{idx: len(items), productID: productID, parentQty: item.Quantity})
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, ref := range bundleRefs {
		children, err := s.bundleChildren(ctx, ref.productID, ref.parentQty)
		if err != nil {
			return nil, err
		}
		items[ref.idx].Children = children
	}
	return items, nil
}

// bundleChildren fetches the components of a bundle product, scaled by the
// parent line's quantity, with the same image-coalesce fallback used for
// main cart rows. Used by listItems and (via Cart) by the abandoned-cart
// email so customers see what's inside the box.
func (s *CartService) bundleChildren(ctx context.Context, bundleProductID string, parentQty int) ([]CartItemChild, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.name, p.slug, pv.sku, pv.name, bi.quantity,
		        COALESCE(
		            CASE WHEN vmf.mime_type LIKE 'video/%' THEN vmf.thumbnail_url END,
		            vmf.webp_url, vmf.url, vi.url,
		            CASE WHEN pmf.mime_type LIKE 'video/%' THEN pmf.thumbnail_url END,
		            pmf.webp_url, pmf.url, pi.url
		        ) AS image_url
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN product_images vi ON vi.variant_id = bi.component_variant_id
		 LEFT JOIN media_files vmf ON vmf.id = vi.media_file_id
		 LEFT JOIN product_images pi
		     ON pi.product_id = pv.product_id AND pi.is_primary = TRUE
		 LEFT JOIN media_files pmf ON pmf.id = pi.media_file_id
		 WHERE bi.bundle_product_id = $1
		 ORDER BY bi.sort_order ASC, bi.created_at ASC`, bundleProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CartItemChild{}
	for rows.Next() {
		var c CartItemChild
		var perBundleQty int
		if err := rows.Scan(&c.ProductName, &c.ProductSlug, &c.SKU, &c.VariantName, &perBundleQty, &c.ImageURL); err != nil {
			return nil, err
		}
		c.Quantity = perBundleQty * parentQty
		out = append(out, c)
	}
	return out, rows.Err()
}

// ChildrenForCart hydrates bundle children for a known cart ID. Exposed so
// adjacent packages (e.g. abandoned-cart email) can reuse the same shape
// without duplicating the SQL.
func (s *CartService) ChildrenForCart(ctx context.Context, cartID string) ([]CartItem, error) {
	return s.listItems(ctx, cartID)
}

func (s *CartService) AddItem(ctx context.Context, cartID string, req AddItemRequest) (*CartItem, error) {
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// check stock — for bundle products use derived stock instead of variant stock_qty
	var stock int
	var productKind, productID string
	err := s.db.QueryRowContext(ctx,
		`SELECT pv.stock_qty, p.kind, p.id
		 FROM product_variants pv
		 JOIN products p ON p.id = pv.product_id
		 WHERE pv.id = $1 AND pv.is_active = TRUE`, req.VariantID).
		Scan(&stock, &productKind, &productID)
	if err != nil {
		return nil, err
	}

	if productKind == "bundle" {
		err = s.db.QueryRowContext(ctx,
			`SELECT COALESCE(MIN(FLOOR(pv.stock_qty::float / bi.quantity)), 0)::int
			 FROM bundle_items bi
			 JOIN product_variants pv ON pv.id = bi.component_variant_id
			 WHERE bi.bundle_product_id = $1`, productID).Scan(&stock)
		if err != nil {
			return nil, err
		}
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
