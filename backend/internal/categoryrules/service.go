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
type Rule struct {
	Role        string `json:"role"`
	CategoryID  string `json:"category_id"`
	CanView     bool   `json:"can_view"`
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
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// List returns every rule row, ordered (role, category_id) for stable admin
// rendering. Bypasses the cache — the admin UI wants fresh data on every
// load, and admin traffic is tiny.
func (s *Service) List(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT role::text, category_id::text, can_view, can_purchase
		 FROM customer_role_category_rules
		 ORDER BY role, category_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Role, &r.CategoryID, &r.CanView, &r.CanPurchase); err != nil {
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
// an inconsistent state. Rows where can_view AND can_purchase are both TRUE
// are dropped (they're the default and shouldn't take up table space).
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
		if r.CanView && r.CanPurchase {
			continue // default state — no row needed
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO customer_role_category_rules
			    (role, category_id, can_view, can_purchase, updated_at)
			 VALUES ($1::customer_role, $2::uuid, $3, $4, NOW())`,
			role, r.CategoryID, r.CanView, r.CanPurchase); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidate()
	return nil
}

// blockedCategoryIDs returns the category UUIDs where the given dimension
// (view or purchase) is denied for the role. Reads from a small in-process
// cache; the rule set rarely changes and the storefront queries this on
// every product list / PDP / add-to-cart.
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
		case "purchase":
			// "Can't see it" implies "can't buy it" — combining both signals
			// here means callers don't need to OR the two filters together
			// when guarding the cart path.
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
		`SELECT role::text, category_id::text, can_view, can_purchase
		 FROM customer_role_category_rules
		 WHERE can_view = FALSE OR can_purchase = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byRole := make(map[string][]Rule, 2)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.Role, &r.CategoryID, &r.CanView, &r.CanPurchase); err != nil {
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
