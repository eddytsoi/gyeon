package shop

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/util"
)

// defaultBundleSKU returns the auto-generated SKU for a bundle product's
// default variant. Computed in Go (rather than via SQL `SUBSTRING($1::text, …)`)
// because lib/pq can't deduce a single type for a parameter used as both
// uuid and text in the same statement (error 42P08).
func defaultBundleSKU(productID string) string {
	if len(productID) >= 8 {
		return "BUNDLE-" + strings.ToUpper(productID[:8])
	}
	return "BUNDLE-" + strings.ToUpper(productID)
}

// productSearchFields are the columns matched by the optional `search` param
// on List / ListAll. Body content (description) is intentionally excluded —
// noisy on substring match and slow without a trigram index.
var productSearchFields = []string{"p.name", "p.slug", "p.number::text"}

type Product struct {
	ID                 string   `json:"id"`
	Number             int64    `json:"number"`
	CategoryID         *string  `json:"category_id,omitempty"`
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Excerpt            *string  `json:"excerpt,omitempty"`
	Description        *string  `json:"description,omitempty"`
	HowToUse           *string  `json:"how_to_use,omitempty"`
	CompatibleSurfaces []string `json:"compatible_surfaces"`
	Status             string   `json:"status"`
	Kind               string   `json:"kind"` // "simple" | "bundle"
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

// ProductWithMeta enriches Product with quick-glance fields useful for list
// views (MCP catalog browsing, agent decision-making) so callers don't need
// an N+1 follow-up GET per product.
type ProductWithMeta struct {
	Product
	VariantCount     int     `json:"variant_count"`
	PrimaryImageURL  *string `json:"primary_image_url,omitempty"`
	DefaultVariantID *string `json:"default_variant_id,omitempty"`
}

// BundleItem represents a component row in a bundle product.
type BundleItem struct {
	ID                   string  `json:"id"`
	BundleProductID      string  `json:"bundle_product_id"`
	ComponentVariantID   string  `json:"component_variant_id"`
	Quantity             int     `json:"quantity"`
	SortOrder            int     `json:"sort_order"`
	DisplayNameOverride  *string `json:"display_name_override,omitempty"`
	// Derived from joined tables
	ComponentProductName string  `json:"component_product_name"`
	ComponentVariantName *string `json:"component_variant_name,omitempty"`
	ComponentSKU         string  `json:"component_sku"`
	ComponentStockQty    int     `json:"component_stock_qty"`
	ComponentPrice       float64 `json:"component_price"`
	CreatedAt            string  `json:"created_at"`
}

// BundleItemInput is used when setting bundle items via SetBundleItems.
type BundleItemInput struct {
	ComponentVariantID  string  `json:"component_variant_id"`
	Quantity            int     `json:"quantity"`
	SortOrder           int     `json:"sort_order"`
	DisplayNameOverride *string `json:"display_name_override,omitempty"`
}

// SetBundleItemsRequest wraps the item list for the PUT handler.
type SetBundleItemsRequest struct {
	Items []BundleItemInput `json:"items"`
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
	ID                 string   `json:"id"`
	ProductID          string   `json:"product_id"`
	SKU                string   `json:"sku"`
	Name               *string  `json:"name,omitempty"`
	Price              float64  `json:"price"`
	CompareAtPrice     *float64 `json:"compare_at_price,omitempty"`
	StockQty           int      `json:"stock_qty"`
	LowStockThreshold  *int     `json:"low_stock_threshold,omitempty"`
	WeightGrams        *int     `json:"weight_grams,omitempty"`
	LengthMM           *int     `json:"length_mm,omitempty"`
	WidthMM            *int     `json:"width_mm,omitempty"`
	HeightMM           *int     `json:"height_mm,omitempty"`
	IsActive           bool     `json:"is_active"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
	ProductName        *string  `json:"product_name,omitempty"`
	ImageURL           *string  `json:"image_url,omitempty"`
}

type ProductImage struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	VariantID     *string `json:"variant_id,omitempty"`
	MediaFileID   *string `json:"media_file_id,omitempty"`
	URL           string  `json:"url"`
	MimeType      *string `json:"mime_type,omitempty"`
	ThumbnailURL  *string `json:"thumbnail_url,omitempty"`
	VideoAutoplay bool    `json:"video_autoplay"`
	VideoFit      string  `json:"video_fit"`
	AltText       *string `json:"alt_text,omitempty"`
	SortOrder     int     `json:"sort_order"`
	IsPrimary     bool    `json:"is_primary"`
	CreatedAt     string  `json:"created_at"`
}

type CreateProductRequest struct {
	CategoryID         *string  `json:"category_id"`
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Excerpt            *string  `json:"excerpt"`
	Description        *string  `json:"description"`
	HowToUse           *string  `json:"how_to_use"`
	CompatibleSurfaces []string `json:"compatible_surfaces"`
	Status             string   `json:"status"`
	Kind               string   `json:"kind"` // "simple" | "bundle"; defaults to "simple"
}

type UpdateProductRequest struct {
	CreateProductRequest
}

type CreateVariantRequest struct {
	SKU               string   `json:"sku"`
	Name              *string  `json:"name"`
	Price             float64  `json:"price"`
	CompareAtPrice    *float64 `json:"compare_at_price"`
	StockQty          int      `json:"stock_qty"`
	LowStockThreshold *int     `json:"low_stock_threshold"`
	WeightGrams       *int     `json:"weight_grams"`
	LengthMM          *int     `json:"length_mm"`
	WidthMM           *int     `json:"width_mm"`
	HeightMM          *int     `json:"height_mm"`
}

type UpdateVariantRequest struct {
	SKU               string   `json:"sku"`
	Name              *string  `json:"name"`
	Price             float64  `json:"price"`
	CompareAtPrice    *float64 `json:"compare_at_price"`
	StockQty          int      `json:"stock_qty"`
	LowStockThreshold *int     `json:"low_stock_threshold"`
	WeightGrams       *int     `json:"weight_grams"`
	LengthMM          *int     `json:"length_mm"`
	WidthMM           *int     `json:"width_mm"`
	HeightMM          *int     `json:"height_mm"`
	IsActive          bool     `json:"is_active"`
}

type AdjustStockRequest struct {
	Delta int    `json:"delta"` // positive = restock, negative = remove
	Note  string `json:"note"`
}

type UpdateImageRequest struct {
	AltText   *string `json:"alt_text"`
	SortOrder *int    `json:"sort_order"`
	IsPrimary *bool   `json:"is_primary"`
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
	       p.excerpt,
	       COALESCE(t.description, p.description) AS description,
	       p.how_to_use, p.compatible_surfaces,
	       p.status, p.kind, p.created_at, p.updated_at
	FROM products p` + productTranslationJoin

func scanProduct(row interface{ Scan(...any) error }) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Number, &p.CategoryID, &p.Slug, &p.Name,
		&p.Excerpt, &p.Description, &p.HowToUse, pq.Array(&p.CompatibleSurfaces),
		&p.Status, &p.Kind, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

const productPrefix = "shop:products:"

// ThumbnailEnsurer is satisfied by media.Handler. Used to lazily backfill
// first-frame thumbnails for video media that pre-date the thumbnail feature.
// Optional dependency: when nil, backfill is skipped (tests/seed paths stay simple).
type ThumbnailEnsurer interface {
	EnsureVideoThumbnail(ctx context.Context, mediaFileID string)
}

type ProductService struct {
	db        *sql.DB
	cache     cache.Store
	ttl       func(context.Context) time.Duration
	thumbnail ThumbnailEnsurer
}

func NewProductService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *ProductService {
	return &ProductService{db: db, cache: c, ttl: ttl}
}

// SetThumbnailEnsurer wires in the media-side helper for lazy video thumbnail
// backfill. Call from app wiring after both services exist.
func (s *ProductService) SetThumbnailEnsurer(t ThumbnailEnsurer) {
	s.thumbnail = t
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

// ListFilters captures every parameter the public storefront product list
// supports. Empty / zero values mean "no filter".
type ListFilters struct {
	Locale       string
	Search       string
	CategorySlug string
	MinPrice     *float64
	MaxPrice     *float64
	Sort         string // "new" | "price_asc" | "price_desc" | "name"
	Limit        int
	Offset       int
}

// ListEnrichedFiltered is the unified storefront product listing with search +
// category + price range + sort. Skips the cache (too many parameter
// combinations to be worth caching for the small expected catalog).
func (s *ProductService) ListEnrichedFiltered(ctx context.Context, f ListFilters) ([]ProductWithMeta, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	args := []any{f.Locale, f.Limit, f.Offset}
	where := "p.status = 'active'"
	idx := 4

	if f.CategorySlug != "" {
		where += fmt.Sprintf(" AND p.category_id = (SELECT id FROM categories WHERE slug = $%d AND is_active = TRUE)", idx)
		args = append(args, f.CategorySlug)
		idx++
	}
	if clause, arg := util.BuildSearchClause(f.Search, productSearchFields, idx); clause != "" {
		where += " AND " + clause
		args = append(args, arg)
		idx++
	}

	const minPriceSQ = "(SELECT MIN(pv.price) FROM product_variants pv WHERE pv.product_id = p.id AND pv.is_active = TRUE)"
	if f.MinPrice != nil {
		where += fmt.Sprintf(" AND %s >= $%d", minPriceSQ, idx)
		args = append(args, *f.MinPrice)
		idx++
	}
	if f.MaxPrice != nil {
		where += fmt.Sprintf(" AND %s <= $%d", minPriceSQ, idx)
		args = append(args, *f.MaxPrice)
		idx++
	}

	orderBy := "p.created_at DESC"
	switch f.Sort {
	case "price_asc":
		orderBy = minPriceSQ + " ASC NULLS LAST, p.created_at DESC"
	case "price_desc":
		orderBy = minPriceSQ + " DESC NULLS LAST, p.created_at DESC"
	case "name":
		orderBy = "COALESCE(t.name, p.name) ASC"
	}

	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       (SELECT pv.id FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE
		        ORDER BY pv.created_at ASC LIMIT 1) AS default_variant_id
		FROM products p` + productTranslationJoin + `
		WHERE ` + where + `
		ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]ProductWithMeta, 0)
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
		); err != nil {
			return nil, err
		}
		products = append(products, pm)
	}
	return products, rows.Err()
}

