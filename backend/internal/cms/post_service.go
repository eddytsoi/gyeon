package cms

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/util"
)

// postSearchFields are matched by the optional admin `search` param on List.
// Body content is intentionally excluded.
var postSearchFields = []string{"p.title", "p.excerpt", "p.slug", "p.number::text"}

type Post struct {
	ID               string  `json:"id"`
	Number           int64   `json:"number"`
	CategoryID       *string `json:"category_id,omitempty"`
	CategorySlug     *string `json:"category_slug,omitempty"`
	CategoryName     *string `json:"category_name,omitempty"`
	Slug             string  `json:"slug"`
	Title            string  `json:"title"`
	Excerpt          *string `json:"excerpt,omitempty"`
	Content          string  `json:"content"`
	CoverMediaFileID *string `json:"cover_media_file_id,omitempty"`
	CoverImageURL    *string `json:"cover_image_url,omitempty"`
	IsPublished      bool    `json:"is_published"`
	PublishedAt      *string `json:"published_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type PostTranslation struct {
	Locale    string  `json:"locale"`
	Title     string  `json:"title"`
	Excerpt   *string `json:"excerpt,omitempty"`
	Content   string  `json:"content"`
	UpdatedAt string  `json:"updated_at"`
}

type UpsertPostTranslationRequest struct {
	Title   string  `json:"title"`
	Excerpt *string `json:"excerpt"`
	Content string  `json:"content"`
}

type CreatePostRequest struct {
	CategoryID       *string `json:"category_id"`
	Slug             string  `json:"slug"`
	Title            string  `json:"title"`
	Excerpt          *string `json:"excerpt"`
	Content          string  `json:"content"`
	CoverMediaFileID *string `json:"cover_media_file_id"`
	CoverImageURL    *string `json:"cover_image_url"`
	IsPublished      bool    `json:"is_published"`
}

// UpdatePostRequest is the same as CreatePostRequest (is_published included in both).
type UpdatePostRequest = CreatePostRequest

const postPrefix = "cms:posts:"

type PostService struct {
	db    *sql.DB
	cache cache.Store
	ttl   func(context.Context) time.Duration
}

func NewPostService(db *sql.DB, c cache.Store, ttl func(context.Context) time.Duration) *PostService {
	return &PostService{db: db, cache: c, ttl: ttl}
}

const postTranslationJoin = `
	LEFT JOIN cms_post_translations t ON t.post_id = p.id AND t.locale = $1`

const postSelect = `
	SELECT p.id, p.number, p.category_id, c.slug, c.name, p.slug,
	       COALESCE(t.title,   p.title)   AS title,
	       COALESCE(t.excerpt, p.excerpt) AS excerpt,
	       COALESCE(t.content, p.content) AS content,
	       p.cover_media_file_id,
	       COALESCE(mf.url, p.cover_image_url) AS cover_image_url,
	       p.is_published, p.published_at, p.created_at, p.updated_at
	FROM cms_posts p` + postTranslationJoin + `
	LEFT JOIN media_files mf ON mf.id = p.cover_media_file_id
	LEFT JOIN cms_post_categories c ON c.id = p.category_id`

func scanPost(row interface{ Scan(...any) error }) (Post, error) {
	var p Post
	err := row.Scan(&p.ID, &p.Number, &p.CategoryID, &p.CategorySlug, &p.CategoryName,
		&p.Slug, &p.Title, &p.Excerpt,
		&p.Content, &p.CoverMediaFileID, &p.CoverImageURL,
		&p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List returns all posts (admin). locale may be empty for base content.
// search is an optional case-insensitive substring matched against
// postSearchFields; pass "" to disable. categorySlug, when non-empty,
// restricts the result to posts whose category matches the given slug.
func (s *PostService) List(ctx context.Context, locale, search, categorySlug string, limit, offset int) ([]Post, error) {
	key := fmt.Sprintf("cms:posts:all:%s:%s:%s:%d:%d", locale, search, categorySlug, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Post), nil
	}

	args := []any{locale, limit, offset}
	wheres := []string{}
	if categorySlug != "" {
		args = append(args, categorySlug)
		wheres = append(wheres, fmt.Sprintf(`p.category_id = (SELECT id FROM cms_post_categories WHERE slug = $%d)`, len(args)))
	}
	if clause, arg := util.BuildSearchClause(search, postSearchFields, len(args)+1); clause != "" {
		args = append(args, arg)
		wheres = append(wheres, clause)
	}
	query := postSelect
	if len(wheres) > 0 {
		query += ` WHERE ` + strings.Join(wheres, ` AND `)
	}
	query += ` ORDER BY p.created_at DESC LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, posts, s.ttl(ctx))
	return posts, nil
}

// ListPublishedByCategorySlug returns published posts filtered by a category
// (matched by its slug). locale may be empty.
func (s *PostService) ListPublishedByCategorySlug(ctx context.Context, locale, categorySlug string, limit, offset int) ([]Post, error) {
	key := fmt.Sprintf("cms:posts:pub:bycat:%s:%s:%d:%d", locale, categorySlug, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Post), nil
	}
	rows, err := s.db.QueryContext(ctx,
		postSelect+` WHERE p.is_published = TRUE
		             AND p.category_id = (SELECT id FROM cms_post_categories WHERE slug = $4)
		             ORDER BY p.published_at DESC NULLS LAST LIMIT $2 OFFSET $3`,
		locale, limit, offset, categorySlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, posts, s.ttl(ctx))
	return posts, nil
}

