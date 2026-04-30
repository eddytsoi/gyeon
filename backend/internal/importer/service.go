package importer

import (
	"context"
	"fmt"
	"strconv"

	"gyeon/backend/internal/media"
	"gyeon/backend/internal/shop"
)

// ImportRequest holds WooCommerce credentials and import options.
type ImportRequest struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
	ClearAll bool   `json:"clear_all"`
}

// ProgressUpdate is sent via SSE for every meaningful step of the import.
type ProgressUpdate struct {
	TotalProducts     int      `json:"total_products"`
	ProcessedProducts int      `json:"processed_products"` // imported + skipped + failed so far
	ImportedProducts  int      `json:"imported_products"`
	ImportedVariants  int      `json:"imported_variants"`
	Skipped           int      `json:"skipped"`
	SkippedDetails    []string `json:"skipped_details"` // human-readable reason per skipped product
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
	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := ProgressUpdate{Errors: []string{}, SkippedDetails: []string{}}

	// Fetch total product count for the progress bar denominator.
	p.TotalProducts = wc.fetchProductTotal()
	send(p)

	// Clear existing products before import.
	if req.ClearAll {
		if err := s.productSvc.DeleteAll(ctx); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("clear products: %v", err))
			p.Done = true
			send(p)
			return
		}
	}

	// Build category slug→ID map, creating missing categories.
	categoryMap, err := s.syncCategories(ctx, wc, &p)
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("category sync: %v", err))
		p.Done = true
		send(p)
		return
	}

	// Build existing-slug set for dedup.
	existingSlugs, err := s.fetchExistingSlugs(ctx)
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("fetch existing products: %v", err))
		p.Done = true
		send(p)
		return
	}

	// Paginate and import products.
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
			s.importProduct(ctx, wc, prod, categoryMap, existingSlugs, &p)
			p.ProcessedProducts++
			p.CurrentProduct = ""
			send(p)
		}
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

func (s *Service) fetchExistingSlugs(ctx context.Context) (map[string]bool, error) {
	slugs := make(map[string]bool)
	for offset := 0; ; offset += 100 {
		products, err := s.productSvc.ListAll(ctx, "", "", 100, offset)
		if err != nil {
			return nil, err
		}
		for _, prod := range products {
			slugs[prod.Slug] = true
		}
		if len(products) < 100 {
			break
		}
	}
	return slugs, nil
}

func (s *Service) importProduct(
	ctx context.Context,
	wc *wcClient,
	prod wcProduct,
	categoryMap map[string]string,
	existingSlugs map[string]bool,
	p *ProgressUpdate,
) {
	if existingSlugs[prod.Slug] {
		p.Skipped++
		p.SkippedDetails = append(p.SkippedDetails,
			fmt.Sprintf("%s — 商品已存在（slug: %s）", prod.Name, prod.Slug))
		return
	}

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

	created, err := s.productSvc.Create(ctx, shop.CreateProductRequest{
		CategoryID:  categoryID,
		Slug:        prod.Slug,
		Name:        prod.Name,
		Description: desc,
	})
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("create product %q: %v", prod.Slug, err))
		p.Failed++
		return
	}
	productID := created.ID
	existingSlugs[prod.Slug] = true

	// Variants
	variantsBefore := p.ImportedVariants
	if prod.Type == "variable" {
		variations, err := wc.fetchVariations(prod.ID)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch variations for %q: %v", prod.Slug, err))
			p.Failed++
			return
		}
		for _, v := range variations {
			if err := s.createVariantFromVariation(ctx, productID, prod.Slug, v); err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("variant for %q (id=%d): %v", prod.Slug, v.ID, err))
			} else {
				p.ImportedVariants++
			}
		}
	} else {
		if err := s.createVariantFromSimple(ctx, productID, prod); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("variant for %q: %v", prod.Slug, err))
		} else {
			p.ImportedVariants++
		}
	}
	_ = variantsBefore

	// Images
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
		mediaID, err := s.mediaSvc.DownloadAndStore(ctx, img.Src, img.Alt)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("media download for %q: %v", prod.Slug, err))
		} else {
			req.MediaFileID = &mediaID
		}
		if _, err := s.productSvc.AddImage(ctx, productID, req); err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("image for %q: %v", prod.Slug, err))
		}
	}

	p.ImportedProducts++
}

func (s *Service) createVariantFromSimple(ctx context.Context, productID string, prod wcProduct) error {
	price, compareAt := parsePrices(prod.RegularPrice, prod.SalePrice)
	stockQty := 0
	if prod.StockQuantity != nil {
		stockQty = *prod.StockQuantity
	}
	sku := prod.SKU
	if sku == "" {
		sku = prod.Slug
	}
	_, err := s.productSvc.CreateVariant(ctx, productID, shop.CreateVariantRequest{
		SKU:            sku,
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
	})
	return err
}

func (s *Service) createVariantFromVariation(ctx context.Context, productID, productSlug string, v wcVariation) error {
	price, compareAt := parsePrices(v.RegularPrice, v.SalePrice)
	stockQty := 0
	if v.StockQuantity != nil {
		stockQty = *v.StockQuantity
	}
	sku := v.SKU
	if sku == "" {
		sku = fmt.Sprintf("%s-%d", productSlug, v.ID)
	}
	_, err := s.productSvc.CreateVariant(ctx, productID, shop.CreateVariantRequest{
		SKU:            sku,
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
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