// ListEnriched returns active products plus variant_count, primary_image_url
// and default_variant_id in a single round-trip. Used by the MCP list_products
// tool so agents can act on a result without N+1 follow-ups (notably: bundles
// expose their single BUNDLE-* variant id directly, ready for add_to_cart).
func (s *ProductService) ListEnriched(ctx context.Context, locale, search string, limit, offset int) ([]ProductWithMeta, error) {
	key := fmt.Sprintf("shop:products:pubmeta:%s:%s:%d:%d", locale, search, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]ProductWithMeta), nil
	}

	args := []any{locale, limit, offset}
	where := "p.status = 'active'"
	if clause, arg := util.BuildSearchClause(search, productSearchFields, 4); clause != "" {
		where += " AND " + clause
		args = append(args, arg)
	}

	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       (SELECT pv.id FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE
		        ORDER BY pv.created_at ASC LIMIT 1) AS default_variant_id
		FROM products p` + productTranslationJoin + `
		WHERE ` + where + `
		ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]ProductWithMeta, 0)
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
		); err != nil {
			return nil, err
		}
		products = append(products, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListEnrichedByCategorySlug is the category-filtered counterpart of
// ListEnriched. Kept in lockstep so the REST list endpoint exposes the
// same shape regardless of the `?category=` filter.
func (s *ProductService) ListEnrichedByCategorySlug(ctx context.Context, locale, categorySlug, search string, limit, offset int) ([]ProductWithMeta, error) {
	key := fmt.Sprintf("shop:products:bycatmeta:%s:%s:%s:%d:%d", locale, categorySlug, search, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]ProductWithMeta), nil
	}

	args := []any{locale, limit, offset, categorySlug}
	where := "p.status = 'active' AND p.category_id = (SELECT id FROM categories WHERE slug = $4 AND is_active = TRUE)"
	if clause, arg := util.BuildSearchClause(search, productSearchFields, 5); clause != "" {
		where += " AND " + clause
		args = append(args, arg)
	}

	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       (SELECT pv.id FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE
		        ORDER BY pv.created_at ASC LIMIT 1) AS default_variant_id
		FROM products p` + productTranslationJoin + `
		WHERE ` + where + `
		ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]ProductWithMeta, 0)
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
		); err != nil {
			return nil, err
		}
		products = append(products, pm)
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
// search is optional; see List. categorySlug, when non-empty, restricts the
// result to products whose category matches the given slug.
func (s *ProductService) ListAll(ctx context.Context, locale, search, categorySlug string, limit, offset int) ([]Product, error) {
	key := fmt.Sprintf("shop:products:all:%s:%s:%s:%d:%d", locale, search, categorySlug, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
	}

	args := []any{locale, limit, offset}
	wheres := []string{}
	if categorySlug != "" {
		args = append(args, categorySlug)
		wheres = append(wheres, fmt.Sprintf(`p.category_id = (SELECT id FROM categories WHERE slug = $%d)`, len(args)))
	}
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		args = append(args, arg)
		wheres = append(wheres, clause)
	}
	query := productSelect
	if len(wheres) > 0 {
		query += ` WHERE ` + strings.Join(wheres, ` AND `)
	}
	query += ` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`
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
	kind := req.Kind
	if kind == "" {
		kind = "simple"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	surfaces := req.CompatibleSurfaces
	if surfaces == nil {
		surfaces = []string{}
	}

	var p Product
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO products (category_id, slug, name, excerpt, description, how_to_use, compatible_surfaces, status, kind)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, category_id, slug, name, excerpt, description, how_to_use, compatible_surfaces, status, kind, created_at, updated_at`,
		req.CategoryID, req.Slug, req.Name, req.Excerpt, req.Description, req.HowToUse, pq.Array(surfaces), req.Status, kind).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Excerpt, &p.Description, &p.HowToUse, pq.Array(&p.CompatibleSurfaces), &p.Status, &p.Kind, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	// Auto-create the default bundle variant so the product is immediately usable.
	if kind == "bundle" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_variants (product_id, sku, price, stock_qty)
			 VALUES ($1, $2, 0, 0)`,
			p.ID, defaultBundleSKU(p.ID)); err != nil {
			return nil, fmt.Errorf("auto-create bundle variant: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	s.cache.DeleteByPrefix(productPrefix)
	return &p, nil
}

func (s *ProductService) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	kind := req.Kind
	if kind == "" {
		kind = "simple"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingKind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, id).Scan(&existingKind); err != nil {
		return nil, err
	}

	// bundle → simple: clean up bundle-specific data so we don't leave orphans.
	// FK cascades handle cart_items (variant DELETE CASCADEs to carts) and
	// order_items.variant_id (SET NULL — order snapshots stay readable).
	if existingKind == "bundle" && kind == "simple" {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM bundle_items WHERE bundle_product_id = $1`, id); err != nil {
			return nil, fmt.Errorf("cleanup bundle_items: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM product_variants WHERE product_id = $1 AND sku LIKE 'BUNDLE-%'`, id); err != nil {
			return nil, fmt.Errorf("cleanup bundle variant: %w", err)
		}
	}

	// simple → bundle (or already bundle with no variants): enforce 1-variant rule.
	if kind == "bundle" {
		var count int
		_ = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM product_variants WHERE product_id = $1 AND is_active = TRUE`, id).
			Scan(&count)
		if count == 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO product_variants (product_id, sku, price, stock_qty)
				 VALUES ($1, $2, 0, 0)`,
				id, defaultBundleSKU(id)); err != nil {
				return nil, fmt.Errorf("auto-create bundle variant: %w", err)
			}
		}
	}

	surfaces := req.CompatibleSurfaces
	if surfaces == nil {
		surfaces = []string{}
	}

	var p Product
	if err := tx.QueryRowContext(ctx,
		`UPDATE products SET category_id=$2, slug=$3, name=$4, excerpt=$5, description=$6,
		                     how_to_use=$7, compatible_surfaces=$8, status=$9, kind=$10
		 WHERE id=$1
		 RETURNING id, category_id, slug, name, excerpt, description, how_to_use, compatible_surfaces, status, kind, created_at, updated_at`,
		id, req.CategoryID, req.Slug, req.Name, req.Excerpt, req.Description,
		req.HowToUse, pq.Array(surfaces), req.Status, kind).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Excerpt, &p.Description,
			&p.HowToUse, pq.Array(&p.CompatibleSurfaces), &p.Status, &p.Kind, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
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

