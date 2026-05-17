package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
	RegularPrice  string        `json:"regular_price"`
	SalePrice     string        `json:"sale_price"`
	StockQuantity *int          `json:"stock_quantity"`
	Weight        string        `json:"weight"`
	Dimensions    wcDimensions  `json:"dimensions"`
	Attributes    []wcAttribute `json:"attributes"`
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

type wcLineItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProductID   int    `json:"product_id"`
	VariationID int    `json:"variation_id"`
	Quantity    int    `json:"quantity"`
	SKU         string `json:"sku"`
	Price       string `json:"price"`    // unit price (ex-tax)
	Subtotal    string `json:"subtotal"` // line subtotal pre-discount, ex-tax
	Total       string `json:"total"`    // line total post-discount, ex-tax
	TotalTax    string `json:"total_tax"`
}

type wcOrder struct {
	ID            int               `json:"id"`
	Number        string            `json:"number"`
	Status        string            `json:"status"`
	DateCreated   string            `json:"date_created"`
	Currency      string            `json:"currency"`
	CustomerID    int               `json:"customer_id"`
	CustomerNote  string            `json:"customer_note"`
	Total         string            `json:"total"`
	TotalTax      string            `json:"total_tax"`
	DiscountTotal string            `json:"discount_total"`
	ShippingTotal string            `json:"shipping_total"`
	LineItems     []wcLineItem      `json:"line_items"`
	Billing       wcOrderBilling    `json:"billing"`
	Shipping      wcCustomerAddress `json:"shipping"`
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
}

// resolveMediaSlug looks up a WP media attachment by its slug and returns
// the matched item. WP's media endpoint is public-readable, so no auth
// is sent — sending WC consumer key/secret here would fail with 401
// because WP REST treats them as a WP user login attempt.
//
// Results are memoised per-client (one import run). errMediaSlugNotFound
// is returned when WP returns an empty list.
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

	u := c.baseURL + "/wp-json/wp/v2/media?slug=" + url.QueryEscape(slug)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return wpMediaItem{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wpMediaItem{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wpMediaItem{}, fmt.Errorf("wp media lookup %q: status %d", slug, resp.StatusCode)
	}
	var items []wpMediaItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return wpMediaItem{}, fmt.Errorf("wp media lookup %q: decode: %w", slug, err)
	}
	if len(items) == 0 {
		return wpMediaItem{}, errMediaSlugNotFound
	}
	// WP can return multiple matches when an attachment slug collides with
	// another's numeric suffix; first match wins (newest on default order).
	item := items[0]
	c.mediaSlugMu.Lock()
	c.mediaSlugCache[slug] = item
	c.mediaSlugMu.Unlock()
	return item, nil
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

// fetchOrderTotal returns the WC store's total order count via X-WP-Total.
// 0 on any error.
func (c *wcClient) fetchOrderTotal() int {
	u := c.baseURL + "/wp-json/wc/v3/orders?per_page=1&page=1&status=any"
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
// processes history in chronological order. status=any so trash and
// draft are also fetched (we filter them out in the orchestrator).
func (c *wcClient) fetchOrders(page int) ([]wcOrder, error) {
	var orders []wcOrder
	params := url.Values{
		"per_page": {"100"},
		"page":     {fmt.Sprintf("%d", page)},
		"status":   {"any"},
		"orderby":  {"id"},
		"order":    {"asc"},
	}
	if err := c.get("/wp-json/wc/v3/orders", params, &orders); err != nil {
		return nil, fmt.Errorf("orders page %d: %w", page, err)
	}
	return orders, nil
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
