package shop

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/settings"
	"gyeon/backend/internal/util"
)

// RoleRulesProvider returns the category IDs a storefront role isn't allowed
// to see, list, or purchase. Implemented by categoryrules.Service — kept as
// an interface here so the shop package doesn't depend directly on
// categoryrules (the wiring graph is shop → categoryrules-by-interface, set
// from main.go).
//
// "List" sits between "View" and "Purchase": a category may be unlisted
// (hidden from the public catalog and search) yet still purchasable via a
// direct PDP link — the per-role replacement for the old global
// hidden_category_ids setting (removed in migration 103). The returned slice
// for BlockedListCategoryIDs already includes everything BlockedViewCategoryIDs
// would return, so listing endpoints only need to filter on the listed set.
type RoleRulesProvider interface {
	BlockedViewCategoryIDs(ctx context.Context, role string) []string
	BlockedListCategoryIDs(ctx context.Context, role string) []string
	BlockedPurchaseCategoryIDs(ctx context.Context, role string) []string
}

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

// Purchasable on Product reflects whether the current request's storefront
// role is allowed to add this product to cart. False when at least one of
// the product's categories is in the role's blocked-purchase set. Annotated
// in Go after the SQL query (see annotateProductsPurchasable); defaults to
// true for any path where roleRules is not wired (admin reads, tests).
type Product struct {
	ID         string  `json:"id"`
	Number     int64   `json:"number"`
	CategoryID *string `json:"category_id,omitempty"`
	// CategoryIDs is the full set of categories the product belongs to
	// (including the primary CategoryID, when set). Populated on single-item
	// reads; nil/empty on list endpoints where we don't fan out per-row.
	CategoryIDs        []string `json:"category_ids,omitempty"`
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Subtitle           *string  `json:"subtitle,omitempty"`
	Excerpt            *string  `json:"excerpt,omitempty"`
	Description        *string  `json:"description,omitempty"`
	HowToUse           *string  `json:"how_to_use,omitempty"`
	CompatibleSurfaces []string `json:"compatible_surfaces"`
	// WCSku mirrors the original WooCommerce product SKU captured at import.
	// Separate from any generated value; nil for manually-created products.
	WCSku *string `json:"wc_sku,omitempty"`
	// Hero video + banner / media strip slots. Video is a YouTube ID
	// (rendered as an embed). Banner / media slots point at media_files
	// rows; the Banner*URL / Media*URL fields are hydrated only by
	// single-product reads (GetBySlug / GetByID) — they stay nil on list
	// queries so we don't pay 6 LEFT JOINs per row.
	VideoID        *string `json:"video_id,omitempty"`
	Banner1MediaID *string `json:"banner_1_media_id,omitempty"`
	Banner2MediaID *string `json:"banner_2_media_id,omitempty"`
	Media1MediaID  *string `json:"media_1_media_id,omitempty"`
	Media2MediaID  *string `json:"media_2_media_id,omitempty"`
	Media3MediaID  *string `json:"media_3_media_id,omitempty"`
	Media4MediaID  *string `json:"media_4_media_id,omitempty"`
	Banner1URL     *string `json:"banner_1_url,omitempty"`
	Banner1WebpURL *string `json:"banner_1_webp_url,omitempty"`
	Banner2URL     *string `json:"banner_2_url,omitempty"`
	Banner2WebpURL *string `json:"banner_2_webp_url,omitempty"`
	Media1URL      *string `json:"media_1_url,omitempty"`
	Media1WebpURL  *string `json:"media_1_webp_url,omitempty"`
	Media2URL      *string `json:"media_2_url,omitempty"`
	Media2WebpURL  *string `json:"media_2_webp_url,omitempty"`
	Media3URL      *string `json:"media_3_url,omitempty"`
	Media3WebpURL  *string `json:"media_3_webp_url,omitempty"`
	Media4URL      *string `json:"media_4_url,omitempty"`
	Media4WebpURL  *string `json:"media_4_webp_url,omitempty"`
	Status         string  `json:"status"`
	Kind           string  `json:"kind"` // "simple" | "bundle"
	Purchasable    bool    `json:"purchasable"`
	// UseTaobaoLayout overrides the site-wide `pdp_taobao_layout_enabled`
	// flag for this single product: nil = follow site default,
	// true = force taobao modal layout, false = force classic layout.
	UseTaobaoLayout *bool  `json:"use_taobao_layout,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// ProductWithMeta enriches Product with quick-glance fields useful for list
// views (MCP catalog browsing, agent decision-making) so callers don't need
// an N+1 follow-up GET per product.
type ProductWithMeta struct {
	Product
	VariantCount                 int      `json:"variant_count"`
	PrimaryImageURL              *string  `json:"primary_image_url,omitempty"`
	DefaultVariantID             *string  `json:"default_variant_id,omitempty"`
	DefaultVariantPrice          *float64 `json:"default_variant_price,omitempty"`
	DefaultVariantCompareAtPrice *float64 `json:"default_variant_compare_at_price,omitempty"`
	DefaultVariantStockQty       *int     `json:"default_variant_stock_qty,omitempty"`
	DefaultVariantName           *string  `json:"default_variant_name,omitempty"`
	MinPrice                     *float64 `json:"min_price,omitempty"`
	MinPriceCompareAt            *float64 `json:"min_compare_at_price,omitempty"`
	MinPriceStock                *int     `json:"min_price_stock_qty,omitempty"`
}

// BundleItem represents a component row in a bundle product.
type BundleItem struct {
	ID                  string  `json:"id"`
	BundleProductID     string  `json:"bundle_product_id"`
	ComponentVariantID  string  `json:"component_variant_id"`
	Quantity            int     `json:"quantity"`
	SortOrder           int     `json:"sort_order"`
	DisplayNameOverride *string `json:"display_name_override,omitempty"`
	// Derived from joined tables
	ComponentProductName     string  `json:"component_product_name"`
	ComponentProductSlug     *string `json:"component_product_slug,omitempty"`
	ComponentProductSubtitle *string `json:"component_product_subtitle,omitempty"`
	ComponentVariantName     *string `json:"component_variant_name,omitempty"`
	ComponentSKU             string  `json:"component_sku"`
	ComponentStockQty        int     `json:"component_stock_qty"`
	ComponentPrice           float64 `json:"component_price"`
	ComponentPrimaryImageURL *string `json:"component_primary_image_url,omitempty"`
	CreatedAt                string  `json:"created_at"`
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

// PromoBundle is a bundle product curated as "優惠套裝" under a parent
// product in the taobao-layout PDP modal. The bundle product is itself a
// kind='bundle' row in `products`; this struct flattens the join with its
// auto-created variant so the storefront can render price/CTA without an
// extra round-trip.
type PromoBundle struct {
	ID              string   `json:"id"`
	ParentProductID string   `json:"parent_product_id"`
	BundleProductID string   `json:"bundle_product_id"`
	SortOrder       int      `json:"sort_order"`
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Excerpt         *string  `json:"excerpt,omitempty"`
	Status          string   `json:"status"`
	VariantID       string   `json:"variant_id"`
	Price           float64  `json:"price"`
	CompareAtPrice  *float64 `json:"compare_at_price,omitempty"`
	StockQty        int      `json:"stock_qty"`
	PrimaryImageURL *string  `json:"primary_image_url,omitempty"`
	CreatedAt       string   `json:"created_at"`
	// Purchasable mirrors the per-(role, category) gate stamped on Product /
	// ProductWithMeta by annotate{Single,Meta}Purchasable. Defaults true and
	// flips false when the bundle product itself sits in a blocked category
	// for the current storefront role. Lets the taobao popup disable the row
	// instead of silently 403-ing on cart-add.
	Purchasable bool `json:"purchasable"`
}

// SetPromoBundlesRequest wraps the ordered list of bundle product IDs to
// associate with a parent product.
type SetPromoBundlesRequest struct {
	BundleProductIDs []string `json:"bundle_product_ids"`
}

// SetUpsellsRequest / SetCrossSellsRequest wrap the ordered list of target
// product IDs for the admin up-sell / cross-sell editors.
type SetUpsellsRequest struct {
	UpsellProductIDs []string `json:"upsell_product_ids"`
}

type SetCrossSellsRequest struct {
	CrossSellProductIDs []string `json:"cross_sell_product_ids"`
}

type ProductTranslation struct {
	Locale      string  `json:"locale"`
	Name        string  `json:"name"`
	Subtitle    *string `json:"subtitle,omitempty"`
	Description *string `json:"description,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

type UpsertProductTranslationRequest struct {
	Name        string  `json:"name"`
	Subtitle    *string `json:"subtitle"`
	Description *string `json:"description"`
}

type Variant struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	// WCSku mirrors the original WooCommerce variant SKU captured at import.
	// Separate from the generated SKU; nil for manually-created variants.
	WCSku             *string  `json:"wc_sku,omitempty"`
	Name              *string  `json:"name,omitempty"`
	Price             float64  `json:"price"`
	CompareAtPrice    *float64 `json:"compare_at_price,omitempty"`
	StockQty          int      `json:"stock_qty"`
	LowStockThreshold *int     `json:"low_stock_threshold,omitempty"`
	WeightGrams       *int     `json:"weight_grams,omitempty"`
	LengthMM          *int     `json:"length_mm,omitempty"`
	WidthMM           *int     `json:"width_mm,omitempty"`
	HeightMM          *int     `json:"height_mm,omitempty"`
	IsActive          bool     `json:"is_active"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	ProductName       *string  `json:"product_name,omitempty"`
	ImageURL          *string  `json:"image_url,omitempty"`
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
	CategoryID *string `json:"category_id"`
	// CategoryIDs is the full set of categories (additional + primary). The
	// primary CategoryID is auto-included by the sync, so callers can send
	// just the extras or the full set — either works.
	CategoryIDs        []string `json:"category_ids"`
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Subtitle           *string  `json:"subtitle"`
	Excerpt            *string  `json:"excerpt"`
	Description        *string  `json:"description"`
	HowToUse           *string  `json:"how_to_use"`
	CompatibleSurfaces []string `json:"compatible_surfaces"`
	WCSku              *string  `json:"wc_sku"`
	VideoID            *string  `json:"video_id"`
	Banner1MediaID     *string  `json:"banner_1_media_id"`
	Banner2MediaID     *string  `json:"banner_2_media_id"`
	Media1MediaID      *string  `json:"media_1_media_id"`
	Media2MediaID      *string  `json:"media_2_media_id"`
	Media3MediaID      *string  `json:"media_3_media_id"`
	Media4MediaID      *string  `json:"media_4_media_id"`
	Status             string   `json:"status"`
	Kind               string   `json:"kind"` // "simple" | "bundle"; defaults to "simple"
	UseTaobaoLayout    *bool    `json:"use_taobao_layout"`
}

type UpdateProductRequest struct {
	CreateProductRequest
}

type CreateVariantRequest struct {
	SKU               string   `json:"sku"`
	WCSku             *string  `json:"wc_sku"`
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
	WCSku             *string  `json:"wc_sku"`
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
	       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
	       p.excerpt,
	       COALESCE(t.description, p.description) AS description,
	       p.how_to_use, p.compatible_surfaces,
	       p.video_id,
	       p.banner_1_media_id, p.banner_2_media_id,
	       p.media_1_media_id, p.media_2_media_id, p.media_3_media_id, p.media_4_media_id,
	       p.wc_sku,
	       p.status, p.kind, p.use_taobao_layout, p.created_at, p.updated_at
	FROM products p` + productTranslationJoin

func scanProduct(row interface{ Scan(...any) error }) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Number, &p.CategoryID, &p.Slug, &p.Name, &p.Subtitle,
		&p.Excerpt, &p.Description, &p.HowToUse, pq.Array(&p.CompatibleSurfaces),
		&p.VideoID,
		&p.Banner1MediaID, &p.Banner2MediaID,
		&p.Media1MediaID, &p.Media2MediaID, &p.Media3MediaID, &p.Media4MediaID,
		&p.WCSku,
		&p.Status, &p.Kind, &p.UseTaobaoLayout, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// hydrateMediaURLs fills in the Banner*URL / Media*URL fields by looking
// up the referenced media_files rows in one query. No-op when every slot
// is nil. Errors are non-fatal — leaving the URL fields nil just means
// the storefront skips rendering that slot.
func (s *ProductService) hydrateMediaURLs(ctx context.Context, p *Product) {
	type slot struct {
		id   *string
		url  **string
		webp **string
	}
	slots := []slot{
		{p.Banner1MediaID, &p.Banner1URL, &p.Banner1WebpURL},
		{p.Banner2MediaID, &p.Banner2URL, &p.Banner2WebpURL},
		{p.Media1MediaID, &p.Media1URL, &p.Media1WebpURL},
		{p.Media2MediaID, &p.Media2URL, &p.Media2WebpURL},
		{p.Media3MediaID, &p.Media3URL, &p.Media3WebpURL},
		{p.Media4MediaID, &p.Media4URL, &p.Media4WebpURL},
	}
	ids := make([]string, 0, len(slots))
	for _, sl := range slots {
		if sl.id != nil {
			ids = append(ids, *sl.id)
		}
	}
	if len(ids) == 0 {
		return
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, webp_url FROM media_files WHERE id = ANY($1::uuid[])`,
		pq.Array(ids))
	if err != nil {
		return
	}
	defer rows.Close()
	urls := make(map[string]struct {
		url  string
		webp *string
	}, len(ids))
	for rows.Next() {
		var id, url string
		var webp *string
		if err := rows.Scan(&id, &url, &webp); err != nil {
			continue
		}
		urls[id] = struct {
			url  string
			webp *string
		}{url, webp}
	}
	for _, sl := range slots {
		if sl.id == nil {
			continue
		}
		if entry, ok := urls[*sl.id]; ok {
			u := entry.url
			*sl.url = &u
			*sl.webp = entry.webp
		}
	}
}

const productPrefix = "shop:products:"

// ThumbnailEnsurer is satisfied by media.Handler. Used to lazily backfill
// first-frame thumbnails for video media that pre-date the thumbnail feature.
// Optional dependency: when nil, backfill is skipped (tests/seed paths stay simple).
type ThumbnailEnsurer interface {
	EnsureVideoThumbnail(ctx context.Context, mediaFileID string)
}

// AuditRecorder is the minimal interface this service needs from the audit
// package. Decoupled to avoid an import cycle.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry)
}

type AuditEntry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

type ProductService struct {
	db          *sql.DB
	cache       cache.Store
	ttl         func(context.Context) time.Duration
	thumbnail   ThumbnailEnsurer
	settingsSvc *settings.Service
	audit       AuditRecorder
	roleRules   RoleRulesProvider
}

// SetRoleRules wires in the per-role category visibility rules. Optional —
// nil keeps the listing queries role-agnostic (same as before the feature
// was added). Call from main during setup.
func (s *ProductService) SetRoleRules(p RoleRulesProvider) { s.roleRules = p }

// InvalidateRoleScopedCaches drops every cache entry whose key encodes a
// storefront role (Product per-role on GetBySlug, FBT per-role on the FBT
// cache). Called from main.go after the categoryrules service commits a
// new ruleset — without this, the role-keyed entries keep their old
// Purchasable annotation until each entry's TTL expires.
func (s *ProductService) InvalidateRoleScopedCaches() {
	s.cache.DeleteByPrefix(productPrefix)
	s.cache.DeleteByPrefix(fbtCachePrefix)
}

// appendRoleListedFilter appends a NOT EXISTS clause that hides any product
// linked (via product_category_links) to a category the storefront role
// isn't allowed to see in listings — i.e. unlisted ("private link")
// categories plus the view-blocked set (BlockedListCategoryIDs is a superset
// of BlockedViewCategoryIDs). Role is read from request context, defaulting
// to "customer" for anonymous visitors. No-op when roleRules is not wired
// or the role has no blocked categories.
//
// This is the per-role replacement for the pre-migration-103 setup, where a
// global appendHiddenCategoryFilter (using p.category_id IN (...) over the
// hidden_category_ids site setting) ran in parallel with a separate
// appendRoleVisibilityFilter. Now a single filter expresses both — and it
// checks product_category_links so multi-category products are filtered
// consistently (the old hidden-id filter only looked at p.category_id).
//
// Returns the (possibly-extended) wheres + args plus a cache scope string
// that includes the role + blocked-id set, so two roles never share a
// cached page.
func (s *ProductService) appendRoleListedFilter(ctx context.Context, wheres []string, args []any) ([]string, []any, string) {
	if s.roleRules == nil {
		return wheres, args, ""
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedListCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return wheres, args, "role:" + role
	}
	args = append(args, pq.Array(blocked))
	wheres = append(wheres, fmt.Sprintf(
		`NOT EXISTS (SELECT 1 FROM product_category_links pcl
		             WHERE pcl.product_id = p.id
		               AND pcl.category_id = ANY($%d::uuid[]))`, len(args)))
	return wheres, args, "role:" + role + ":" + strings.Join(blocked, ",")
}

// annotatePurchasableMeta sets Purchasable on each row of a ProductWithMeta
// slice. Implemented as a single batched lookup against product_category_links
// so a list page costs one extra round-trip regardless of how many products
// it returns. When the role has no blocked-purchase rules (the common case
// out of the box), the function just stamps true on every row without
// touching the DB. This must run before caching so cached entries already
// reflect role state — the cache key includes the role, so different roles
// won't share an entry.
func (s *ProductService) annotatePurchasableMeta(ctx context.Context, products []ProductWithMeta) {
	for i := range products {
		products[i].Purchasable = true
	}
	if s.roleRules == nil || len(products) == 0 {
		return
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedPurchaseCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return
	}
	ids := make([]string, len(products))
	idx := make(map[string]int, len(products))
	for i, p := range products {
		ids[i] = p.ID
		idx[p.ID] = i
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT product_id::text
		 FROM product_category_links
		 WHERE product_id = ANY($1::uuid[]) AND category_id = ANY($2::uuid[])`,
		pq.Array(ids), pq.Array(blocked))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if i, ok := idx[id]; ok {
			products[i].Purchasable = false
		}
	}
}

