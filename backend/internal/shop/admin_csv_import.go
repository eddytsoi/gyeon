package shop

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"gyeon/backend/internal/catalog"
	"gyeon/backend/internal/respond"
)

// CSVRowErr is the per-row error shape shared by the product-detail CSV
// importers. Row is 1-based (header is row 1 when present, first data row is
// row 2). Mirrors the orders importer's contract so the admin UI renders bad
// rows the same way across pages.
type CSVRowErr struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ── 套裝內容 / Bundle-items CSV ─────────────────────────────────────────────

// BundleItemCSVResolveItem is one resolved component row, shaped for the admin
// product-detail bundle-contents table (an EditableBundleItem on the client).
type BundleItemCSVResolveItem struct {
	ComponentVariantID       string  `json:"component_variant_id"`
	ComponentProductName     string  `json:"component_product_name"`
	ComponentSKU             string  `json:"component_sku"`
	ComponentVariantName     *string `json:"component_variant_name,omitempty"`
	ComponentPrice           float64 `json:"component_price"`
	ComponentStockQty        int     `json:"component_stock_qty"`
	ComponentPrimaryImageURL *string `json:"component_primary_image_url,omitempty"`
	Quantity                 int     `json:"quantity"`
}

// BundleItemsCSVResolveResult is the response for POST
// /admin/products/bundle-items/csv-resolve.
type BundleItemsCSVResolveResult struct {
	Items   []BundleItemCSVResolveItem `json:"items"`
	Skipped int                        `json:"skipped"`
	Errors  []CSVRowErr                `json:"errors,omitempty"`
}

// resolveBundleItemsCSV backs POST /admin/products/bundle-items/csv-resolve.
// Admins upload a `name,variant,quantity` CSV (same grammar as the order and
// stock-mutation importers) and receive resolved component rows to append to
// the bundle being edited. MatchVariant is called with allowBundles=false so
// bundle products are rejected — that is exactly the no-nesting rule for
// bundle components, enforced for free. Always 200 OK with per-row errors.
func (h *ProductHandler) resolveBundleItemsCSV(w http.ResponseWriter, r *http.Request) {
	fh, ok := openCSVUpload(w, r)
	if !ok {
		return
	}
	defer fh.Close()
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	respond.JSON(w, http.StatusOK, h.resolveBundleCSV(r.Context(), fh))
}

func (h *ProductHandler) resolveBundleCSV(ctx context.Context, r io.Reader) *BundleItemsCSVResolveResult {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err == io.EOF {
		return &BundleItemsCSVResolveResult{Items: []BundleItemCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: "empty file"}}}
	}
	if err != nil {
		return &BundleItemsCSVResolveResult{Items: []BundleItemCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: "read header: " + err.Error()}}}
	}
	if err := catalog.ValidateHeader(header); err != nil {
		return &BundleItemsCSVResolveResult{Items: []BundleItemCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: err.Error()}}}
	}

	out := &BundleItemsCSVResolveResult{Items: []BundleItemCSVResolveItem{}}
	// Aggregate duplicate variant_id rows by summing quantities — same as the
	// order/stock importers so a CSV listing the same SKU twice is one line.
	agg := map[string]int{}
	order := []string{}

	rowNum := 1
	for {
		rec, rerr := cr.Read()
		if rerr == io.EOF {
			break
		}
		rowNum++
		if rerr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: rerr.Error()})
			continue
		}
		if catalog.IsBlankRow(rec) {
			continue
		}
		if len(rec) < 3 {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "expected 3 columns (name, variant, quantity)"})
			continue
		}
		productName := strings.TrimSpace(rec[0])
		variantHint := strings.TrimSpace(rec[1])
		qtyStr := strings.TrimSpace(rec[2])
		if productName == "" {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "name is required"})
			continue
		}
		qty, qerr := strconv.Atoi(qtyStr)
		if qerr != nil || qty <= 0 {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "quantity must be a positive integer"})
			continue
		}
		variantID, _, mErr := catalog.MatchVariant(ctx, h.svc.DB(), productName, variantHint, false)
		if mErr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: mErr.Error()})
			continue
		}
		if _, seen := agg[variantID]; !seen {
			order = append(order, variantID)
		}
		agg[variantID] += qty
	}

	for _, vid := range order {
		item, ierr := h.buildBundleResolvedItem(ctx, vid, agg[vid])
		if ierr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: 0, Message: "failed to enrich variant " + vid + ": " + ierr.Error()})
			continue
		}
		out.Items = append(out.Items, *item)
	}
	return out
}

