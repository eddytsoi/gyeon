// Package categoryrules manages the per-(customer_role, category) visibility
// and purchase restriction rules stored in customer_role_category_rules.
//
// Rules are *negative*: a row in the table means "this role has at least one
// thing they can't do with this category". Missing rows are allowed by
// default. The admin matrix UI hides this asymmetry — toggling a cell to
// "allowed" simply removes the row (or flips the relevant flag back to TRUE
// if the other dimension is still restricted).
package categoryrules

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/customers"
)

// Rule mirrors one customer_role_category_rules row.
//
// Three dimensions, all default-TRUE (allowed):
//   - CanView:     PDP resolves and product appears everywhere for this role.
//   - IsListed:    product appears in listings / category nav / search.
//                  FALSE = "private link" — PDP-by-slug still works, but the
//                  category is hidden from public discovery. Replaces the
//                  old global hidden_category_ids setting (migration 103).
//   - CanPurchase: cart-add accepts the variant.
//
// Implications (enforced in the service layer, not the schema): !CanView
// implies !IsListed and !CanPurchase — you can't list or buy what the role
// can't see.
type Rule struct {
	Role        string `json:"role"`
	CategoryID  string `json:"category_id"`
	CanView     bool   `json:"can_view"`
	IsListed    bool   `json:"is_listed"`
	CanPurchase bool   `json:"can_purchase"`
}

// Service owns the rules table and exposes both admin CRUD and the
// hot-path "which categories are blocked for role R" lookup used by the
// storefront product list and cart-add path.
type Service struct {
	db *sql.DB

	mu       sync.RWMutex
	cached   map[string][]Rule // keyed by role
	cacheExp time.Time

	// onInvalidate fires after SaveBulk commits + the local Rule cache is
	// dropped. Wired from main.go to drop the shop package's product / FBT
	// caches — those entries stamp Purchasable per role using the OLD rules,
	// so without this the storefront keeps reading stale purchasable flags
	// until the per-entry TTL expires (could be minutes). nil-safe.
	onInvalidate func()
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetOnInvalidate wires a callback invoked after each successful SaveBulk.
// Optional — when nil, Service still works but downstream caches may serve
// stale Purchasable annotations until they expire on their own.
func (s *Service) SetOnInvalidate(fn func()) { s.onInvalidate = fn }

// List returns every rule row, ordered (role, category_id) for stable admin
// rendering. Bypasses the cache — the admin UI wants fresh data on every
// load, and admin traffic is tiny.
func (s *Service) List(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT role::text, category_id::text, can_view, is_listed, can_purchase
		 FROM customer_role_category_rules
		 ORDER BY role, category_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Role, &r.CategoryID, &r.CanView, &r.IsListed, &r.CanPurchase); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveBulk replaces the entire ruleset with the given rows. The matrix UI
// always sends the complete state — partial deltas would force the admin to
// reason about whether a missing row meant "still allowed" or "untouched".
// Wrapped in a transaction so a partial failure doesn't leave the rules in
// an inconsistent state. Rows where can_view AND is_listed AND can_purchase
// are all TRUE are dropped (they're the default and shouldn't take up table
// space).
func (s *Service) SaveBulk(ctx context.Context, rules []Rule) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_role_category_rules`); err != nil {
		return err
	}
	for _, r := range rules {
		role := customers.NormalizeRole(r.Role)
		if r.CategoryID == "" {
			continue
		}
		if r.CanView && r.IsListed && r.CanPurchase {
			continue // default state — no row needed
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO customer_role_category_rules
			    (role, category_id, can_view, is_listed, can_purchase, updated_at)
			 VALUES ($1::customer_role, $2::uuid, $3, $4, $5, NOW())`,
			role, r.CategoryID, r.CanView, r.IsListed, r.CanPurchase); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidate()
	if s.onInvalidate != nil {
		s.onInvalidate()
	}
	return nil
}