// annotatePurchasable is the []Product counterpart to annotatePurchasableMeta.
// Two variants exist because Product and ProductWithMeta don't share a useful
// supertype the caller could pass through.
func (s *ProductService) annotatePurchasable(ctx context.Context, products []Product) {
	for i := range products {
		products[i].Purchasable = true
	}
	if s.roleRules == nil || len(products) == 0 {
		return
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedPurchaseCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return
	}
	ids := make([]string, len(products))
	idx := make(map[string]int, len(products))
	for i, p := range products {
		ids[i] = p.ID
		idx[p.ID] = i
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT product_id::text
		 FROM product_category_links
		 WHERE product_id = ANY($1::uuid[]) AND category_id = ANY($2::uuid[])`,
		pq.Array(ids), pq.Array(blocked))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if i, ok := idx[id]; ok {
			products[i].Purchasable = false
		}
	}
}

// annotateSinglePurchasable sets Purchasable on a single product. Used by
// PDP paths (GetBySlug). Defaults to true and only flips when at least one
// of the product's categories is in the role's blocked-purchase set.
func (s *ProductService) annotateSinglePurchasable(ctx context.Context, p *Product) {
	p.Purchasable = true
	if s.roleRules == nil {
		return
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedPurchaseCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return
	}
	var hit int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_category_links
		 WHERE product_id = $1::uuid AND category_id = ANY($2::uuid[])`,
		p.ID, pq.Array(blocked)).Scan(&hit); err != nil {
		return
	}
	if hit > 0 {
		p.Purchasable = false
	}
}

// roleBlocksProductCategories returns true when at least one of the
// product's categories is in the role's blocked-view set. Used by
// GetBySlug / GetByID to 404 a direct-URL PDP hit for a hidden product —
// the public list filter alone isn't enough since direct links bypass it.
func (s *ProductService) roleBlocksProductCategories(ctx context.Context, productID string) bool {
	if s.roleRules == nil {
		return false
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedViewCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return false
	}
	var hit int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_category_links
		 WHERE product_id = $1::uuid AND category_id = ANY($2::uuid[])`,
		productID, pq.Array(blocked)).Scan(&hit); err != nil {
		return false
	}
	return hit > 0
}

func NewProductService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration, settingsSvc *settings.Service) *ProductService {
	return &ProductService{db: db, cache: c, ttl: ttl, settingsSvc: settingsSvc}
}

// DB returns the underlying database handle so adjacent packages (e.g.
// catalog matchers used by the orders CSV importer) can run raw queries
// without duplicating connection wiring.
func (s *ProductService) DB() *sql.DB { return s.db }

// SetAudit wires an optional audit recorder. Call from main during setup.
func (s *ProductService) SetAudit(rec AuditRecorder) { s.audit = rec }

func (s *ProductService) record(ctx context.Context, action, entityType, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: entityType, EntityID: entityID,
		Before: before, After: after,
	})
}

// getVariant fetches a single variant by ID. Used as a before-snapshot for
// audit on update/delete.
func (s *ProductService) getVariant(ctx context.Context, variantID string) (*Variant, error) {
	var v Variant
	err := s.db.QueryRowContext(ctx,
		`SELECT id, product_id, sku, wc_sku, name, price, compare_at_price, stock_qty, low_stock_threshold,
		        weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at
		 FROM product_variants WHERE id=$1`, variantID).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.WCSku, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM,
			&v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// getImage fetches a single product image by ID. Used as a before-snapshot
// for audit on update/delete.
func (s *ProductService) getImage(ctx context.Context, imageID string) (*ProductImage, error) {
	img, err := scanProductImage(s.db.QueryRowContext(ctx,
		`SELECT pi.id, pi.product_id, pi.variant_id, pi.media_file_id,
		        COALESCE(mf.url, pi.url, '') AS url,
		        mf.mime_type, mf.thumbnail_url, mf.video_autoplay, mf.video_fit,
		        pi.alt_text, pi.sort_order, pi.is_primary, pi.created_at
		 FROM product_images pi
		 LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		 WHERE pi.id = $1`, imageID))
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// listVariantIDs returns the current variant IDs for a product in their
// stored sort order. Used as a before-snapshot for ReorderVariants.
func (s *ProductService) listVariantIDs(ctx context.Context, productID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM product_variants WHERE product_id=$1 ORDER BY sort_order ASC, created_at ASC`,
		productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// getProductTranslation fetches a single product translation. Used as a
// before-snapshot for audit on upsert/delete.
func (s *ProductService) getProductTranslation(ctx context.Context, productID, locale string) (*ProductTranslation, error) {
	var t ProductTranslation
	err := s.db.QueryRowContext(ctx,
		`SELECT locale, name, subtitle, description, updated_at
		 FROM product_translations WHERE product_id=$1 AND locale=$2`, productID, locale).
		Scan(&t.Locale, &t.Name, &t.Subtitle, &t.Description, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// roleListedScope returns the (blocked-list-ids, cache-scope-string) tuple
// for the current request's storefront role. Used by call sites that need
// the blocked-id set as a slice (FBT pool helpers feed it into SQL ANY
// arrays) rather than via appendRoleListedFilter's WHERE injection.
//
// The cache scope is "role:<name>:<sorted-ids-csv>" matching what the
// appender returns, so cache keys built from this stay in sync with keys
// built from appendRoleListedFilter.
func (s *ProductService) roleListedScope(ctx context.Context) ([]string, string) {
	role := auth.CustomerRoleFromContext(ctx)
	if s.roleRules == nil {
		return nil, "role:" + role
	}
	blocked := s.roleRules.BlockedListCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return nil, "role:" + role
	}
	return blocked, "role:" + role + ":" + strings.Join(blocked, ",")
}

// fbtExcludedCategoryIDs reads the fbt_excluded_category_slugs site setting,
// resolves the slugs to category UUIDs in a single SELECT, and returns
// (ids, rawSettingValue). Used by FBT pool queries to drop products linked
// to service-only categories (coating, ppf-film, installers, …) via
// product_category_links — different from the per-role listed filter which
// scopes by storefront role. Returns empty + "" on any error so a
// misconfigured setting just stops excluding rather than breaking FBT.
func (s *ProductService) fbtExcludedCategoryIDs(ctx context.Context) ([]string, string) {
	if s.settingsSvc == nil {
		return nil, ""
	}
	st, err := s.settingsSvc.Get(ctx, "fbt_excluded_category_slugs")
	if err != nil || st == nil || strings.TrimSpace(st.Value) == "" {
		return nil, ""
	}
	var slugs []string
	if err := json.Unmarshal([]byte(st.Value), &slugs); err != nil {
		return nil, ""
	}
	cleaned := make([]string, 0, len(slugs))
	for _, sl := range slugs {
		sl = strings.TrimSpace(sl)
		if sl != "" {
			cleaned = append(cleaned, sl)
		}
	}
	if len(cleaned) == 0 {
		return nil, st.Value
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM categories WHERE slug = ANY($1::text[]) AND is_active = TRUE`,
		pq.Array(cleaned))
	if err != nil {
		return nil, st.Value
	}
	defer rows.Close()
	ids := make([]string, 0, len(cleaned))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, st.Value
		}
		ids = append(ids, id)
	}
	return ids, st.Value
}

// SetThumbnailEnsurer wires in the media-side helper for lazy video thumbnail
// backfill. Call from app wiring after both services exist.
func (s *ProductService) SetThumbnailEnsurer(t ThumbnailEnsurer) {
	s.thumbnail = t
}

// syncCategoryLinks rewrites product_category_links for productID to match
// finalSet = unique(primary + extras). Primary may be nil; extras may be
// empty. Idempotent. Caller supplies the tx so this composes with Create /
// Update inside their existing transactions.
func (s *ProductService) syncCategoryLinks(ctx context.Context, tx *sql.Tx, productID string, primary *string, extras []string) error {
	final := make(map[string]struct{}, len(extras)+1)
	if primary != nil && *primary != "" {
		final[*primary] = struct{}{}
	}
	for _, id := range extras {
		if id != "" {
			final[id] = struct{}{}
		}
	}
	ids := make([]string, 0, len(final))
	for id := range final {
		ids = append(ids, id)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM product_category_links
		 WHERE product_id = $1 AND NOT (category_id = ANY($2::uuid[]))`,
		productID, pq.Array(ids)); err != nil {
		return fmt.Errorf("delete stale category links: %w", err)
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_category_links (product_id, category_id)
			 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			productID, id); err != nil {
			return fmt.Errorf("insert category link: %w", err)
		}
	}
	return nil
}

// loadCategoryIDs returns the full set of categories linked to productID.
// Returns an empty slice (never nil) so callers can attach directly to the
// JSON response field without a nil check.
func (s *ProductService) loadCategoryIDs(ctx context.Context, productID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT category_id::text FROM product_category_links WHERE product_id = $1 ORDER BY created_at`,
		productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// List returns active products. locale may be empty for base content.
// search is an optional case-insensitive substring matched against
// productSearchFields; pass "" to disable.
func (s *ProductService) List(ctx context.Context, locale, search string, limit, offset int) ([]Product, error) {
	args := []any{locale, limit, offset}
	wheres := []string{"p.status = 'active'"} // public: active only
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		wheres = append(wheres, clause)
		args = append(args, arg)
	}
	var roleScope string
	wheres, args, roleScope = s.appendRoleListedFilter(ctx, wheres, args)

	key := fmt.Sprintf("shop:products:pub:%s:%s:%d:%d:%s", locale, search, limit, offset, roleScope)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
	}
	rows, err := s.db.QueryContext(ctx,
		productSelect+` WHERE `+strings.Join(wheres, " AND ")+` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`,
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
	s.annotatePurchasable(ctx, products)
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
func (s *ProductService) ListEnrichedFiltered(ctx context.Context, f ListFilters) ([]ProductWithMeta, int, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	args := []any{f.Locale, f.Limit, f.Offset}
	wheres := []string{"p.status = 'active'"}

	if f.CategorySlug != "" {
		args = append(args, f.CategorySlug)
		wheres = append(wheres, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM product_category_links pcl
			         JOIN categories c ON c.id = pcl.category_id
			         WHERE pcl.product_id = p.id AND c.slug = $%d AND c.is_active = TRUE)`,
			len(args)))
	}
	if clause, arg := util.BuildSearchClause(f.Search, productSearchFields, len(args)+1); clause != "" {
		wheres = append(wheres, clause)
		args = append(args, arg)
	}

	const minPriceSQ = "(SELECT MIN(pv.price) FROM product_variants pv WHERE pv.product_id = p.id AND pv.is_active = TRUE)"
	if f.MinPrice != nil {
		args = append(args, *f.MinPrice)
		wheres = append(wheres, fmt.Sprintf("%s >= $%d", minPriceSQ, len(args)))
	}
	if f.MaxPrice != nil {
		args = append(args, *f.MaxPrice)
		wheres = append(wheres, fmt.Sprintf("%s <= $%d", minPriceSQ, len(args)))
	}
	wheres, args, _ = s.appendRoleListedFilter(ctx, wheres, args)
	where := strings.Join(wheres, " AND ")

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
		       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       defv.id   AS default_variant_id,
		       defv.price AS default_variant_price,
		       defv.compare_at_price AS default_variant_compare_at_price,
		       defv.stock_qty AS default_variant_stock_qty,
		       defv.name AS default_variant_name,
		       cheapest.price AS min_price,
		       cheapest.compare_at_price AS min_compare_at_price,
		       cheapest.stock_qty AS min_price_stock_qty,
		       COUNT(*) OVER() AS total_count
		FROM products p` + productTranslationJoin + `
		LEFT JOIN LATERAL (
		    SELECT pv.price, pv.compare_at_price, pv.stock_qty
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.price ASC
		    LIMIT 1
		) cheapest ON TRUE
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty, pv.name
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE
		WHERE ` + where + `
		ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products := make([]ProductWithMeta, 0)
	total := 0
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name, &pm.Subtitle,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
			&pm.DefaultVariantPrice, &pm.DefaultVariantCompareAtPrice,
			&pm.DefaultVariantStockQty, &pm.DefaultVariantName,
			&pm.MinPrice, &pm.MinPriceCompareAt, &pm.MinPriceStock,
			&total,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	s.overrideBundleStock(ctx, products)
	s.annotatePurchasableMeta(ctx, products)
	return products, total, nil
}

