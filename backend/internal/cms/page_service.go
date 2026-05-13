package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/util"
)

// pageSearchFields are matched by the optional admin `search` param on List.
// Body content is intentionally excluded.
var pageSearchFields = []string{"p.title", "p.slug", "p.number::text"}

type Page struct {
	ID          string  `json:"id"`
	Number      int64   `json:"number"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	MetaTitle   *string `json:"meta_title,omitempty"`
	MetaDesc    *string `json:"meta_desc,omitempty"`
	IsPublished bool    `json:"is_published"`
	ShowTitle   bool    `json:"show_title"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type PageTranslation struct {
	Locale    string  `json:"locale"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	MetaTitle *string `json:"meta_title,omitempty"`
	MetaDesc  *string `json:"meta_desc,omitempty"`
	UpdatedAt string  `json:"updated_at"`
}

type UpsertPageTranslationRequest struct {
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	MetaTitle *string `json:"meta_title"`
	MetaDesc  *string `json:"meta_desc"`
}

type CreatePageRequest struct {
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	MetaTitle   *string `json:"meta_title"`
	MetaDesc    *string `json:"meta_desc"`
	IsPublished bool    `json:"is_published"`
	ShowTitle   bool    `json:"show_title"`
}

// UpdatePageRequest is the same as CreatePageRequest (is_published included in both).
type UpdatePageRequest = CreatePageRequest

var ErrNotFound = errors.New("not found")

const pagePrefix = "cms:pages:"

// AuditRecorder is the minimal interface CMS services need from the audit
// package. Decoupled to avoid an import cycle. Shared by PageService and
// PostService since they live in the same package.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry)
}

type AuditEntry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

type PageService struct {
	db    *sql.DB
	cache cache.Store
	ttl   func(context.Context) time.Duration
	audit AuditRecorder
}

func NewPageService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *PageService {
	return &PageService{db: db, cache: c, ttl: ttl}
}

// SetAudit wires an optional audit recorder. Call from main during setup.
func (s *PageService) SetAudit(rec AuditRecorder) { s.audit = rec }

func (s *PageService) record(ctx context.Context, action, entityType, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: entityType, EntityID: entityID,
		Before: before, After: after,
	})
}

// getTranslation fetches a single page translation by (pageID, locale). Used
// as a before-snapshot for audit on upsert/delete.
func (s *PageService) getTranslation(ctx context.Context, pageID, locale string) (*PageTranslation, error) {
	var t PageTranslation
	err := s.db.QueryRowContext(ctx,
		`SELECT locale, title, content, meta_title, meta_desc, updated_at
		 FROM cms_page_translations WHERE page_id=$1 AND locale=$2`, pageID, locale).
		Scan(&t.Locale, &t.Title, &t.Content, &t.MetaTitle, &t.MetaDesc, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

const pageTranslationJoin = `
	LEFT JOIN cms_page_translations t ON t.page_id = p.id AND t.locale = $1`

const pageSelect = `
	SELECT p.id, p.number, p.slug,
	       COALESCE(t.title,     p.title)     AS title,
	       COALESCE(t.content,   p.content)   AS content,
	       COALESCE(t.meta_title, p.meta_title) AS meta_title,
	       COALESCE(t.meta_desc,  p.meta_desc)  AS meta_desc,
	       p.is_published, p.show_title, p.created_at, p.updated_at
	FROM cms_pages p` + pageTranslationJoin

func scanPage(row interface{ Scan(...any) error }) (Page, error) {
	var p Page
	err := row.Scan(&p.ID, &p.Number, &p.Slug, &p.Title, &p.Content,
		&p.MetaTitle, &p.MetaDesc, &p.IsPublished, &p.ShowTitle, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List returns all pages. locale may be empty for base content.
// search is an optional case-insensitive substring matched against
// pageSearchFields; pass "" to disable.
func (s *PageService) List(ctx context.Context, locale, search string) ([]Page, error) {
	key := fmt.Sprintf("cms:pages:list:%s:%s", locale, search)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Page), nil
	}

	args := []any{locale}
	query := pageSelect + ` ORDER BY p.created_at DESC`
	if clause, arg := util.BuildSearchClause(search, pageSearchFields, 2); clause != "" {
		query = pageSelect + ` WHERE ` + clause + ` ORDER BY p.created_at DESC`
		args = append(args, arg)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := make([]Page, 0)
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, pages, s.ttl(ctx))
	return pages, nil
}

// GetBySlug fetches a published page; locale may be empty for base content.
func (s *PageService) GetBySlug(ctx context.Context, slug, locale string) (*Page, error) {
	key := fmt.Sprintf("cms:pages:slug:%s:%s", slug, locale)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Page)
		return &p, nil
	}
	p, err := scanPage(s.db.QueryRowContext(ctx,
		pageSelect+` WHERE p.slug = $2 AND p.is_published = TRUE`, locale, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
}

// GetByID fetches any page by ID; locale may be empty for base content.
func (s *PageService) GetByID(ctx context.Context, id, locale string) (*Page, error) {
	key := fmt.Sprintf("cms:pages:id:%s:%s", id, locale)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Page)
		return &p, nil
	}
	p, err := scanPage(s.db.QueryRowContext(ctx,
		pageSelect+` WHERE p.id = $2`, locale, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
}

func (s *PageService) Create(ctx context.Context, req CreatePageRequest) (*Page, error) {
	var p Page
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_pages (slug, title, content, meta_title, meta_desc, is_published, show_title)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, number, slug, title, content, meta_title, meta_desc, is_published, show_title, created_at, updated_at`,
		req.Slug, req.Title, req.Content, req.MetaTitle, req.MetaDesc, req.IsPublished, req.ShowTitle).
		Scan(&p.ID, &p.Number, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.ShowTitle, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	s.record(ctx, "cms_page.create", "cms_page", p.ID, nil, p)
	return &p, nil
}

func (s *PageService) Update(ctx context.Context, id string, req UpdatePageRequest) (*Page, error) {
	var before *Page
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id, ""); err == nil {
			before = prev
		}
	}
	var p Page
	err := s.db.QueryRowContext(ctx,
		`UPDATE cms_pages SET slug=$2, title=$3, content=$4, meta_title=$5, meta_desc=$6, is_published=$7, show_title=$8
		 WHERE id = $1
		 RETURNING id, number, slug, title, content, meta_title, meta_desc, is_published, show_title, created_at, updated_at`,
		id, req.Slug, req.Title, req.Content, req.MetaTitle, req.MetaDesc, req.IsPublished, req.ShowTitle).
		Scan(&p.ID, &p.Number, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.ShowTitle, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	s.record(ctx, "cms_page.update", "cms_page", p.ID, before, p)
	return &p, nil
}

