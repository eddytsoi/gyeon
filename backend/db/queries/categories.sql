-- name: ListCategories :many
SELECT * FROM categories
WHERE is_active = TRUE
ORDER BY sort_order ASC, name ASC;

-- name: ListCategoriesByParent :many
SELECT * FROM categories
WHERE parent_id = $1 AND is_active = TRUE
ORDER BY sort_order ASC, name ASC;

-- name: GetCategoryByID :one
SELECT * FROM categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories
WHERE slug = $1 AND is_active = TRUE;

-- name: CreateCategory :one
INSERT INTO categories (parent_id, slug, name, description, image_url,
                        desktop_banner_url, mobile_banner_url, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories
SET
    parent_id          = $2,
    slug               = $3,
    name               = $4,
    description        = $5,
    image_url          = $6,
    desktop_banner_url = $7,
    mobile_banner_url  = $8,
    sort_order         = $9,
    is_active          = $10
WHERE id = $1
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;