// overrideBundleStock replaces the stock fields for bundle products with the
// derived stock (min over components/quantity). The list SQL surfaces the
// bundle's synthetic variant stock which is not kept in sync with components,
// so both DefaultVariantStockQty and MinPriceStock need patching — the latter
// is what storefront cards key the "sold out" badge off of.
func (s *ProductService) overrideBundleStock(ctx context.Context, products []ProductWithMeta) {
	for i := range products {
		if products[i].Kind != "bundle" {
			continue
		}
		derived, err := s.GetDerivedStock(ctx, products[i].ID)
		if err != nil {
			continue
		}
		products[i].DefaultVariantStockQty = &derived
		products[i].MinPriceStock = &derived
	}
}

// ListEnriched returns active products plus variant_count, primary_image_url
// and default_variant_id in a single round-trip. Used by the MCP list_products
// tool so agents can act on a result without N+1 follow-ups (notably: bundles
// expose their single BUNDLE-* variant id directly, ready for add_to_cart).
func (s *ProductService) ListEnriched(ctx context.Context, locale, search string, limit, offset int) ([]ProductWithMeta, error) {
	args := []any{locale, limit, offset}
	wheres := []string{"p.status = 'active'"}
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		wheres = append(wheres, clause)
		args = append(args, arg)
	}
	var roleScope string
	wheres, args, roleScope = s.appendRoleListedFilter(ctx, wheres, args)
	where := strings.Join(wheres, " AND ")

	key := fmt.Sprintf("shop:products:pubmeta:%s:%s:%d:%d:%s", locale, search, limit, offset, roleScope)
	if v, ok := s.cache.Get(key); ok {
		return v.([]ProductWithMeta), nil
	}

	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       defv.id   AS default_variant_id,
		       defv.price AS default_variant_price,
		       defv.compare_at_price AS default_variant_compare_at_price,
		       defv.stock_qty AS default_variant_stock_qty,
		       defv.name AS default_variant_name
		FROM products p` + productTranslationJoin + `
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty, pv.name
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE
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
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name, &pm.Subtitle,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
			&pm.DefaultVariantPrice, &pm.DefaultVariantCompareAtPrice,
			&pm.DefaultVariantStockQty, &pm.DefaultVariantName,
		); err != nil {
			return nil, err
		}
		products = append(products, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.overrideBundleStock(ctx, products)
	s.annotatePurchasableMeta(ctx, products)
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListEnrichedByCategorySlug is the category-filtered counterpart of
// ListEnriched. Kept in lockstep so the REST list endpoint exposes the
// same shape regardless of the `?category=` filter.
func (s *ProductService) ListEnrichedByCategorySlug(ctx context.Context, locale, categorySlug, search string, limit, offset int) ([]ProductWithMeta, error) {
	args := []any{locale, limit, offset, categorySlug}
	wheres := []string{
		"p.status = 'active'",
		`EXISTS (SELECT 1 FROM product_category_links pcl
		         JOIN categories c ON c.id = pcl.category_id
		         WHERE pcl.product_id = p.id AND c.slug = $4 AND c.is_active = TRUE)`,
	}
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		wheres = append(wheres, clause)
		args = append(args, arg)
	}
	var roleScope string
	wheres, args, roleScope = s.appendRoleListedFilter(ctx, wheres, args)
	where := strings.Join(wheres, " AND ")

	key := fmt.Sprintf("shop:products:bycatmeta:%s:%s:%s:%d:%d:%s", locale, categorySlug, search, limit, offset, roleScope)
	if v, ok := s.cache.Get(key); ok {
		return v.([]ProductWithMeta), nil
	}

	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       defv.id   AS default_variant_id,
		       defv.price AS default_variant_price,
		       defv.compare_at_price AS default_variant_compare_at_price,
		       defv.stock_qty AS default_variant_stock_qty,
		       defv.name AS default_variant_name
		FROM products p` + productTranslationJoin + `
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty, pv.name
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE
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
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name, &pm.Subtitle,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
			&pm.DefaultVariantPrice, &pm.DefaultVariantCompareAtPrice,
			&pm.DefaultVariantStockQty, &pm.DefaultVariantName,
		); err != nil {
			return nil, err
		}
		products = append(products, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.overrideBundleStock(ctx, products)
	s.annotatePurchasableMeta(ctx, products)
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListByCategorySlug returns active products filtered to a single category
// (resolved from its slug). locale and search behave like List.
func (s *ProductService) ListByCategorySlug(ctx context.Context, locale, categorySlug, search string, limit, offset int) ([]Product, error) {
	args := []any{locale, limit, offset, categorySlug}
	wheres := []string{
		"p.status = 'active'",
		`EXISTS (SELECT 1 FROM product_category_links pcl
		         JOIN categories c ON c.id = pcl.category_id
		         WHERE pcl.product_id = p.id AND c.slug = $4 AND c.is_active = TRUE)`,
	}
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		wheres = append(wheres, clause)
		args = append(args, arg)
	}
	var roleScope string
	wheres, args, roleScope = s.appendRoleListedFilter(ctx, wheres, args)
	where := strings.Join(wheres, " AND ")

	key := fmt.Sprintf("shop:products:bycat:%s:%s:%s:%d:%d:%s", locale, categorySlug, search, limit, offset, roleScope)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Product), nil
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
	s.annotatePurchasable(ctx, products)
	s.cache.Set(key, products, s.ttl(ctx))
	return products, nil
}

// ListAll returns all products regardless of is_active (admin). locale may be empty.
// search is optional; see List. categorySlug, when non-empty, restricts the
// result to products whose category matches the given slug.
// adminListPage caches a page of admin product results plus the total
// row count so the handler can return both without a second roundtrip.
type adminListPage struct {
	Items []ProductWithMeta
	Total int
}

