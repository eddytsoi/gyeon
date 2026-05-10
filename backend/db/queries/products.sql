-- name: ListProducts :many
SELECT * FROM products
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListProductsByCategory :many
SELECT * FROM products
WHERE category_id = $1 AND is_active = TRUE
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountProducts :one
SELECT COUNT(*) FROM products WHERE is_active = TRUE;

-- name: CountProductsByCategory :one
SELECT COUNT(*) FROM products
WHERE category_id = $1 AND is_active = TRUE;

-- name: GetProductByID :one
SELECT * FROM products WHERE id = $1;

-- name: GetProductBySlug :one
SELECT * FROM products
WHERE slug = $1 AND is_active = TRUE;

-- name: CreateProduct :one
INSERT INTO products (category_id, slug, name, subtitle, excerpt, description, how_to_use, compatible_surfaces)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateProduct :one
UPDATE products
SET
    category_id         = $2,
    slug                = $3,
    name                = $4,
    subtitle            = $5,
    excerpt             = $6,
    description         = $7,
    how_to_use          = $8,
    compatible_surfaces = $9,
    is_active           = $10
WHERE id = $1
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products WHERE id = $1;
