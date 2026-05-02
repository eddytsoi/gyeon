package importer

import (
	"context"
	"fmt"
	"strconv"

	"gyeon/backend/internal/media"
	"gyeon/backend/internal/shop"
)

// ImportMode controls how an existing Gyeon product matches a WC product
// during re-import. Both modes preserve manually-created products
// (wc_product_id IS NULL); they differ only in how they treat
// previously-imported rows.
const (
	// ModeUpsert (default): match by wc_product_id; UPDATE in place. Admin
	// edits to translations / images / extra variants survive.
	ModeUpsert = "upsert"
	// ModeReplace: DELETE all wc_product_id IS NOT NULL rows first, then
	// re-import fresh. Admin edits to imported rows are lost. Useful when
	// the merchant wants to start over.
	ModeReplace = "replace"
)

// ImportRequest holds WooCommerce credentials and import options.
type ImportRequest struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
	// Mode is "upsert" (default) or "replace". Empty falls back to upsert.
	Mode string `json:"mode"`
}

// ProgressUpdate is sent via SSE for every meaningful step of the import.
type ProgressUpdate struct {
	TotalProducts     int      `json:"total_products"`
	ProcessedProducts int      `json:"processed_products"` // imported + updated + failed so far
	ImportedProducts  int      `json:"imported_products"`  // newly inserted
	UpdatedProducts   int      `json:"updated_products"`   // matched by wc_product_id, updated in place
	ImportedVariants  int      `json:"imported_variants"`  // new + updated variants combined
	StaleDeleted      int      `json:"stale_deleted"`      // WC-imported products no longer present in WC
	Failed            int      `json:"failed"`
	CurrentProduct    string   `json:"current_product,omitempty"`
	Done              bool     `json:"done"`
	Errors            []string `json:"errors"`
}

// Service orchestrates the WooCommerce → Gyeon product import.
type Service struct {
	categorySvc *shop.CategoryService
	productSvc  *shop.ProductService
	mediaSvc    *media.Service
}

// NewService creates an import Service.
func NewService(categorySvc *shop.CategoryService, productSvc *shop.ProductService, mediaSvc *media.Service) *Service {
	return &Service{categorySvc: categorySvc, productSvc: productSvc, mediaSvc: mediaSvc}
}

// TestConnection verifies that the WooCommerce credentials are valid and
// the required API endpoints are reachable. Returns nil on success.
func (s *Service) TestConnection(req ImportRequest) error {
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).testConnection()
}

// RunStreaming performs the full import, calling send() with a ProgressUpdate
// after each meaningful step. The final call always has Done = true.
func (s *Service) RunStreaming(ctx context.Context, req ImportRequest, send func(ProgressUpdate)) {
	mode := req.Mode
	if mode == "" {
		mode = ModeUpsert
	}

	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := ProgressUpdate{Errors: []string{}}

	p.TotalProducts = wc.fetchProductTotal()
	send(p)

	// Replace mode: nuke all previously WC-imported rows up front.
	// Manual products (wc_product_id IS NULL) survive.
	if mode == ModeReplace {
		if err := s.productSvc.DeleteAllWCImported(ctx); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("clear WC-imported products: %v", err))
			p.Done = true
			send(p)
			return
		}
	}

	categoryMap, err := s.syncCategories(ctx, wc, &p)
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("category sync: %v", err))
		p.Done = true
		send(p)
		return
	}

	// Track every WC product ID we see so we can delete stale rows after
	// the run finishes (products that no longer exist in WC).
	seenWCProductIDs := make([]int, 0, p.TotalProducts)

	for page := 1; ; page++ {
		products, err := wc.fetchProducts(page)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch products page %d: %v", page, err))
			break
		}
		if len(products) == 0 {
			break
		}
		for _, prod := range products {
			p.CurrentProduct = prod.Name
			send(p)
			seenWCProductIDs = append(seenWCProductIDs, prod.ID)
			s.importProduct(ctx, wc, prod, categoryMap, &p)
			p.ProcessedProducts++
			p.CurrentProduct = ""
			send(p)
		}
	}

	// Stale cleanup: delete WC-imported products whose WC ID was not seen
	// in this run. In replace mode this is a no-op (table was wiped), but
	// we still call it so the field has a meaningful value.
	if n, derr := s.productSvc.DeleteStaleWCProducts(ctx, seenWCProductIDs); derr != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("delete stale products: %v", derr))
	} else {
		p.StaleDeleted = int(n)
	}

	p.Done = true
	send(p)
}