func (s *ProductService) ListAll(ctx context.Context, locale, search, categorySlug, kind, stockState, sort string, limit, offset int) ([]ProductWithMeta, int, error) {
	key := fmt.Sprintf("shop:products:all:%s:%s:%s:%s:%s:%s:%d:%d", locale, search, categorySlug, kind, stockState, sort, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		page := v.(adminListPage)
		return page.Items, page.Total, nil
	}

	args := []any{locale, limit, offset}
	wheres := []string{}
	if categorySlug != "" {
		args = append(args, categorySlug)
		wheres = append(wheres, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM product_category_links pcl
			         JOIN categories c ON c.id = pcl.category_id
			         WHERE pcl.product_id = p.id AND c.slug = $%d)`,
			len(args)))
	}
	if clause, arg := util.BuildSearchClause(search, productSearchFields, len(args)+1); clause != "" {
		args = append(args, arg)
		wheres = append(wheres, clause)
	}
	if kind != "" {
		args = append(args, kind)
		wheres = append(wheres, fmt.Sprintf("p.kind = $%d", len(args)))
	}
	// Stock-state filter operates on the default variant's stock_qty (the
	// same number the admin Stock column displays). Bundle products have
	// synthetic stock here — overrideBundleStock() corrects display values
	// after the fetch, but filtering happens pre-override, so bundle stock
	// filtering is approximate (acceptable for now; v1 trade-off).
	switch stockState {
	case "in_stock":
		wheres = append(wheres, `defv.stock_qty > 0`)
	case "low_stock":
		wheres = append(wheres, `defv.stock_qty > 0 AND defv.stock_qty < 5`)
	case "out_of_stock":
		wheres = append(wheres, `COALESCE(defv.stock_qty, 0) = 0`)
	}
	where := ""
	if len(wheres) > 0 {
		where = ` WHERE ` + strings.Join(wheres, ` AND `)
	}
	// Sort clause — secondary sort by p.id keeps pagination stable when the
	// primary column has ties. price/stock use cheapest.price + defv.stock_qty
	// (consistent with the columns the UI shows) and NULLS LAST so products
	// without active variants sink to the bottom.
	orderBy := ""
	switch sort {
	case "updated_asc":
		orderBy = ` ORDER BY p.updated_at ASC, p.id ASC`
	case "created_desc":
		orderBy = ` ORDER BY p.created_at DESC, p.id DESC`
	case "created_asc":
		orderBy = ` ORDER BY p.created_at ASC, p.id ASC`
	case "name_asc":
		orderBy = ` ORDER BY name ASC, p.id ASC`
	case "name_desc":
		orderBy = ` ORDER BY name DESC, p.id DESC`
	case "price_asc":
		orderBy = ` ORDER BY cheapest.price ASC NULLS LAST, p.id ASC`
	case "price_desc":
		orderBy = ` ORDER BY cheapest.price DESC NULLS LAST, p.id DESC`
	case "stock_asc":
		orderBy = ` ORDER BY defv.stock_qty ASC NULLS LAST, p.id ASC`
	case "stock_desc":
		orderBy = ` ORDER BY defv.stock_qty DESC NULLS LAST, p.id DESC`
	default: // "updated_desc" + empty
		orderBy = ` ORDER BY p.updated_at DESC, p.id DESC`
	}
	// `variant_count` and `total` are computed inline so the admin list
	// page renders without N+1 follow-ups: previously the SvelteKit
	// loader fired one /variants call per row just to display a count,
	// and the handler had no way to surface the matching-row total for
	// pagination.
	//
	// `primary_image_url`, `default_variant_*` and `min_price*` mirror the
	// public List query so admin search pickers (e.g. the new-order page)
	// can show a thumbnail + price next to each result without fanning out
	// one /variants request per row.
	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       defv.id   AS default_variant_id,
		       defv.price AS default_variant_price,
		       defv.compare_at_price AS default_variant_compare_at_price,
		       defv.stock_qty AS default_variant_stock_qty,
		       defv.name AS default_variant_name,
		       cheapest.price AS min_price,
		       cheapest.compare_at_price AS min_compare_at_price,
		       cheapest.stock_qty AS min_price_stock_qty,
		       COUNT(*) OVER () AS total_rows
		FROM products p` + productTranslationJoin + `
		LEFT JOIN LATERAL (
		    SELECT pv.price, pv.compare_at_price, pv.stock_qty
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.price ASC
		    LIMIT 1
		) cheapest ON TRUE
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty, pv.name
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE` + where + orderBy +
		` LIMIT $2 OFFSET $3`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	products := make([]ProductWithMeta, 0)
	total := 0
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name, &pm.Subtitle,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL,
			&pm.DefaultVariantID, &pm.DefaultVariantPrice, &pm.DefaultVariantCompareAtPrice,
			&pm.DefaultVariantStockQty, &pm.DefaultVariantName,
			&pm.MinPrice, &pm.MinPriceCompareAt, &pm.MinPriceStock,
			&total,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// When the page is empty (e.g. offset past the last row) the window
	// function never runs and `total` stays 0 — fall back to a dedicated
	// COUNT so the UI's "Page X of N" math stays sane. Reuse `args` so
	// the parameter numbering in `where` keeps matching ($1=locale even
	// though we drop the translation join here, $2/$3 are unused).
	// The defv LATERAL join is included so a stock-state WHERE clause
	// referencing defv.stock_qty resolves; the planner prunes it
	// otherwise.
	if len(products) == 0 && (offset > 0 || search != "" || categorySlug != "" || kind != "" || stockState != "") {
		countQuery := `SELECT COUNT(*) FROM products p` + productTranslationJoin + `
			LEFT JOIN LATERAL (
			    SELECT pv.id, pv.stock_qty
			    FROM product_variants pv
			    WHERE pv.product_id = p.id AND pv.is_active = TRUE
			    ORDER BY pv.sort_order ASC, pv.created_at ASC
			    LIMIT 1
			) defv ON TRUE` + where
		_ = s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	}
	// Bundles store a synthetic variant stock that isn't kept in sync with
	// the underlying components — derive the real stock from components so
	// the admin search shows the same "in stock" picture as the storefront.
	s.overrideBundleStock(ctx, products)
	s.cache.Set(key, adminListPage{Items: products, Total: total}, s.ttl(ctx))
	return products, total, nil
}

// GetBySlug fetches a single active product by its slug. Bypasses the
// hidden-category filter on purpose — direct URLs (and "private link" sales
// flows) need to keep working even when the product's category is hidden
// from the public listing. Returns sql.ErrNoRows when the slug doesn't
// match an active product.
func (s *ProductService) GetBySlug(ctx context.Context, slug, locale string) (*Product, error) {
	role := auth.CustomerRoleFromContext(ctx)
	key := fmt.Sprintf("shop:products:slug:%s:%s:%s", slug, locale, role)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Product)
		return &p, nil
	}
	p, err := scanProduct(s.db.QueryRowContext(ctx,
		productSelect+` WHERE p.slug = $2 AND p.status = 'active'`, locale, slug))
	if err != nil {
		return nil, err
	}
	// Role can_view applies to direct PDP hits too — surfacing a product
	// through a deep link that the role isn't allowed to see at all would
	// be a confusing dead end. The role "listed" dimension (is_listed=FALSE,
	// i.e. "private link") is intentionally NOT enforced here: the whole
	// point of an unlisted category is that the PDP-by-slug keeps working
	// for direct-link sales. Only can_view=FALSE 404s the PDP.
	if s.roleBlocksProductCategories(ctx, p.ID) {
		return nil, sql.ErrNoRows
	}
	if ids, err := s.loadCategoryIDs(ctx, p.ID); err == nil {
		p.CategoryIDs = ids
	}
	s.hydrateMediaURLs(ctx, &p)
	s.annotateSinglePurchasable(ctx, &p)
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
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
	if ids, err := s.loadCategoryIDs(ctx, p.ID); err == nil {
		p.CategoryIDs = ids
	}
	s.hydrateMediaURLs(ctx, &p)
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
		`INSERT INTO products (category_id, slug, name, subtitle, excerpt, description, how_to_use, compatible_surfaces,
		                       video_id, banner_1_media_id, banner_2_media_id,
		                       media_1_media_id, media_2_media_id, media_3_media_id, media_4_media_id,
		                       status, kind, use_taobao_layout, wc_sku)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		 RETURNING id, category_id, slug, name, subtitle, excerpt, description, how_to_use, compatible_surfaces,
		           video_id, banner_1_media_id, banner_2_media_id,
		           media_1_media_id, media_2_media_id, media_3_media_id, media_4_media_id,
		           status, kind, use_taobao_layout, wc_sku, created_at, updated_at`,
		req.CategoryID, req.Slug, req.Name, req.Subtitle, req.Excerpt, req.Description, req.HowToUse, pq.Array(surfaces),
		req.VideoID, req.Banner1MediaID, req.Banner2MediaID,
		req.Media1MediaID, req.Media2MediaID, req.Media3MediaID, req.Media4MediaID,
		req.Status, kind, req.UseTaobaoLayout, req.WCSku).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Subtitle, &p.Excerpt, &p.Description, &p.HowToUse, pq.Array(&p.CompatibleSurfaces),
			&p.VideoID, &p.Banner1MediaID, &p.Banner2MediaID,
			&p.Media1MediaID, &p.Media2MediaID, &p.Media3MediaID, &p.Media4MediaID,
			&p.Status, &p.Kind, &p.UseTaobaoLayout, &p.WCSku, &p.CreatedAt, &p.UpdatedAt); err != nil {
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

	if err := s.syncCategoryLinks(ctx, tx, p.ID, req.CategoryID, req.CategoryIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	p.CategoryIDs, _ = s.loadCategoryIDs(ctx, p.ID)
	s.cache.DeleteByPrefix(productPrefix)
	s.record(ctx, "product.create", "product", p.ID, nil, p)
	return &p, nil
}

func (s *ProductService) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	kind := req.Kind
	if kind == "" {
		kind = "simple"
	}

	var before *Product
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id, ""); err == nil {
			before = prev
		}
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
		`UPDATE products SET category_id=$2, slug=$3, name=$4, subtitle=$5, excerpt=$6, description=$7,
		                     how_to_use=$8, compatible_surfaces=$9,
		                     video_id=$10, banner_1_media_id=$11, banner_2_media_id=$12,
		                     media_1_media_id=$13, media_2_media_id=$14, media_3_media_id=$15, media_4_media_id=$16,
		                     status=$17, kind=$18, use_taobao_layout=$19, wc_sku=$20
		 WHERE id=$1
		 RETURNING id, category_id, slug, name, subtitle, excerpt, description, how_to_use, compatible_surfaces,
		           video_id, banner_1_media_id, banner_2_media_id,
		           media_1_media_id, media_2_media_id, media_3_media_id, media_4_media_id,
		           status, kind, use_taobao_layout, wc_sku, created_at, updated_at`,
		id, req.CategoryID, req.Slug, req.Name, req.Subtitle, req.Excerpt, req.Description,
		req.HowToUse, pq.Array(surfaces),
		req.VideoID, req.Banner1MediaID, req.Banner2MediaID,
		req.Media1MediaID, req.Media2MediaID, req.Media3MediaID, req.Media4MediaID,
		req.Status, kind, req.UseTaobaoLayout, req.WCSku).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.Subtitle, &p.Excerpt, &p.Description,
			&p.HowToUse, pq.Array(&p.CompatibleSurfaces),
			&p.VideoID, &p.Banner1MediaID, &p.Banner2MediaID,
			&p.Media1MediaID, &p.Media2MediaID, &p.Media3MediaID, &p.Media4MediaID,
			&p.Status, &p.Kind, &p.UseTaobaoLayout, &p.WCSku, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}

	if err := s.syncCategoryLinks(ctx, tx, p.ID, req.CategoryID, req.CategoryIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	p.CategoryIDs, _ = s.loadCategoryIDs(ctx, p.ID)
	s.cache.DeleteByPrefix(productPrefix)
	s.record(ctx, "product.update", "product", p.ID, before, p)
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
	var before *Product
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id, ""); err == nil {
			before = prev
		}
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	s.record(ctx, "product.delete", "product", id, before, nil)
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
//
// CategoryID is the primary category (also written to products.category_id).
// CategoryIDs is the full set of WC-derived categories to link into
// product_category_links; the primary should be included. Link writes are
// additive (INSERT ... ON CONFLICT DO NOTHING) so admin-added extras and
// previously-imported categories that are no longer in WC are preserved.
type UpsertWCProductRequest struct {
	WCProductID    int
	CategoryID     *string
	CategoryIDs    []string
	Slug           string
	Name           string
	Subtitle       *string
	Excerpt        *string
	Description    *string
	HowToUse       *string
	WCSku          *string
	VideoID        *string
	Banner1MediaID *string
	Banner2MediaID *string
	Media1MediaID  *string
	Media2MediaID  *string
	Media3MediaID  *string
	Media4MediaID  *string
	Status         string
	Kind           string
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
		`INSERT INTO products (wc_product_id, category_id, slug, name, subtitle, excerpt, description, how_to_use, wc_sku,
		                       video_id, banner_1_media_id, banner_2_media_id,
		                       media_1_media_id, media_2_media_id, media_3_media_id, media_4_media_id,
		                       status, kind)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 RETURNING id`,
		req.WCProductID, req.CategoryID, req.Slug, req.Name, req.Subtitle, req.Excerpt, req.Description, req.HowToUse, req.WCSku,
		req.VideoID, req.Banner1MediaID, req.Banner2MediaID,
		req.Media1MediaID, req.Media2MediaID, req.Media3MediaID, req.Media4MediaID,
		req.Status, kind).Scan(&id); err != nil {
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

	// Link every WC-derived category (primary + extras). Additive only —
	// admin-added extras and previously-imported links that WC no longer
	// reports are preserved.
	if err := linkWCCategories(ctx, tx, id, req.CategoryID, req.CategoryIDs); err != nil {
		return "", err
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
		    SET category_id        = $2,
		        slug               = $3,
		        name               = $4,
		        subtitle           = $5,
		        excerpt            = $6,
		        description        = $7,
		        how_to_use         = $8,
		        video_id           = $9,
		        banner_1_media_id  = $10,
		        banner_2_media_id  = $11,
		        media_1_media_id   = $12,
		        media_2_media_id   = $13,
		        media_3_media_id   = $14,
		        media_4_media_id   = $15,
		        status             = $16,
		        kind               = $17,
		        wc_sku             = $18,
		        updated_at         = NOW()
		  WHERE id = $1`,
		productID, req.CategoryID, req.Slug, req.Name, req.Subtitle, req.Excerpt, req.Description, req.HowToUse,
		req.VideoID, req.Banner1MediaID, req.Banner2MediaID,
		req.Media1MediaID, req.Media2MediaID, req.Media3MediaID, req.Media4MediaID,
		req.Status, kind, req.WCSku); err != nil {
		return err
	}

	// Link every WC-derived category (primary + extras). Additive only —
	// admin-added extras and previously-imported links that WC no longer
	// reports are preserved.
	if err := linkWCCategories(ctx, tx, productID, req.CategoryID, req.CategoryIDs); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.DeleteByPrefix(productPrefix)
	return nil
}

// linkWCCategories upserts product_category_links for productID with primary
// + extras. Both inputs may be empty; nothing happens if the combined set is
// empty. Idempotent (INSERT ... ON CONFLICT DO NOTHING) and additive — never
// deletes existing links, so admin-added categories survive re-imports.
func linkWCCategories(ctx context.Context, tx *sql.Tx, productID string, primary *string, extras []string) error {
	seen := make(map[string]struct{}, len(extras)+1)
	add := func(id string) error {
		if id == "" {
			return nil
		}
		if _, dup := seen[id]; dup {
			return nil
		}
		seen[id] = struct{}{}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_category_links (product_id, category_id)
			 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			productID, id); err != nil {
			return fmt.Errorf("link category %s: %w", id, err)
		}
		return nil
	}
	if primary != nil {
		if err := add(*primary); err != nil {
			return err
		}
	}
	for _, id := range extras {
		if err := add(id); err != nil {
			return err
		}
	}
	return nil
}

// UpsertWCVariantRequest carries the WC-derived fields for a variant upsert.
// WCVariationID is nil for the simple-product fallback variant (one per
// product, identified by wc_variation_id IS NULL).
type UpsertWCVariantRequest struct {
	WCVariationID  *int
	SKU            string
	WCSku          *string
	Name           *string
	Price          float64
	CompareAtPrice *float64
	StockQty       int
	WeightGrams    *int
	LengthMM       *int
	WidthMM        *int
	HeightMM       *int
	IsActive       bool
}

// UpsertWCVariant inserts or updates a variant keyed by wc_variation_id
// (for variations) or by (product_id, wc_variation_id IS NULL) for the
// simple-product fallback. Returns the variant ID.
func (s *ProductService) UpsertWCVariant(ctx context.Context, productID string, req UpsertWCVariantRequest) (string, error) {
	if req.WCVariationID != nil {
		var id string
		err := s.db.QueryRowContext(ctx,
			`INSERT INTO product_variants
			     (product_id, wc_variation_id, sku, name, price, compare_at_price, stock_qty, weight_grams, length_mm, width_mm, height_mm, is_active, wc_sku)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			 ON CONFLICT (wc_variation_id) DO UPDATE
			    SET product_id        = EXCLUDED.product_id,
			        sku               = EXCLUDED.sku,
			        name              = EXCLUDED.name,
			        price             = EXCLUDED.price,
			        compare_at_price  = EXCLUDED.compare_at_price,
			        stock_qty         = EXCLUDED.stock_qty,
			        weight_grams      = EXCLUDED.weight_grams,
			        length_mm         = EXCLUDED.length_mm,
			        width_mm          = EXCLUDED.width_mm,
			        height_mm         = EXCLUDED.height_mm,
			        is_active         = EXCLUDED.is_active,
			        wc_sku            = EXCLUDED.wc_sku,
			        updated_at        = NOW()
			 RETURNING id`,
			productID, *req.WCVariationID, req.SKU, req.Name, req.Price,
			req.CompareAtPrice, req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM, req.IsActive, req.WCSku).Scan(&id)
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
			     (product_id, sku, name, price, compare_at_price, stock_qty, weight_grams, length_mm, width_mm, height_mm, is_active, wc_sku)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 RETURNING id`,
			productID, req.SKU, req.Name, req.Price, req.CompareAtPrice,
			req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM, req.IsActive, req.WCSku).Scan(&id)
		return id, err
	case err != nil:
		return "", err
	default:
		_, uerr := s.db.ExecContext(ctx,
			`UPDATE product_variants
			    SET sku=$2, name=$3, price=$4, compare_at_price=$5,
			        stock_qty=$6, weight_grams=$7, length_mm=$8, width_mm=$9, height_mm=$10, is_active=$11, wc_sku=$12, updated_at=NOW()
			  WHERE id=$1`,
			existing, req.SKU, req.Name, req.Price, req.CompareAtPrice,
			req.StockQty, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM, req.IsActive, req.WCSku)
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

// GetVariantIDByBundleRef resolves a WC bundled_item to a Gyeon variant
// using SKU suffix as the primary signal (importer writes
// "{product_slug}-{wcVariationID}") and wc_variation_id column as backup
// for legacy rows where the column is NULL. Scoped to one product so a
// stray same-suffix SKU on another product can't cross-match.
func (s *ProductService) GetVariantIDByBundleRef(
	ctx context.Context,
	productID string,
	wcVariationID int,
) (string, error) {
	var id string
	// Cast both $2 usages: lib/pq announces one type per parameter, so a
	// mix of `$2::text` and bare `$2` against an int column would fail with
	// `operator does not exist: integer = text`. Explicit casts on both
	// sides let the driver choose any type.
	err := s.db.QueryRowContext(ctx, `
		SELECT pv.id
		  FROM product_variants pv
		  JOIN products p ON p.id = pv.product_id
		 WHERE pv.product_id = $1
		   AND (
		       pv.sku = p.slug || '-' || $2::text
		       OR pv.wc_variation_id = $2::int
		   )
		 ORDER BY (pv.sku = p.slug || '-' || $2::text) DESC
		 LIMIT 1`,
		productID, wcVariationID).Scan(&id)
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

// DeleteImagesForVariants removes every product_images row tagged with one
// of the given variant IDs. Used by the importer to mirror WC: when a WC
// variation has no own image, the Gyeon variant must have no image. Keyed
// on variant_id only — does NOT filter by media linkage, so it also clears
// rows with media_file_id IS NULL that DeleteWCSourcedImages would miss.
// Media file rows and files on disk are not touched. No-op on empty input.
func (s *ProductService) DeleteImagesForVariants(ctx context.Context, variantIDs []string) error {
	if len(variantIDs) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM product_images WHERE variant_id = ANY($1)`,
		pq.Array(variantIDs))
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

// ListVariants returns variants for a product. When includeInactive is false
// (storefront use), only is_active = TRUE rows are returned. The admin PDP
// passes true so it can render and re-enable disabled variants — without that,
// flipping a variant's status to inactive would make the row vanish from the
// admin UI on the next refresh.
func (s *ProductService) ListVariants(ctx context.Context, productID string, includeInactive bool) ([]Variant, error) {
	// Determine product kind upfront so we can apply derived stock to bundles.
	var productKind string
	_ = s.db.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, productID).Scan(&productKind)

	activeFilter := "AND pv.is_active = TRUE"
	if includeInactive {
		activeFilter = ""
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT pv.id, pv.product_id, pv.sku, pv.wc_sku, pv.name, pv.price, pv.compare_at_price,
		        pv.stock_qty, pv.low_stock_threshold, pv.weight_grams, pv.length_mm, pv.width_mm, pv.height_mm,
		        pv.is_active, pv.created_at, pv.updated_at,
		        COALESCE(mf.url, pi.url) AS image_url
		 FROM product_variants pv
		 LEFT JOIN product_images pi ON pi.variant_id = pv.id
		 LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		 WHERE pv.product_id = $1 `+activeFilter+`
		 ORDER BY pv.sort_order ASC, pv.created_at ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.WCSku, &v.Name, &v.Price, &v.CompareAtPrice,
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
		`INSERT INTO product_variants (product_id, sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, wc_sku)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, product_id, sku, wc_sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at`,
		productID, req.SKU, req.Name, req.Price, req.CompareAtPrice, req.StockQty, req.LowStockThreshold, req.WeightGrams, req.LengthMM, req.WidthMM, req.HeightMM, req.WCSku).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.WCSku, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "product.variant.create", "product_variant", v.ID, nil, v)
	return &v, nil
}

func (s *ProductService) UpdateVariant(ctx context.Context, variantID string, req UpdateVariantRequest) (*Variant, error) {
	// Snapshot the prior stock so we can write inventory_history only when the
	// quantity actually changes (avoids spamming the log on pure metadata edits).
	var beforeQty int
	if err := s.db.QueryRowContext(ctx, `SELECT stock_qty FROM product_variants WHERE id = $1`, variantID).Scan(&beforeQty); err != nil {
		return nil, err
	}
	var before *Variant
	if s.audit != nil {
		if prev, err := s.getVariant(ctx, variantID); err == nil {
			before = prev
		}
	}

	var v Variant
	err := s.db.QueryRowContext(ctx,
		`UPDATE product_variants SET sku=$2, name=$3, price=$4, compare_at_price=$5, stock_qty=$6, low_stock_threshold=$7, weight_grams=$8, is_active=$9, length_mm=$10, width_mm=$11, height_mm=$12, wc_sku=$13
		 WHERE id=$1
		 RETURNING id, product_id, sku, wc_sku, name, price, compare_at_price, stock_qty, low_stock_threshold, weight_grams, length_mm, width_mm, height_mm, is_active, created_at, updated_at`,
		variantID, req.SKU, req.Name, req.Price, req.CompareAtPrice, req.StockQty, req.LowStockThreshold, req.WeightGrams, req.IsActive, req.LengthMM, req.WidthMM, req.HeightMM, req.WCSku).
		Scan(&v.ID, &v.ProductID, &v.SKU, &v.WCSku, &v.Name, &v.Price, &v.CompareAtPrice,
			&v.StockQty, &v.LowStockThreshold, &v.WeightGrams, &v.LengthMM, &v.WidthMM, &v.HeightMM, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	recordStockChange(ctx, s.db, variantID, beforeQty, v.StockQty, "admin.variant_update", nil, nil)
	s.record(ctx, "product.variant.update", "product_variant", v.ID, before, v)
	return &v, nil
}

func (s *ProductService) DeleteVariant(ctx context.Context, variantID string) error {
	var before *Variant
	if s.audit != nil {
		if prev, err := s.getVariant(ctx, variantID); err == nil {
			before = prev
		}
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM product_variants WHERE id = $1`, variantID)
	if err != nil {
		return err
	}
	s.record(ctx, "product.variant.delete", "product_variant", variantID, before, nil)
	return nil
}

// ReorderVariants rewrites product_variants.sort_order for the given
// productID so that variantIDs[i] receives sort_order = i. IDs that don't
// belong to productID are silently ignored (scoped by the WHERE clause), so
// stale browser state can't reorder another product's variants.
func (s *ProductService) ReorderVariants(ctx context.Context, productID string, variantIDs []string) error {
	if len(variantIDs) == 0 {
		return nil
	}
	var beforeIDs []string
	if s.audit != nil {
		beforeIDs, _ = s.listVariantIDs(ctx, productID)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE product_variants AS pv
		    SET sort_order = data.sort_order
		   FROM (
		       SELECT unnest($2::uuid[])                          AS id,
		              generate_subscripts($2::uuid[], 1)::int      AS sort_order
		   ) AS data
		  WHERE pv.id = data.id AND pv.product_id = $1`,
		productID, pq.Array(variantIDs))
	if err != nil {
		return err
	}
	s.record(ctx, "product.variant.reorder", "product", productID,
		map[string]any{"variant_ids": beforeIDs},
		map[string]any{"variant_ids": variantIDs})
	return nil
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
	s.record(ctx, "product.variant.adjust_stock", "product_variant", v.ID,
		map[string]any{"stock_qty": beforeQty},
		map[string]any{"stock_qty": v.StockQty, "delta": req.Delta, "note": req.Note})
	return &v, nil
}

func (s *ProductService) UpdateImage(ctx context.Context, imageID string, req UpdateImageRequest) (*ProductImage, error) {
	var before *ProductImage
	if s.audit != nil {
		if prev, err := s.getImage(ctx, imageID); err == nil {
			before = prev
		}
	}
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
	s.record(ctx, "product.image.update", "product_image", img.ID, before, img)
	return &img, nil
}

func (s *ProductService) DeleteImage(ctx context.Context, imageID string) error {
	var before *ProductImage
	if s.audit != nil {
		if prev, err := s.getImage(ctx, imageID); err == nil {
			before = prev
		}
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM product_images WHERE id = $1`, imageID)
	if err != nil {
		return err
	}
	s.record(ctx, "product.image.delete", "product_image", imageID, before, nil)
	return nil
}

func (s *ProductService) LowStock(ctx context.Context, threshold int) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx,
		// Exclude bundle products: a bundle holds no inventory of its own (its
		// availability is derived from components via overrideBundleStock, not the
		// stored stock_qty), so its raw stock is a meaningless placeholder here.
		`SELECT pv.id, pv.product_id, pv.sku, pv.name, pv.price, pv.compare_at_price, pv.stock_qty, pv.low_stock_threshold, pv.weight_grams, pv.is_active, pv.created_at, pv.updated_at
		 FROM product_variants pv
		 JOIN products p ON p.id = pv.product_id
		 WHERE pv.is_active = TRUE
		   AND p.kind <> 'bundle'
		   AND pv.stock_qty <= COALESCE(pv.low_stock_threshold, $1)
		 ORDER BY pv.stock_qty ASC`,
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
		        COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.url
		                 ELSE mf.webp_url END,
		            mf.url, pi.url, '') AS url,
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
	s.record(ctx, "product.image.create", "product_image", img.ID, nil, img)
	return &img, nil
}

