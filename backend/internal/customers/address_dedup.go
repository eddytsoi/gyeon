package customers

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/lib/pq"
)

// AddressFields is the payload for a dedup-aware address write. The match key
// is the normalized *location* only (line1/line2/city/state/postal_code/
// country); first/last name and phone are stored on a freshly-inserted row but
// are NOT part of the dedup key — they drift across orders (typos, "Customer"
// fallbacks, billing-vs-shipping names) and would re-fragment what is really
// the same physical address.
type AddressFields struct {
	FirstName  string
	LastName   string
	Phone      *string
	Line1      string
	Line2      *string
	City       string
	State      *string
	PostalCode string
	Country    string
}

// addrQuerier is satisfied by both *sql.DB and *sql.Tx, so the dedup helpers
// work whether the caller is mid-transaction (the WC importer) or hitting the
// pool directly (checkout / admin order create, which insert the address
// before their order transaction begins).
type addrQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// normalizeCountry mirrors the country handling used by every legacy insert
// path: trim, upper-case, fall back to "HK" when it isn't a 2-letter code.
// Kept identical to the expression baked into uq_addresses_customer_signature.
func normalizeCountry(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if len(c) != 2 {
		return "HK"
	}
	return c
}

// findAddressID returns the id of an existing address-book row for customerID
// whose normalized location matches f. The WHERE clause uses the SAME
// normalization (lower(btrim(coalesce(...)))) as the uq_addresses_customer_
// signature unique index, so a row that the index considers a duplicate is the
// row this finds. ok=false when nothing matches.
func findAddressID(ctx context.Context, q addrQuerier, customerID string, f AddressFields) (id string, ok bool, err error) {
	scanErr := q.QueryRowContext(ctx, `
		SELECT id FROM addresses
		 WHERE customer_id = $1
		   AND lower(btrim(coalesce(line1,'')))       = lower(btrim($2))
		   AND lower(btrim(coalesce(line2,'')))       = lower(btrim(coalesce($3,'')))
		   AND lower(btrim(coalesce(city,'')))        = lower(btrim($4))
		   AND lower(btrim(coalesce(state,'')))       = lower(btrim(coalesce($5,'')))
		   AND lower(btrim(coalesce(postal_code,''))) = lower(btrim($6))
		   AND upper(btrim(coalesce(country,'HK')))   = $7
		 ORDER BY is_default DESC, created_at ASC, id ASC
		 LIMIT 1`,
		customerID, f.Line1, f.Line2, f.City, f.State, f.PostalCode, normalizeCountry(f.Country),
	).Scan(&id)
	if scanErr == nil {
		return id, true, nil
	}
	if errors.Is(scanErr, sql.ErrNoRows) {
		return "", false, nil
	}
	return "", false, scanErr
}

// insertAddressRow inserts a single address row and returns its id. line1/
// city/postal_code are trimmed and country normalized so stored values stay
// consistent with the dedup key.
func insertAddressRow(ctx context.Context, q addrQuerier, customerID *string, f AddressFields, isDefault bool) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `
		INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		customerID, f.FirstName, f.LastName, f.Phone,
		strings.TrimSpace(f.Line1), f.Line2,
		strings.TrimSpace(f.City), f.State,
		strings.TrimSpace(f.PostalCode), normalizeCountry(f.Country),
		isDefault,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// FindOrCreateAddress is the dedup-on-write entry point shared by the WC
// importer, storefront checkout, and admin order creation. It returns an
// existing address-book row for the customer whose location matches, otherwise
// inserts a new one. This is what stops a customer with N orders from
// accumulating N identical address-book rows.
//
// customerID == nil (guest snapshot) ALWAYS inserts: guest rows must never be
// shared, and the unique index excludes NULL customer_id, so there is nothing
// to dedup against. isDefault only applies to a freshly-inserted, non-guest
// row.
func FindOrCreateAddress(ctx context.Context, q addrQuerier, customerID *string, f AddressFields, isDefault bool) (string, error) {
	if customerID == nil {
		return insertAddressRow(ctx, q, nil, f, false)
	}

	if id, ok, err := findAddressID(ctx, q, *customerID, f); err != nil {
		return "", err
	} else if ok {
		return id, nil
	}

	id, err := insertAddressRow(ctx, q, customerID, f, isDefault)
	if err == nil {
		return id, nil
	}
	// Race backstop: a concurrent write inserted the same address first and
	// the unique index rejected ours (SQLSTATE 23505). Re-select the winner.
	if isUniqueViolation(err) {
		if id, ok, e := findAddressID(ctx, q, *customerID, f); e == nil && ok {
			return id, nil
		}
	}
	return "", err
}

// isUniqueViolation reports whether err is a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
