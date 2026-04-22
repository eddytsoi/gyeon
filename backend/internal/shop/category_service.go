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

func (s *CategoryService) List(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, parent_id, slug, name, description, image_url, sort_order, is_active, created_at, updated_at
		 FROM categories WHERE is_active = TRUE ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
			&c.ImageURL, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *CategoryService) GetByID(ctx context.Context, id string) (*Category, error) {
	var c Category
	err := s.db.QueryRowContext(ctx,
		`SELECT id, parent_id, slug, name, description, image_url, sort_order, is_active, created_at, updated_at
		 FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
			&c.ImageURL, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Create(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	var c Category
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO categories (parent_id, slug, name, description, image_url, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, parent_id, slug, name, description, image_url, sort_order, is_active, created_at, updated_at`,
		req.ParentID, req.Slug, req.Name, req.Description, req.ImageURL, req.SortOrder).
		Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
			&c.ImageURL, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Update(ctx context.Context, id string, req UpdateCategoryRequest) (*Category, error) {
	var c Category
	err := s.db.QueryRowContext(ctx,
		`UPDATE categories
		 SET parent_id=$2, slug=$3, name=$4, description=$5, image_url=$6, sort_order=$7, is_active=$8
		 WHERE id = $1
		 RETURNING id, parent_id, slug, name, description, image_url, sort_order, is_active, created_at, updated_at`,
		id, req.ParentID, req.Slug, req.Name, req.Description, req.ImageURL, req.SortOrder, req.IsActive).
		Scan(&c.ID, &c.ParentID, &c.Slug, &c.Name, &c.Description,
			&c.ImageURL, &c.SortOrder, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CategoryService) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}
