package shop

import (
	"context"
	"database/sql"
)

type Category struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parent_id,omitempty"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	MediaFileID *string `json:"media_file_id,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
	SortOrder   int     `json:"sort_order"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type CreateCategoryRequest struct {
	ParentID    *string `json:"parent_id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	MediaFileID *string `json:"media_file_id"`
	ImageURL    *string `json:"image_url"`
	SortOrder   int     `json:"sort_order"`
}

type UpdateCategoryRequest struct {
	CreateCategoryRequest
	IsActive bool `json:"is_active"`
}

type CategoryService struct {
	db *sql.DB
}

func NewCategoryService(db *sql.DB) *CategoryService {
	return &CategoryService{db: db}
}

func scanCategory(row interface{ Scan(...any) error }) (Category, error) {
	var c Category
	err := row.Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
		&c.MediaFileID, &c.ImageURL, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *CategoryService) List(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.parent_id, c.slug, c.name, c.description,
		        c.media_file_id, COALESCE(mf.url, c.image_url) AS image_url,
		        c.sort_order, c.is_active, c.created_at, c.updated_at
		 FROM categories c
		 LEFT JOIN media_files mf ON mf.id = c.media_file_id
		 WHERE c.is_active = TRUE ORDER BY c.sort_order ASC, c.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := make([]Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *CategoryService) GetByID(ctx context.Context, id string) (*Category, error) {
	c, err := scanCategory(s.db.QueryRowContext(ctx,
		`SELECT c.id, c.parent_id, c.slug, c.name, c.description,
		        c.media_file_id, COALESCE(mf.url, c.image_url) AS image_url,
		        c.sort_order, c.is_active, c.created_at, c.updated_at
		 FROM categories c
		 LEFT JOIN media_files mf ON mf.id = c.media_file_id
		 WHERE c.id = $1`, id))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Create(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	c, err := scanCategory(s.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO categories (parent_id, slug, name, description, media_file_id, image_url, sort_order)
		     VALUES ($1, $2, $3, $4, $5, $6, $7)
		     RETURNING *
		 )
		 SELECT ins.id, ins.parent_id, ins.slug, ins.name, ins.description,
		        ins.media_file_id, COALESCE(mf.url, ins.image_url) AS image_url,
		        ins.sort_order, ins.is_active, ins.created_at, ins.updated_at
		 FROM ins LEFT JOIN media_files mf ON mf.id = ins.media_file_id`,
		req.ParentID, req.Slug, req.Name, req.Description, req.MediaFileID, req.ImageURL, req.SortOrder))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Update(ctx context.Context, id string, req UpdateCategoryRequest) (*Category, error) {
	c, err := scanCategory(s.db.QueryRowContext(ctx,
		`WITH upd AS (
		     UPDATE categories
		     SET parent_id=$2, slug=$3, name=$4, description=$5,
		         media_file_id=$6, image_url=$7, sort_order=$8, is_active=$9
		     WHERE id = $1
		     RETURNING *
		 )
		 SELECT upd.id, upd.parent_id, upd.slug, upd.name, upd.description,
		        upd.media_file_id, COALESCE(mf.url, upd.image_url) AS image_url,
		        upd.sort_order, upd.is_active, upd.created_at, upd.updated_at
		 FROM upd LEFT JOIN media_files mf ON mf.id = upd.media_file_id`,
		id, req.ParentID, req.Slug, req.Name, req.Description,
		req.MediaFileID, req.ImageURL, req.SortOrder, req.IsActive))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}
