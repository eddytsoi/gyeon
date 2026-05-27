package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// errMediaSlugNotFound is returned by resolveMediaSlug when WP's media
// endpoint returns an empty array for the slug — caller logs + skips.
var errMediaSlugNotFound = errors.New("wp media slug not found")

type wcCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type wcProductCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type wcImage struct {
	ID       int    `json:"id"`
	Src      string `json:"src"`
	Alt      string `json:"alt"`
	Position int    `json:"position"`
}

type wcDimensions struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type wcProduct struct {
	ID            int                 `json:"id"`
	Name          string              `json:"name"`
	Slug          string              `json:"slug"`
	Type          string              `json:"type"`
	Status        string              `json:"status"`
	Description      string              `json:"description"`
	ShortDescription string              `json:"short_description"`
	RegularPrice     string              `json:"regular_price"`
	SalePrice     string              `json:"sale_price"`
	StockQuantity *int                `json:"stock_quantity"`
	Weight        string              `json:"weight"`
	Dimensions    wcDimensions        `json:"dimensions"`
	Categories    []wcProductCategory `json:"categories"`
	Images        []wcImage           `json:"images"`
	Variations    []int               `json:"variations"`
	// BundledItems is populated only when WooCommerce Product Bundles is
	// active and the product Type is "bundle". Each entry references a
	// component product (and optionally a specific variation) along with
	// the default quantity to ship inside the bundle.
	BundledItems []wcBundledItem `json:"bundled_items"`
	// MetaData carries WC custom meta (including ACF fields). We decode
	// values as RawMessage because ACF stores arrays/objects under some
	// keys; the helpers on wcMeta extract the string form when callers
	// only care about scalar fields (title_1, content_1, …).
	MetaData []wcMeta `json:"meta_data"`
}

// wcMeta is one row from a product's meta_data array. Value is left as
// RawMessage because WooCommerce returns mixed types (string, array,
// object) depending on the meta key — string-only callers use String().
type wcMeta struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// String returns the meta value as a string when it was stored as a JSON
// string; arrays / objects / null fall back to "" so callers don't have
// to type-switch every key.
func (m wcMeta) String() string {
	if len(m.Value) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(m.Value, &s); err == nil {
		return s
	}
	return ""
}

// wcBundledItem mirrors a single component row exposed by the WooCommerce
// Product Bundles plugin. We deliberately read only the fields needed to
// resolve the component variant in Gyeon — discount / stock_status / etc.
// are ignored on first-pass import.
//
// Variation pinning is expressed as (override_variations, allowed_variations):
// when override is true and the list is non-empty, the bundle restricts the
// component to those WC variation IDs. WC's REST does NOT emit a single
// `variation_id` field on bundled items.
type wcBundledItem struct {
	BundledItemID      int   `json:"bundled_item_id"`
	ProductID          int   `json:"product_id"`
	OverrideVariations bool  `json:"override_variations"`
	AllowedVariations  []int `json:"allowed_variations"`
	MenuOrder          int   `json:"menu_order"`
	QuantityDefault    int   `json:"quantity_default"`
}

type wcVariation struct {
	ID            int           `json:"id"`
	Status        string        `json:"status"`
	RegularPrice  string        `json:"regular_price"`
	SalePrice     string        `json:"sale_price"`
	StockQuantity *int          `json:"stock_quantity"`
	Weight        string        `json:"weight"`
	Dimensions    wcDimensions  `json:"dimensions"`
	Attributes    []wcAttribute `json:"attributes"`
	// Image is the variation-specific image WC exposes for this variation.
	// Despite the WC docs implying otherwise, /products/{id}/variations
	// never returns nil here: when no image is set on the variation in WC
	// admin, WC silently substitutes the parent product's featured image
	// (images[0]). Callers must compare Image.ID against the parent's
	// featured image ID to detect "no own image."
	Image *wcImage `json:"image"`
}

type wcAttribute struct {
	Name   string `json:"name"`
	Option string `json:"option"`
}

// wcCustomerAddress mirrors the billing / shipping payload returned by
// /wc/v3/customers. Only the fields we map into Gyeon are decoded; ignored
// keys (company, country mapped names, etc.) just get dropped.
type wcCustomerAddress struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Address1  string `json:"address_1"`
	Address2  string `json:"address_2"`
	City      string `json:"city"`
	State     string `json:"state"`
	Postcode  string `json:"postcode"`
	Country   string `json:"country"`
	Phone     string `json:"phone"`
}

