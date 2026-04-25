package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gyeon/backend/internal/cache"
)

type Page struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	MetaTitle   *string `json:"meta_title,omitempty"`
	MetaDesc    *string `json:"meta_desc,omitempty"`
	IsPublished bool    `json:"is_published"`
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
}

// UpdatePageRequest is the same as CreatePageRequest (is_published included in both).
type UpdatePageRequest = CreatePageRequest

var ErrNotFound = errors.New("not found")

const pagePrefix = "cms:pages:"

type PageService struct {
	db    *sql.DB
	cache cache.Store
	ttl   func(context.Context) time.Duration
}

func NewPageService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *PageService {
	return &PageService{db: db, cache: c, ttl: ttl}
}

const pageTranslationJoin = `
	LEFT JOIN cms_page_translations t ON t.page_id = p.id AND t.locale = $1`

const pageSelect = `
	SELECT p.id, p.slug,
	       COALESCE(t.title,     p.title)     AS title,
	       COALESCE(t.content,   p.content)   AS content,
	       COALESCE(t.meta_title, p.meta_title) AS meta_title,
	       COALESCE(t.meta_desc,  p.meta_desc)  AS meta_desc,
	       p.is_published, p.created_at, p.updated_at
	FROM cms_pages p` + pageTranslationJoin

func scanPage(row interface{ Scan(...any) error }) (Page, error) {
	var p Page
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Content,
		&p.MetaTitle, &p.MetaDesc, &p.IsPublished, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List returns all pages. locale may be empty for base content.
func (s *PageService) List(ctx context.Context, locale string) ([]Page, error) {
	key := fmt.Sprintf("cms:pages:list:%s", locale)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Page), nil
	}
	rows, err := s.db.QueryContext(ctx,
		pageSelect+` ORDER BY p.created_at DESC`, locale)
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
		`INSERT INTO cms_pages (slug, title, content, meta_title, meta_desc, is_published)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, slug, title, content, meta_title, meta_desc, is_published, created_at, updated_at`,
		req.Slug, req.Title, req.Content, req.MetaTitle, req.MetaDesc, req.IsPublished).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	return &p, nil
}

func (s *PageService) Update(ctx context.Context, id string, req UpdatePageRequest) (*Page, error) {
	var p Page
	err := s.db.QueryRowContext(ctx,
		`UPDATE cms_pages SET slug=$2, title=$3, content=$4, meta_title=$5, meta_desc=$6, is_published=$7
		 WHERE id = $1
		 RETURNING id, slug, title, content, meta_title, meta_desc, is_published, created_at, updated_at`,
		id, req.Slug, req.Title, req.Content, req.MetaTitle, req.MetaDesc, req.IsPublished).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(pagePrefix)
	return &p, nil
}

func (s *PageService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cms_pages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(pagePrefix)
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
	return &t, nil
}

func (s *PageService) DeleteTranslation(ctx context.Context, pageID, locale string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM cms_page_translations WHERE page_id = $1 AND locale = $2`, pageID, locale)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.cache.DeleteByPrefix(pagePrefix)
	return nil
}
