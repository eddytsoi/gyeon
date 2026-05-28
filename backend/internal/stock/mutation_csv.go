package stock

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gyeon/backend/internal/catalog"
)

// MutationImportResult is the response shape for a CSV import. Mutation is
// nil when zero CSV rows resolved to valid line items — the importer
// deliberately doesn't create an empty draft in that case to avoid cluttering
// the list with zero-line-item drafts.
type MutationImportResult struct {
	Mutation *Mutation      `json:"mutation,omitempty"`
	Imported int            `json:"imported"`
	Skipped  int            `json:"skipped"`
	Errors   []ImportRowErr `json:"errors,omitempty"`
}

// ImportRowErr is the per-row error shape: Row is 1-based (header is row 1).
type ImportRowErr struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportCSV parses a `name,variant,quantity` CSV and creates one draft
// mutation of the requested type containing every successfully matched row.
// Duplicate variants in the CSV are aggregated (summed). Bad rows are
// reported per-row; valid rows still import. Returns a result with
// Mutation == nil when zero rows succeeded.
func (s *Service) ImportCSV(ctx context.Context, t MutationType, r io.Reader) (*MutationImportResult, error) {
	if err := validateType(t); err != nil {
		return nil, err
	}

	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err == io.EOF {
		return &MutationImportResult{
			Errors: []ImportRowErr{{Row: 1, Message: "empty file"}},
		}, nil
	}
	if err != nil {
		return &MutationImportResult{
			Errors: []ImportRowErr{{Row: 1, Message: "read header: " + err.Error()}},
		}, nil
	}
	if err := catalog.ValidateHeader(header); err != nil {
		return &MutationImportResult{
			Errors: []ImportRowErr{{Row: 1, Message: err.Error()}},
		}, nil
	}

	type aggKey struct {
		variantID string
	}
	agg := map[string]int{}
	order := []string{} // variant_id first-seen order

	result := &MutationImportResult{}
	rowNum := 1 // header was line 1
	for {
		rec, rerr := cr.Read()
		if rerr == io.EOF {
			break
		}
		rowNum++
		if rerr != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowErr{Row: rowNum, Message: rerr.Error()})
			continue
		}
		if catalog.IsBlankRow(rec) {
			continue
		}
		if len(rec) < 3 {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowErr{Row: rowNum, Message: "expected 3 columns (name, variant, quantity)"})
			continue
		}
		productName := strings.TrimSpace(rec[0])
		variantHint := strings.TrimSpace(rec[1])
		qtyStr := strings.TrimSpace(rec[2])
		if productName == "" {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowErr{Row: rowNum, Message: "name is required"})
			continue
		}
		qty, qerr := strconv.Atoi(qtyStr)
		if qerr != nil || qty <= 0 {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowErr{Row: rowNum, Message: "quantity must be a positive integer"})
			continue
		}

		variantID, _, mErr := catalog.MatchVariant(ctx, s.db, productName, variantHint, false)
		if mErr != nil {
			result.Skipped++
			result.Errors = append(result.Errors, ImportRowErr{Row: rowNum, Message: mErr.Error()})
			continue
		}

		if _, seen := agg[variantID]; !seen {
			order = append(order, variantID)
		}
		agg[variantID] += qty
		_ = aggKey{} // silence unused
	}

	if len(agg) == 0 {
		return result, nil
	}

	items := make([]CreateRequestItem, 0, len(order))
	for _, vid := range order {
		items = append(items, CreateRequestItem{VariantID: vid, Quantity: agg[vid]})
	}
	m, err := s.Create(ctx, CreateRequest{Type: t, Items: items})
	if err != nil {
		return nil, err
	}
	result.Mutation = m
	result.Imported = len(items)
	return result, nil
}