func (h *ProductHandler) buildBundleResolvedItem(ctx context.Context, variantID string, qty int) (*BundleItemCSVResolveItem, error) {
	v, err := h.svc.GetVariantByID(ctx, variantID)
	if err != nil {
		return nil, err
	}
	productName := ""
	if v.ProductName != nil {
		productName = *v.ProductName
	}
	return &BundleItemCSVResolveItem{
		ComponentVariantID:       v.ID,
		ComponentProductName:     productName,
		ComponentSKU:             v.SKU,
		ComponentVariantName:     v.Name,
		ComponentPrice:           v.Price,
		ComponentStockQty:        v.StockQty,
		ComponentPrimaryImageURL: v.ImageURL,
		Quantity:                 qty,
	}, nil
}

// ── 優惠套裝 / Promo-bundles CSV ────────────────────────────────────────────

// PromoBundleCSVResolveItem is one resolved bundle product. Kept minimal — the
// client hydrates variant + image via the same path a search-box pick uses, so
// CSV import and search converge on one "add candidate" code path.
type PromoBundleCSVResolveItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// PromoBundlesCSVResolveResult is the response for POST
// /admin/products/promo-bundles/csv-resolve.
type PromoBundlesCSVResolveResult struct {
	Items   []PromoBundleCSVResolveItem `json:"items"`
	Skipped int                         `json:"skipped"`
	Errors  []CSVRowErr                 `json:"errors,omitempty"`
}

// resolvePromoBundlesCSV backs POST /admin/products/promo-bundles/csv-resolve.
// Admins upload a single-column CSV of bundle-product names or slugs; each is
// matched to one active bundle product. The header row (name/slug/product) is
// optional. Always 200 OK with per-row errors.
func (h *ProductHandler) resolvePromoBundlesCSV(w http.ResponseWriter, r *http.Request) {
	fh, ok := openCSVUpload(w, r)
	if !ok {
		return
	}
	defer fh.Close()
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	respond.JSON(w, http.StatusOK, h.resolvePromoCSV(r.Context(), fh))
}

func (h *ProductHandler) resolvePromoCSV(ctx context.Context, r io.Reader) *PromoBundlesCSVResolveResult {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	out := &PromoBundlesCSVResolveResult{Items: []PromoBundleCSVResolveItem{}}
	seen := map[string]bool{}
	rowNum := 0
	headerChecked := false

	for {
		rec, rerr := cr.Read()
		if rerr == io.EOF {
			break
		}
		rowNum++
		if rerr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: rerr.Error()})
			continue
		}
		if catalog.IsBlankRow(rec) {
			continue
		}
		key := strings.TrimSpace(rec[0])
		if rowNum == 1 {
			key = strings.TrimPrefix(key, "\ufeff") // strip UTF-8 BOM
		}
		// First non-blank row: skip it if it looks like a header.
		if !headerChecked {
			headerChecked = true
			if isPromoHeader(key) {
				continue
			}
		}
		if key == "" {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "name or slug is required"})
			continue
		}
		match, mErr := h.svc.LookupBundleProductByNameOrSlug(ctx, key)
		if mErr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: mErr.Error()})
			continue
		}
		if seen[match.ID] {
			continue // dedupe within the same CSV
		}
		seen[match.ID] = true
		out.Items = append(out.Items, PromoBundleCSVResolveItem{ID: match.ID, Name: match.Name, Slug: match.Slug, Status: match.Status})
	}
	return out
}

