package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/cache"
)

type NavItem struct {
	ID        string    `json:"id"`
	MenuID    string    `json:"menu_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Label     string    `json:"label"`
	URL       string    `json:"url"`
	Target    string    `json:"target"`
	SortOrder int       `json:"sort_order"`
	Children  []NavItem `json:"children,omitempty"`
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
	ParentID  *string `json:"parent_id"`
	Label     string  `json:"label"`
	URL       string  `json:"url"`
	Target    string  `json:"target"`
	SortOrder int     `json:"sort_order"`
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

// GetMenuByHandle returns a menu with its full item tree.
func (s *NavService) GetMenuByHandle(ctx context.Context, handle string) (*NavMenu, error) {
	key := fmt.Sprintf("nav:handle:%s", handle)
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
	m.Items = buildTree(items)
	s.cache.Set(key, m, s.ttl(ctx))
	return &m, nil
}

// GetMenuByID returns a menu with items.
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

// AddItem appends a new nav item to a menu.
func (s *NavService) AddItem(ctx context.Context, menuID string, req UpsertNavItemRequest) (*NavItem, error) {
	target := req.Target
	if target == "" {
		target = "_self"
	}
	var it NavItem
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_nav_items (menu_id, parent_id, label, url, target, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, menu_id, parent_id, label, url, target, sort_order`,
		menuID, req.ParentID, req.Label, req.URL, target, req.SortOrder).
		Scan(&it.ID, &it.MenuID, &it.ParentID, &it.Label, &it.URL, &it.Target, &it.SortOrder)
	if err != nil {
		return nil, err
	}
	it.Children = []NavItem{}
	s.cache.DeleteByPrefix(navPrefix)
	return &it, nil
}

// UpdateItem updates an existing nav item.
func (s *NavService) UpdateItem(ctx context.Context, itemID string, req UpsertNavItemRequest) (*NavItem, error) {
	target := req.Target
	if target == "" {
		target = "_self"
	}
	var it NavItem
	err := s.db.QueryRowContext(ctx,
		`UPDATE cms_nav_items
		 SET parent_id=$2, label=$3, url=$4, target=$5, sort_order=$6
		 WHERE id = $1
		 RETURNING id, menu_id, parent_id, label, url, target, sort_order`,
		itemID, req.ParentID, req.Label, req.URL, target, req.SortOrder).
		Scan(&it.ID, &it.MenuID, &it.ParentID, &it.Label, &it.URL, &it.Target, &it.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	it.Children = []NavItem{}
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
