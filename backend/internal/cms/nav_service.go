package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/customers"
)

type NavItem struct {
	ID        string    `json:"id"`
	MenuID    string    `json:"menu_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Label     string    `json:"label"`
	URL       string    `json:"url"`
	Target    string    `json:"target"`
	SortOrder int       `json:"sort_order"`
	// HiddenForRoles lists the customer_role values this item is hidden
	// from. Populated by the admin-facing fetch paths; left nil for
	// storefront-facing fetches (those filter and drop hidden items
	// instead of surfacing the rule).
	HiddenForRoles []string  `json:"hidden_for_roles,omitempty"`
	Children       []NavItem `json:"children,omitempty"`
}

type NavMenu struct {
	ID        string    `json:"id"`
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	Items     []NavItem `json:"items"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type UpsertNavItemRequest struct {
	ParentID       *string  `json:"parent_id"`
	Label          string   `json:"label"`
	URL            string   `json:"url"`
	Target         string   `json:"target"`
	SortOrder      int      `json:"sort_order"`
	HiddenForRoles []string `json:"hidden_for_roles"`
}

const navPrefix = "nav:"

type NavService struct {
	db    *sql.DB
	cache cache.Store
	ttl   func(context.Context) time.Duration
}

func NewNavService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *NavService {
	return &NavService{db: db, cache: c, ttl: ttl}
}

// ListMenus returns all menus without items.
func (s *NavService) ListMenus(ctx context.Context) ([]NavMenu, error) {
	const key = "nav:menus"
	if v, ok := s.cache.Get(key); ok {
		return v.([]NavMenu), nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, handle, name, created_at, updated_at FROM cms_nav_menus ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	menus := make([]NavMenu, 0)
	for rows.Next() {
		var m NavMenu
		if err := rows.Scan(&m.ID, &m.Handle, &m.Name, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Items = []NavItem{}
		menus = append(menus, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, menus, s.ttl(ctx))
	return menus, nil
}

// GetMenuByHandle returns a menu filtered for a storefront role. Pass an
// empty role for the unfiltered tree (admin use). Anonymous storefront
// visitors should be passed as customers.RoleCustomer so they see the
// same filtered set as a logged-in customer.
//
// Filtering cascades: when a parent item is hidden for the role, its
// entire subtree is dropped, even if children have no rule.
func (s *NavService) GetMenuByHandle(ctx context.Context, handle, role string) (*NavMenu, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	key := fmt.Sprintf("nav:handle:%s:role:%s", handle, role)
	if v, ok := s.cache.Get(key); ok {
		m := v.(NavMenu)
		return &m, nil
	}
	var m NavMenu
	err := s.db.QueryRowContext(ctx,
		`SELECT id, handle, name, created_at, updated_at FROM cms_nav_menus WHERE handle = $1`, handle).
		Scan(&m.ID, &m.Handle, &m.Name, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	items, err := s.fetchItems(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	if role != "" {
		hidden, err := s.hiddenNavItemIDs(ctx, role)
		if err != nil {
			return nil, err
		}
		items = filterHiddenItems(items, hidden)
	}
	m.Items = buildTree(items)
	s.cache.Set(key, m, s.ttl(ctx))
	return &m, nil
}

// GetMenuByID returns a menu with items, including each item's
// hidden_for_roles set so the admin edit modal can prefill its
// per-role visibility checkboxes. Always unfiltered — admin views see
// every item regardless of role rules.
func (s *NavService) GetMenuByID(ctx context.Context, id string) (*NavMenu, error) {
	key := fmt.Sprintf("nav:id:%s", id)
	if v, ok := s.cache.Get(key); ok {
		m := v.(NavMenu)
		return &m, nil
	}
	var m NavMenu
	err := s.db.QueryRowContext(ctx,
		`SELECT id, handle, name, created_at, updated_at FROM cms_nav_menus WHERE id = $1`, id).
		Scan(&m.ID, &m.Handle, &m.Name, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	items, err := s.fetchItems(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	rulesByItem, err := s.rolesByNavItem(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if roles, ok := rulesByItem[items[i].ID]; ok {
			items[i].HiddenForRoles = roles
		}
	}
	m.Items = buildTree(items)
	s.cache.Set(key, m, s.ttl(ctx))
	return &m, nil
}

func (s *NavService) fetchItems(ctx context.Context, menuID string) ([]NavItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, menu_id, parent_id, label, url, target, sort_order
		 FROM cms_nav_items WHERE menu_id = $1
		 ORDER BY sort_order ASC, label ASC`, menuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]NavItem, 0)
	for rows.Next() {
		var it NavItem
		if err := rows.Scan(&it.ID, &it.MenuID, &it.ParentID,
			&it.Label, &it.URL, &it.Target, &it.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// buildTree nests children under their parents.
func buildTree(flat []NavItem) []NavItem {
	byID := make(map[string]*NavItem, len(flat))
	for i := range flat {
		flat[i].Children = []NavItem{}
		byID[flat[i].ID] = &flat[i]
	}
	roots := make([]NavItem, 0)
	for i := range flat {
		it := &flat[i]
		if it.ParentID == nil {
			roots = append(roots, *it)
		} else if parent, ok := byID[*it.ParentID]; ok {
			parent.Children = append(parent.Children, *it)
		}
	}
	return roots
}

// AddItem appends a new nav item to a menu. When req.HiddenForRoles is
// non-empty, the rules are written in the same transaction so a partial
// failure can't leave an item without its intended visibility.
func (s *NavService) AddItem(ctx context.Context, menuID string, req UpsertNavItemRequest) (*NavItem, error) {
	target := req.Target
	if target == "" {
		target = "_self"
	}
	roles, err := normalizeHiddenRoles(req.HiddenForRoles)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var it NavItem
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO cms_nav_items (menu_id, parent_id, label, url, target, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, menu_id, parent_id, label, url, target, sort_order`,
		menuID, req.ParentID, req.Label, req.URL, target, req.SortOrder).
		Scan(&it.ID, &it.MenuID, &it.ParentID, &it.Label, &it.URL, &it.Target, &it.SortOrder); err != nil {
		return nil, err
	}
	if err := replaceNavItemRolesTx(ctx, tx, it.ID, roles); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	it.Children = []NavItem{}
	it.HiddenForRoles = roles
	s.cache.DeleteByPrefix(navPrefix)
	return &it, nil
}

// UpdateItem updates an existing nav item and replaces its
// hidden_for_roles set. Both writes share a transaction so the visible
// state can never disagree between the two tables.
func (s *NavService) UpdateItem(ctx context.Context, itemID string, req UpsertNavItemRequest) (*NavItem, error) {
	target := req.Target
	if target == "" {
		target = "_self"
	}
	roles, err := normalizeHiddenRoles(req.HiddenForRoles)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var it NavItem
	if err := tx.QueryRowContext(ctx,
		`UPDATE cms_nav_items
		 SET parent_id=$2, label=$3, url=$4, target=$5, sort_order=$6
		 WHERE id = $1
		 RETURNING id, menu_id, parent_id, label, url, target, sort_order`,
		itemID, req.ParentID, req.Label, req.URL, target, req.SortOrder).
		Scan(&it.ID, &it.MenuID, &it.ParentID, &it.Label, &it.URL, &it.Target, &it.SortOrder); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := replaceNavItemRolesTx(ctx, tx, it.ID, roles); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	it.Children = []NavItem{}
	it.HiddenForRoles = roles
	s.cache.DeleteByPrefix(navPrefix)
	return &it, nil
}

// DeleteItem removes a nav item (cascades to children).
func (s *NavService) DeleteItem(ctx context.Context, itemID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cms_nav_items WHERE id = $1`, itemID)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(navPrefix)
	return nil
}

// ReorderItems rewrites sort_order for the given nav-item IDs to match the
// supplied list (0-based natural order, matching the flat depth-first display).
// IDs are scoped to the menu so a stale browser state can't reorder another
// menu's items. Preserves parent_id and every other field — only sort_order
// changes.
func (s *NavService) ReorderItems(ctx context.Context, menuID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	orders := make([]int64, len(ids))
	for i := range ids {
		orders[i] = int64(i)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE cms_nav_items AS n
		 SET sort_order = u.idx
		 FROM unnest($1::uuid[], $2::int[]) AS u(iid, idx)
		 WHERE n.id = u.iid AND n.menu_id = $3`,
		pq.Array(ids), pq.Array(orders), menuID)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(navPrefix)
	return nil
}

// ReplaceItems atomically replaces all items in a menu (used for drag-drop reorder).
func (s *NavService) ReplaceItems(ctx context.Context, menuID string, items []UpsertNavItemRequest) ([]NavItem, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM cms_nav_items WHERE menu_id = $1`, menuID); err != nil {
		return nil, err
	}

	result := make([]NavItem, 0, len(items))
	for i, req := range items {
		target := req.Target
		if target == "" {
			target = "_self"
		}
		var it NavItem
		err := tx.QueryRowContext(ctx,
			`INSERT INTO cms_nav_items (menu_id, parent_id, label, url, target, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id, menu_id, parent_id, label, url, target, sort_order`,
			menuID, req.ParentID, req.Label, req.URL, target, i).
			Scan(&it.ID, &it.MenuID, &it.ParentID, &it.Label, &it.URL, &it.Target, &it.SortOrder)
		if err != nil {
			return nil, err
		}
		it.Children = []NavItem{}
		result = append(result, it)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(navPrefix)
	return result, nil
}

// hiddenNavItemIDs returns the set of nav_item_ids that should be
// hidden for the given storefront role. Each call hits the DB; the
// menu-level cache (nav:handle:{handle}:role:{role}) is the real hot
// path, so this stays simple.
func (s *NavService) hiddenNavItemIDs(ctx context.Context, role string) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT nav_item_id::text FROM customer_role_nav_item_rules WHERE role = $1::customer_role`,
		role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

// rolesByNavItem returns the hidden-roles list per item ID for items
// belonging to the given menu. Used by the admin GetMenuByID path to
// prefill the per-item edit modal.
func (s *NavService) rolesByNavItem(ctx context.Context, menuID string) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.nav_item_id::text, r.role::text
		 FROM customer_role_nav_item_rules r
		 JOIN cms_nav_items i ON i.id = r.nav_item_id
		 WHERE i.menu_id = $1`, menuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]string)
	for rows.Next() {
		var itemID, role string
		if err := rows.Scan(&itemID, &role); err != nil {
			return nil, err
		}
		out[itemID] = append(out[itemID], role)
	}
	return out, rows.Err()
}

// filterHiddenItems drops items whose ID is in hidden, and cascades —
// any descendants of a hidden parent are also removed even if they
// have no rule of their own. Operates on the flat list (pre-buildTree)
// since parent IDs are stable nullable strings there.
func filterHiddenItems(items []NavItem, hidden map[string]struct{}) []NavItem {
	if len(hidden) == 0 {
		return items
	}
	dropped := make(map[string]struct{}, len(hidden))
	for id := range hidden {
		dropped[id] = struct{}{}
	}
	// Cascade: walk repeatedly until no new drops. Item count is tiny
	// (dozens at most), so a quadratic pass is fine.
	for {
		grew := false
		for _, it := range items {
			if _, alreadyDropped := dropped[it.ID]; alreadyDropped {
				continue
			}
			if it.ParentID == nil {
				continue
			}
			if _, parentDropped := dropped[*it.ParentID]; parentDropped {
				dropped[it.ID] = struct{}{}
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	out := items[:0:0]
	for _, it := range items {
		if _, drop := dropped[it.ID]; drop {
			continue
		}
		out = append(out, it)
	}
	return out
}

// normalizeHiddenRoles validates and dedupes the role list submitted
// by the admin UI. Unknown roles return ErrInvalidNavRole so a typo
// can't silently no-op. Empty input is fine — the item just has no
// restrictions.
func normalizeHiddenRoles(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		role := strings.ToLower(strings.TrimSpace(raw))
		if role == "" {
			continue
		}
		switch role {
		case customers.RoleCustomer, customers.RoleInstaller:
			// valid
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidNavRole, raw)
		}
		if _, dup := seen[role]; dup {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out, nil
}

// replaceNavItemRolesTx wipes and re-inserts the rules for a single
// nav item inside the caller's transaction so the item upsert and its
// rules either both land or both roll back.
func replaceNavItemRolesTx(ctx context.Context, tx *sql.Tx, itemID string, roles []string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM customer_role_nav_item_rules WHERE nav_item_id = $1::uuid`, itemID); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO customer_role_nav_item_rules (role, nav_item_id, updated_at)
			 VALUES ($1::customer_role, $2::uuid, NOW())`,
			role, itemID); err != nil {
			return err
		}
	}
	return nil
}

// ErrInvalidNavRole signals an admin payload that names a role the
// system doesn't know about. Bubbles up to a 400 response.
var ErrInvalidNavRole = errors.New("invalid nav role")