// GetIDByNumber resolves a sequential display number to its UUID.
// Returns sql.ErrNoRows if no row matches.
func (s *PageService) GetIDByNumber(ctx context.Context, n int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM cms_pages WHERE number = $1`, n).Scan(&id)
	return id, err
}

func (s *PageService) Delete(ctx context.Context, id string) error {
	var before *Page
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id, ""); err == nil {
			before = prev
		}
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM cms_pages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	s.record(ctx, "cms_page.delete", "cms_page", id, before, nil)
	return nil
}

// --- Translation management ---

func (s *PageService) ListTranslations(ctx context.Context, pageID string) ([]PageTranslation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT locale, title, content, meta_title, meta_desc, updated_at
		 FROM cms_page_translations WHERE page_id = $1 ORDER BY locale`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PageTranslation, 0)
	for rows.Next() {
		var t PageTranslation
		if err := rows.Scan(&t.Locale, &t.Title, &t.Content, &t.MetaTitle, &t.MetaDesc, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PageService) UpsertTranslation(ctx context.Context, pageID, locale string, req UpsertPageTranslationRequest) (*PageTranslation, error) {
	var before *PageTranslation
	if s.audit != nil {
		if prev, err := s.getTranslation(ctx, pageID, locale); err == nil {
			before = prev
		}
	}
	var t PageTranslation
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_page_translations (page_id, locale, title, content, meta_title, meta_desc)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (page_id, locale) DO UPDATE
		   SET title=$3, content=$4, meta_title=$5, meta_desc=$6, updated_at=NOW()
		 RETURNING locale, title, content, meta_title, meta_desc, updated_at`,
		pageID, locale, req.Title, req.Content, req.MetaTitle, req.MetaDesc).
		Scan(&t.Locale, &t.Title, &t.Content, &t.MetaTitle, &t.MetaDesc, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	s.record(ctx, "cms_page.translation.upsert", "cms_page_translation",
		pageID+":"+locale, before, t)
	return &t, nil
}

func (s *PageService) DeleteTranslation(ctx context.Context, pageID, locale string) error {
	var before *PageTranslation
	if s.audit != nil {
		if prev, err := s.getTranslation(ctx, pageID, locale); err == nil {
			before = prev
		}
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM cms_page_translations WHERE page_id = $1 AND locale = $2`, pageID, locale)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.cache.DeleteByPrefix(pagePrefix)
	s.record(ctx, "cms_page.translation.delete", "cms_page_translation",
		pageID+":"+locale, before, nil)
	return nil
}
