package shop

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/util"
)

// productSearchFields are the columns matched by the optional `search` param
// on List / ListAll. Body content (description) is intentionally excluded —
// noisy on substring match and slow without a trigram index.
var productSearchFields = []string{"p.name", "p.slug", "p.number::text"}

type Product struct {
	ID          string  `json:"id"`
	Number      int64   `json:"number"`
	CategoryID  *string `json:"category_id,omitempty"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type ProductTranslation struct {
	Locale      string  `json:"locale"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

type UpsertProductTranslationRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
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
	ProductName    *string  `json:"product_name,omitempty"`
	ImageURL       *string  `json:"image_url,omitempty"`
}

type ProductImage struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	VariantID   *string `json:"variant_id,omitempty"`
	MediaFileID *string `json:"media_file_id,omitempty"`
	URL         string  `json:"url"`
	AltText     *string `json:"alt_text,omitempty"`
	SortOrder   int     `json:"sort_order"`
	IsPrimary   bool    `json:"is_primary"`
	CreatedAt   string  `json:"created_at"`
}

type CreateProductRequest struct {
	CategoryID  *string `json:"category_id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
}

type UpdateProductRequest struct {
	CreateProductRequest
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
	VariantID   *string `json:"variant_id"`
	MediaFileID *string `json:"media_file_id"`
	URL         *string `json:"url"`
	AltText     *string `json:"alt_text"`
	SortOrder   int     `json:"sort_order"`
	IsPrimary   bool    `json:"is_primary"`
}

// productSelect LEFT JOINs translations so name/description fall back to base when no translation exists.
// $1 = locale (empty string → JOIN never matches → base content returned).
const productTranslationJoin = `
	LEFT JOIN product_translations t ON t.product_id = p.id AND t.locale = $1`

const productSelect = `
	SELECT p.id, p.number, p.category_id, p.slug,
	       COALESCE(t.name,        p.name)        AS name,
	       COALESCE(t.description, p.description) AS description,
	       p.status, p.created_at, p.updated_at
	FROM products p` + productTranslationJoin

func scanProduct(row interface{ Scan(...any) error }) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Number, &p.CategoryID, &p.Slug, &p.Name,
		&p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

const productPrefix = "shop:products:"

type ProductService struct {
	db    *sql.DB
	cache cache.Store
	ttl   func(context.Context) time.Duration
}

func NewProductService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *ProductService {
	return &ProductService{db: db, cache: c, ttl: ttl}
}

// List returns active products. locale may be empty for base content.
// search is an optional case-insensitive substring matched against
// productSearchFields; pass "" to disable.
func (s *ProductService) List(ctx context.Context, locale, search string, limit, offset int) ([]Product, error) {
	key := fmt.Sprintf("shop:products:pub:%s:%s:%d:%d", locale, search, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
	}

	args := []any{locale, limit, offset}
	where := "p.status = 'active'" // public: active only
	if clause, arg := util.BuildSearchClause(search, productSearchFields, 4); clause != "" {
		where += " AND " + clause
		args = append(args, arg)
	}
	rows, err := s.db.QueryContext(ctx,
		productSelect+` WHERE `+where+` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListByCategorySlug returns active products filtered to a single category
// (resolved from its slug). locale and search behave like List.
func (s *ProductService) ListByCategorySlug(ctx context.Context, locale, categorySlug, search string, limit, offset int) ([]Product, error) {
	key := fmt.Sprintf("shop:products:bycat:%s:%s:%s:%d:%d", locale, categorySlug, search, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
	}

	args := []any{locale, limit, offset, categorySlug}
	where := "p.status = 'active' AND p.category_id = (SELECT id FROM categories WHERE slug = $4 AND is_active = TRUE)"
	if clause, arg := util.BuildSearchClause(search, productSearchFields, 5); clause != "" {
		where += " AND " + clause
		args = append(args, arg)
	}
	rows, err := s.db.QueryContext(ctx,
		productSelect+` WHERE `+where+` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListAll returns all products regardless of is_active (admin). locale may be empty.
// search is optional; see List.
func (s *ProductService) ListAll(ctx context.Context, locale, search string, limit, offset int) ([]Product, error) {
	key := fmt.Sprintf("shop:products:all:%s:%s:%d:%d", locale, search, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
	}

	args := []any{locale, limit, offset}
	query := productSelect + ` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`
	if clause, arg := util.BuildSearchClause(search, productSearchFields, 4); clause != "" {
		query = productSelect + ` WHERE ` + clause + ` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, arg)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// GetByID fetches a product by ID. locale may be empty for base content.
func (s *ProductService) GetByID(ctx context.Context, id, locale string) (*Product, error) {
	key := fmt.Sprintf("shop:products:id:%s:%s", id, locale)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Product)
		return &p, nil
	}
	p, err := scanProduct(s.db.QueryRowContext(ctx,
		productSelect+` WHERE p.id = $2`, locale, id))
	if err != nil {
		return nil, err
	}
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
}

func (s *ProductService) Create(ctx context.Context, req CreateProductRequest) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO products (category_id, slug, name, description, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, category_id, slug, name, description, status, created_at, updated_at`,
		req.CategoryID, req.Slug, req.Name, req.Description, req.Status).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return &p, nil
}

func (s *ProductService) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	var p Product
	err := s.db.QueryRowContext(ctx,
		`UPDATE products SET category_id=$2, slug=$3, name=$4, description=$5, status=$6
		 WHERE id=$1
		 RETURNING id, category_id, slug, name, description, status, created_at, updated_at`,
		id, req.CategoryID, req.Slug, req.Name, req.Description, req.Status).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return &p, nil
}

// GetIDByNumber resolves a sequential display number to its UUID.
// Returns sql.ErrNoRows if no row matches.
func (s *ProductService) GetIDByNumber(ctx context.Context, n int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM products WHERE number = $1`, n).Scan(&id)
	return id, err
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

