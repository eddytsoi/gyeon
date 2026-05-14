package cms

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

type PostCategory struct {
	ID               string  `json:"id"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	DesktopBannerURL *string `json:"desktop_banner_url,omitempty"`
	MobileBannerURL  *string `json:"mobile_banner_url,omitempty"`
	SortOrder        int     `json:"sort_order"`
}

type CreatePostCategoryRequest struct {
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	DesktopBannerURL *string `json:"desktop_banner_url"`
	MobileBannerURL  *string `json:"mobile_banner_url"`
	SortOrder        int     `json:"sort_order"`
}

type UpdatePostCategoryRequest = CreatePostCategoryRequest

type PostCategoryService struct{ db *sql.DB }

func NewPostCategoryService(db *sql.DB) *PostCategoryService {
	return &PostCategoryService{db: db}
}

func (s *PostCategoryService) List(ctx context.Context) ([]PostCategory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, name, desktop_banner_url, mobile_banner_url, sort_order
		 FROM cms_post_categories ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cats := make([]PostCategory, 0)
	for rows.Next() {
		var c PostCategory
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.DesktopBannerURL, &c.MobileBannerURL, &c.SortOrder); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *PostCategoryService) GetByID(ctx context.Context, id string) (*PostCategory, error) {
	var c PostCategory
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, desktop_banner_url, mobile_banner_url, sort_order
		 FROM cms_post_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Slug, &c.Name, &c.DesktopBannerURL, &c.MobileBannerURL, &c.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostCategoryService) GetBySlug(ctx context.Context, slug string) (*PostCategory, error) {
	var c PostCategory
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, desktop_banner_url, mobile_banner_url, sort_order
		 FROM cms_post_categories WHERE slug = $1`, slug).
		Scan(&c.ID, &c.Slug, &c.Name, &c.DesktopBannerURL, &c.MobileBannerURL, &c.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostCategoryService) Create(ctx context.Context, req CreatePostCategoryRequest) (*PostCategory, error) {
	var c PostCategory
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO cms_post_categories (slug, name, desktop_banner_url, mobile_banner_url, sort_order)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, slug, name, desktop_banner_url, mobile_banner_url, sort_order`,
		req.Slug, req.Name, req.DesktopBannerURL, req.MobileBannerURL, req.SortOrder).
		Scan(&c.ID, &c.Slug, &c.Name, &c.DesktopBannerURL, &c.MobileBannerURL, &c.SortOrder)
	return &c, err
}

func (s *PostCategoryService) Update(ctx context.Context, id string, req UpdatePostCategoryRequest) (*PostCategory, error) {
	var c PostCategory
	err := s.db.QueryRowContext(ctx,
		`UPDATE cms_post_categories
		 SET slug=$2, name=$3, desktop_banner_url=$4, mobile_banner_url=$5, sort_order=$6
		 WHERE id = $1
		 RETURNING id, slug, name, desktop_banner_url, mobile_banner_url, sort_order`,
		id, req.Slug, req.Name, req.DesktopBannerURL, req.MobileBannerURL, req.SortOrder).
		Scan(&c.ID, &c.Slug, &c.Name, &c.DesktopBannerURL, &c.MobileBannerURL, &c.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *PostCategoryService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cms_post_categories WHERE id = $1`, id)
	return err
}

// Reorder rewrites sort_order for the given post-category IDs to match
// the supplied list (1-based natural order). IDs not in the list are
// left untouched.
func (s *PostCategoryService) Reorder(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	orders := make([]int64, len(ids))
	for i := range ids {
		orders[i] = int64(i + 1)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE cms_post_categories AS c
		 SET sort_order = u.idx
		 FROM unnest($1::uuid[], $2::int[]) AS u(cid, idx)
		 WHERE c.id = u.cid`,
		pq.Array(ids), pq.Array(orders))
	return err
}