// --- WooCommerce import helpers -----------------------------------------
// These methods exist only for the importer; regular CRUD does not touch
// wc_product_id / wc_variation_id. They keep the importer self-contained
// so its dedup model (WC ID is the stable key) doesn't leak into the rest
// of the shop service.

// GetIDByWCProductID resolves a WC product ID to its Gyeon UUID.
// Returns sql.ErrNoRows if no row is mapped to that WC ID.
func (s *ProductService) GetIDByWCProductID(ctx context.Context, wcID int) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM products WHERE wc_product_id = $1`, wcID).Scan(&id)
	return id, err
}

// UpsertWCProductRequest carries the WC-derived fields for an upsert.
// Kind defaults to "simple" when empty; pass "bundle" for WC Product
// Bundles imports so CreateWCProduct also seeds the auto-generated
// BUNDLE-* variant the rest of the system expects.
type UpsertWCProductRequest struct {
	WCProductID int
	CategoryID  *string
	Slug        string
	Name        string
	Excerpt     *string
	Description *string
	Status      string
	Kind        string
}

// CreateWCProduct does a plain INSERT for a brand-new WC import row. The
// caller must have verified (e.g. via GetIDByWCProductID) that no row
// with this wc_product_id exists yet — otherwise the unique constraint
// surfaces as an error.
//
// We deliberately do NOT use INSERT ... ON CONFLICT DO UPDATE here:
// PostgreSQL allocates a sequence value before checking the conflict,
// so an UPDATE-via-conflict burns a products.number (BIGSERIAL) every
// time, leaving large gaps in the human-readable PRD-{number} sequence
// after every re-import. Splitting INSERT and UPDATE keeps the sequence
// monotone-without-gaps for the upsert mode (replace mode is already
// gap-free because it deletes rows up front so no conflicts ever fire).
func (s *ProductService) CreateWCProduct(ctx context.Context, req UpsertWCProductRequest) (string, error) {
	kind := req.Kind
	if kind == "" {
		kind = "simple"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO products (wc_product_id, category_id, slug, name, excerpt, description, status, kind)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		req.WCProductID, req.CategoryID, req.Slug, req.Name, req.Excerpt, req.Description, req.Status, kind).Scan(&id); err != nil {
		return "", err
	}

	// Bundle products require the auto-generated BUNDLE-* variant the rest
	// of the system uses for cart/order linkage. Mirror Create()'s logic so
	// imported bundles are immediately usable.
	if kind == "bundle" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_variants (product_id, sku, price, stock_qty)
			 VALUES ($1, $2, 0, 0)`,
			id, defaultBundleSKU(id)); err != nil {
			return "", fmt.Errorf("auto-create bundle variant: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return id, nil
}

// UpdateWCProduct syncs WC-sourced fields onto an existing row. id /
// number are intentionally untouched so admin URLs (PRD-N) remain stable
// across re-imports. When the WC product type changes between runs (e.g.
// merchant converts simple → bundle in WC), this also handles the kind
// transition by mirroring Update()'s cleanup / variant-seeding logic.
func (s *ProductService) UpdateWCProduct(ctx context.Context, productID string, req UpsertWCProductRequest) error {
	kind := req.Kind
	if kind == "" {
		kind = "simple"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var existingKind string
	if err := tx.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, productID).Scan(&existingKind); err != nil {
		return err
	}

	// bundle → simple: drop bundle_items and the BUNDLE-* variant so the
	// product's invariants stay clean. The simple-fallback variant the
	// importer is about to upsert will fill the gap.
	if existingKind == "bundle" && kind != "bundle" {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM bundle_items WHERE bundle_product_id = $1`, productID); err != nil {
			return fmt.Errorf("cleanup bundle_items: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM product_variants WHERE product_id = $1 AND sku LIKE 'BUNDLE-%'`, productID); err != nil {
			return fmt.Errorf("cleanup bundle variant: %w", err)
		}
	}

	// non-bundle → bundle: seed the BUNDLE-* variant if one isn't already
	// present (idempotent for repeated bundle re-imports).
	if existingKind != "bundle" && kind == "bundle" {
		var count int
		_ = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM product_variants WHERE product_id = $1 AND sku LIKE 'BUNDLE-%'`, productID).
			Scan(&count)
		if count == 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO product_variants (product_id, sku, price, stock_qty)
				 VALUES ($1, $2, 0, 0)`,
				productID, defaultBundleSKU(productID)); err != nil {
				return fmt.Errorf("auto-create bundle variant: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE products
		    SET category_id = $2,
		        slug        = $3,
		        name        = $4,
		        excerpt     = $5,
		        description = $6,
		        status      = $7,
		        kind        = $8,
		        updated_at  = NOW()
		  WHERE id = $1`,
		productID, req.CategoryID, req.Slug, req.Name, req.Excerpt, req.Description, req.Status, kind); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

// UpsertWCVariantRequest carries the WC-derived fields for a variant upsert.
// WCVariationID is nil for the simple-product fallback variant (one per
// product, identified by wc_variation_id IS NULL).
type UpsertWCVariantRequest struct {
	WCVariationID  *int
	SKU            string
	Price          float64
	CompareAtPrice *float64
	StockQty       int
	WeightGrams    *int
	LengthMM       *int
	WidthMM        *int
	HeightMM       *int
}

// UpsertWCVariant inserts or updates a variant keyed by wc_variation_id
// (for variations) or by (product_id, wc_variation_id IS NULL) for the
// simple-product fallback. Returns the variant ID.
func (s *ProductService) UpsertWCVariant(ctx context.Context, productID string, req UpsertWCVariantRequest) (string, error) {
	if req.WCVariationID != nil {
		var id string
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO product_variants
			     (product_id, wc_variation_id, sku, price, compare_at_price, stock_qty, weight_grams, length_mm, width_mm, height_mm)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			 ON CONFLICT (wc_variation_id) DO UPDATE
			    SET product_id        = EXCLUDED.product_id,
			        sku               = EXCLUDED.sku,
			        price             = EXCLUDED.price,
			        compare_at_price  = EXCLUDED.compare_at_price,
			        stock_qty         = EXCLUDED.stock_qty,
			        weight_grams      = EXCLUDED.weight_grams,
			        length_mm         = EXCLUDED.length_mm,
			        width_mm          = EXCLUDED.width_mm,
			        height_mm         = EXCLUDED.height_mm,
			        updated_at        = NOW()
			 RETURNING id`,
			productID, *req.WCVariationID, req.SKU, req.Price,
			req.CompareAtPrice, req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM).Scan(&id)
		return id, err
	}

	// Simple-product fallback: look up existing (product_id, wc_variation_id IS NULL),
	// then UPDATE or INSERT. ON CONFLICT can't help here because there's no
	// matching unique index for the IS NULL predicate.
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM product_variants
		 WHERE product_id = $1 AND wc_variation_id IS NULL`, productID).Scan(&existing)
	switch {
	case err == sql.ErrNoRows:
		var id string
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO product_variants
			     (product_id, sku, price, compare_at_price, stock_qty, weight_grams, length_mm, width_mm, height_mm)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING id`,
			productID, req.SKU, req.Price, req.CompareAtPrice,
			req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM).Scan(&id)
		return id, err
	case err != nil:
		return "", err
	default:
		_, uerr := s.db.ExecContext(ctx,
			`UPDATE product_variants
			    SET sku=$2, price=$3, compare_at_price=$4,
			        stock_qty=$5, weight_grams=$6, length_mm=$7, width_mm=$8, height_mm=$9, updated_at=NOW()
			  WHERE id=$1`,
			existing, req.SKU, req.Price, req.CompareAtPrice,
			req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM)
		return existing, uerr
	}
}

