package importer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

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
}

// wcBundledItem mirrors a single component row exposed by the WooCommerce
// Product Bundles plugin. We deliberately read only the fields needed to
// resolve the component variant in Gyeon — discount / stock_status / etc.
// are ignored on first-pass import.
type wcBundledItem struct {
	BundledItemID   int `json:"bundled_item_id"`
	ProductID       int `json:"product_id"`
	VariationID     int `json:"variation_id"`
	MenuOrder       int `json:"menu_order"`
	QuantityDefault int `json:"quantity_default"`
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

type wcClient struct {
	baseURL    string
	key        string
	secret     string
	httpClient *http.Client
}

func newWCClient(baseURL, key, secret string) *wcClient {
	return &wcClient{
		baseURL:    baseURL,
		key:        key,
		secret:     secret,
		httpClient: &http.Client{},
	}
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

// fetchProductTotal returns the total number of products via the X-WP-Total header.
// When wcType is non-empty, the count is scoped to that WooCommerce product
// type (e.g. "bundle") so the progress bar denominator reflects only the
// products that will actually be processed in this run.
func (c *wcClient) fetchProductTotal(wcType string) int {
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
	return products, nil
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
