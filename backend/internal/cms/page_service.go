package cms

import (
	"context"
	"database/sql"
	"errors"
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

type PageService struct{ db *sql.DB }

func NewPageService(db *sql.DB) *PageService { return &PageService{db: db} }

func (s *PageService) List(ctx context.Context) ([]Page, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, title, content, meta_title, meta_desc, is_published, created_at, updated_at
		 FROM cms_pages ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := make([]Page, 0)
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Content,
			&p.MetaTitle, &p.MetaDesc, &p.IsPublished, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (s *PageService) GetBySlug(ctx context.Context, slug string) (*Page, error) {
	var p Page
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, title, content, meta_title, meta_desc, is_published, created_at, updated_at
		 FROM cms_pages WHERE slug = $1 AND is_published = TRUE`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *PageService) GetByID(ctx context.Context, id string) (*Page, error) {
	var p Page
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, title, content, meta_title, meta_desc, is_published, created_at, updated_at
		 FROM cms_pages WHERE id = $1`, id).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.MetaTitle, &p.MetaDesc,
			&p.IsPublished, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
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
	return &p, err
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
	return &p, err
}

func (s *PageService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cms_pages WHERE id = $1`, id)
	return err
}