// DeleteStaleWCProducts removes WC-imported products whose wc_product_id
// was not seen in the current import (i.e. the WC store no longer has
// them). Manually-created products (wc_product_id IS NULL) are never
// touched. Returns rows deleted.
//
// kind scopes the delete to one product kind ("simple" or "bundle"); pass
// empty to delete across kinds. The importer always runs one kind per
// invocation, so without this scope a "products" import would wipe every
// previously-imported bundle (their wc_product_ids weren't seen) and vice
// versa.
func (s *ProductService) DeleteStaleWCProducts(ctx context.Context, kind string, keepWCIDs []int) (int64, error) {
	q := `DELETE FROM products
	       WHERE wc_product_id IS NOT NULL
	         AND NOT (wc_product_id = ANY($1))`
	args := []any{pq.Array(keepWCIDs)}
	if kind != "" {
		q += ` AND kind = $2`
		args = append(args, kind)
	}
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return res.RowsAffected()
}

// DeleteStaleWCVariants removes WC-imported variants for one product whose
// wc_variation_id is not in keepWCIDs. Variants without a wc_variation_id
// (manually added by admin) are kept. Pass an empty slice to remove all
// WC-imported variants of the product (e.g. variable→simple conversion).
func (s *ProductService) DeleteStaleWCVariants(ctx context.Context, productID string, keepWCIDs []int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM product_variants
		  WHERE product_id = $1
		    AND wc_variation_id IS NOT NULL
		    AND NOT (wc_variation_id = ANY($2))`,
		productID, pq.Array(keepWCIDs))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteSimpleWCVariant removes the simple-product fallback variant of a
// product (wc_variation_id IS NULL) — used when a WC product converts from
// simple to variable. No-op if the product has none.
func (s *ProductService) DeleteSimpleWCVariant(ctx context.Context, productID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM product_variants
		  WHERE product_id = $1 AND wc_variation_id IS NULL`, productID)
	return err
}

