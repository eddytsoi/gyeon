package shop

import "strings"

// VariantSuffix extracts the value portion of a variant name. A variant's
// `name` column is stored as `key:value` pairs joined by ` / `, e.g.
// "容量:500ml" or "尺寸:L / 顏色:紅". For display we want the bare value —
// "500ml" or "L / 紅" — to use as a suffix on the product name.
//
// Returns "" when the input is empty or contains no usable values; callers
// can then fall back to the plain product name.
func VariantSuffix(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, " / ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if i := strings.Index(p, ":"); i >= 0 {
			p = p[i+1:]
		}
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " / ")
}

// ProductDisplayName builds the customer-facing line-item name:
// "{productName} {variantSuffix}", or just "{productName}" when the variant
// has no usable suffix.
func ProductDisplayName(productName, variantName string) string {
	s := VariantSuffix(variantName)
	if s == "" {
		return productName
	}
	return productName + " " + s
}