// blockedCategoryIDs returns the category UUIDs where the given dimension
// (view, list, or purchase) is denied for the role. Reads from a small
// in-process cache; the rule set rarely changes and the storefront queries
// this on every product list / PDP / add-to-cart.
//
// Implication chain: !CanView implies !IsListed implies (along with
// !CanPurchase explicitly) the row's other dimensions are denied — fold the
// implications here so callers don't have to OR sets together at every
// guard point.
func (s *Service) blockedCategoryIDs(ctx context.Context, role, dimension string) ([]string, error) {
	role = customers.NormalizeRole(role)
	rules, err := s.rulesForRole(ctx, role)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		switch dimension {
		case "view":
			if !r.CanView {
				out = append(out, r.CategoryID)
			}
		case "list":
			// Hidden-from-listings: explicit IsListed=FALSE OR fully blocked
			// (!CanView). The latter ensures view-blocked categories also
			// stay out of nav / listings without callers having to combine.
			if !r.IsListed || !r.CanView {
				out = append(out, r.CategoryID)
			}
		case "purchase":
			// "Can't see it" implies "can't buy it" — combining both signals
			// here means callers don't need to OR the two filters together
			// when guarding the cart path. IsListed does NOT imply
			// !CanPurchase: a category may be unlisted (private link) yet
			// still purchasable for shoppers who reach the PDP directly.
			if !r.CanPurchase || !r.CanView {
				out = append(out, r.CategoryID)
			}
		}
	}
	return out, nil
}

// BlockedViewCategoryIDs returns category IDs the role isn't allowed to see.
// Storefront product listings, PDP, and category nav exclude these.
func (s *Service) BlockedViewCategoryIDs(ctx context.Context, role string) []string {
	ids, err := s.blockedCategoryIDs(ctx, role, "view")
	if err != nil {
		return nil
	}
	return ids
}

// BlockedListCategoryIDs returns category IDs the role can technically reach
// via direct PDP URL but should not see in product listings, category nav,
// or search. The per-role replacement for the old global hidden_category_ids
// site setting (removed in migration 103). Includes the view-blocked set,
// since "can't see" implies "can't list".
func (s *Service) BlockedListCategoryIDs(ctx context.Context, role string) []string {
	ids, err := s.blockedCategoryIDs(ctx, role, "list")
	if err != nil {
		return nil
	}
	return ids
}

// BlockedPurchaseCategoryIDs returns category IDs the role isn't allowed to
// purchase from. Includes the view-blocked set, since "can't see" implies
// "can't buy".
func (s *Service) BlockedPurchaseCategoryIDs(ctx context.Context, role string) []string {
	ids, err := s.blockedCategoryIDs(ctx, role, "purchase")
	if err != nil {
		return nil
	}
	return ids
}

// VariantPurchasable answers "is this variant blocked for that role?". Used
// by the cart add path so a 403 fires before stock check and the customer
// gets a meaningful error. Returns true (purchasable) on lookup failure to
// fail open — losing a sale to a stale rule cache is worse than letting one
// edge-case purchase slip through.
func (s *Service) VariantPurchasable(ctx context.Context, variantID, role string) bool {
	blocked := s.BlockedPurchaseCategoryIDs(ctx, role)
	if len(blocked) == 0 {
		return true
	}
	var hit int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM product_variants pv
		 WHERE pv.id = $1::uuid
		   AND EXISTS (
		       SELECT 1 FROM product_category_links pcl
		       WHERE pcl.product_id = pv.product_id
		         AND pcl.category_id = ANY($2::uuid[])
		   )`,
		variantID, pq.Array(blocked)).Scan(&hit)
	if err != nil {
		return true
	}
	return hit == 0
}

// rulesForRole returns the cached rules slice for a role, refreshing the
// cache when stale. The cache covers all roles in one map so we don't
// thrash the lock when roles are queried in interleaving requests.
func (s *Service) rulesForRole(ctx context.Context, role string) ([]Rule, error) {
	s.mu.RLock()
	if s.cached != nil && time.Now().Before(s.cacheExp) {
		out := s.cached[role]
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Now().Before(s.cacheExp) {
		return s.cached[role], nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT role::text, category_id::text, can_view, is_listed, can_purchase
		 FROM customer_role_category_rules
		 WHERE can_view = FALSE OR is_listed = FALSE OR can_purchase = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byRole := make(map[string][]Rule, 2)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Role, &r.CategoryID, &r.CanView, &r.IsListed, &r.CanPurchase); err != nil {
			return nil, err
		}
		byRole[r.Role] = append(byRole[r.Role], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cached = byRole
	s.cacheExp = time.Now().Add(60 * time.Second)
	return byRole[role], nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.cacheExp = time.Time{}
	s.mu.Unlock()
}

// ValidateRole returns an error if the role isn't one of the canonical
// values. Exposed so the admin handler can reject obviously bad payloads
// before they reach SaveBulk.
var ErrInvalidRole = errors.New("invalid role")

func ValidateRole(role string) error {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case customers.RoleCustomer, customers.RoleInstaller:
		return nil
	default:
		return ErrInvalidRole
	}
}