type wcCustomer struct {
	ID        int               `json:"id"`
	Email     string            `json:"email"`
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Username  string            `json:"username"`
	Role      string            `json:"role"`
	Billing   wcCustomerAddress `json:"billing"`
	Shipping  wcCustomerAddress `json:"shipping"`
}

// wcOrderBilling adds Email on top of wcCustomerAddress because the order
// payload (unlike the customer payload) includes the billing email of guest
// checkouts. Shipping addresses on orders never carry an email field.
type wcOrderBilling struct {
	wcCustomerAddress
	Email string `json:"email"`
}

// wcDecimal accepts either a JSON string ("5.99") or a JSON number (5.99).
// WC's /wc/v3/orders endpoint emits line_items.price as a number, while
// the sibling totals are strings; the underlying form here stays a string
// so existing parseDecimal callers keep working unchanged.
type wcDecimal string

func (d *wcDecimal) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*d = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*d = wcDecimal(s)
		return nil
	}
	*d = wcDecimal(b)
	return nil
}

type wcLineItem struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ProductID   int       `json:"product_id"`
	VariationID int       `json:"variation_id"`
	Quantity    int       `json:"quantity"`
	SKU         string    `json:"sku"`
	Price       wcDecimal `json:"price"`    // unit price (ex-tax) — WC returns this as a JSON number
	Subtotal    string    `json:"subtotal"` // line subtotal pre-discount, ex-tax
	Total       string    `json:"total"`    // line total post-discount, ex-tax
	TotalTax    string    `json:"total_tax"`
	// MetaData carries WC's per-line-item meta_data array. We need it for the
	// WC Product Bundles plugin which links a bundle parent line item to its
	// component children via _bundle_cart_key (on the parent) and _bundled_by
	// (on each child, value = parent's _bundle_cart_key).
	MetaData []wcMeta `json:"meta_data"`
}

// bundleKeys reads the two meta keys that the WC Product Bundles plugin uses
// to link a bundle parent line item to its component children. cartKey is set
// on parents (and also on each child, identifying them — but we don't use that
// side). bundledBy is set on children only and equals the parent's cartKey;
// its presence is what marks a line item as a bundle child.
func (li wcLineItem) bundleKeys() (cartKey, bundledBy string) {
	for _, m := range li.MetaData {
		switch m.Key {
		case "_bundle_cart_key":
			cartKey = m.String()
		case "_bundled_by":
			bundledBy = m.String()
		}
	}
	return
}