func (s *Service) syncCategories(ctx context.Context, wc *wcClient, p *ProgressUpdate) (map[string]string, error) {
	gyeonCats, err := s.categorySvc.List(ctx)
	if err != nil {
		return nil, err
	}
	catMap := make(map[string]string, len(gyeonCats))
	for _, c := range gyeonCats {
		catMap[c.Slug] = c.ID
	}

	wcCats, err := wc.fetchCategories()
	if err != nil {
		return nil, err
	}
	for _, wcat := range wcCats {
		if _, ok := catMap[wcat.Slug]; ok {
			continue
		}
		created, err := s.categorySvc.Create(ctx, shop.CreateCategoryRequest{
			Slug: wcat.Slug,
			Name: wcat.Name,
		})
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("create category %q: %v", wcat.Slug, err))
			continue
		}
		catMap[wcat.Slug] = created.ID
	}
	return catMap, nil
}

func (s *Service) importProduct(
	ctx context.Context,
	wc *wcClient,
	prod wcProduct,
	categoryMap map[string]string,
	p *ProgressUpdate,
) {
	var categoryID *string
	if len(prod.Categories) > 0 {
		if id, ok := categoryMap[prod.Categories[0].Slug]; ok {
			categoryID = &id
		}
	}

	var desc *string
	if prod.Description != "" {
		desc = &prod.Description
	}

	// Lookup first: lets us pick INSERT vs UPDATE explicitly so we don't
	// burn a products.number sequence value on every re-imported row
	// (ON CONFLICT DO UPDATE always allocates one before the conflict
	// check). Also feeds the new/updated counter split.
	existingID, lookupErr := s.productSvc.GetIDByWCProductID(ctx, prod.ID)
	existedBefore := lookupErr == nil

	upsertReq := shop.UpsertWCProductRequest{
		WCProductID: prod.ID,
		CategoryID:  categoryID,
		Slug:        prod.Slug,
		Name:        prod.Name,
		Description: desc,
		Status:      mapStatus(prod.Status),
	}
	var productID string
	var err error
	if existedBefore {
		productID = existingID
		err = s.productSvc.UpdateWCProduct(ctx, productID, upsertReq)
	} else {
		productID, err = s.productSvc.CreateWCProduct(ctx, upsertReq)
	}
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("upsert product %q: %v", prod.Slug, err))
		p.Failed++
		return
	}

	// Variants — Gyeon requires at least one variant per product.
	// Variable products map each WC variation to a Gyeon variant; everything
	// else (simple, grouped, external, or variable-with-no-variations) gets
	// one default variant built from the product-level fields.
	seenVariationIDs := make([]int, 0)
	variantCount := 0
	if prod.Type == "variable" {
		variations, err := wc.fetchVariations(prod.ID)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch variations for %q: %v", prod.Slug, err))
		}
		for _, v := range variations {
			if err := s.upsertVariantFromVariation(ctx, productID, prod.Slug, v); err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("variant for %q (id=%d): %v", prod.Slug, v.ID, err))
				continue
			}
			seenVariationIDs = append(seenVariationIDs, v.ID)
			p.ImportedVariants++
			variantCount++
		}
		// A variable product can keep its variations only — drop any leftover
		// simple-fallback variant from a previous "simple → variable" lifecycle.
		if variantCount > 0 {
			if err := s.productSvc.DeleteSimpleWCVariant(ctx, productID); err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("drop simple variant for %q: %v", prod.Slug, err))
			}
		}
	}
	if variantCount == 0 {
		if err := s.upsertVariantFromSimple(ctx, productID, prod); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("default variant for %q: %v", prod.Slug, err))
			// Roll back: a product with zero variants violates Gyeon's invariant.
			// (Only roll back if the product was newly inserted in this run —
			// preserving an existing product with translations is more important
			// than the invariant being temporarily violated; admin can fix.)
			if !existedBefore {
				if delErr := s.productSvc.Delete(ctx, productID); delErr != nil {
					p.Errors = append(p.Errors, fmt.Sprintf("rollback orphan product %q: %v", prod.Slug, delErr))
				}
			}
			p.Failed++
			return
		}
		p.ImportedVariants++
	}

	// Stale variant cleanup for this product: drop any wc_variation_id that
	// is no longer in the WC variations list (variation removed in WC).
	// Manually-added variants (wc_variation_id IS NULL) are kept, except we
	// already explicitly handled the simple-fallback above.
	if prod.Type == "variable" && variantCount > 0 {
		if _, derr := s.productSvc.DeleteStaleWCVariants(ctx, productID, seenVariationIDs); derr != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("delete stale variants for %q: %v", prod.Slug, derr))
		}
	}

	// Images — only refresh the WC-sourced ones; admin uploads (source_url
	// IS NULL) survive. We delete then re-add so removed-in-WC images
	// disappear from Gyeon as well, and re-use existing media_files when
	// the URL is unchanged so we don't re-download.
	if err := s.productSvc.DeleteWCSourcedImages(ctx, productID); err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("clear WC images for %q: %v", prod.Slug, err))
	}
	for _, img := range prod.Images {
		var alt *string
		if img.Alt != "" {
			alt = &img.Alt
		}
		req := shop.AddImageRequest{
			URL:       &img.Src,
			AltText:   alt,
			SortOrder: img.Position,
			IsPrimary: img.Position == 0,
		}
		// Reuse existing media_files row if we've downloaded this URL before.
		if id, ok := s.mediaSvc.FindIDBySourceURL(ctx, img.Src); ok {
			req.MediaFileID = &id
		} else {
			mediaID, err := s.mediaSvc.DownloadAndStore(ctx, img.Src, img.Alt)
			if err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("media download for %q: %v", prod.Slug, err))
			} else {
				req.MediaFileID = &mediaID
			}
		}
		if _, err := s.productSvc.AddImage(ctx, productID, req); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("image for %q: %v", prod.Slug, err))
		}
	}

	if existedBefore {
		p.UpdatedProducts++
	} else {
		p.ImportedProducts++
	}
}