// DeleteAll removes every product (cascades to variants and images).
func (s *ProductService) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM products`)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

func (s *ProductService) GetVariantByID(ctx context.Context, variantID string) (*Variant, error) {
	var v Variant
	err := s.db.QueryRowContext(ctx,
		`SELECT pv.id, pv.product_id, pv.sku, pv.price, pv.compare_at_price,
		        pv.stock_qty, pv.is_active, pv.created_at, pv.updated_at,
		        p.name AS product_name,
		        pi.url AS image_url
		 FROM product_variants pv
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN product_images pi
		     ON pi.product_id = pv.product_id AND pi.is_primary = TRUE
		 WHERE pv.id = $1
		 LIMIT 1`, variantID).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
			&v.ProductName, &v.ImageURL)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *ProductService) ListVariants(ctx context.Context, productID string) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pv.id, pv.product_id, pv.sku, pv.price, pv.compare_at_price,
		        pv.stock_qty, pv.is_active, pv.created_at, pv.updated_at,
		        COALESCE(mf.url, pi.url) AS image_url
		 FROM product_variants pv
		 LEFT JOIN product_images pi ON pi.variant_id = pv.id
		 LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		 WHERE pv.product_id = $1 AND pv.is_active = TRUE
		 ORDER BY pv.created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.IsActive, &v.CreatedAt, &v.UpdatedAt, &v.ImageURL); err != nil {
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
	img, err := scanProductImage(s.db.QueryRowContext(ctx,
		`WITH unset_others AS (
		     UPDATE product_images SET is_primary = FALSE
		     WHERE product_id = (SELECT product_id FROM product_images WHERE id = $1)
		       AND id <> $1
		       AND $4 = TRUE
		 ),
		 upd AS (
		     UPDATE product_images SET alt_text=$2, sort_order=$3, is_primary=$4
		     WHERE id=$1
		     RETURNING *
		 )
		 SELECT upd.id, upd.product_id, upd.variant_id, upd.media_file_id,
		        COALESCE(mf.url, upd.url, '') AS url,
		        upd.alt_text, upd.sort_order, upd.is_primary, upd.created_at
		 FROM upd LEFT JOIN media_files mf ON mf.id = upd.media_file_id`,
		imageID, req.AltText, req.SortOrder, req.IsPrimary))
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

func scanProductImage(row interface{ Scan(...any) error }) (ProductImage, error) {
	var img ProductImage
	err := row.Scan(&img.ID, &img.ProductID, &img.VariantID, &img.MediaFileID,
		&img.URL, &img.AltText, &img.SortOrder, &img.IsPrimary, &img.CreatedAt)
	return img, err
}

func (s *ProductService) ListImages(ctx context.Context, productID string) ([]ProductImage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pi.id, pi.product_id, pi.variant_id, pi.media_file_id,
		        COALESCE(mf.url, pi.url, '') AS url,
		        pi.alt_text, pi.sort_order, pi.is_primary, pi.created_at
		 FROM product_images pi
		 LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		 WHERE pi.product_id = $1
		 ORDER BY pi.sort_order ASC, pi.is_primary DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]ProductImage, 0)
	for rows.Next() {
		img, err := scanProductImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (s *ProductService) AddImage(ctx context.Context, productID string, req AddImageRequest) (*ProductImage, error) {
	img, err := scanProductImage(s.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO product_images (product_id, variant_id, media_file_id, url, alt_text, sort_order, is_primary)
		     VALUES ($1, $2, $3, $4, $5, $6, $7)
		     RETURNING *
		 )
		 SELECT ins.id, ins.product_id, ins.variant_id, ins.media_file_id,
		        COALESCE(mf.url, ins.url, '') AS url,
		        ins.alt_text, ins.sort_order, ins.is_primary, ins.created_at
		 FROM ins LEFT JOIN media_files mf ON mf.id = ins.media_file_id`,
		productID, req.VariantID, req.MediaFileID, req.URL, req.AltText, req.SortOrder, req.IsPrimary))
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// --- Translation management ---

func (s *ProductService) ListTranslations(ctx context.Context, productID string) ([]ProductTranslation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT locale, name, description, updated_at
		 FROM product_translations WHERE product_id = $1 ORDER BY locale`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProductTranslation, 0)
	for rows.Next() {
		var t ProductTranslation
		if err := rows.Scan(&t.Locale, &t.Name, &t.Description, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *ProductService) UpsertTranslation(ctx context.Context, productID, locale string, req UpsertProductTranslationRequest) (*ProductTranslation, error) {
	var t ProductTranslation
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_translations (product_id, locale, name, description)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (product_id, locale) DO UPDATE
		   SET name=$3, description=$4, updated_at=NOW()
		 RETURNING locale, name, description, updated_at`,
		productID, locale, req.Name, req.Description).
		Scan(&t.Locale, &t.Name, &t.Description, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// Translation changes affect localized list/detail responses
	s.cache.DeleteByPrefix(productPrefix)
	return &t, nil
}

func (s *ProductService) DeleteTranslation(ctx context.Context, productID, locale string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM product_translations WHERE product_id = $1 AND locale = $2`, productID, locale)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errProductNotFound
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

var errProductNotFound = sql.ErrNoRows
