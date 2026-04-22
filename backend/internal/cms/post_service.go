package cms

import (
	"context"
	"database/sql"
	"errors"
)

type Post struct {
	ID            string  `json:"id"`
	CategoryID    *string `json:"category_id,omitempty"`
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Excerpt       *string `json:"excerpt,omitempty"`
	Content       string  `json:"content"`
	CoverImageURL *string `json:"cover_image_url,omitempty"`
	IsPublished   bool    `json:"is_published"`
	PublishedAt   *string `json:"published_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type CreatePostRequest struct {
	CategoryID    *string `json:"category_id"`
	Slug          string  `json:"slug"`
	Title         string  `json:"title"`
	Excerpt       *string `json:"excerpt"`
	Content       string  `json:"content"`
	CoverImageURL *string `json:"cover_image_url"`
	IsPublished   bool    `json:"is_published"`
}

// UpdatePostRequest is the same as CreatePostRequest (is_published included in both).
type UpdatePostRequest = CreatePostRequest

type PostService struct{ db *sql.DB }

func NewPostService(db *sql.DB) *PostService { return &PostService{db: db} }

func (s *PostService) List(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, category_id, slug, title, excerpt, content, cover_image_url,
		        is_published, published_at, created_at, updated_at
		 FROM cms_posts ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt,
			&p.Content, &p.CoverImageURL, &p.IsPublished, &p.PublishedAt,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (s *PostService) ListPublished(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, category_id, slug, title, excerpt, content, cover_image_url,
		        is_published, published_at, created_at, updated_at
		 FROM cms_posts WHERE is_published = TRUE
		 ORDER BY published_at DESC NULLS LAST LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt,
			&p.Content, &p.CoverImageURL, &p.IsPublished, &p.PublishedAt,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (s *PostService) GetByID(ctx context.Context, id string) (*Post, error) {
	var p Post
	err := s.db.QueryRowContext(ctx,
		`SELECT id, category_id, slug, title, excerpt, content, cover_image_url,
		        is_published, published_at, created_at, updated_at
		 FROM cms_posts WHERE id = $1`, id).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt, &p.Content,
			&p.CoverImageURL, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *PostService) GetBySlug(ctx context.Context, slug string) (*Post, error) {
	var p Post
	err := s.db.QueryRowContext(ctx,
		`SELECT id, category_id, slug, title, excerpt, content, cover_image_url,
		        is_published, published_at, created_at, updated_at
		 FROM cms_posts WHERE slug = $1 AND is_published = TRUE`, slug).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt, &p.Content,
			&p.CoverImageURL, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *PostService) Create(ctx context.Context, req CreatePostRequest) (*Post, error) {
	var p Post
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_posts (category_id, slug, title, excerpt, content, cover_image_url,
		                        is_published, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7,
		         CASE WHEN $7 = TRUE THEN NOW() ELSE NULL END)
		 RETURNING id, category_id, slug, title, excerpt, content, cover_image_url,
		           is_published, published_at, created_at, updated_at`,
		req.CategoryID, req.Slug, req.Title, req.Excerpt, req.Content, req.CoverImageURL, req.IsPublished).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt, &p.Content,
			&p.CoverImageURL, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	return &p, err
}

func (s *PostService) Update(ctx context.Context, id string, req UpdatePostRequest) (*Post, error) {
	var p Post
	err := s.db.QueryRowContext(ctx,
		`UPDATE cms_posts
		 SET category_id=$2, slug=$3, title=$4, excerpt=$5, content=$6,
		     cover_image_url=$7, is_published=$8,
		     published_at = CASE WHEN $8 = TRUE AND published_at IS NULL THEN NOW() ELSE published_at END
		 WHERE id = $1
		 RETURNING id, category_id, slug, title, excerpt, content, cover_image_url,
		           is_published, published_at, created_at, updated_at`,
		id, req.CategoryID, req.Slug, req.Title, req.Excerpt, req.Content,
		req.CoverImageURL, req.IsPublished).
		Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Title, &p.Excerpt, &p.Content,
			&p.CoverImageURL, &p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	return &p, err
}

func (s *PostService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cms_posts WHERE id = $1`, id)
	return err
}