// --- Translation management ---

func (s *ProductService) ListTranslations(ctx context.Context, productID string) ([]ProductTranslation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT locale, name, subtitle, description, updated_at
		 FROM product_translations WHERE product_id = $1 ORDER BY locale`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProductTranslation, 0)
	for rows.Next() {
		var t ProductTranslation
		if err := rows.Scan(&t.Locale, &t.Name, &t.Subtitle, &t.Description, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *ProductService) UpsertTranslation(ctx context.Context, productID, locale string, req UpsertProductTranslationRequest) (*ProductTranslation, error) {
	var before *ProductTranslation
	if s.audit != nil {
		if prev, err := s.getProductTranslation(ctx, productID, locale); err == nil {
			before = prev
		}
	}
	var t ProductTranslation
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO product_translations (product_id, locale, name, subtitle, description)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (product_id, locale) DO UPDATE
		   SET name=$3, subtitle=$4, description=$5, updated_at=NOW()
		 RETURNING locale, name, subtitle, description, updated_at`,
		productID, locale, req.Name, req.Subtitle, req.Description).
		Scan(&t.Locale, &t.Name, &t.Subtitle, &t.Description, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// Translation changes affect localized list/detail responses
	s.cache.DeleteByPrefix(productPrefix)
	s.record(ctx, "product.translation.upsert", "product_translation",
		productID+":"+locale, before, t)
	return &t, nil
}