// GetVariantIDByWCVariationID resolves a WC variation ID to its Gyeon
// variant UUID. Used by the bundle importer to link bundled_items that
// pin a specific variation. Returns sql.ErrNoRows if no variant matches —
// caller falls back to FindFirstActiveVariantID.
func (s *ProductService) GetVariantIDByWCVariationID(ctx context.Context, wcVariationID int) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM product_variants WHERE wc_variation_id = $1`, wcVariationID).Scan(&id)
	return id, err
}

// FindFirstActiveVariantID returns the lowest-id active variant for the
// given product. Used by the bundle importer when a bundled_item points
// at a variable component without specifying which variation, so we pick
// a deterministic default the admin can adjust later.
func (s *ProductService) FindFirstActiveVariantID(ctx context.Context, productID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM product_variants
		 WHERE product_id = $1 AND is_active = TRUE
		 ORDER BY created_at ASC, id ASC
		 LIMIT 1`, productID).Scan(&id)
	return id, err
}

// GetBundleVariantID returns the auto-generated BUNDLE-* variant ID for a
// bundle product so the importer can update its price (stock is derived).
func (s *ProductService) GetBundleVariantID(ctx context.Context, productID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM product_variants
		 WHERE product_id = $1 AND sku LIKE 'BUNDLE-%'
		 LIMIT 1`, productID).Scan(&id)
	return id, err
}

// UpdateBundleVariantPrice writes a fresh price onto the bundle's BUNDLE-*
// variant. Other fields (stock_qty, weight) are intentionally left alone:
// stock is derived from components and weight is irrelevant for a bundle.
func (s *ProductService) UpdateBundleVariantPrice(ctx context.Context, variantID string, price float64, compareAt *float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE product_variants
		    SET price = $2, compare_at_price = $3, updated_at = NOW()
		  WHERE id = $1`,
		variantID, price, compareAt)
	return err
}

