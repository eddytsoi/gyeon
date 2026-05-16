package importer

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/media"
	"gyeon/backend/internal/settings"
	"gyeon/backend/internal/shop"
)

// site_settings keys for the saved WC credentials. Mirrors the keys
// inserted by migration 033.
const (
	settingKeyWCURL    = "wc_url"
	settingKeyWCKey    = "wc_consumer_key"
	settingKeyWCSecret = "wc_consumer_secret"
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

// ProductTypeFilter narrows what the import pulls from WooCommerce.
// Each run handles one filter so its delete/cleanup logic stays scoped:
// running "products" never wipes previously imported bundles and vice
// versa. The merchant can re-run with the other filter when ready.
const (
	// ProductTypeProducts (default): WC simple + variable products mapped
	// to Gyeon kind="simple".
	ProductTypeProducts = "products"
	// ProductTypeBundleProducts: WC Product Bundles plugin entries mapped
	// to Gyeon kind="bundle". Components are resolved by wc_product_id
	// against products imported in a previous "products" run.
	ProductTypeBundleProducts = "bundle_products"
)

// ImportRequest holds WooCommerce credentials and import options.
type ImportRequest struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
	// Mode is "upsert" (default) or "replace". Empty falls back to upsert.
	Mode string `json:"mode"`
	// Limit caps the number of products processed in this run. 0 = no cap.
	// Used for partial / smoke-test imports; stale cleanup is skipped when
	// Limit > 0 so a small subset run cannot wipe out the rest of the catalog.
	Limit int `json:"limit"`
	// ProductType chooses what to import: "products" (simple + variable,
	// default) or "bundle_products" (WC Product Bundles plugin entries).
	// One filter per run keeps stale-cleanup scoped to that kind.
	ProductType string `json:"product_type"`
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

// Service orchestrates the WooCommerce → Gyeon product / customers / orders
// import. customerSvc / emailSvc are used only by the customers import to
// optionally send setup-password emails to newly-inserted customers.
type Service struct {
	db          *sql.DB
	categorySvc *shop.CategoryService
	productSvc  *shop.ProductService
	mediaSvc    *media.Service
	settingsSvc *settings.Service
	customerSvc *customers.Service
	emailSvc    *email.Service
}

// NewService creates an import Service. The *sql.DB is used by the
// customers / orders import paths for direct upserts; the products path
// goes through productSvc and never touches db directly.
func NewService(db *sql.DB, categorySvc *shop.CategoryService, productSvc *shop.ProductService, mediaSvc *media.Service, settingsSvc *settings.Service, customerSvc *customers.Service, emailSvc *email.Service) *Service {
	return &Service{
		db:          db,
		categorySvc: categorySvc,
		productSvc:  productSvc,
		mediaSvc:    mediaSvc,
		settingsSvc: settingsSvc,
		customerSvc: customerSvc,
		emailSvc:    emailSvc,
	}
}

// Credentials carries the persisted WooCommerce REST API credentials.
// Empty strings mean nothing has been saved yet.
type Credentials struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
}

// GetCredentials reads the saved WC credentials from site_settings.
// Missing rows are treated as empty strings rather than errors so a
// fresh deployment without migration-seeded rows still returns cleanly.
func (s *Service) GetCredentials(ctx context.Context) (Credentials, error) {
	read := func(key string) string {
		st, err := s.settingsSvc.Get(ctx, key)
		if err != nil || st == nil {
			return ""
		}
		return st.Value
	}
	return Credentials{
		WCURL:    read(settingKeyWCURL),
		WCKey:    read(settingKeyWCKey),
		WCSecret: read(settingKeyWCSecret),
	}, nil
}

// SaveCredentials writes all three values atomically. Empty strings
// clear the corresponding key (admin can reset by saving blanks).
func (s *Service) SaveCredentials(ctx context.Context, c Credentials) error {
	_, err := s.settingsSvc.BulkSet(ctx, map[string]string{
		settingKeyWCURL:    c.WCURL,
		settingKeyWCKey:    c.WCKey,
		settingKeyWCSecret: c.WCSecret,
	})
	return err
}

