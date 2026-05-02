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

type wcProduct struct {
	ID            int                 `json:"id"`
	Name          string              `json:"name"`
	Slug          string              `json:"slug"`
	Type          string              `json:"type"`
	Status        string              `json:"status"`
	Description   string              `json:"description"`
	RegularPrice  string              `json:"regular_price"`
	SalePrice     string              `json:"sale_price"`
	StockQuantity *int                `json:"stock_quantity"`
	Weight        string              `json:"weight"`
	Categories    []wcProductCategory `json:"categories"`
	Images        []wcImage           `json:"images"`
	Variations    []int               `json:"variations"`
}

type wcVariation struct {
	ID            int           `json:"id"`
	RegularPrice  string        `json:"regular_price"`
	SalePrice     string        `json:"sale_price"`
	StockQuantity *int          `json:"stock_quantity"`
	Weight        string        `json:"weight"`
	Attributes    []wcAttribute `json:"attributes"`
}

type wcAttribute struct {
	Name   string `json:"name"`
	Option string `json:"option"`
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
func (c *wcClient) fetchProductTotal() int {
	req, err := http.NewRequest(http.MethodGet,
		c.baseURL+"/wp-json/wc/v3/products?per_page=1&page=1", nil)
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

func (c *wcClient) fetchProducts(page int) ([]wcProduct, error) {
	var products []wcProduct
	params := url.Values{"per_page": {"100"}, "page": {fmt.Sprintf("%d", page)}}
	if err := c.get("/wp-json/wc/v3/products", params, &products); err != nil {
		return nil, fmt.Errorf("products page %d: %w", page, err)
	}
	return products, nil
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