// DeleteAllWCImported removes all WC-imported products. Manually-created
// products are kept. Used by the "replace" import mode.
//
// kind scopes the delete to one product kind; pass empty to wipe across
// kinds. See DeleteStaleWCProducts for the rationale — same isolation
// concern between "products" and "bundle products" import runs.
func (s *ProductService) DeleteAllWCImported(ctx context.Context, kind string) error {
	q := `DELETE FROM products WHERE wc_product_id IS NOT NULL`
	args := []any{}
	if kind != "" {
		q += ` AND kind = $1`
		args = append(args, kind)
	}
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

// DeleteWCSourcedImages removes product_images for the given product whose
// underlying media_files row was downloaded from WC (source_url IS NOT NULL).
// Admin-uploaded images survive. Used by the importer to refresh the WC
// image set on upsert without trampling manual additions.
func (s *ProductService) DeleteWCSourcedImages(ctx context.Context, productID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM product_images
		  WHERE product_id = $1
		    AND media_file_id IN (
		        SELECT id FROM media_files WHERE source_url IS NOT NULL
		    )`, productID)
	return err
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
	var productKind string
	err := s.db.QueryRowContext(ctx,
		`SELECT pv.id, pv.product_id, pv.sku, pv.name, pv.price, pv.compare_at_price,
		        pv.stock_qty, pv.low_stock_threshold, pv.weight_grams, pv.length_mm, pv.width_mm, pv.height_mm,
		        pv.is_active, pv.created_at, pv.updated_at,
		        p.name AS product_name, p.kind AS product_kind,
		        COALESCE(mf.url, pi.url) AS image_url
		 FROM product_variants pv
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN product_images pi
		     ON pi.product_id = pv.product_id AND pi.is_primary = TRUE
		 LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		 WHERE pv.id = $1
		 LIMIT 1`, variantID).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM,
			&v.IsActive, &v.CreatedAt, &v.UpdatedAt,
			&v.ProductName, &productKind, &v.ImageURL)
	if err != nil {
		return nil, err
	}
	// For bundle products, replace stock_qty with dynamically derived stock.
	if productKind == "bundle" {
		v.StockQty, _ = s.GetDerivedStock(ctx, v.ProductID)
	}
	return &v, nil
}

func (s *ProductService) ListVariants(ctx context.Context, productID string) ([]Variant, error) {
	// Determine product kind upfront so we can apply derived stock to bundles.
	var productKind string
	_ = s.db.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, productID).Scan(&productKind)

	rows, err := s.db.QueryContext(ctx,
		`SELECT pv.id, pv.product_id, pv.sku, pv.name, pv.price, pv.compare_at_price,
		        pv.stock_qty, pv.low_stock_threshold, pv.weight_grams, pv.length_mm, pv.width_mm, pv.height_mm,
		        pv.is_active, pv.created_at, pv.updated_at,
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
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM,
			&v.IsActive, &v.CreatedAt, &v.UpdatedAt, &v.ImageURL); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// For bundle products, replace stock_qty with derived stock.
	if productKind == "bundle" && len(variants) > 0 {
		derived, _ := s.GetDerivedStock(ctx, productID)
		for i := range variants {
			variants[i].StockQty = derived
		}
	}
	return variants, nil
}

func (s *ProductService) CreateVariant(ctx context.Context, productID string, req CreateVariantRequest) (*Variant, error) {
	// Enforce 1-variant rule for bundle products.
	var kind string
	if err := s.db.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, productID).Scan(&kind); err != nil {
		return nil, err
	}
	if kind == "bundle" {
		var count int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM product_variants WHERE product_id = $1 AND is_active = TRUE`, productID).
			Scan(&count); err != nil {
			return nil, err
		}
		if count >= 1 {
			return nil, fmt.Errorf("bundle products can only have one variant")
		}
	}

	var v Variant
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_variants (product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at`,
		productID, req.SKU, req.Name, req.Price, req.CompareAtPrice, req.StockQty, req.LowStockThreshold, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *ProductService) UpdateVariant(ctx context.Context, variantID string, req UpdateVariantRequest) (*Variant, error) {
	// Snapshot the prior stock so we can write inventory_history only when the
	// quantity actually changes (avoids spamming the log on pure metadata edits).
	var beforeQty int
	if err := s.db.QueryRowContext(ctx, `SELECT stock_qty FROM product_variants WHERE id = $1`, variantID).Scan(&beforeQty); err != nil {
		return nil, err
	}

	var v Variant
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_variants SET sku=$2, name=$3, price=$4, compare_at_price=$5, stock_qty=$6, low_stock_threshold=$7, weight_grams=$8, is_active=$9, length_mm=$10, width_mm=$11, height_mm=$12
		 WHERE id=$1
		 RETURNING id, product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at`,
		variantID, req.SKU, req.Name, req.Price, req.CompareAtPrice, req.StockQty, req.LowStockThreshold, req.WeightGrams, req.IsActive, req.LengthMM, req.WidthMM, req.HeightMM).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	recordStockChange(ctx, s.db, variantID, beforeQty, v.StockQty, "admin.variant_update", nil, nil)
	return &v, nil
}

func (s *ProductService) DeleteVariant(ctx context.Context, variantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM product_variants WHERE id = $1`, variantID)
	return err
}