// TestConnection verifies that the WooCommerce credentials are valid and
// the required API endpoints are reachable. Returns nil on success.
func (s *Service) TestConnection(req ImportRequest) error {
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).testConnection()
}

// ProductTotal returns the WC store's total product count via the
// X-WP-Total header. Returns 0 on any error — the test endpoint already
// validated connectivity, so a missing total is just a UX nicety we can
// surface or skip without breaking the success case. Scoped to the
// requested ProductType so the displayed count matches what will actually
// be imported (the "products" preset sums the simple + variable counts
// instead of using the unfiltered total, which would inflate the
// denominator with bundles / grouped / external).
func (s *Service) ProductTotal(req ImportRequest) int {
	_, countTypes, _ := resolveProductTypeFilter(req.ProductType)
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).fetchProductTotal(countTypes)
}

// resolveProductTypeFilter maps the public product_type to:
//   - fetchType:  the value passed to ?type= when paging (empty = no
//     server-side filter; the loop relies on matchesProductType to skip
//     unwanted rows client-side).
//   - countTypes: the list of WC types whose totals are summed for the
//     progress denominator. The total has to match what the import
//     actually processes — a single-value ?type= can't express
//     "simple OR variable", so we do two cheap count requests instead.
//   - gyeonKind:  the local kind string written to products.kind.
//
// Unknown values fall back to the "products" preset.
func resolveProductTypeFilter(productType string) (fetchType string, countTypes []string, gyeonKind string) {
	switch productType {
	case ProductTypeBundleProducts:
		return "bundle", []string{"bundle"}, "bundle"
	default:
		// "products" preset — server-side ?type= can only carry one value,
		// so we fetch unfiltered and rely on matchesProductType to skip
		// bundles / grouped / external. The denominator, however, sums
		// simple + variable totals so it never inflates past what the
		// client-side filter actually processes.
		return "", []string{"simple", "variable"}, "simple"
	}
}

// matchesProductType reports whether a WC product type matches the
// requested filter. Used to defensively skip rows the WC API may have
// returned despite a server-side ?type= filter (e.g. plugin quirks).
func matchesProductType(productType, wcProductType string) bool {
	switch productType {
	case ProductTypeBundleProducts:
		return wcProductType == "bundle"
	default:
		return wcProductType == "simple" || wcProductType == "variable"
	}
}