type wcOrder struct {
	ID          int    `json:"id"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	DateCreated string `json:"date_created"`
	// DateCreatedGMT is WC's UTC equivalent of DateCreated. Preferred for
	// orders.created_at so the import is timezone-agnostic — the site-time
	// DateCreated is naive and would be 8h off when the WC store runs in HKT.
	DateCreatedGMT string            `json:"date_created_gmt"`
	Currency       string            `json:"currency"`
	CustomerID     int               `json:"customer_id"`
	CustomerNote   string            `json:"customer_note"`
	Total          string            `json:"total"`
	TotalTax       string            `json:"total_tax"`
	DiscountTotal  string            `json:"discount_total"`
	ShippingTotal  string            `json:"shipping_total"`
	LineItems      []wcLineItem      `json:"line_items"`
	Billing        wcOrderBilling    `json:"billing"`
	Shipping       wcCustomerAddress `json:"shipping"`
}

type wcClient struct {
	baseURL    string
	key        string
	secret     string
	httpClient *http.Client
	// mediaSlugCache memoises slug → wpMediaItem lookups for the lifetime
	// of one import run. WC products often reuse the same media slug
	// across SKUs; without this we'd burn one HTTP round-trip per occurrence.
	mediaSlugMu    sync.Mutex
	mediaSlugCache map[string]wpMediaItem
}

func newWCClient(baseURL, key, secret string) *wcClient {
	return &wcClient{
		baseURL:        baseURL,
		key:            key,
		secret:         secret,
		httpClient:     &http.Client{},
		mediaSlugCache: make(map[string]wpMediaItem),
	}
}

// wpMediaItem is the subset of WP's /wp-json/wp/v2/media response we use
// to resolve a slug to its public source URL when downloading
// banner/media images referenced by product ACF meta.
type wpMediaItem struct {
	ID        int    `json:"id"`
	Slug      string `json:"slug"`
	SourceURL string `json:"source_url"`
	AltText   string `json:"alt_text"`
	MimeType  string `json:"mime_type"`
	Title     struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
}

// resolveMediaSlug resolves a value from a product's ACF meta (banner_1,
// media_2, …) to a WP media attachment. Merchants don't always type a real
// WP slug into those fields — we also see attachment titles, full source
// URLs, and bare attachment IDs. We walk a ladder of strategies in order
// of confidence; only the first that yields a match is used.
//
//  1. URL  → synthesize a wpMediaItem{SourceURL: value}; download path
//     handles arbitrary URLs.
//  2. Digits-only → GET /wp/v2/media/{id}.
//  3. Slug as-is → GET /wp/v2/media?slug={value}.
//  4. WP-sanitized slug ("IK FOAM Pro 2-1" → "ik-foam-pro-2-1").
//  5. Title search → GET /wp/v2/media?search={value}, accept only an
//     exact (case-insensitive) title match to avoid content-search noise.
//
// WP's media endpoint is public-readable, so no auth is sent — the WC
// consumer key/secret would 401 here as WP treats them as a user login.
// Results are memoised per-client (one import run) keyed by the raw input.
func (c *wcClient) resolveMediaSlug(slug string) (wpMediaItem, error) {
	if slug == "" {
		return wpMediaItem{}, errMediaSlugNotFound
	}
	c.mediaSlugMu.Lock()
	if hit, ok := c.mediaSlugCache[slug]; ok {
		c.mediaSlugMu.Unlock()
		return hit, nil
	}
	c.mediaSlugMu.Unlock()

	item, err := c.lookupMedia(slug)
	if err != nil {
		return wpMediaItem{}, err
	}
	c.mediaSlugMu.Lock()
	c.mediaSlugCache[slug] = item
	c.mediaSlugMu.Unlock()
	return item, nil
}

func (c *wcClient) lookupMedia(value string) (wpMediaItem, error) {
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return wpMediaItem{SourceURL: value}, nil
	}
	if id, err := strconv.Atoi(value); err == nil && id > 0 {
		if item, ok, err := c.fetchMediaByID(id); err != nil {
			return wpMediaItem{}, err
		} else if ok {
			return item, nil
		}
	}
	if item, ok, err := c.fetchMediaBySlug(value); err != nil {
		return wpMediaItem{}, err
	} else if ok {
		return item, nil
	}
	if sanitized := wpSanitizeTitle(value); sanitized != "" && sanitized != value {
		if item, ok, err := c.fetchMediaBySlug(sanitized); err != nil {
			return wpMediaItem{}, err
		} else if ok {
			return item, nil
		}
	}
	if item, ok, err := c.searchMediaByTitle(value); err != nil {
		return wpMediaItem{}, err
	} else if ok {
		return item, nil
	}
	return wpMediaItem{}, errMediaSlugNotFound
}

func (c *wcClient) fetchMediaBySlug(slug string) (wpMediaItem, bool, error) {
	u := c.baseURL + "/wp-json/wp/v2/media?slug=" + url.QueryEscape(slug)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return wpMediaItem{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wpMediaItem{}, false, fmt.Errorf("wp media lookup %q: status %d", slug, resp.StatusCode)
	}
	var items []wpMediaItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return wpMediaItem{}, false, fmt.Errorf("wp media lookup %q: decode: %w", slug, err)
	}
	if len(items) == 0 {
		return wpMediaItem{}, false, nil
	}
	// WP can return multiple matches when an attachment slug collides with
	// another's numeric suffix; first match wins (newest on default order).
	return items[0], true, nil
}

func (c *wcClient) fetchMediaByID(id int) (wpMediaItem, bool, error) {
	u := c.baseURL + "/wp-json/wp/v2/media/" + strconv.Itoa(id)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return wpMediaItem{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return wpMediaItem{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return wpMediaItem{}, false, fmt.Errorf("wp media id %d: status %d", id, resp.StatusCode)
	}
	var item wpMediaItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return wpMediaItem{}, false, fmt.Errorf("wp media id %d: decode: %w", id, err)
	}
	if item.SourceURL == "" {
		return wpMediaItem{}, false, nil
	}
	return item, true, nil
}

func (c *wcClient) searchMediaByTitle(title string) (wpMediaItem, bool, error) {
	u := c.baseURL + "/wp-json/wp/v2/media?per_page=10&search=" + url.QueryEscape(title)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return wpMediaItem{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wpMediaItem{}, false, fmt.Errorf("wp media search %q: status %d", title, resp.StatusCode)
	}
	var items []wpMediaItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return wpMediaItem{}, false, fmt.Errorf("wp media search %q: decode: %w", title, err)
	}
	for _, it := range items {
		// title.rendered is HTML-escaped (&amp; etc.) — unescape before
		// comparing so values containing entities still match.
		decoded := html.UnescapeString(it.Title.Rendered)
		if strings.EqualFold(strings.TrimSpace(decoded), strings.TrimSpace(title)) {
			return it, true, nil
		}
	}
	return wpMediaItem{}, false, nil
}

// wpSanitizeTitle approximates WordPress's sanitize_title_with_dashes for
// ASCII inputs: lowercase, collapse whitespace/underscore/slash runs into
// single dashes, drop any other non-[a-z0-9-] character, then trim/collapse
// dashes. For "IK FOAM Pro 2-1" this produces "ik-foam-pro-2-1", matching
// the slug WP would have assigned the attachment at upload time.
//
// CJK and other non-ASCII inputs collapse to "" — the caller treats that
// as "skip this strategy" and falls through to the title-search step.
func wpSanitizeTitle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ', r == '\t', r == '\n', r == '_', r == '/', r == '\\', r == '-':
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (c *wcClient) get(path string, params url.Values, out interface{}) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.key, c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WooCommerce API returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// fetchProductTotal returns the total number of products to be processed
// in a run, summed across the given WC types. WC's ?type= parameter only
// accepts a single value, so we issue one cheap (per_page=1) request per
// type and add the X-WP-Total values together. Passing nil / empty falls
// back to an unfiltered count (every product type) — this preserves the
// behaviour for callers that don't care about a type-aware denominator.
func (c *wcClient) fetchProductTotal(wcTypes []string) int {
	if len(wcTypes) == 0 {
		return c.fetchTypedProductTotal("")
	}
	total := 0
	for _, t := range wcTypes {
		total += c.fetchTypedProductTotal(t)
	}
	return total
}

func (c *wcClient) fetchTypedProductTotal(wcType string) int {
	u := c.baseURL + "/wp-json/wc/v3/products?per_page=1&page=1"
	if wcType != "" {
		u += "&type=" + url.QueryEscape(wcType)
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0
	}
	req.SetBasicAuth(c.key, c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	return total
}

// testConnection verifies credentials by fetching one product and one category.
// Returns an error describing the first failure, nil if both succeed.
func (c *wcClient) testConnection() error {
	var products []wcProduct
	params := url.Values{"per_page": {"1"}, "page": {"1"}}
	if err := c.get("/wp-json/wc/v3/products", params, &products); err != nil {
		return fmt.Errorf("無法讀取商品列表：%w", err)
	}
	var cats []wcCategory
	if err := c.get("/wp-json/wc/v3/products/categories", params, &cats); err != nil {
		return fmt.Errorf("無法讀取分類列表：%w", err)
	}
	return nil
}

func (c *wcClient) fetchCategories() ([]wcCategory, error) {
	var all []wcCategory
	for page := 1; ; page++ {
		var batch []wcCategory
		params := url.Values{"per_page": {"100"}, "page": {fmt.Sprintf("%d", page)}}
		if err := c.get("/wp-json/wc/v3/products/categories", params, &batch); err != nil {
			return nil, fmt.Errorf("categories page %d: %w", page, err)
		}
		for i := range batch {
			batch[i].Slug = decodeSlug(batch[i].Slug)
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

// fetchProducts returns one page of products. When wcType is non-empty
// (e.g. "bundle"), WooCommerce filters server-side to that type so we
// don't waste bandwidth pulling products we'll skip client-side.
func (c *wcClient) fetchProducts(page int, wcType string) ([]wcProduct, error) {
	var products []wcProduct
	params := url.Values{"per_page": {"100"}, "page": {fmt.Sprintf("%d", page)}}
	if wcType != "" {
		params.Set("type", wcType)
	}
	if err := c.get("/wp-json/wc/v3/products", params, &products); err != nil {
		return nil, fmt.Errorf("products page %d: %w", page, err)
	}
	for i := range products {
		products[i].Slug = decodeSlug(products[i].Slug)
		for j := range products[i].Categories {
			products[i].Categories[j].Slug = decodeSlug(products[i].Categories[j].Slug)
		}
	}
	return products, nil
}

// fetchProduct returns a single product by its WooCommerce ID. Used by
// the single-product import path; the multi-product path uses fetchProducts
// with pagination. Slug + category slugs are normalized the same way as
// fetchProducts so downstream matching stays aligned.
func (c *wcClient) fetchProduct(id int) (wcProduct, error) {
	var prod wcProduct
	if err := c.get(fmt.Sprintf("/wp-json/wc/v3/products/%d", id), nil, &prod); err != nil {
		return wcProduct{}, fmt.Errorf("product %d: %w", id, err)
	}
	prod.Slug = decodeSlug(prod.Slug)
	for j := range prod.Categories {
		prod.Categories[j].Slug = decodeSlug(prod.Categories[j].Slug)
	}
	return prod, nil
}

// decodeSlug normalizes WooCommerce-returned slugs. WC stores non-ASCII
// slugs as percent-encoded UTF-8 (e.g. "%e7%b6%a0%e8%8c%b6"), but browsers
// decode URLs so SvelteKit's params.slug is the decoded form. Storing the
// decoded slug keeps both sides aligned. Falls back to the original on
// decode failure.
func decodeSlug(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

// fetchCustomer returns a single customer by its WooCommerce ID. Used by
// the single-customer import path so an admin can re-sync one row without
// paging through the whole store; the multi-customer path uses
// fetchCustomers with pagination.
func (c *wcClient) fetchCustomer(id int) (wcCustomer, error) {
	var cust wcCustomer
	if err := c.get(fmt.Sprintf("/wp-json/wc/v3/customers/%d", id), nil, &cust); err != nil {
		return wcCustomer{}, fmt.Errorf("customer %d: %w", id, err)
	}
	return cust, nil
}

// fetchCustomerTotal returns the total number of customers via the
// X-WP-Total header. Returns 0 on any error — caller treats a missing
// total as "unknown" rather than failing the whole import.
func (c *wcClient) fetchCustomerTotal() int {
	u := c.baseURL + "/wp-json/wc/v3/customers?per_page=1&page=1&role=all"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0
	}
	req.SetBasicAuth(c.key, c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	return total
}

// fetchCustomers returns one page of customers. role=all is required —
// /wc/v3/customers defaults to role=customer and silently hides admins
// and other roles, which would surprise merchants whose WC store has
// non-customer accounts they want migrated.
func (c *wcClient) fetchCustomers(page int) ([]wcCustomer, error) {
	var customers []wcCustomer
	params := url.Values{
		"per_page": {"100"},
		"page":     {fmt.Sprintf("%d", page)},
		"role":     {"all"},
		"orderby":  {"id"},
		"order":    {"asc"},
	}
	if err := c.get("/wp-json/wc/v3/customers", params, &customers); err != nil {
		return nil, fmt.Errorf("customers page %d: %w", page, err)
	}
	return customers, nil
}

// fetchOrderTotal returns the WC store's total order count via X-WP-Total,
// filtered by the given WC status (empty → "any") and optional calendar
// year (0 = no year filter). 0 on any error.
func (c *wcClient) fetchOrderTotal(status string, year int) int {
	if status == "" {
		status = "any"
	}
	params := url.Values{
		"per_page": {"1"},
		"page":     {"1"},
		"status":   {status},
	}
	applyOrderYearFilter(params, year)
	u := c.baseURL + "/wp-json/wc/v3/orders?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0
	}
	req.SetBasicAuth(c.key, c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	return total
}

// fetchOrders returns one page of orders, oldest first so the import
// processes history in chronological order. The status param filters
// server-side ("any" returns trash/draft too — orchestrator still filters
// them via mapWCOrderStatus). year > 0 restricts to that calendar year via
// WC's after/before params.
func (c *wcClient) fetchOrders(page int, status string, year int) ([]wcOrder, error) {
	if status == "" {
		status = "any"
	}
	var orders []wcOrder
	params := url.Values{
		"per_page": {"100"},
		"page":     {fmt.Sprintf("%d", page)},
		"status":   {status},
		"orderby":  {"id"},
		"order":    {"asc"},
	}
	applyOrderYearFilter(params, year)
	if err := c.get("/wp-json/wc/v3/orders", params, &orders); err != nil {
		return nil, fmt.Errorf("orders page %d: %w", page, err)
	}
	return orders, nil
}

// applyOrderYearFilter mutates params to bracket the given calendar year
// using WC's after/before ISO 8601 params (interpreted in the site's
// timezone server-side). year <= 0 is a no-op.
func applyOrderYearFilter(params url.Values, year int) {
	if year <= 0 {
		return
	}
	params.Set("after", fmt.Sprintf("%04d-01-01T00:00:00", year))
	params.Set("before", fmt.Sprintf("%04d-01-01T00:00:00", year+1))
}

func (c *wcClient) fetchVariations(productID int) ([]wcVariation, error) {
	var all []wcVariation
	for page := 1; ; page++ {
		var batch []wcVariation
		params := url.Values{"per_page": {"100"}, "page": {fmt.Sprintf("%d", page)}}
		path := fmt.Sprintf("/wp-json/wc/v3/products/%d/variations", productID)
		if err := c.get(path, params, &batch); err != nil {
			return nil, fmt.Errorf("variations page %d: %w", page, err)
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}