func (s *ProductService) AdjustStock(ctx context.Context, variantID string, req AdjustStockRequest) (*Variant, error) {
	var beforeQty int
	if err := s.db.QueryRowContext(ctx, `SELECT stock_qty FROM product_variants WHERE id = $1`, variantID).Scan(&beforeQty); err != nil {
		return nil, err
	}

	var v Variant
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_variants SET stock_qty = GREATEST(0, stock_qty + $2)
		 WHERE id = $1
		 RETURNING id, product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at`,
		variantID, req.Delta).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	recordStockChange(ctx, s.db, variantID, beforeQty, v.StockQty, "admin.adjust", nil, nil)
	return &v, nil
}

func (s *ProductService) UpdateImage(ctx context.Context, imageID string, req UpdateImageRequest) (*ProductImage, error) {
	img, err := scanProductImage(s.db.QueryRowContext(ctx,
		`WITH unset_others AS (
		     UPDATE product_images SET is_primary = FALSE
		     WHERE product_id = (SELECT product_id FROM product_images WHERE id = $1)
		       AND id <> $1
		       AND COALESCE($4, FALSE) = TRUE
		 ),
		 upd AS (
		     UPDATE product_images
		     SET alt_text   = COALESCE($2, alt_text),
		         sort_order = COALESCE($3, sort_order),
		         is_primary = COALESCE($4, is_primary)
		     WHERE id=$1
		     RETURNING *
		 )
		 SELECT upd.id, upd.product_id, upd.variant_id, upd.media_file_id,
		        COALESCE(mf.url, upd.url, '') AS url,
		        mf.mime_type,
		        mf.thumbnail_url,
		        mf.video_autoplay,
		        mf.video_fit,
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
		`SELECT id, product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, is_active, created_at, updated_at
		 FROM product_variants
		 WHERE is_active = TRUE
		   AND stock_qty <= COALESCE(low_stock_threshold, $1)
		 ORDER BY stock_qty ASC`,
		threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func scanProductImage(row interface{ Scan(...any) error }) (ProductImage, error) {
	var img ProductImage
	var autoplay sql.NullBool
	var fit sql.NullString
	err := row.Scan(&img.ID, &img.ProductID, &img.VariantID, &img.MediaFileID,
		&img.URL, &img.MimeType, &img.ThumbnailURL, &autoplay, &fit, &img.AltText, &img.SortOrder, &img.IsPrimary, &img.CreatedAt)
	if autoplay.Valid {
		img.VideoAutoplay = autoplay.Bool
	}
	if fit.Valid {
		img.VideoFit = fit.String
	}
	return img, err
}

func (s *ProductService) ListImages(ctx context.Context, productID string) ([]ProductImage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pi.id, pi.product_id, pi.variant_id, pi.media_file_id,
		        COALESCE(mf.url, pi.url, '') AS url,
		        mf.mime_type,
		        mf.thumbnail_url,
		        mf.video_autoplay,
		        mf.video_fit,
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.backfillVideoThumbnails(images)
	return images, nil
}

// backfillVideoThumbnails kicks off thumbnail generation in the background for
// any video ProductImage whose backing media row is missing a thumbnail_url.
// Best-effort: errors are logged inside EnsureVideoThumbnail. Once persisted,
// the next request returns the thumbnail URL through the normal query path.
func (s *ProductService) backfillVideoThumbnails(imgs []ProductImage) {
	if s.thumbnail == nil {
		return
	}
	for _, img := range imgs {
		if img.MediaFileID == nil || img.MimeType == nil {
			continue
		}
		if !strings.HasPrefix(*img.MimeType, "video/") {
			continue
		}
		if img.ThumbnailURL != nil && *img.ThumbnailURL != "" {
			continue
		}
		mediaID := *img.MediaFileID
		go s.thumbnail.EnsureVideoThumbnail(context.Background(), mediaID)
	}
}

func (s *ProductService) AddImage(ctx context.Context, productID string, req AddImageRequest) (*ProductImage, error) {
	img, err := scanProductImage(s.db.QueryRowContext(ctx,
		`WITH unset_others AS (
		     UPDATE product_images SET is_primary = FALSE
		     WHERE product_id = $1 AND $7 = TRUE
		 ),
		 ins AS (
		     INSERT INTO product_images (product_id, variant_id, media_file_id, url, alt_text, sort_order, is_primary)
		     VALUES ($1, $2, $3, $4, $5, $6, $7)
		     RETURNING *
		 )
		 SELECT ins.id, ins.product_id, ins.variant_id, ins.media_file_id,
		        COALESCE(mf.url, ins.url, '') AS url,
		        mf.mime_type,
		        mf.thumbnail_url,
		        mf.video_autoplay,
		        mf.video_fit,
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

// --- Bundle product methods ---

// GetDerivedStock computes min(component.stock_qty / bundle_item.quantity) across
// all bundle_items for the given bundle_product_id. Returns 0 if no items exist.
func (s *ProductService) GetDerivedStock(ctx context.Context, productID string) (int, error) {
	var derived int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MIN(FLOOR(pv.stock_qty::float / bi.quantity)), 0)::int
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 WHERE bi.bundle_product_id = $1`, productID).Scan(&derived)
	return derived, err
}

// GetBundleItems returns all component rows for a bundle product, enriched with
// the component variant's product name, SKU, stock, and price.
func (s *ProductService) GetBundleItems(ctx context.Context, productID string) ([]BundleItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT bi.id, bi.bundle_product_id, bi.component_variant_id, bi.quantity,
		        bi.sort_order, bi.display_name_override,
		        p.name  AS component_product_name,
		        pv.name AS component_variant_name,
		        pv.sku  AS component_sku,
		        pv.stock_qty AS component_stock_qty,
		        pv.price     AS component_price,
		        bi.created_at
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 JOIN products p ON p.id = pv.product_id
		 WHERE bi.bundle_product_id = $1
		 ORDER BY bi.sort_order ASC, bi.created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]BundleItem, 0)
	for rows.Next() {
		var bi BundleItem
		if err := rows.Scan(
			&bi.ID, &bi.BundleProductID, &bi.ComponentVariantID, &bi.Quantity,
			&bi.SortOrder, &bi.DisplayNameOverride,
			&bi.ComponentProductName, &bi.ComponentVariantName, &bi.ComponentSKU,
			&bi.ComponentStockQty, &bi.ComponentPrice, &bi.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, bi)
	}
	return items, rows.Err()
}