// RunStreaming performs the full import, calling send() with a ProgressUpdate
// after each meaningful step. The final call always has Done = true.
func (s *Service) RunStreaming(ctx context.Context, req ImportRequest, send func(ProgressUpdate)) {
	mode := req.Mode
	if mode == "" {
		mode = ModeUpsert
	}
	wcType, countTypes, gyeonKind := resolveProductTypeFilter(req.ProductType)

	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := ProgressUpdate{Errors: []string{}}

	p.TotalProducts = wc.fetchProductTotal(countTypes)
	// When the caller capped the run, show the cap as the denominator so
	// the progress bar represents work scheduled, not the WC store size.
	if req.Limit > 0 && (p.TotalProducts == 0 || p.TotalProducts > req.Limit) {
		p.TotalProducts = req.Limit
	}
	send(p)

	// Replace mode: nuke previously WC-imported rows up front. Scoped to
	// the current kind so a "products" replace doesn't wipe bundles and
	// vice versa. Manual products (wc_product_id IS NULL) survive.
	if mode == ModeReplace {
		if err := s.productSvc.DeleteAllWCImported(ctx, gyeonKind); err != nil {
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

pages:
	for page := 1; ; page++ {
		products, err := wc.fetchProducts(page, wcType)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch products page %d: %v", page, err))
			break
		}
		if len(products) == 0 {
			break
		}
		for _, prod := range products {
			if req.Limit > 0 && p.ProcessedProducts >= req.Limit {
				break pages
			}
			// Defensive client-side filter — server-side ?type= is missing
			// for the "products" preset, and even when set we don't trust
			// every plugin to honour it perfectly.
			if !matchesProductType(req.ProductType, prod.Type) {
				continue
			}
			p.CurrentProduct = prod.Name
			send(p)
			seenWCProductIDs = append(seenWCProductIDs, prod.ID)
			if gyeonKind == "bundle" {
				s.importBundleProduct(ctx, prod, categoryMap, &p)
			} else {
				s.importProduct(ctx, wc, prod, categoryMap, &p)
			}
			p.ProcessedProducts++
			p.CurrentProduct = ""
			send(p)
		}
	}

	// Stale cleanup: delete WC-imported products whose WC ID was not seen
	// in this run. Skipped under Limit > 0 — a partial run hasn't seen the
	// rest of the catalog, so its "seen" set isn't authoritative and the
	// delete would wipe products the run never visited. Scoped to gyeonKind
	// so the other kind's previously-imported rows aren't affected.
	if req.Limit == 0 {
		if n, derr := s.productSvc.DeleteStaleWCProducts(ctx, gyeonKind, seenWCProductIDs); derr != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("delete stale products: %v", derr))
		} else {
			p.StaleDeleted = int(n)
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

// resolveWCCategoryIDs translates a WC product's category slugs into Gyeon
// category UUIDs using categoryMap. The first slug that resolves becomes the
// primary; every other resolving slug is included in the full list (with
// the primary first, dedup preserved by order). Slugs absent from the map
// are silently skipped — the caller has already created missing categories
// in syncCategories before iterating products.
func resolveWCCategoryIDs(prod wcProduct, categoryMap map[string]string) (primary *string, all []string) {
	for _, c := range prod.Categories {
		id, ok := categoryMap[c.Slug]
		if !ok {
			continue
		}
		if primary == nil {
			pid := id
			primary = &pid
		}
		all = append(all, id)
	}
	return primary, all
}

func (s *Service) importProduct(
	ctx context.Context,
	wc *wcClient,
	prod wcProduct,
	categoryMap map[string]string,
	p *ProgressUpdate,
) {
	categoryID, categoryIDs := resolveWCCategoryIDs(prod, categoryMap)

	desc, howToUse, excerpt := buildContentFromMeta(prod)

	// Lookup first: lets us pick INSERT vs UPDATE explicitly so we don't
	// burn a products.number sequence value on every re-imported row
	// (ON CONFLICT DO UPDATE always allocates one before the conflict
	// check). Also feeds the new/updated counter split.
	existingID, lookupErr := s.productSvc.GetIDByWCProductID(ctx, prod.ID)
	existedBefore := lookupErr == nil

	upsertReq := shop.UpsertWCProductRequest{
		WCProductID: prod.ID,
		CategoryID:  categoryID,
		CategoryIDs: categoryIDs,
		Slug:        prod.Slug,
		Name:        prod.Name,
		Subtitle:    metaStringPtr(prod.MetaData, "wc_ps_subtitle"),
		Excerpt:     excerpt,
		Description: desc,
		HowToUse:    howToUse,
		Status:      mapStatus(prod.Status),
		Kind:        "simple",
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

// importBundleProduct handles a WC product whose Type == "bundle". The
// product itself is upserted as kind="bundle" (which auto-seeds the
// BUNDLE-* variant); the bundled_items array is resolved against
// previously-imported component products (matched by wc_product_id) and
// stored atomically via SetBundleItems. Components missing from Gyeon
// are recorded as warnings — the merchant should run "Products" import
// before "Bundle Products" so all components exist.
func (s *Service) importBundleProduct(
	ctx context.Context,
	prod wcProduct,
	categoryMap map[string]string,
	p *ProgressUpdate,
) {
	categoryID, categoryIDs := resolveWCCategoryIDs(prod, categoryMap)

	desc, howToUse, excerpt := buildContentFromMeta(prod)

	existingID, lookupErr := s.productSvc.GetIDByWCProductID(ctx, prod.ID)
	existedBefore := lookupErr == nil

	upsertReq := shop.UpsertWCProductRequest{
		WCProductID: prod.ID,
		CategoryID:  categoryID,
		CategoryIDs: categoryIDs,
		Slug:        prod.Slug,
		Name:        prod.Name,
		Subtitle:    metaStringPtr(prod.MetaData, "wc_ps_subtitle"),
		Excerpt:     excerpt,
		Description: desc,
		HowToUse:    howToUse,
		Status:      mapStatus(prod.Status),
		Kind:        "bundle",
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
		p.Errors = append(p.Errors, fmt.Sprintf("upsert bundle %q: %v", prod.Slug, err))
		p.Failed++
		return
	}

	// Update the bundle's BUNDLE-* variant price from WC. Stock is derived
	// from components by GetDerivedStock, so we don't write it here.
	bundleVariantID, err := s.productSvc.GetBundleVariantID(ctx, productID)
	if err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("locate BUNDLE variant for %q: %v", prod.Slug, err))
	} else {
		price, compareAt := parsePrices(prod.RegularPrice, prod.SalePrice)
		if uerr := s.productSvc.UpdateBundleVariantPrice(ctx, bundleVariantID, price, compareAt); uerr != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("price for bundle %q: %v", prod.Slug, uerr))
		}
		p.ImportedVariants++
	}

	// Resolve bundled_items to Gyeon component variants. Items whose
	// component product hasn't been imported yet are skipped with a
	// warning so the run continues; admin can fill the gap by running
	// "Products" import then re-running "Bundle Products".
	inputs := make([]shop.BundleItemInput, 0, len(prod.BundledItems))
	for _, bi := range prod.BundledItems {
		variantID, ok := s.resolveBundleComponent(ctx, bi)
		if !ok {
			if bi.VariationID != 0 {
				p.Errors = append(p.Errors, fmt.Sprintf(
					"bundle %q: variation %d (product %d) not found — run Products import first or check SKU suffix",
					prod.Slug, bi.VariationID, bi.ProductID))
			} else {
				p.Errors = append(p.Errors, fmt.Sprintf(
					"bundle %q: component WC product %d not found in Gyeon (run Products import first)",
					prod.Slug, bi.ProductID))
			}
			continue
		}
		qty := bi.QuantityDefault
		if qty < 1 {
			qty = 1
		}
		inputs = append(inputs, shop.BundleItemInput{
			ComponentVariantID: variantID,
			Quantity:           qty,
			SortOrder:          bi.MenuOrder,
		})
	}
	if _, err := s.productSvc.SetBundleItems(ctx, productID, inputs); err != nil {
		p.Errors = append(p.Errors, fmt.Sprintf("set bundle_items for %q: %v", prod.Slug, err))
	}

	// Images — same refresh policy as simple/variable: drop WC-sourced,
	// re-add from current WC payload, reuse media_files when URL unchanged.
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

// resolveBundleComponent maps a WC bundled_item to a Gyeon variant ID.
// When the bundle pins a specific variation, the match must be exact —
// either the SKU suffix "{slug}-{wcVariationID}" or the wc_variation_id
// column. Falling back to the product's first active variant would
// silently produce wrong bundle contents, so we refuse instead. The
// fallback is reserved for unpinned items (simple-product components).
func (s *Service) resolveBundleComponent(ctx context.Context, bi wcBundledItem) (string, bool) {
	productID, err := s.productSvc.GetIDByWCProductID(ctx, bi.ProductID)
	if err != nil {
		return "", false
	}
	if bi.VariationID != 0 {
		if id, err := s.productSvc.GetVariantIDByBundleRef(ctx, productID, bi.VariationID); err == nil {
			return id, true
		}
		return "", false
	}
	if id, err := s.productSvc.FindFirstActiveVariantID(ctx, productID); err == nil {
		return id, true
	}
	return "", false
}

func (s *Service) upsertVariantFromSimple(ctx context.Context, productID string, prod wcProduct) error {
	price, compareAt := parsePrices(prod.RegularPrice, prod.SalePrice)
	stockQty := 0
	if prod.StockQuantity != nil {
		stockQty = *prod.StockQuantity
	}
	// SKU is generated from the product slug — WC's own SKU is ignored so
	// every Gyeon variant follows one predictable scheme.
	_, err := s.productSvc.UpsertWCVariant(ctx, productID, shop.UpsertWCVariantRequest{
		WCVariationID:  nil, // simple-product fallback — identified by NULL
		SKU:            prod.Slug,
		Name:           nil, // simple products have no variation attributes
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
		WeightGrams:    parseWeightGrams(prod.Weight),
		LengthMM:       parseDimensionCM(prod.Dimensions.Length),
		WidthMM:        parseDimensionCM(prod.Dimensions.Width),
		HeightMM:       parseDimensionCM(prod.Dimensions.Height),
	})
	return err
}

func (s *Service) upsertVariantFromVariation(ctx context.Context, productID, productSlug string, v wcVariation) error {
	price, compareAt := parsePrices(v.RegularPrice, v.SalePrice)
	stockQty := 0
	if v.StockQuantity != nil {
		stockQty = *v.StockQuantity
	}
	wcID := v.ID
	// SKU is generated from product slug + WC variation ID; WC's own SKU
	// is ignored so every Gyeon variant follows one predictable scheme.
	_, err := s.productSvc.UpsertWCVariant(ctx, productID, shop.UpsertWCVariantRequest{
		WCVariationID:  &wcID,
		SKU:            fmt.Sprintf("%s-%d", productSlug, v.ID),
		Name:           formatVariantName(v.Attributes),
		Price:          price,
		CompareAtPrice: compareAt,
		StockQty:       stockQty,
		WeightGrams:    parseWeightGrams(v.Weight),
		LengthMM:       parseDimensionCM(v.Dimensions.Length),
		WidthMM:        parseDimensionCM(v.Dimensions.Width),
		HeightMM:       parseDimensionCM(v.Dimensions.Height),
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

// formatVariantName joins WC variation attributes into Gyeon's variant
// name format. A single attribute {Name: "容量", Option: "500ml"} becomes
// "容量:500ml"; multiple pairs are joined with " / ". Returns nil if no
// usable pairs are present so the variant's name column stays NULL rather
// than holding an empty string.
func formatVariantName(attrs []wcAttribute) *string {
	parts := make([]string, 0, len(attrs))
	for _, a := range attrs {
		name := strings.TrimSpace(a.Name)
		opt := strings.TrimSpace(a.Option)
		if name == "" || opt == "" {
			continue
		}
		parts = append(parts, name+":"+opt)
	}
	if len(parts) == 0 {
		return nil
	}
	s := strings.Join(parts, " / ")
	return &s
}

// parseWeightGrams parses a WooCommerce weight (decimal string) as grams.
// The merchant must configure WooCommerce → Settings → Products → Weight
// unit = g; we don't read the store-level unit, we just trust the value.
// Returns nil for empty / zero / invalid input so the variant falls back
// to shipany_default_weight_grams instead of being shipped as 0g.
func parseWeightGrams(s string) *int {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return nil
	}
	g := int(f + 0.5)
	if g <= 0 {
		return nil
	}
	return &g
}

// parseDimensionCM converts a WooCommerce dimension string (cm, decimal) to
// *int millimetres. Returns nil for empty / zero / invalid input.
func parseDimensionCM(cm string) *int {
	if cm == "" {
		return nil
	}
	f, err := strconv.ParseFloat(cm, 64)
	if err != nil || f <= 0 {
		return nil
	}
	mm := int(f*10 + 0.5)
	if mm <= 0 {
		return nil
	}
	return &mm
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