// ListPublished returns published posts for the storefront. locale may be empty.
func (s *PostService) ListPublished(ctx context.Context, locale string, limit, offset int) ([]Post, error) {
	key := fmt.Sprintf("cms:posts:pub:%s:%d:%d", locale, limit, offset)
	if v, ok := s.cache.Get(key); ok {
		return v.([]Post), nil
	}
	rows, err := s.db.QueryContext(ctx,
		postSelect+` WHERE p.is_published = TRUE ORDER BY p.published_at DESC NULLS LAST LIMIT $2 OFFSET $3`,
		locale, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.cache.Set(key, posts, s.ttl(ctx))
	return posts, nil
}

// GetByID fetches any post. locale may be empty for base content.
func (s *PostService) GetByID(ctx context.Context, id, locale string) (*Post, error) {
	key := fmt.Sprintf("cms:posts:id:%s:%s", id, locale)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Post)
		return &p, nil
	}
	p, err := scanPost(s.db.QueryRowContext(ctx,
		postSelect+` WHERE p.id = $2`, locale, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
}

// GetBySlug fetches a published post. locale may be empty for base content.
func (s *PostService) GetBySlug(ctx context.Context, slug, locale string) (*Post, error) {
	key := fmt.Sprintf("cms:posts:slug:%s:%s", slug, locale)
	if v, ok := s.cache.Get(key); ok {
		p := v.(Post)
		return &p, nil
	}
	p, err := scanPost(s.db.QueryRowContext(ctx,
		postSelect+` WHERE p.slug = $2 AND p.is_published = TRUE`, locale, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.cache.Set(key, p, s.ttl(ctx))
	return &p, nil
}

func (s *PostService) Create(ctx context.Context, req CreatePostRequest) (*Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO cms_posts (category_id, slug, title, excerpt, content,
		                            cover_media_file_id, cover_image_url,
		                            is_published, published_at)
		     VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
		             CASE WHEN $8 = TRUE THEN NOW() ELSE NULL END)
		     RETURNING *
		 )
		 SELECT ins.id, ins.number, ins.category_id, c.slug, c.name,
		        ins.slug, ins.title, ins.excerpt, ins.content,
		        ins.cover_media_file_id,
		        COALESCE(mf.url, ins.cover_image_url) AS cover_image_url,
		        ins.is_published, ins.published_at, ins.created_at, ins.updated_at
		 FROM ins
		 LEFT JOIN media_files mf ON mf.id = ins.cover_media_file_id
		 LEFT JOIN cms_post_categories c ON c.id = ins.category_id`,
		req.CategoryID, req.Slug, req.Title, req.Excerpt, req.Content,
		req.CoverMediaFileID, req.CoverImageURL, req.IsPublished))
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(postPrefix)
	return &p, nil
}

func (s *PostService) Update(ctx context.Context, id string, req UpdatePostRequest) (*Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx,
		`WITH upd AS (
		     UPDATE cms_posts
		     SET category_id=$2, slug=$3, title=$4, excerpt=$5, content=$6,
		         cover_media_file_id=$7, cover_image_url=$8, is_published=$9,
		         published_at = CASE WHEN $9 = TRUE AND published_at IS NULL THEN NOW() ELSE published_at END
		     WHERE id = $1
		     RETURNING *
		 )
		 SELECT upd.id, upd.number, upd.category_id, c.slug, c.name,
		        upd.slug, upd.title, upd.excerpt, upd.content,
		        upd.cover_media_file_id,
		        COALESCE(mf.url, upd.cover_image_url) AS cover_image_url,
		        upd.is_published, upd.published_at, upd.created_at, upd.updated_at
		 FROM upd
		 LEFT JOIN media_files mf ON mf.id = upd.cover_media_file_id
		 LEFT JOIN cms_post_categories c ON c.id = upd.category_id`,
		id, req.CategoryID, req.Slug, req.Title, req.Excerpt, req.Content,
		req.CoverMediaFileID, req.CoverImageURL, req.IsPublished))
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(postPrefix)
	return &p, nil
}

// GetIDByNumber resolves a sequential display number to its UUID.
// Returns sql.ErrNoRows if no row matches.
func (s *PostService) GetIDByNumber(ctx context.Context, n int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM cms_posts WHERE number = $1`, n).Scan(&id)
	return id, err
}

func (s *PostService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cms_posts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	s.cache.DeleteByPrefix(postPrefix)
	return nil
}

// --- Translation management ---

func (s *PostService) ListTranslations(ctx context.Context, postID string) ([]PostTranslation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT locale, title, excerpt, content, updated_at
		 FROM cms_post_translations WHERE post_id = $1 ORDER BY locale`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PostTranslation, 0)
	for rows.Next() {
		var t PostTranslation
		if err := rows.Scan(&t.Locale, &t.Title, &t.Excerpt, &t.Content, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PostService) UpsertTranslation(ctx context.Context, postID, locale string, req UpsertPostTranslationRequest) (*PostTranslation, error) {
	var t PostTranslation
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_post_translations (post_id, locale, title, excerpt, content)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (post_id, locale) DO UPDATE
		   SET title=$3, excerpt=$4, content=$5, updated_at=NOW()
		 RETURNING locale, title, excerpt, content, updated_at`,
		postID, locale, req.Title, req.Excerpt, req.Content).
		Scan(&t.Locale, &t.Title, &t.Excerpt, &t.Content, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.cache.DeleteByPrefix(postPrefix)
	return &t, nil
}

func (s *PostService) DeleteTranslation(ctx context.Context, postID, locale string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM cms_post_translations WHERE post_id = $1 AND locale = $2`, postID, locale)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.cache.DeleteByPrefix(postPrefix)
	return nil
}