func isPromoHeader(cell string) bool {
	switch strings.ToLower(strings.TrimSpace(cell)) {
	case "name", "slug", "name/slug", "product", "product name", "product_name":
		return true
	}
	return false
}

// ── 關聯產品 / Up-sell + cross-sell CSV ──────────────────────────────────────

// ProductRefCSVResolveItem is one resolved up-sell / cross-sell association,
// shaped for the admin editor row. Unlike the bundle-items resolver it allows
// any product kind (\u55ae\u54c1 or \u5957\u88dd) and pins a concrete variant (the matched
// variant, or a \u5957\u88dd's default variant). Carries enough detail to render the row
// directly, so CSV import and the search-box picker converge on one client
// "add resolved row" path.
type ProductRefCSVResolveItem struct {
	ProductID       string   `json:"product_id"`
	VariantID       string   `json:"variant_id"`
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	Status          string   `json:"status"`
	Kind            string   `json:"kind"`
	SKU             string   `json:"sku"`
	VariantName     *string  `json:"variant_name,omitempty"`
	Price           *float64 `json:"price,omitempty"`
	CompareAtPrice  *float64 `json:"compare_at_price,omitempty"`
	StockQty        *int     `json:"stock_qty,omitempty"`
	PrimaryImageURL *string  `json:"primary_image_url,omitempty"`
}

// ProductRefsCSVResolveResult is the response for POST
// /admin/products/related-refs/csv-resolve.
type ProductRefsCSVResolveResult struct {
	Items   []ProductRefCSVResolveItem `json:"items"`
	Skipped int                        `json:"skipped"`
	Errors  []CSVRowErr                `json:"errors,omitempty"`
}

// resolveProductRefsCSV backs POST /admin/products/related-refs/csv-resolve.
// Shared by both the up-sell and cross-sell editors: a `name,variant` CSV (same
// grammar as the bundle-items importer minus quantity), each row resolved to one
// variant via MatchVariant with allowBundles=true so \u5957\u88dd resolve to their
// default variant. Always 200 OK with per-row errors.
func (h *ProductHandler) resolveProductRefsCSV(w http.ResponseWriter, r *http.Request) {
	fh, ok := openCSVUpload(w, r)
	if !ok {
		return
	}
	defer fh.Close()
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	respond.JSON(w, http.StatusOK, h.resolveProductRefCSV(r.Context(), fh))
}

func (h *ProductHandler) resolveProductRefCSV(ctx context.Context, r io.Reader) *ProductRefsCSVResolveResult {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err == io.EOF {
		return &ProductRefsCSVResolveResult{Items: []ProductRefCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: "empty file"}}}
	}
	if err != nil {
		return &ProductRefsCSVResolveResult{Items: []ProductRefCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: "read header: " + err.Error()}}}
	}
	if err := catalog.ValidateRefHeader(header); err != nil {
		return &ProductRefsCSVResolveResult{Items: []ProductRefCSVResolveItem{}, Errors: []CSVRowErr{{Row: 1, Message: err.Error()}}}
	}

	out := &ProductRefsCSVResolveResult{Items: []ProductRefCSVResolveItem{}}
	seen := map[string]bool{}
	rowNum := 1
	for {
		rec, rerr := cr.Read()
		if rerr == io.EOF {
			break
		}
		rowNum++
		if rerr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: rerr.Error()})
			continue
		}
		if catalog.IsBlankRow(rec) {
			continue
		}
		if len(rec) < 2 {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "expected 2 columns (name, variant)"})
			continue
		}
		productName := strings.TrimSpace(rec[0])
		variantHint := strings.TrimSpace(rec[1])
		if productName == "" {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "name is required"})
			continue
		}
		variantID, kind, mErr := catalog.MatchVariant(ctx, h.svc.DB(), productName, variantHint, true)
		if mErr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: mErr.Error()})
			continue
		}
		if seen[variantID] {
			continue // dedupe within the same CSV
		}
		seen[variantID] = true
		item, ierr := h.buildProductRefResolvedItem(ctx, variantID, kind)
		if ierr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVRowErr{Row: rowNum, Message: "failed to enrich variant " + variantID + ": " + ierr.Error()})
			continue
		}
		out.Items = append(out.Items, *item)
	}
	return out
}

