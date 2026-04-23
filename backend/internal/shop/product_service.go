package shop

import (
	"context"
	"database/sql"
)

type Product struct {
	ID          string  `json:"id"`
	CategoryID  *string `json:"category_id,omitempty"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type Variant struct {
	ID             string   `json:"id"`
	ProductID      string   `json:"product_id"`
	SKU            string   `json:"sku"`
	Price          float64  `json:"price"`
	CompareAtPrice *float64 `json:"compare_at_price,omitempty"`
	StockQty       int      `json:"stock_qty"`
	IsActive       bool     `json:"is_active"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type ProductImage struct {
	ID        string  `json:"id"`
	ProductID string  `json:"product_id"`
	VariantID *string `json:"variant_id,omitempty"`
	URL       string  `json:"url"`
	AltText   *string `json:"alt_text,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsPrimary bool    `json:"is_primary"`
	CreatedAt string  `json:"created_at"`
}

type CreateProductRequest struct {
	CategoryID  *string `json:"category_id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateProductRequest struct {
	CreateProductRequest
	IsActive bool `json:"is_active"`
}

type CreateVariantRequest struct {
	SKU            string   `json:"sku"`
	Price          float64  `json:"price"`
	CompareAtPrice *float64 `json:"compare_at_price"`
	StockQty       int      `json:"stock_qty"`
}

type UpdateVariantRequest struct {
	SKU            string   `json:"sku"`
	Price          float64  `json:"price"`
	CompareAtPrice *float64 `json:"compare_at_price"`
	StockQty       int      `json:"stock_qty"`
	IsActive       bool     `json:"is_active"`
}

type AdjustStockRequest struct {
	Delta int    `json:"delta"` // positive = restock, negative = remove
	Note  string `json:"note"`
}

type UpdateImageRequest struct {
	AltText   *string `json:"alt_text"`
	SortOrder int     `json:"sort_order"`
	IsPrimary bool    `json:"is_primary"`
}

type AddImageRequest struct {
	VariantID *string `json:"variant_id"`
	URL       string  `json:"url"`
	AltText   *string `json:"alt_text"`
	SortOrder int     `json:"sort_order"`
	IsPrimary bool    `json:"is_primary"`
}

type ProductService struct {
	db *sql.DB
}

func NewProductService(db *sql.DB) *ProductService {
	return &ProductService{db: db}
}

func (s *ProductService) List(ctx context.Context, limit, offset int) ([]Product, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, category_id, slug, name, description, is_active, created_at, updated_at
		 FROM products WHERE is_active = TRUE ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name,
			&p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *ProductService) GetByID(ctx context.Context, id string) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx,
		`SELECT id, category_id, slug, name, description, is_active, created_at, updated_at
		 FROM products WHERE id = $1`, id).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProductService) Create(ctx context.Context, req CreateProductRequest) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO products (category_id, slug, name, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, category_id, slug, name, description, is_active, created_at, updated_at`,
		req.CategoryID, req.Slug, req.Name, req.Description).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProductService) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx,
		`UPDATE products SET category_id=$2, slug=$3, name=$4, description=$5, is_active=$6
		 WHERE id=$1
		 RETURNING id, category_id, slug, name, description, is_active, created_at, updated_at`,
		id, req.CategoryID, req.Slug, req.Name, req.Description, req.IsActive).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Description, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	return err
}

func (s *ProductService) ListVariants(ctx context.Context, productID string) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, product_id, sku, price, compare_at_price, stock_qty, is_active, created_at, updated_at
		 FROM product_variants WHERE product_id = $1 AND is_active = TRUE ORDER BY created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (s *ProductService) CreateVariant(ctx context.Context, productID string, req CreateVariantRequest) (*Variant, error) {
	var v Variant
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_variants (product_id, sku, price, compare_at_price, stock_qty)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, product_id, sku, price, compare_at_price, stock_qty, is_active, created_at, updated_at`,
		productID, req.SKU, req.Price, req.CompareAtPrice, req.StockQty).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *ProductService) UpdateVariant(ctx context.Context, variantID string, req UpdateVariantRequest) (*Variant, error) {
	var v Variant
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_variants SET sku=$2, price=$3, compare_at_price=$4, stock_qty=$5, is_active=$6
		 WHERE id=$1
		 RETURNING id, product_id, sku, price, compare_at_price, stock_qty, is_active, created_at, updated_at`,
		variantID, req.SKU, req.Price, req.CompareAtPrice, req.StockQty, req.IsActive).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *ProductService) DeleteVariant(ctx context.Context, variantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM product_variants WHERE id = $1`, variantID)
	return err
}

func (s *ProductService) AdjustStock(ctx context.Context, variantID string, req AdjustStockRequest) (*Variant, error) {
	var v Variant
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_variants SET stock_qty = GREATEST(0, stock_qty + $2)
		 WHERE id = $1
		 RETURNING id, product_id, sku, price, compare_at_price, stock_qty, is_active, created_at, updated_at`,
		variantID, req.Delta).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *ProductService) UpdateImage(ctx context.Context, imageID string, req UpdateImageRequest) (*ProductImage, error) {
	var img ProductImage
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_images SET alt_text=$2, sort_order=$3, is_primary=$4
		 WHERE id=$1
		 RETURNING id, product_id, variant_id, url, alt_text, sort_order, is_primary, created_at`,
		imageID, req.AltText, req.SortOrder, req.IsPrimary).
		Scan(&img.ID, &img.ProductID, &img.VariantID, &img.URL,
			&img.AltText, &img.SortOrder, &img.IsPrimary, &img.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (s *ProductService) DeleteImage(ctx context.Context, imageID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM product_images WHERE id = $1`, imageID)
	return err
}

func (s *ProductService) LowStock(ctx context.Context, threshold int) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, product_id, sku, price, compare_at_price, stock_qty, is_active, created_at, updated_at
		 FROM product_variants WHERE stock_qty <= $1 AND is_active = TRUE ORDER BY stock_qty ASC`,
		threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (s *ProductService) ListImages(ctx context.Context, productID string) ([]ProductImage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, url, alt_text, sort_order, is_primary, created_at
		 FROM product_images WHERE product_id = $1 ORDER BY sort_order ASC, is_primary DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]ProductImage, 0)
	for rows.Next() {
		var img ProductImage
		if err := rows.Scan(&img.ID, &img.ProductID, &img.VariantID, &img.URL,
			&img.AltText, &img.SortOrder, &img.IsPrimary, &img.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (s *ProductService) AddImage(ctx context.Context, productID string, req AddImageRequest) (*ProductImage, error) {
	var img ProductImage
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_images (product_id, variant_id, url, alt_text, sort_order, is_primary)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, product_id, variant_id, url, alt_text, sort_order, is_primary, created_at`,
		productID, req.VariantID, req.URL, req.AltText, req.SortOrder, req.IsPrimary).
		Scan(&img.ID, &img.ProductID, &img.VariantID, &img.URL,
			&img.AltText, &img.SortOrder, &img.IsPrimary, &img.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &img, nil
}