func (s *ProductService) DeleteTranslation(ctx context.Context, productID, locale string) error {
	var before *ProductTranslation
	if s.audit != nil {
		if prev, err := s.getProductTranslation(ctx, productID, locale); err == nil {
			before = prev
		}
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM product_translations WHERE product_id = $1 AND locale = $2`, productID, locale)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errProductNotFound
	}
	s.cache.DeleteByPrefix(productPrefix)
	s.record(ctx, "product.translation.delete", "product_translation",
		productID+":"+locale, before, nil)
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
		        p.name     AS component_product_name,
		        p.slug     AS component_product_slug,
		        p.subtitle AS component_product_subtitle,
		        pv.name    AS component_variant_name,
		        pv.sku     AS component_sku,
		        pv.stock_qty AS component_stock_qty,
		        pv.price     AS component_price,
		        pi.url       AS component_primary_image_url,
		        bi.created_at
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN LATERAL (
		   SELECT url FROM product_images
		    WHERE product_id = p.id
		    ORDER BY is_primary DESC, sort_order ASC, created_at ASC
		    LIMIT 1
		 ) pi ON TRUE
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
			&bi.ComponentProductName, &bi.ComponentProductSlug, &bi.ComponentProductSubtitle,
			&bi.ComponentVariantName, &bi.ComponentSKU,
			&bi.ComponentStockQty, &bi.ComponentPrice, &bi.ComponentPrimaryImageURL,
			&bi.CreatedAt,
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
		        p.name, p.slug, p.subtitle,
		        pv.name, pv.sku, pv.stock_qty, pv.price,
		        pi.url,
		        bi.created_at
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 JOIN products p ON p.id = pv.product_id
		 LEFT JOIN LATERAL (
		   SELECT url FROM product_images
		    WHERE product_id = p.id
		    ORDER BY is_primary DESC, sort_order ASC, created_at ASC
		    LIMIT 1
		 ) pi ON TRUE
		 WHERE bi.id = $1`, id).Scan(
		&bi.ID, &bi.BundleProductID, &bi.ComponentVariantID, &bi.Quantity,
		&bi.SortOrder, &bi.DisplayNameOverride,
		&bi.ComponentProductName, &bi.ComponentProductSlug, &bi.ComponentProductSubtitle,
		&bi.ComponentVariantName, &bi.ComponentSKU,
		&bi.ComponentStockQty, &bi.ComponentPrice, &bi.ComponentPrimaryImageURL,
		&bi.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	s.cache.DeleteByPrefix(productPrefix)
	// FBT excludes this bundle's components; if the component set just changed,
	// the cached FBT rows for this bundle are stale.
	s.cache.DeleteByPrefix(fbtCachePrefix + productID + ":")
	s.record(ctx, "product.bundle_item.upsert", "product_bundle_item", bi.ID, nil, bi)
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
	s.cache.DeleteByPrefix(fbtCachePrefix + productID + ":")
	s.record(ctx, "product.bundle_item.delete", "product_bundle_item",
		productID+":"+componentVariantID, nil, nil)
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

	var beforeItems []BundleItem
	if s.audit != nil {
		beforeItems, _ = s.GetBundleItems(ctx, productID)
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
	s.cache.DeleteByPrefix(fbtCachePrefix + productID + ":")
	after, err := s.GetBundleItems(ctx, productID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "product.bundle.set", "product_bundle", productID, beforeItems, after)
	return after, nil
}

// ListPromoBundles returns the curated bundle products associated with a
// parent product for the taobao-layout PDP modal. Each row is flattened
// with the bundle's default variant (price / compare_at / stock /
// variant_id) and primary image so the storefront can render rows + CTA
// without any follow-up fetches.
func (s *ProductService) ListPromoBundles(ctx context.Context, parentProductID string) ([]PromoBundle, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ppb.id, ppb.parent_product_id, ppb.bundle_product_id, ppb.sort_order,
		        p.slug, p.name, p.excerpt, p.status,
		        pv.id, pv.price, pv.compare_at_price, pv.stock_qty,
		        pi.url,
		        ppb.created_at
		 FROM product_promo_bundles ppb
		 JOIN products p          ON p.id = ppb.bundle_product_id
		 JOIN product_variants pv ON pv.product_id = p.id AND pv.is_active = TRUE
		 LEFT JOIN LATERAL (
		   SELECT url FROM product_images
		    WHERE product_id = p.id
		    ORDER BY is_primary DESC, sort_order ASC, created_at ASC
		    LIMIT 1
		 ) pi ON TRUE
		 WHERE ppb.parent_product_id = $1
		 ORDER BY ppb.sort_order ASC, ppb.created_at ASC`, parentProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PromoBundle, 0)
	for rows.Next() {
		var pb PromoBundle
		if err := rows.Scan(
			&pb.ID, &pb.ParentProductID, &pb.BundleProductID, &pb.SortOrder,
			&pb.Slug, &pb.Name, &pb.Excerpt, &pb.Status,
			&pb.VariantID, &pb.Price, &pb.CompareAtPrice, &pb.StockQty,
			&pb.PrimaryImageURL,
			&pb.CreatedAt,
		); err != nil {
			return nil, err
		}
		pb.Purchasable = true
		items = append(items, pb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.annotatePromoBundlesPurchasable(ctx, items)
	return items, nil
}

// annotatePromoBundlesPurchasable flips Purchasable=false on every row whose
// underlying bundle product sits in a category the current storefront role
// can't purchase from. Mirrors annotatePurchasableMeta — one extra round-trip
// when the role has any blocked categories, zero otherwise. Defaults to true
// before this is called so callers can rely on the field regardless of
// whether roleRules is wired.
func (s *ProductService) annotatePromoBundlesPurchasable(ctx context.Context, items []PromoBundle) {
	if s.roleRules == nil || len(items) == 0 {
		return
	}
	role := auth.CustomerRoleFromContext(ctx)
	blocked := s.roleRules.BlockedPurchaseCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return
	}
	ids := make([]string, len(items))
	idx := make(map[string]int, len(items))
	for i, pb := range items {
		ids[i] = pb.BundleProductID
		idx[pb.BundleProductID] = i
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT product_id::text
		 FROM product_category_links
		 WHERE product_id = ANY($1::uuid[]) AND category_id = ANY($2::uuid[])`,
		pq.Array(ids), pq.Array(blocked))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if i, ok := idx[id]; ok {
			items[i].Purchasable = false
		}
	}
}

// SetPromoBundles atomically replaces all promo-bundle associations for
// a parent product. Rejects associations where the candidate row isn't a
// bundle product (kind != 'bundle') or points back at the parent itself
// (which would create a self-referencing modal entry).
func (s *ProductService) SetPromoBundles(ctx context.Context, parentProductID string, bundleProductIDs []string) ([]PromoBundle, error) {
	// Validate: every candidate must be kind='bundle' and not the parent itself.
	for _, bid := range bundleProductIDs {
		if bid == parentProductID {
			return nil, fmt.Errorf("a product cannot be its own promo bundle: %s", bid)
		}
		var bKind string
		err := s.db.QueryRowContext(ctx, `SELECT kind FROM products WHERE id = $1`, bid).Scan(&bKind)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("promo bundle product %s not found", bid)
		}
		if err != nil {
			return nil, err
		}
		if bKind != "bundle" {
			return nil, fmt.Errorf("promo bundle product %s is not a bundle (kind=%s)", bid, bKind)
		}
	}

	var before []PromoBundle
	if s.audit != nil {
		before, _ = s.ListPromoBundles(ctx, parentProductID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM product_promo_bundles WHERE parent_product_id = $1`, parentProductID); err != nil {
		return nil, err
	}

	for i, bid := range bundleProductIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO product_promo_bundles (parent_product_id, bundle_product_id, sort_order)
			 VALUES ($1, $2, $3)`,
			parentProductID, bid, i,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	s.cache.DeleteByPrefix(productPrefix)
	after, err := s.ListPromoBundles(ctx, parentProductID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "product.promo_bundles.set", "product_promo_bundles", parentProductID, before, after)
	return after, nil
}

// fbtCachePrefix namespaces FrequentlyBoughtTogether cache entries so the
// admin rebuild endpoint can drop them in one call after repopulating
// product_copurchase.
const fbtCachePrefix = "shop:fbt:"

// Tunables for the multi-algorithm FBT mix. Comment block lives here (rather
// than the plan doc) so future readers can adjust without grepping for the
// values. Pool caps stay larger than weights so the same source product yields
// fresh picks day-over-day (different seed → different sample from the pool).
const (
	fbtSalesWindowDays   = 30 // bestseller + slow-mover lookback
	fbtSlowMoverMaxSales = 2  // total units over the window classifying as "slow"
	fbtSlowMoverMinStock = 5  // total active-variant stock to qualify as "has stock"
	fbtPoolCapCopurchase = 12
	fbtPoolCapBestseller = 20
	fbtPoolCapSlowMover  = 20
	fbtWeightCopurchase  = 2 // up to N picks per response
	fbtWeightBestseller  = 1
	fbtWeightSlowMover   = 1
)

// FrequentlyBoughtTogether returns up to `limit` simple, in-stock products to
// display under the PDP. Three candidate pools are combined: paid+ co-purchase
// pairs (relevance), 30-day bestsellers (social proof, biased to the source
// product's categories), and 30-day slow-movers with stock (gives stagnant
// inventory a slot). Each pool's SQL restricts to `kind = 'simple'` and at
// least one in-stock active variant, so the row never offers a bundle or a
// sold-out tile. The final pick is seeded by productID + UTC date so the row
// is stable across refreshes within a day but rotates daily.
//
// Cache key includes the day; old entries fall out naturally as the date
// advances. Bundle-component exclusion (when the source is a bundle) carries
// over from the previous co-purchase-only implementation.
func (s *ProductService) FrequentlyBoughtTogether(ctx context.Context, productID, locale string, limit int) ([]ProductWithMeta, error) {
	if limit <= 0 || limit > 12 {
		limit = 4
	}

	// hiddenIDs here is the per-role "blocked from listings" set (see
	// roleListedScope) — the replacement for the pre-migration-103 global
	// hidden_category_ids feed. The SQL pool helpers below still take it as a
	// `hiddenIDs []string` arg to keep the signature stable; just the source
	// changed. FBT is a listing surface, so private-link categories should
	// stay out even when the source PDP resolves.
	hiddenIDs, hiddenRaw := s.roleListedScope(ctx)
	excludedCatIDs, excludedCatRaw := s.fbtExcludedCategoryIDs(ctx)
	day := time.Now().UTC().Format("2006-01-02")
	// roleListedScope already bakes the role + blocked-id-set into hiddenRaw,
	// so the cache key needn't repeat the role separately.
	cacheKey := fmt.Sprintf("%s%s:%s:%d:%s:%s:%s", fbtCachePrefix, productID, locale, limit, hiddenRaw, excludedCatRaw, day)
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]ProductWithMeta), nil
	}

	// Source metadata: kind drives bundle-component exclusion; categoryIDs
	// drive same-category preference for bestsellers/slow-movers.
	sourceCategoryIDs, err := s.fbtSourceCategoryIDs(ctx, productID)
	if err != nil {
		return nil, err
	}

	// Build the base exclude list: source product + (if source is bundle) its
	// component products. Co-purchase already won't surface the source itself,
	// but the bundle component exclusion is the carryover from v0.9.248.
	baseExcludes := []string{productID}
	if componentIDs, err := s.fbtBundleComponentProductIDs(ctx, productID); err == nil {
		baseExcludes = append(baseExcludes, componentIDs...)
	}

	// Pool A: co-purchase pairs from product_copurchase.
	poolA, err := s.fbtFetchCopurchasePool(ctx, productID, fbtPoolCapCopurchase, baseExcludes, hiddenIDs, excludedCatIDs)
	if err != nil {
		return nil, err
	}

	// Pool B: bestsellers — same category first, top up storewide if short.
	poolB, err := s.fbtFetchPoolWithCategoryFallback(ctx, sourceCategoryIDs, fbtPoolCapBestseller, baseExcludes, hiddenIDs, excludedCatIDs, s.fbtFetchBestsellersPool)
	if err != nil {
		return nil, err
	}

	// Pool C: slow-movers — same category first, top up storewide if short.
	poolC, err := s.fbtFetchPoolWithCategoryFallback(ctx, sourceCategoryIDs, fbtPoolCapSlowMover, baseExcludes, hiddenIDs, excludedCatIDs, s.fbtFetchSlowMoversPool)
	if err != nil {
		return nil, err
	}

	// Seeded selection: 2 co-purchase + 1 bestseller + 1 slow-mover, then
	// backfill from whatever remains until limit (or pools are exhausted).
	rng := rand.New(rand.NewSource(fbtDailySeed(productID, day)))
	pickedIDs := fbtSelectMixed(rng, []fbtPool{
		{ids: poolA, weight: fbtWeightCopurchase},
		{ids: poolB, weight: fbtWeightBestseller},
		{ids: poolC, weight: fbtWeightSlowMover},
	}, limit)

	if len(pickedIDs) == 0 {
		empty := []ProductWithMeta{}
		s.cache.Set(cacheKey, empty, s.ttl(ctx))
		return empty, nil
	}

	products, err := s.fbtLoadProductsByIDs(ctx, pickedIDs, locale)
	if err != nil {
		return nil, err
	}
	// Stamp Purchasable per the current role's purchase-block rules and drop
	// items the role can't add to cart — otherwise the BundleComposer would
	// render a tile the user can't actually use. In the no-role-rules default,
	// annotatePurchasableMeta short-circuits without a DB hit and the filter
	// is a no-op.
	s.annotatePurchasableMeta(ctx, products)
	kept := products[:0]
	for _, p := range products {
		if p.Purchasable {
			kept = append(kept, p)
		}
	}
	products = kept

	s.cache.Set(cacheKey, products, s.ttl(ctx))
	return products, nil
}

// fbtPool is a candidate set for the weighted FBT picker.
type fbtPool struct {
	ids    []string
	weight int
}

// fbtSelectMixed picks up to `limit` IDs from the pools using a seeded RNG.
// Each pool first contributes up to its `weight` (shuffled, deduped against
// previously-picked IDs). When all pools have taken their share but the picks
// are still short of `limit`, remaining unpicked IDs across all pools are
// shuffled together and pulled in order.
func fbtSelectMixed(rng *rand.Rand, pools []fbtPool, limit int) []string {
	picks := make([]string, 0, limit)
	seen := make(map[string]bool)

	for _, p := range pools {
		if len(picks) >= limit {
			break
		}
		avail := make([]string, 0, len(p.ids))
		for _, id := range p.ids {
			if !seen[id] {
				avail = append(avail, id)
			}
		}
		rng.Shuffle(len(avail), func(i, j int) { avail[i], avail[j] = avail[j], avail[i] })
		take := p.weight
		if take > len(avail) {
			take = len(avail)
		}
		if take > limit-len(picks) {
			take = limit - len(picks)
		}
		for i := 0; i < take; i++ {
			picks = append(picks, avail[i])
			seen[avail[i]] = true
		}
	}

	if len(picks) >= limit {
		return picks
	}

	backfill := make([]string, 0)
	for _, p := range pools {
		for _, id := range p.ids {
			if !seen[id] {
				backfill = append(backfill, id)
				seen[id] = true
			}
		}
	}
	rng.Shuffle(len(backfill), func(i, j int) { backfill[i], backfill[j] = backfill[j], backfill[i] })
	for _, id := range backfill {
		if len(picks) >= limit {
			break
		}
		picks = append(picks, id)
	}
	return picks
}