// buildProductRefResolvedItem enriches a resolved variant id into the editor row
// shape: target product (id, name, slug, status, kind), the pinned variant
// (id, sku, name, price, stock), and a thumbnail (the variant's own image when
// it has one, else the product's primary image).
func (h *ProductHandler) buildProductRefResolvedItem(ctx context.Context, variantID, kind string) (*ProductRefCSVResolveItem, error) {
	var it ProductRefCSVResolveItem
	var variantName, image sql.NullString
	var compareAt sql.NullFloat64
	var price float64
	var stock int
	err := h.svc.DB().QueryRowContext(ctx, `
		SELECT pv.product_id::text, pv.id::text, p.name, p.slug, p.status, p.kind,
		       pv.sku, pv.name, pv.price, pv.compare_at_price, pv.stock_qty,
		       (SELECT COALESCE(
		            CASE WHEN mf.mime_type LIKE 'video/%' THEN mf.thumbnail_url END,
		            mf.webp_url, mf.url, pi.url)
		        FROM product_images pi
		        LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		        WHERE pi.product_id = pv.product_id
		        ORDER BY (pi.variant_id = pv.id) DESC NULLS LAST,
		                 pi.is_primary DESC, pi.sort_order ASC, pi.created_at ASC
		        LIMIT 1) AS image_url
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		WHERE pv.id = $1`, variantID).
		Scan(&it.ProductID, &it.VariantID, &it.Name, &it.Slug, &it.Status, &it.Kind,
			&it.SKU, &variantName, &price, &compareAt, &stock, &image)
	if err != nil {
		return nil, err
	}
	// kind from MatchVariant and p.kind agree; prefer the column value.
	if it.Kind == "" {
		it.Kind = kind
	}
	it.Price = &price
	it.StockQty = &stock
	if variantName.Valid {
		it.VariantName = &variantName.String
	}
	if compareAt.Valid {
		c := compareAt.Float64
		it.CompareAtPrice = &c
	}
	if image.Valid {
		it.PrimaryImageURL = &image.String
	}
	return &it, nil
}

// ── shared lookup + upload helper ───────────────────────────────────────────

// PromoBundleMatch is a minimal bundle-product lookup result for CSV import.
type PromoBundleMatch struct {
	ID     string
	Name   string
	Slug   string
	Status string
}

var (
	ErrBundleProductNotFound  = errors.New("bundle product not found")
	ErrBundleProductAmbiguous = errors.New("ambiguous bundle product name")
)

// LookupBundleProductByNameOrSlug resolves a CSV key to a single active bundle
// product, matching name (case-insensitive, trimmed) first and slug
// (case-insensitive) as a fallback. Used by the 優惠套裝 CSV importer.
func (s *ProductService) LookupBundleProductByNameOrSlug(ctx context.Context, key string) (*PromoBundleMatch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, status FROM products
		  WHERE kind = 'bundle' AND status = 'active'
		    AND (LOWER(TRIM(name)) = LOWER(TRIM($1)) OR LOWER(slug) = LOWER($1))`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []PromoBundleMatch
	for rows.Next() {
		var m PromoBundleMatch
		if err := rows.Scan(&m.ID, &m.Name, &m.Slug, &m.Status); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrBundleProductNotFound
	}
	if len(matches) > 1 {
		return nil, ErrBundleProductAmbiguous
	}
	return &matches[0], nil
}

// openCSVUpload parses the multipart upload and returns the "file" part. On
// failure it writes a 400 and returns ok=false. The caller must Close the file
// and RemoveAll the form. 8 MB body cap — well above any manual-entry CSV.
func openCSVUpload(w http.ResponseWriter, r *http.Request) (multipart.File, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		respond.BadRequest(w, "could not parse multipart form")
		return nil, false
	}
	fh, _, err := r.FormFile("file")
	if err != nil {
		respond.BadRequest(w, "missing 'file' field")
		return nil, false
	}
	return fh, true
}
