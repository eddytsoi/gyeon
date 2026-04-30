// Package util holds tiny, dependency-free helpers shared across modules.
package util

import (
	"fmt"
	"strings"
)

// BuildSearchClause returns a SQL fragment and one positional argument
// (the %q% pattern) for case-insensitive substring search across `fields`.
//
// Each field is matched with `field ILIKE $N` and the field expressions
// are OR'd together inside parentheses. The caller passes the next free
// placeholder index for $N.
//
// Returns ("", nil) when q is empty so callers can ignore the result.
//
// Example:
//
//	clause, arg := util.BuildSearchClause("foo", []string{"p.name", "p.slug"}, 4)
//	// clause = "(p.name ILIKE $4 OR p.slug ILIKE $4)"
//	// arg    = "%foo%"
func BuildSearchClause(q string, fields []string, placeholderIdx int) (string, any) {
	q = strings.TrimSpace(q)
	if q == "" || len(fields) == 0 {
		return "", nil
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = fmt.Sprintf("%s ILIKE $%d", f, placeholderIdx)
	}
	return "(" + strings.Join(parts, " OR ") + ")", "%" + q + "%"
}
