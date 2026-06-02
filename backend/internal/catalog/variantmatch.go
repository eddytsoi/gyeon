// Package catalog provides shared helpers for resolving human-readable
// product / variant references (the kind admins paste from spreadsheets)
// to internal `product_variants.id` values.
//
// It exists so stock-mutation imports and admin-side order CSV imports
// share one source of truth for the matching rules — same fallback chain,
// same error wording, same legacy WooCommerce-name quirks.
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ExpectedImportHeader is the canonical CSV header for name/variant/quantity
// imports. Both the stock-mutation and admin-order importers expect this.
const ExpectedImportHeader = "name,variant,quantity"

// ExpectedRefImportHeader is the canonical CSV header for the up-sell /
// cross-sell importer \u2014 name + variant, no quantity (associations, not lines).
const ExpectedRefImportHeader = "name,variant"

// ValidateHeader normalises the first CSV row (BOM strip, trim, lowercase,
// trailing whitespace tolerance) and returns an error when it does not
// match ExpectedImportHeader.
func ValidateHeader(header []string) error {
	return validateHeaderEquals(header, ExpectedImportHeader)
}

// ValidateRefHeader is the ValidateHeader counterpart for the name,variant
// up-sell / cross-sell importer.
func ValidateRefHeader(header []string) error {
	return validateHeaderEquals(header, ExpectedRefImportHeader)
}

func validateHeaderEquals(header []string, expected string) error {
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	got := strings.ToLower(strings.TrimSpace(strings.Join(header, ",")))
	parts := strings.Split(got, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if strings.Join(parts, ",") != expected {
		return errors.New("expected header: " + expected)
	}
	return nil
}

// IsBlankRow reports whether every cell in rec is empty / whitespace.
func IsBlankRow(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// MatchVariant resolves (productName, variantHint) → variant_id using a
// fallback chain: empty hint + single variant → that variant; SKU exact
// match → variant.name exact match → legacy "label:value" suffix match →
// attribute value match. All comparisons are case-insensitive and trimmed.
//
// When allowBundles is false, matching a bundle product returns an error
// ("bundles must be added manually") — the stock importer uses this
// because component variants are tracked separately. The admin order
// importer passes true and handles bundle expansion downstream.
//
// Returns (variantID, productKind, error). productKind is "simple" or
// "bundle"; callers can use it to fan out bundle component lookups.
func MatchVariant(ctx context.Context, db *sql.DB, productName, variantHint string, allowBundles bool) (string, string, error) {
	// 1) Product lookup by case-insensitive trimmed name.
	rows, err := db.QueryContext(ctx,
		`SELECT id, kind FROM products
		  WHERE LOWER(TRIM(name)) = LOWER(TRIM($1))
		    AND status = 'active'`,
		productName)
	if err != nil {
		return "", "", err
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
			return "", "", err
		}
		prods = append(prods, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	if len(prods) == 0 {
		return "", "", errors.New("product not found")
	}
	if len(prods) > 1 {
		return "", "", errors.New("ambiguous product name")
	}
	if prods[0].kind == "bundle" && !allowBundles {
		return "", "", errors.New("bundles must be added manually")
	}

	// 2) Load all active variants for the product with their attribute values.
	vRows, err := db.QueryContext(ctx,
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
		return "", "", err
	}
	type cand struct {
		id     string
		sku    string
		name   sql.NullString
		attrLc []string
	}
	var cands []cand
	for vRows.Next() {
		var c cand
		var arr pgTextArray
		if err := vRows.Scan(&c.id, &c.sku, &c.name, &arr); err != nil {
			vRows.Close()
			return "", "", err
		}
		c.attrLc = arr.values
		cands = append(cands, c)
	}
	vRows.Close()
	if err := vRows.Err(); err != nil {
		return "", "", err
	}
	if len(cands) == 0 {
		return "", "", errors.New("product has no active variants")
	}

	hintLc := strings.ToLower(strings.TrimSpace(variantHint))

	// 3) Empty hint + single variant → use it.
	if hintLc == "" {
		if len(cands) == 1 {
			return cands[0].id, prods[0].kind, nil
		}
		return "", "", errors.New("variant is required when product has multiple variants")
	}

	// 4) SKU exact match (case-insensitive, trimmed).
	for _, c := range cands {
		if strings.EqualFold(strings.TrimSpace(c.sku), variantHint) {
			return c.id, prods[0].kind, nil
		}
	}
	// 5) variant.name exact match.
	for _, c := range cands {
		if c.name.Valid && strings.EqualFold(strings.TrimSpace(c.name.String), variantHint) {
			return c.id, prods[0].kind, nil
		}
	}
	// 5a) Legacy variant.name "label:value" suffix match — e.g. a CSV
	// "500ml" should match a variant whose pv.name is "容量:500ml" but whose
	// product_variant_attribute_values M2M rows are missing (typical for
	// WooCommerce-imported variants).
	for _, c := range cands {
		if !c.name.Valid {
			continue
		}
		n := strings.TrimSpace(c.name.String)
		if n == "" || strings.ContainsAny(n, ",/") {
			continue
		}
		if idx := strings.Index(n, ":"); idx >= 0 {
			after := strings.TrimSpace(n[idx+1:])
			if after != "" && strings.EqualFold(after, variantHint) {
				return c.id, prods[0].kind, nil
			}
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
		return hits[0], prods[0].kind, nil
	}
	if len(hits) > 1 {
		return "", "", errors.New("ambiguous variant for product")
	}
	return "", "", errors.New("variant not found for product")
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

// parsePgArray parses a Postgres text-array literal (`{a,b,"c,d"}`) into Go
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
	clean := out[:0]
	for _, v := range out {
		if v == "NULL" {
			continue
		}
		clean = append(clean, v)
	}
	return clean
}