// ExportMutationCSV writes the mutation's top-level line items as a
// `name,variant,quantity` CSV — same shape as the import format so an export
// can be edited and re-imported as a template. Bundle component rows are
// skipped; the bundle parent row exports as one line with its parent
// variant.
//
// Output shape (matches the sample input format the importer expects):
//   - `name`: the bare `products.name` — no variant suffix.
//   - `variant`: the single attribute value (e.g. "500ml") when the variant
//     has exactly one attribute; SKU otherwise. This keeps the file
//     round-trippable through the import matcher (which tries SKU →
//     variant.name → attribute value).
func (s *Service) ExportMutationCSV(ctx context.Context, id string, w io.Writer) error {
	rows, err := s.db.QueryContext(ctx, exportRowsSQL, id)
	if err != nil {
		return err
	}
	defer rows.Close()

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"name", "variant", "quantity"}); err != nil {
		return err
	}
	for rows.Next() {
		var productName, pvName, sku string
		var attrValues pgTextArray
		var quantity int
		if err := rows.Scan(&productName, &attrValues, &pvName, &sku, &quantity); err != nil {
			return err
		}
		variant := cleanVariantLabel(attrValues.values, pvName, sku)
		if err := cw.Write([]string{
			safeCSVCell(productName),
			safeCSVCell(variant),
			strconv.Itoa(quantity),
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// ExportInventoryCSV writes a snapshot of all active variants of active
// products as `product_name,variant,sku,current_stock`. `variant` is the
// single attribute value when the variant has exactly one attribute; SKU
// otherwise — matching the per-mutation export format.
func (s *Service) ExportInventoryCSV(ctx context.Context, w io.Writer) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.name,
		        COALESCE(
		            (SELECT array_agg(pav.value ORDER BY pa.sort_order, pav.sort_order)
		               FROM product_variant_attribute_values pvav
		               JOIN product_attribute_values pav ON pav.id = pvav.attribute_value_id
		               JOIN product_attributes pa ON pa.id = pav.attribute_id
		              WHERE pvav.variant_id = pv.id),
		            ARRAY[]::TEXT[]
		        ) AS attr_values,
		        COALESCE(pv.name, '') AS pv_name,
		        pv.sku,
		        pv.stock_qty
		   FROM product_variants pv
		   JOIN products p ON p.id = pv.product_id
		  WHERE pv.is_active = TRUE
		    AND p.status = 'active'
		  ORDER BY p.name, pv.sort_order, pv.sku`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"product_name", "variant", "sku", "current_stock"}); err != nil {
		return err
	}
	for rows.Next() {
		var productName, pvName, sku string
		var attrValues pgTextArray
		var stock int
		if err := rows.Scan(&productName, &attrValues, &pvName, &sku, &stock); err != nil {
			return err
		}
		variant := cleanVariantLabel(attrValues.values, pvName, sku)
		if err := cw.Write([]string{
			safeCSVCell(productName),
			safeCSVCell(variant),
			safeCSVCell(sku),
			strconv.Itoa(stock),
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// exportRowsSQL pulls the bare product name, every attribute value attached
// to the variant (in attribute → value sort order), the SKU, and the
// top-level line quantity. Bundle component rows (parent_item_id IS NOT
// NULL) are excluded so each export row maps 1:1 to a user-edited input
// row.
const exportRowsSQL = `
SELECT p.name,
       COALESCE(
           (SELECT array_agg(pav.value ORDER BY pa.sort_order, pav.sort_order)
              FROM product_variant_attribute_values pvav
              JOIN product_attribute_values pav ON pav.id = pvav.attribute_value_id
              JOIN product_attributes pa ON pa.id = pav.attribute_id
             WHERE pvav.variant_id = pv.id),
           ARRAY[]::TEXT[]
       ) AS attr_values,
       COALESCE(pv.name, '') AS pv_name,
       pv.sku,
       smi.quantity
  FROM stock_mutation_items smi
  JOIN product_variants pv ON pv.id = smi.variant_id
  JOIN products p ON p.id = pv.product_id
 WHERE smi.mutation_id = $1
   AND smi.parent_item_id IS NULL
 ORDER BY smi.position, smi.created_at`

// cleanVariantLabel picks the human-friendly variant cell:
//
//   - exactly one attribute value (modern M2M variants) → just that value
//     (e.g. "500ml").
//   - legacy variants whose `pv.name` carries a single attribute as
//     "label:value" (e.g. "容量:500ml") with no M2M rows → strip the
//     "label:" prefix and return just the value ("500ml"). Multi-attribute
//     legacy names (containing "," or "/") fall through to SKU because we
//     can't safely round-trip them via the importer's single-value match.
//   - zero or multiple attribute values, no usable legacy name → SKU
//     (still round-trippable through the importer's SKU match).
func cleanVariantLabel(attrValues []string, pvName, sku string) string {
	if len(attrValues) == 1 {
		if v := strings.TrimSpace(attrValues[0]); v != "" {
			return v
		}
	}
	n := strings.TrimSpace(pvName)
	if n != "" && !strings.ContainsAny(n, ",/") {
		if idx := strings.Index(n, ":"); idx >= 0 {
			if after := strings.TrimSpace(n[idx+1:]); after != "" {
				return after
			}
		} else {
			return n
		}
	}
	return sku
}

// ── helpers ─────────────────────────────────────────────────────────────

// safeCSVCell prefixes a single quote in front of cells whose first
// character is one of `= + - @`, neutralising CSV-injection when the file
// is opened in Excel / Google Sheets.
func safeCSVCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}

// pgTextArray scans a Postgres text[] without pulling in pq. The driver
// delivers the array as a string like `{abc,"def ghi"}` for us to parse.
// Empty arrays come through as `{}`.
type pgTextArray struct {
	values []string
}

func (a *pgTextArray) Scan(src any) error {
	if src == nil {
		a.values = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("pgTextArray: unsupported source type %T", src)
	}
	a.values = parsePgArray(s)
	return nil
}

// parsePgArray parses Postgres text-array literal (`{a,b,"c,d"}`) into Go
// strings. Handles double-quoted elements, escaped quotes (\"), and NULL.
func parsePgArray(s string) []string {
	if s == "" || s == "{}" || s == "{NULL}" {
		return nil
	}
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil
	}
	inner := s[1 : len(s)-1]
	var out []string
	var cur strings.Builder
	inQuotes := false
	i := 0
	for i < len(inner) {
		c := inner[i]
		if inQuotes {
			if c == '\\' && i+1 < len(inner) {
				cur.WriteByte(inner[i+1])
				i += 2
				continue
			}
			if c == '"' {
				inQuotes = false
				i++
				continue
			}
			cur.WriteByte(c)
			i++
			continue
		}
		if c == '"' {
			inQuotes = true
			i++
			continue
		}
		if c == ',' {
			out = append(out, cur.String())
			cur.Reset()
			i++
			continue
		}
		cur.WriteByte(c)
		i++
	}
	out = append(out, cur.String())
	// Drop literal NULLs (we COALESCE-ed array_agg already but be defensive).
	clean := out[:0]
	for _, v := range out {
		if v == "NULL" {
			continue
		}
		clean = append(clean, v)
	}
	return clean
}