func (s *Service) upsertVariantFromSimple(ctx context.Context, productID string, prod wcProduct) error {
	price, compareAt := parsePrices(prod.RegularPrice, prod.SalePrice)
	stockQty := 0
	if prod.StockQuantity != nil {
		stockQty = *prod.StockQuantity
	}
	sku := prod.SKU
	if sku == "" {
		sku = prod.Slug
	}
	_, err := s.productSvc.UpsertWCVariant(ctx, productID, shop.UpsertWCVariantRequest{
		WCVariationID:  nil, // simple-product fallback — identified by NULL
		SKU:            sku,
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
		WeightGrams:    parseWeightKg(prod.Weight),
	})
	return err
}

func (s *Service) upsertVariantFromVariation(ctx context.Context, productID, productSlug string, v wcVariation) error {
	price, compareAt := parsePrices(v.RegularPrice, v.SalePrice)
	stockQty := 0
	if v.StockQuantity != nil {
		stockQty = *v.StockQuantity
	}
	sku := v.SKU
	if sku == "" {
		sku = fmt.Sprintf("%s-%d", productSlug, v.ID)
	}
	wcID := v.ID
	_, err := s.productSvc.UpsertWCVariant(ctx, productID, shop.UpsertWCVariantRequest{
		WCVariationID:  &wcID,
		SKU:            sku,
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
		WeightGrams:    parseWeightKg(v.Weight),
	})
	return err
}

// parsePrices converts WC price strings to Gyeon price/compareAtPrice.
func parsePrices(regularPrice, salePrice string) (price float64, compareAt *float64) {
	regular, _ := strconv.ParseFloat(regularPrice, 64)
	if salePrice != "" && salePrice != regularPrice {
		sale, err := strconv.ParseFloat(salePrice, 64)
		if err == nil && sale < regular {
			compareAt = &regular
			return sale, compareAt
		}
	}
	return regular, nil
}

// parseWeightKg converts a WooCommerce weight (kg, decimal string) to *int
// grams. Returns nil for empty / zero / invalid input so the variant falls
// back to shipany_default_weight_grams instead of being shipped as 0g.
func parseWeightKg(kg string) *int {
	if kg == "" {
		return nil
	}
	f, err := strconv.ParseFloat(kg, 64)
	if err != nil || f <= 0 {
		return nil
	}
	g := int(f*1000 + 0.5)
	if g <= 0 {
		return nil
	}
	return &g
}

// mapStatus translates a WooCommerce product status to Gyeon's. WC uses
// publish/draft/pending/private; only "publish" maps to "active" — everything
// else is imported as "inactive" so the merchant can review before exposing it.
func mapStatus(wcStatus string) string {
	if wcStatus == "publish" {
		return "active"
	}
	return "inactive"
}