// AddBundleItem upserts a single bundle item. If a row already exists for
// (bundle_product_id, component_variant_id), its quantity / sort_order /
// display_name_override are overwritten; otherwise a new row is inserted.
// Rejects nested bundles (component variants whose product is itself a bundle).
// Returns the resulting row enriched with component metadata, like GetBundleItems.
func (s *ProductService) AddBundleItem(ctx context.Context, productID string, input BundleItemInput) (*BundleItem, error) {
	var compKind string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.kind FROM product_variants pv JOIN products p ON p.id = pv.product_id WHERE pv.id = $1`,
		input.ComponentVariantID).Scan(&compKind)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("component variant %s not found", input.ComponentVariantID)
	}
	if err != nil {
		return nil, err
	}
	if compKind == "bundle" {
		return nil, fmt.Errorf("nested bundles are not allowed: component variant %s belongs to a bundle product", input.ComponentVariantID)
	}

	qty := input.Quantity
	if qty <= 0 {
		qty = 1
	}

	var id string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO bundle_items (bundle_product_id, component_variant_id, quantity, sort_order, display_name_override)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (bundle_product_id, component_variant_id) DO UPDATE
		   SET quantity              = EXCLUDED.quantity,
		       sort_order            = EXCLUDED.sort_order,
		       display_name_override = EXCLUDED.display_name_override
		 RETURNING id`,
		productID, input.ComponentVariantID, qty, input.SortOrder, input.DisplayNameOverride).Scan(&id)
	if err != nil {
		return nil, err
	}

	var bi BundleItem
	err = s.db.QueryRowContext(ctx,
		`SELECT bi.id, bi.bundle_product_id, bi.component_variant_id, bi.quantity,
		        bi.sort_order, bi.display_name_override,
		        p.name, pv.name, pv.sku, pv.stock_qty, pv.price, bi.created_at
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 JOIN products p ON p.id = pv.product_id
		 WHERE bi.id = $1`, id).Scan(
		&bi.ID, &bi.BundleProductID, &bi.ComponentVariantID, &bi.Quantity,
		&bi.SortOrder, &bi.DisplayNameOverride,
		&bi.ComponentProductName, &bi.ComponentVariantName, &bi.ComponentSKU,
		&bi.ComponentStockQty, &bi.ComponentPrice, &bi.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.cache.DeleteByPrefix(productPrefix)
	return &bi, nil
}

// RemoveBundleItem deletes a single bundle item by (bundle_product_id,
// component_variant_id). Returns errProductNotFound if no such row existed.
func (s *ProductService) RemoveBundleItem(ctx context.Context, productID, componentVariantID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM bundle_items
		 WHERE bundle_product_id = $1 AND component_variant_id = $2`,
		productID, componentVariantID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errProductNotFound
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

// SetBundleItems atomically replaces all bundle_items for a product.
// Validates that no component variant belongs to another bundle product (no nesting).
func (s *ProductService) SetBundleItems(ctx context.Context, productID string, inputs []BundleItemInput) ([]BundleItem, error) {
	// Validate: no nested bundles.
	for _, input := range inputs {
		var compKind string
		err := s.db.QueryRowContext(ctx,
			`SELECT p.kind FROM product_variants pv JOIN products p ON p.id = pv.product_id WHERE pv.id = $1`,
			input.ComponentVariantID).Scan(&compKind)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("component variant %s not found", input.ComponentVariantID)
		}
		if err != nil {
			return nil, err
		}
		if compKind == "bundle" {
			return nil, fmt.Errorf("nested bundles are not allowed: component variant %s belongs to a bundle product", input.ComponentVariantID)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Delete all existing items for this product.
	if _, err := tx.ExecContext(ctx, `DELETE FROM bundle_items WHERE bundle_product_id = $1`, productID); err != nil {
		return nil, err
	}

	// Insert new items.
	for _, input := range inputs {
		qty := input.Quantity
		if qty <= 0 {
			qty = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO bundle_items (bundle_product_id, component_variant_id, quantity, sort_order, display_name_override)
			 VALUES ($1, $2, $3, $4, $5)`,
			productID, input.ComponentVariantID, qty, input.SortOrder, input.DisplayNameOverride,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	s.cache.DeleteByPrefix(productPrefix)
	return s.GetBundleItems(ctx, productID)
}
