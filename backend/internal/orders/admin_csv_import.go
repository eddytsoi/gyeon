package orders

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gyeon/backend/internal/catalog"
	"gyeon/backend/internal/respond"
	"gyeon/backend/internal/shop"
)

// CSVResolveItem is one row resolved from the admin's name/variant/quantity
// CSV — already enriched with the fields the admin order-creation UI needs
// to render a line in the items table.
type CSVResolveItem struct {
	VariantID       string            `json:"variant_id"`
	ProductID       string            `json:"product_id"`
	ProductName     string            `json:"product_name"`
	ProductKind     string            `json:"product_kind"`
	VariantName     *string           `json:"variant_name,omitempty"`
	SKU             string            `json:"sku"`
	UnitPrice       float64           `json:"unit_price"`
	StockQty        int               `json:"stock_qty"`
	PrimaryImageURL *string           `json:"primary_image_url,omitempty"`
	Quantity        int               `json:"quantity"`
	BundleItems     []shop.BundleItem `json:"bundle_items,omitempty"`
}

// CSVResolveRowErr is the per-row error shape: Row is 1-based (header is
// row 1, first data row is row 2).
type CSVResolveRowErr struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// CSVResolveResult is the response shape for POST /admin/orders/items/csv-resolve.
type CSVResolveResult struct {
	Items   []CSVResolveItem   `json:"items"`
	Skipped int                `json:"skipped"`
	Errors  []CSVResolveRowErr `json:"errors,omitempty"`
}

// adminResolveCSVItems backs POST /admin/orders/items/csv-resolve — admins
// upload a `name,variant,quantity` CSV (same grammar as the stock-mutation
// importer) and receive a list of resolved line items they can append to
// the order being composed client-side. Bundles are supported: each bundle
// row arrives with its component rows pre-loaded.
//
// Always returns 200 OK with per-row errors in the body, mirroring the
// stock-mutation importer's UX contract.
func (h *OrderHandler) adminResolveCSVItems(w http.ResponseWriter, r *http.Request) {
	if h.productSvc == nil {
		respond.InternalError(w)
		return
	}
	// 8 MB body cap — well above any reasonable manual-entry CSV.
	r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		respond.BadRequest(w, "could not parse multipart form")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	fh, _, err := r.FormFile("file")
	if err != nil {
		respond.BadRequest(w, "missing 'file' field")
		return
	}
	defer fh.Close()

	result := h.resolveCSV(r.Context(), fh)
	respond.JSON(w, http.StatusOK, result)
}

func (h *OrderHandler) resolveCSV(ctx context.Context, r io.Reader) *CSVResolveResult {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err == io.EOF {
		return &CSVResolveResult{
			Items:  []CSVResolveItem{},
			Errors: []CSVResolveRowErr{{Row: 1, Message: "empty file"}},
		}
	}
	if err != nil {
		return &CSVResolveResult{
			Items:  []CSVResolveItem{},
			Errors: []CSVResolveRowErr{{Row: 1, Message: "read header: " + err.Error()}},
		}
	}
	if err := catalog.ValidateHeader(header); err != nil {
		return &CSVResolveResult{
			Items:  []CSVResolveItem{},
			Errors: []CSVResolveRowErr{{Row: 1, Message: err.Error()}},
		}
	}

	out := &CSVResolveResult{Items: []CSVResolveItem{}}
	// Aggregate duplicate variant_id rows by summing their quantities —
	// same behaviour as the stock-mutation importer so a CSV that lists
	// the same SKU twice is not double-counted as two separate lines.
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
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: rowNum, Message: rerr.Error()})
			continue
		}
		if catalog.IsBlankRow(rec) {
			continue
		}
		if len(rec) < 3 {
			out.Skipped++
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: rowNum, Message: "expected 3 columns (name, variant, quantity)"})
			continue
		}
		productName := strings.TrimSpace(rec[0])
		variantHint := strings.TrimSpace(rec[1])
		qtyStr := strings.TrimSpace(rec[2])
		if productName == "" {
			out.Skipped++
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: rowNum, Message: "name is required"})
			continue
		}
		qty, qerr := strconv.Atoi(qtyStr)
		if qerr != nil || qty <= 0 {
			out.Skipped++
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: rowNum, Message: "quantity must be a positive integer"})
			continue
		}
		variantID, _, mErr := catalog.MatchVariant(ctx, h.productSvc.DB(), productName, variantHint, true)
		if mErr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: rowNum, Message: mErr.Error()})
			continue
		}
		if _, seen := agg[variantID]; !seen {
			order = append(order, variantID)
		}
		agg[variantID] += qty
	}

	for _, vid := range order {
		item, ierr := h.buildResolvedItem(ctx, vid, agg[vid])
		if ierr != nil {
			out.Skipped++
			out.Errors = append(out.Errors, CSVResolveRowErr{Row: 0, Message: "failed to enrich variant " + vid + ": " + ierr.Error()})
			continue
		}
		out.Items = append(out.Items, *item)
	}
	return out
}

func (h *OrderHandler) buildResolvedItem(ctx context.Context, variantID string, qty int) (*CSVResolveItem, error) {
	v, err := h.productSvc.GetVariantByID(ctx, variantID)
	if err != nil {
		return nil, err
	}
	// Look up product name + kind. GetVariantByID already returns
	// v.ProductName, but kind isn't on Variant — fetch it directly.
	p, err := h.productSvc.GetByID(ctx, v.ProductID, "")
	if err != nil {
		return nil, err
	}
	productName := p.Name
	if v.ProductName != nil && *v.ProductName != "" {
		productName = *v.ProductName
	}
	item := &CSVResolveItem{
		VariantID:       v.ID,
		ProductID:       v.ProductID,
		ProductName:     productName,
		ProductKind:     p.Kind,
		VariantName:     v.Name,
		SKU:             v.SKU,
		UnitPrice:       v.Price,
		StockQty:        v.StockQty,
		PrimaryImageURL: v.ImageURL,
		Quantity:        qty,
	}
	if p.Kind == "bundle" {
		items, berr := h.productSvc.GetBundleItems(ctx, v.ProductID)
		if berr == nil {
			item.BundleItems = items
		}
	}
	return item, nil
}