// fbtDailySeed produces a stable int64 seed from (productID, day) so picks are
// consistent across refreshes within the same UTC day but rotate at midnight.
func fbtDailySeed(productID, day string) int64 {
	h := sha256.Sum256([]byte(productID + ":" + day))
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// fbtSourceCategoryIDs returns the source product's category set (primary +
// any links from product_category_links). Used to bias bestsellers/slow-movers
// toward the same departments. Returns nil if the product has no categories.
func (s *ProductService) fbtSourceCategoryIDs(ctx context.Context, productID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT category_id FROM (
		     SELECT category_id FROM products WHERE id = $1 AND category_id IS NOT NULL
		     UNION
		     SELECT category_id FROM product_category_links WHERE product_id = $1
		 ) c`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// fbtBundleComponentProductIDs returns the distinct product IDs of components
// of the source product if it's a bundle, otherwise an empty slice. Mirrors
// the subselect from the pre-mix implementation (excludes a bundle's own
// components from its FBT row).
func (s *ProductService) fbtBundleComponentProductIDs(ctx context.Context, sourceProductID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT pv.product_id
		 FROM bundle_items bi
		 JOIN product_variants pv ON pv.id = bi.component_variant_id
		 WHERE bi.bundle_product_id = $1`, sourceProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// fbtFetchPoolWithCategoryFallback runs `fetch` with the source's categories
// first; if that yields fewer than `cap` IDs, it appends a storewide pass with
// the same-category picks added to the exclude list. Both calls are cheap (top-
// N indexed reads) so we always check the category-biased set first.
func (s *ProductService) fbtFetchPoolWithCategoryFallback(
	ctx context.Context,
	categoryIDs []string,
	cap int,
	baseExcludes, hiddenIDs, excludedCatIDs []string,
	fetch func(ctx context.Context, categoryIDs []string, cap int, excludes, hiddenIDs, excludedCatIDs []string) ([]string, error),
) ([]string, error) {
	var ids []string
	if len(categoryIDs) > 0 {
		got, err := fetch(ctx, categoryIDs, cap, baseExcludes, hiddenIDs, excludedCatIDs)
		if err != nil {
			return nil, err
		}
		ids = got
	}
	if len(ids) >= cap {
		return ids, nil
	}
	exclSet := make([]string, 0, len(baseExcludes)+len(ids))
	exclSet = append(exclSet, baseExcludes...)
	exclSet = append(exclSet, ids...)
	more, err := fetch(ctx, nil, cap-len(ids), exclSet, hiddenIDs, excludedCatIDs)
	if err != nil {
		return nil, err
	}
	return append(ids, more...), nil
}

// fbtFetchCopurchasePool returns up to `cap` simple, in-stock product IDs
// ranked by historical co-purchase strength with the source product. Bundles
// and sold-out products are filtered at SQL so the row never offers something
// the customer can't add to cart. "In-stock" here means the *default* variant
// (first active by sort_order, created_at) — same row the FBT card displays
// and adds to cart — so a product whose default variant is OOS but a sibling
// variant has stock is excluded rather than rendered as a disabled tile.
// Also drops excluded IDs (source + bundle components) and products in hidden
// categories.
func (s *ProductService) fbtFetchCopurchasePool(ctx context.Context, sourceProductID string, cap int, excludes, hiddenIDs, excludedCatIDs []string) ([]string, error) {
	query := `
		SELECT p.id
		FROM product_copurchase cp
		JOIN products p ON p.id = cp.related_product_id
		WHERE cp.product_id = $1
		  AND p.status = 'active'
		  AND p.kind = 'simple'
		  AND EXISTS (
		      SELECT 1 FROM (
		          SELECT pv.stock_qty
		          FROM product_variants pv
		          WHERE pv.product_id = p.id
		            AND pv.is_active = TRUE
		          ORDER BY pv.sort_order ASC, pv.created_at ASC
		          LIMIT 1
		      ) defv
		      WHERE defv.stock_qty > 0
		  )
		  AND p.id <> ALL($2::uuid[])
		  AND (cardinality($3::uuid[]) = 0 OR p.category_id IS NULL OR p.category_id <> ALL($3::uuid[]))
		  AND NOT EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id
		        AND pcl.category_id = ANY($4::uuid[])
		  )
		ORDER BY cp.together_order_count DESC, p.created_at DESC
		LIMIT $5`
	return s.fbtScanIDs(ctx, query, sourceProductID, fbtUUIDArray(excludes), fbtUUIDArray(hiddenIDs), fbtUUIDArray(excludedCatIDs), cap)
}

// fbtFetchBestsellersPool returns up to `cap` simple, in-stock product IDs
// ranked by units sold over fbtSalesWindowDays in paid+ orders. Only top-level
// order_items count (`parent_item_id IS NULL`) so components shipped as part of
// a bundle don't double-credit the parent product. Bundles and sold-out
// products are filtered at SQL so every result is purchasable. "In-stock" here
// means the *default* variant has stock (see fbtFetchCopurchasePool for the
// rationale). When `categoryIDs` is non-empty, the result is restricted to
// products linked to any of those categories.
func (s *ProductService) fbtFetchBestsellersPool(ctx context.Context, categoryIDs []string, cap int, excludes, hiddenIDs, excludedCatIDs []string) ([]string, error) {
	query := `
		WITH sales AS (
		    SELECT pv.product_id, SUM(oi.quantity)::bigint AS qty
		    FROM order_items oi
		    JOIN orders o            ON o.id  = oi.order_id
		    JOIN product_variants pv ON pv.id = oi.variant_id
		    WHERE oi.parent_item_id IS NULL
		      AND o.status IN ('paid','processing','shipped','delivered')
		      AND o.created_at >= NOW() - ($1 || ' days')::interval
		    GROUP BY pv.product_id
		)
		SELECT p.id
		FROM products p
		JOIN sales s ON s.product_id = p.id
		WHERE p.status = 'active'
		  AND p.kind = 'simple'
		  AND EXISTS (
		      SELECT 1 FROM (
		          SELECT pv.stock_qty
		          FROM product_variants pv
		          WHERE pv.product_id = p.id
		            AND pv.is_active = TRUE
		          ORDER BY pv.sort_order ASC, pv.created_at ASC
		          LIMIT 1
		      ) defv
		      WHERE defv.stock_qty > 0
		  )
		  AND p.id <> ALL($2::uuid[])
		  AND (cardinality($3::uuid[]) = 0 OR p.category_id IS NULL OR p.category_id <> ALL($3::uuid[]))
		  AND (cardinality($4::uuid[]) = 0 OR EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($4::uuid[])
		  ))
		  AND NOT EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id
		        AND pcl.category_id = ANY($5::uuid[])
		  )
		ORDER BY s.qty DESC, p.created_at DESC
		LIMIT $6`
	return s.fbtScanIDs(ctx, query,
		fbtSalesWindowDays, fbtUUIDArray(excludes), fbtUUIDArray(hiddenIDs), fbtUUIDArray(categoryIDs), fbtUUIDArray(excludedCatIDs), cap)
}

// fbtFetchSlowMoversPool returns up to `cap` simple products (bundles
// excluded — their stock is derived) that have stock above
// fbtSlowMoverMinStock and at most fbtSlowMoverMaxSales over the window.
// Ranked stock-rich-first so the picks meaningfully move inventory.
//
// The total_stock threshold defines "slow mover with real inventory", but the
// FBT card only offers the *default* variant — so we additionally require the
// default variant itself to have stock. Otherwise a product with one fat
// non-default variant and an OOS default would render as a disabled tile.
func (s *ProductService) fbtFetchSlowMoversPool(ctx context.Context, categoryIDs []string, cap int, excludes, hiddenIDs, excludedCatIDs []string) ([]string, error) {
	query := `
		WITH sales AS (
		    SELECT pv.product_id, COALESCE(SUM(oi.quantity), 0)::bigint AS qty
		    FROM product_variants pv
		    LEFT JOIN order_items oi ON oi.variant_id = pv.id AND oi.parent_item_id IS NULL
		    LEFT JOIN orders o       ON o.id = oi.order_id
		                            AND o.status IN ('paid','processing','shipped','delivered')
		                            AND o.created_at >= NOW() - ($1 || ' days')::interval
		    WHERE pv.is_active = TRUE
		    GROUP BY pv.product_id
		),
		stock AS (
		    SELECT pv.product_id, SUM(pv.stock_qty)::bigint AS total_stock
		    FROM product_variants pv
		    WHERE pv.is_active = TRUE
		    GROUP BY pv.product_id
		)
		SELECT p.id
		FROM products p
		JOIN stock st ON st.product_id = p.id
		LEFT JOIN sales s ON s.product_id = p.id
		WHERE p.status = 'active'
		  AND p.kind = 'simple'
		  AND st.total_stock > $2
		  AND EXISTS (
		      SELECT 1 FROM (
		          SELECT pv.stock_qty
		          FROM product_variants pv
		          WHERE pv.product_id = p.id
		            AND pv.is_active = TRUE
		          ORDER BY pv.sort_order ASC, pv.created_at ASC
		          LIMIT 1
		      ) defv
		      WHERE defv.stock_qty > 0
		  )
		  AND COALESCE(s.qty, 0) <= $3
		  AND p.id <> ALL($4::uuid[])
		  AND (cardinality($5::uuid[]) = 0 OR p.category_id IS NULL OR p.category_id <> ALL($5::uuid[]))
		  AND (cardinality($6::uuid[]) = 0 OR EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($6::uuid[])
		  ))
		  AND NOT EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id
		        AND pcl.category_id = ANY($7::uuid[])
		  )
		ORDER BY st.total_stock DESC, COALESCE(s.qty, 0) ASC, p.created_at DESC
		LIMIT $8`
	return s.fbtScanIDs(ctx, query,
		fbtSalesWindowDays, fbtSlowMoverMinStock, fbtSlowMoverMaxSales,
		fbtUUIDArray(excludes), fbtUUIDArray(hiddenIDs), fbtUUIDArray(categoryIDs), fbtUUIDArray(excludedCatIDs), cap)
}

// fbtUUIDArray wraps pq.Array but guarantees an empty (non-NULL) Postgres
// array when the input slice is nil. Pool queries use
// `cardinality($N::uuid[]) = 0` to skip optional filters; with a NULL input
// `cardinality` also returns NULL, which fails the WHERE clause and silently
// drops all rows.
func fbtUUIDArray(ids []string) interface{} {
	if ids == nil {
		ids = []string{}
	}
	return pq.Array(ids)
}

// fbtScanIDs runs a SELECT-id-only query and returns the result as a slice.
// Tiny helper so each pool fetch reads as one expression instead of repeating
// the rows-iterate boilerplate.
func (s *ProductService) fbtScanIDs(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// fbtLoadProductsByIDs hydrates a list of product IDs into ProductWithMeta
// rows in the order requested. Output order matters so the seeded picker can
// guarantee the same display order across refreshes within a day.
func (s *ProductService) fbtLoadProductsByIDs(ctx context.Context, ids []string, locale string) ([]ProductWithMeta, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `
		SELECT p.id, p.number, p.category_id, p.slug,
		       COALESCE(t.name,        p.name)        AS name,
		       COALESCE(t.subtitle,    p.subtitle)    AS subtitle,
		       p.excerpt,
		       COALESCE(t.description, p.description) AS description,
		       p.how_to_use, p.compatible_surfaces,
		       p.status, p.kind, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM product_variants pv
		        WHERE pv.product_id = p.id AND pv.is_active = TRUE) AS variant_count,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = p.id
		        ORDER BY pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS primary_image_url,
		       defv.id   AS default_variant_id,
		       defv.price AS default_variant_price,
		       defv.compare_at_price AS default_variant_compare_at_price,
		       defv.stock_qty AS default_variant_stock_qty,
		       defv.name AS default_variant_name
		FROM products p` + productTranslationJoin + `
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty, pv.name
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE
		WHERE p.id = ANY($2::uuid[])`
	rows, err := s.db.QueryContext(ctx, query, locale, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[string]ProductWithMeta, len(ids))
	for rows.Next() {
		var pm ProductWithMeta
		if err := rows.Scan(
			&pm.ID, &pm.Number, &pm.CategoryID, &pm.Slug, &pm.Name, &pm.Subtitle,
			&pm.Excerpt, &pm.Description, &pm.HowToUse, pq.Array(&pm.CompatibleSurfaces),
			&pm.Status, &pm.Kind, &pm.CreatedAt, &pm.UpdatedAt,
			&pm.VariantCount, &pm.PrimaryImageURL, &pm.DefaultVariantID,
			&pm.DefaultVariantPrice, &pm.DefaultVariantCompareAtPrice,
			&pm.DefaultVariantStockQty, &pm.DefaultVariantName,
		); err != nil {
			return nil, err
		}
		byID[pm.ID] = pm
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ordered := make([]ProductWithMeta, 0, len(ids))
	for _, id := range ids {
		if pm, ok := byID[id]; ok {
			ordered = append(ordered, pm)
		}
	}
	return ordered, nil
}

// RebuildCopurchase replaces every row in product_copurchase with a fresh
// aggregation from paid+ orders. Returns the number of pair rows written.
// Wrapped in a single tx so a concurrent reader sees either the old set or
// the new set, never an empty table.
func (s *ProductService) RebuildCopurchase(ctx context.Context) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `TRUNCATE TABLE product_copurchase`); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO product_copurchase (product_id, related_product_id, together_order_count)
		SELECT v1.product_id, v2.product_id, COUNT(DISTINCT o.id)
		FROM orders o
		JOIN order_items oi1 ON oi1.order_id = o.id
		JOIN product_variants v1 ON v1.id = oi1.variant_id
		JOIN order_items oi2 ON oi2.order_id = o.id AND oi2.id <> oi1.id
		JOIN product_variants v2 ON v2.id = oi2.variant_id
		WHERE o.status IN ('paid','processing','shipped','delivered')
		  AND v1.product_id <> v2.product_id
		GROUP BY v1.product_id, v2.product_id
		HAVING COUNT(DISTINCT o.id) >= 2`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.cache.DeleteByPrefix(fbtCachePrefix)
	return int(n), nil
}

// =====================================================================
// WooCommerce up-sells (PDP) & cross-sells (cart).
//
// Both are merchant-curated, ordered lists imported from WooCommerce
// (product.upsell_ids / cross_sell_ids), stored in product_upsells /
// product_cross_sells. Deliberately separate from the algorithmic FBT
// feature above: no co-purchase aggregation and no daily-rotating seed —
// the merchant's imported ordering (position) is authoritative. They
// reuse FBT's hydration + purchasability gating helpers only.
// =====================================================================

const (
	upsellCachePrefix    = "shop:upsells:"
	crossSellCachePrefix = "shop:crosssells:"
)

// Upsells returns the merchant-curated WC up-sells (the "buy this instead"
// alternatives shown on the PDP) for a product, ordered by the imported
// position and filtered to active, role-listable, role-purchasable products.
// Fully deterministic — unlike FrequentlyBoughtTogether there is no pool mixing
// or daily seed.
func (s *ProductService) Upsells(ctx context.Context, productID, locale string, limit int) ([]ProductWithMeta, error) {
	if limit <= 0 || limit > 12 {
		limit = 4
	}
	// Up-sells are a listing surface, so honour the per-role "unlisted
	// category" set exactly like FBT does (hiddenRaw bakes the role into the
	// cache key so roles don't share an entry).
	hiddenIDs, hiddenRaw := s.roleListedScope(ctx)
	cacheKey := fmt.Sprintf("%s%s:%s:%d:%s", upsellCachePrefix, productID, locale, limit, hiddenRaw)
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]ProductWithMeta), nil
	}

	ids, err := s.fbtScanIDs(ctx, `
		SELECT u.upsell_product_id
		FROM product_upsells u
		JOIN products p ON p.id = u.upsell_product_id
		WHERE u.product_id = $1
		  AND p.status = 'active'
		  AND NOT EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($2::uuid[])
		  )
		ORDER BY u.position ASC, p.created_at DESC
		LIMIT $3`,
		productID, fbtUUIDArray(hiddenIDs), limit)
	if err != nil {
		return nil, err
	}

	products, err := s.upsellHydrate(ctx, ids, locale)
	if err != nil {
		return nil, err
	}
	s.cache.Set(cacheKey, products, s.ttl(ctx))
	return products, nil
}

// CrossSellsForCart unions the cross-sells of the products represented by the
// given cart variant IDs, dedupes (earliest position wins), drops products
// already in the cart, and returns active + role-purchasable products. The
// cart only carries variant IDs (no product UUID), so step one resolves
// variants → products. Returns an empty slice for an empty/invalid input
// rather than erroring on an empty ::uuid[] cast.
func (s *ProductService) CrossSellsForCart(ctx context.Context, variantIDs []string, locale string, limit int) ([]ProductWithMeta, error) {
	if limit <= 0 || limit > 12 {
		limit = 4
	}
	// The variant IDs come straight off the client cart, so anything that isn't
	// a UUID would abort the ::uuid[] cast. Drop invalids + de-dup, and the
	// sorted result doubles as a stable cache key.
	clean := sanitizeUUIDs(variantIDs)
	if len(clean) == 0 {
		return []ProductWithMeta{}, nil
	}

	hiddenIDs, hiddenRaw := s.roleListedScope(ctx)
	cacheKey := fmt.Sprintf("%s%s:%s:%d:%s", crossSellCachePrefix, strings.Join(clean, ","), locale, limit, hiddenRaw)
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]ProductWithMeta), nil
	}

	// Resolve cart variant IDs → distinct cart product IDs. These serve both as
	// the "source" set (whose cross-sells we want) and the "already in cart"
	// exclusion set below.
	cartProductIDs, err := s.fbtScanIDs(ctx,
		`SELECT DISTINCT product_id::text FROM product_variants WHERE id = ANY($1::uuid[])`,
		pq.Array(clean))
	if err != nil {
		return nil, err
	}
	if len(cartProductIDs) == 0 {
		empty := []ProductWithMeta{}
		s.cache.Set(cacheKey, empty, s.ttl(ctx))
		return empty, nil
	}

	ids, err := s.fbtScanIDs(ctx, `
		SELECT cs.cross_sell_product_id::text
		FROM product_cross_sells cs
		JOIN products p ON p.id = cs.cross_sell_product_id
		WHERE cs.product_id = ANY($1::uuid[])
		  AND cs.cross_sell_product_id <> ALL($1::uuid[])
		  AND p.status = 'active'
		  AND NOT EXISTS (
		      SELECT 1 FROM product_category_links pcl
		      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($2::uuid[])
		  )
		GROUP BY cs.cross_sell_product_id
		ORDER BY MIN(cs.position) ASC, cs.cross_sell_product_id
		LIMIT $3`,
		pq.Array(cartProductIDs), fbtUUIDArray(hiddenIDs), limit)
	if err != nil {
		return nil, err
	}

	products, err := s.upsellHydrate(ctx, ids, locale)
	if err != nil {
		return nil, err
	}
	s.cache.Set(cacheKey, products, s.ttl(ctx))
	return products, nil
}

// upsellHydrate loads product IDs into ProductWithMeta (in order), stamps
// Purchasable per the current role, and drops anything the role can't add to
// cart — shared by Upsells and CrossSellsForCart so both apply the same gate as
// FBT.
func (s *ProductService) upsellHydrate(ctx context.Context, ids []string, locale string) ([]ProductWithMeta, error) {
	if len(ids) == 0 {
		return []ProductWithMeta{}, nil
	}
	products, err := s.fbtLoadProductsByIDs(ctx, ids, locale)
	if err != nil {
		return nil, err
	}
	s.annotatePurchasableMeta(ctx, products)
	kept := make([]ProductWithMeta, 0, len(products))
	for _, p := range products {
		if p.Purchasable {
			kept = append(kept, p)
		}
	}
	return kept, nil
}

// ReplaceUpsells atomically replaces the up-sell list for a parent product with
// the given ordered target IDs (position = index). Used by the importer's
// reconcile pass; idempotent across re-imports. Unlike the admin promo-bundle
// setter it records no audit entry (bulk import would flood the audit log) and
// performs no per-target validation — the importer only passes IDs it already
// resolved via GetIDByWCProductID.
func (s *ProductService) ReplaceUpsells(ctx context.Context, productID string, upsellProductIDs []string) error {
	return s.replaceProductRelations(ctx, "product_upsells", "upsell_product_id", productID, upsellProductIDs, upsellCachePrefix)
}

// ReplaceCrossSells is the cross-sell counterpart to ReplaceUpsells.
func (s *ProductService) ReplaceCrossSells(ctx context.Context, productID string, crossSellProductIDs []string) error {
	return s.replaceProductRelations(ctx, "product_cross_sells", "cross_sell_product_id", productID, crossSellProductIDs, crossSellCachePrefix)
}

// replaceProductRelations is the shared delete-by-parent + ordered-insert used
// by ReplaceUpsells / ReplaceCrossSells. table/col are hardcoded constants (not
// user input), so the fmt.Sprintf interpolation is safe. Self-references and
// duplicate targets are tolerated defensively (the CHECK + PK would otherwise
// reject them).
func (s *ProductService) replaceProductRelations(ctx context.Context, table, col, productID string, targetIDs []string, cachePrefix string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE product_id = $1`, table), productID); err != nil {
		return err
	}
	pos := 0
	for _, tid := range targetIDs {
		if tid == productID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO %s (product_id, %s, position) VALUES ($1, $2, $3)
			             ON CONFLICT (product_id, %s) DO UPDATE SET position = EXCLUDED.position`, table, col, col),
			productID, tid, pos); err != nil {
			return err
		}
		pos++
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.DeleteByPrefix(cachePrefix)
	return nil
}

// sanitizeUUIDs keeps only well-formed UUID strings, canonicalised (lower-case,
// hyphenated) and de-duped, returned sorted so callers get a stable form for
// cache keys and ANY(...) filters. The cart hands us raw variant IDs from the
// client, so one malformed value would otherwise abort the ::uuid[] cast.
func sanitizeUUIDs(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		parsed, err := uuid.Parse(strings.TrimSpace(v))
		if err != nil {
			continue
		}
		key := parsed.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// =====================================================================
// Admin manual editing of up-sells / cross-sells.
//
// The storefront getters (Upsells / CrossSellsForCart) return role-filtered,
// purchasability-gated ProductWithMeta. The admin product-edit page instead
// needs the RAW curated list — every association, in position order, with
// enough product detail to render the editor row. These mirror ListPromoBundles
// / SetPromoBundles, but the join tables have a composite PK (no surrogate id)
// and any product may be a target (no kind restriction). Writes are audited;
// the importer keeps using the audit-free Replace* path above.
// =====================================================================

// RelatedProductRef is one curated up-sell / cross-sell association, shaped for
// the admin editor row. ProductID is the *target* (the up-sell or cross-sell
// product); the parent product is implicit in the query.
type RelatedProductRef struct {
	ProductID       string   `json:"product_id"`
	Position        int      `json:"position"`
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Excerpt         *string  `json:"excerpt,omitempty"`
	Status          string   `json:"status"`
	VariantID       *string  `json:"variant_id,omitempty"`
	Price           *float64 `json:"price,omitempty"`
	CompareAtPrice  *float64 `json:"compare_at_price,omitempty"`
	StockQty        *int     `json:"stock_qty,omitempty"`
	PrimaryImageURL *string  `json:"primary_image_url,omitempty"`
}

// ListUpsells returns the raw curated up-sell associations for a product in
// position order (admin editor). Unlike the storefront Upsells() it does no
// role filtering and no cap.
func (s *ProductService) ListUpsells(ctx context.Context, productID string) ([]RelatedProductRef, error) {
	return s.listRelatedRefs(ctx, "product_upsells", "upsell_product_id", productID)
}

// ListCrossSells is the cross-sell counterpart to ListUpsells.
func (s *ProductService) ListCrossSells(ctx context.Context, productID string) ([]RelatedProductRef, error) {
	return s.listRelatedRefs(ctx, "product_cross_sells", "cross_sell_product_id", productID)
}

// listRelatedRefs is the shared raw-list query for ListUpsells / ListCrossSells.
// Mirrors ListPromoBundles' join shape (target product + its default active
// variant via LATERAL + LATERAL primary image), ordered by position. table/col
// are hardcoded constants so the fmt.Sprintf interpolation is safe. The variant
// join is LATERAL+LIMIT 1 (not a plain JOIN like promo-bundles) because a
// target can be a simple product with several active variants — a plain join
// would emit one row per variant and duplicate the association.
func (s *ProductService) listRelatedRefs(ctx context.Context, table, col, productID string) ([]RelatedProductRef, error) {
	query := fmt.Sprintf(`
		SELECT r.%s, r.position,
		       p.slug, p.name, p.excerpt, p.status,
		       defv.id, defv.price, defv.compare_at_price, defv.stock_qty,
		       pi.url
		FROM %s r
		JOIN products p ON p.id = r.%s
		LEFT JOIN LATERAL (
		    SELECT pv.id, pv.price, pv.compare_at_price, pv.stock_qty
		    FROM product_variants pv
		    WHERE pv.product_id = p.id AND pv.is_active = TRUE
		    ORDER BY pv.sort_order ASC, pv.created_at ASC
		    LIMIT 1
		) defv ON TRUE
		LEFT JOIN LATERAL (
		    SELECT url FROM product_images
		    WHERE product_id = p.id
		    ORDER BY is_primary DESC, sort_order ASC, created_at ASC
		    LIMIT 1
		) pi ON TRUE
		WHERE r.product_id = $1
		ORDER BY r.position ASC`, col, table, col)
	rows, err := s.db.QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RelatedProductRef, 0)
	for rows.Next() {
		var ref RelatedProductRef
		if err := rows.Scan(
			&ref.ProductID, &ref.Position,
			&ref.Slug, &ref.Name, &ref.Excerpt, &ref.Status,
			&ref.VariantID, &ref.Price, &ref.CompareAtPrice, &ref.StockQty,
			&ref.PrimaryImageURL,
		); err != nil {
			return nil, err
		}
		items = append(items, ref)
	}
	return items, rows.Err()
}

// SetUpsells atomically replaces a product's up-sell associations with the given
// ordered target IDs (admin editor). Audited. Mirrors SetPromoBundles but allows
// any product as a target (no kind restriction); it delegates the tx to
// replaceProductRelations and layers validation + audit on top.
func (s *ProductService) SetUpsells(ctx context.Context, productID string, upsellProductIDs []string) ([]RelatedProductRef, error) {
	return s.setRelatedRefs(ctx, "product_upsells", "upsell_product_id", upsellCachePrefix,
		"product.upsells.set", "product_upsells", productID, upsellProductIDs, s.ListUpsells)
}

// SetCrossSells is the cross-sell counterpart to SetUpsells.
func (s *ProductService) SetCrossSells(ctx context.Context, productID string, crossSellProductIDs []string) ([]RelatedProductRef, error) {
	return s.setRelatedRefs(ctx, "product_cross_sells", "cross_sell_product_id", crossSellCachePrefix,
		"product.cross_sells.set", "product_cross_sells", productID, crossSellProductIDs, s.ListCrossSells)
}

// setRelatedRefs is the shared validate + audited-replace used by SetUpsells /
// SetCrossSells. lister is the matching List* used for the before/after audit
// snapshots and the returned result.
func (s *ProductService) setRelatedRefs(
	ctx context.Context,
	table, col, cachePrefix, action, entityType, productID string,
	targetIDs []string,
	lister func(context.Context, string) ([]RelatedProductRef, error),
) ([]RelatedProductRef, error) {
	for _, tid := range targetIDs {
		if tid == productID {
			return nil, fmt.Errorf("a product cannot reference itself: %s", tid)
		}
		var exists bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1)`, tid).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("product %s not found", tid)
		}
	}

	var before []RelatedProductRef
	if s.audit != nil {
		before, _ = lister(ctx, productID)
	}
	if err := s.replaceProductRelations(ctx, table, col, productID, targetIDs, cachePrefix); err != nil {
		return nil, err
	}
	after, err := lister(ctx, productID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, action, entityType, productID, before, after)
	return after, nil
}
