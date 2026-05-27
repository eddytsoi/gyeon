package stock

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// expectedImportHeader is the canonical CSV header for stock-mutation imports.
const expectedImportHeader = "name,variant,quantity"

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
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	gotHeader := strings.ToLower(strings.TrimSpace(strings.Join(header, ",")))
	// Tolerate trailing whitespace in each header cell.
	parts := strings.Split(gotHeader, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	gotHeader = strings.Join(parts, ",")
	if gotHeader != expectedImportHeader {
		return &MutationImportResult{
			Errors: []ImportRowErr{{Row: 1, Message: "expected header: " + expectedImportHeader}},
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
		if csvRowIsBlank(rec) {
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

		variantID, mErr := s.matchVariant(ctx, productName, variantHint)
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

// matchVariant resolves (productName, variantHint) → variant_id using the
// fallback chain described in the plan: SKU → variant.name → attribute value.
// All comparisons are case-insensitive and trimmed.
func (s *Service) matchVariant(ctx context.Context, productName, variantHint string) (string, error) {
	// 1) Product lookup by case-insensitive trimmed name.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind FROM products
		  WHERE LOWER(TRIM(name)) = LOWER(TRIM($1))
		    AND status = 'active'`,
		productName)
	if err != nil {
		return "", err
	}
	type prod struct {
		id   string
		kind string
	}
	var prods []prod
	for rows.Next() {
		var p prod
		if err := rows.Scan(&p.id, &p.kind); err != nil {
			rows.Close()
			return "", err
		}
		prods = append(prods, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(prods) == 0 {
		return "", errors.New("product not found")
	}
	if len(prods) > 1 {
		return "", errors.New("ambiguous product name")
	}
	if prods[0].kind == "bundle" {
		return "", errors.New("bundles must be added manually")
	}

	// 2) Load all active variants for the product with their attribute values.
	vRows, err := s.db.QueryContext(ctx,
		`SELECT pv.id, pv.sku, pv.name,
		        COALESCE(
		            (SELECT array_agg(LOWER(TRIM(pav.value)))
		               FROM product_variant_attribute_values pvav
		               JOIN product_attribute_values pav ON pav.id = pvav.attribute_value_id
		              WHERE pvav.variant_id = pv.id),
		            ARRAY[]::TEXT[]
		        ) AS attr_values
		   FROM product_variants pv
		  WHERE pv.product_id = $1
		    AND pv.is_active = TRUE
		  ORDER BY pv.sort_order, pv.created_at`,
		prods[0].id)
	if err != nil {
		return "", err
	}
	type cand struct {
		id      string
		sku     string
		name    sql.NullString
		attrLc  []string
	}
	var cands []cand
	for vRows.Next() {
		var c cand
		var arr pgTextArray
		if err := vRows.Scan(&c.id, &c.sku, &c.name, &arr); err != nil {
			vRows.Close()
			return "", err
		}
		c.attrLc = arr.values
		cands = append(cands, c)
	}
	vRows.Close()
	if err := vRows.Err(); err != nil {
		return "", err
	}
	if len(cands) == 0 {
		return "", errors.New("product has no active variants")
	}

	hintLc := strings.ToLower(strings.TrimSpace(variantHint))

	// 3) Empty hint + single variant → use it.
	if hintLc == "" {
		if len(cands) == 1 {
			return cands[0].id, nil
		}
		return "", errors.New("variant is required when product has multiple variants")
	}

	// 4) SKU exact match (case-insensitive, trimmed).
	for _, c := range cands {
		if strings.EqualFold(strings.TrimSpace(c.sku), variantHint) {
			return c.id, nil
		}
	}
	// 5) variant.name exact match.
	for _, c := range cands {
		if c.name.Valid && strings.EqualFold(strings.TrimSpace(c.name.String), variantHint) {
			return c.id, nil
		}
	}
	// 6) Attribute value match — collect all hits.
	var hits []string
	for _, c := range cands {
		for _, av := range c.attrLc {
			if av == hintLc {
				hits = append(hits, c.id)
				break
			}
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) > 1 {
		return "", errors.New("ambiguous variant for product")
	}
	return "", errors.New("variant not found for product")
}

// ExportMutationCSV writes the mutation's top-level line items as a
// `name,variant,quantity` CSV — same shape as the import format so an export
// can be edited and re-imported as a template. Bundle component rows are
// skipped; the bundle parent row exports as one line with its parent
// variant.
func (s *Service) ExportMutationCSV(ctx context.Context, id string, w io.Writer) error {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"name", "variant", "quantity"}); err != nil {
		return err
	}
	for _, it := range m.Items {
		// Skip bundle children — the parent already represents the line.
		if it.ParentItemID != nil {
			continue
		}
		name := ""
		if it.ProductName != nil {
			// ProductName for simple variants comes already composed
			// (e.g. "Q²M PPF Wash 500ml"). For round-trip we need the bare
			// product name; rely on VariantName/SKU columns for the variant
			// hint. Recompute by dropping the trailing variant if present.
			name = decomposeProductName(*it.ProductName, it.VariantName)
		}
		// Prefer variant.name when it carries a non-empty human label; fall
		// back to SKU otherwise. Some legacy rows store an empty string for
		// variant.name and treating that as present would produce a blank
		// variant column in the export.
		variant := ""
		if it.VariantName != nil && strings.TrimSpace(*it.VariantName) != "" {
			variant = *it.VariantName
		} else if it.VariantSKU != nil {
			variant = *it.VariantSKU
		}
		row := []string{safeCSVCell(name), safeCSVCell(variant), strconv.Itoa(it.Quantity)}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ExportInventoryCSV writes a snapshot of all active variants of active
// products as `product_name,variant,sku,current_stock`.
func (s *Service) ExportInventoryCSV(ctx context.Context, w io.Writer) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.name, COALESCE(pv.name, ''), pv.sku, pv.stock_qty
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
		var name, variant, sku string
		var stock int
		if err := rows.Scan(&name, &variant, &sku, &stock); err != nil {
			return err
		}
		if err := cw.Write([]string{
			safeCSVCell(name),
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

func csvRowIsBlank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// decomposeProductName tries to recover the bare product name from a
// composed display name. ProductName returned by GetByID for simple variants
// is `ProductDisplayName(product, variant)` which typically concatenates the
// variant label. If the composed string ends with the variant label, strip
// it; otherwise return as-is.
func decomposeProductName(composed string, variantName *string) string {
	if variantName == nil {
		return composed
	}
	v := strings.TrimSpace(*variantName)
	if v == "" {
		return composed
	}
	if strings.HasSuffix(composed, " "+v) {
		return strings.TrimSpace(strings.TrimSuffix(composed, " "+v))
	}
	if strings.HasSuffix(composed, v) {
		return strings.TrimSpace(strings.TrimSuffix(composed, v))
	}
	return composed
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
