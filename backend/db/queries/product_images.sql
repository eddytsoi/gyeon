-- name: ListImagesByProduct :many
SELECT * FROM product_images
WHERE product_id = $1
ORDER BY sort_order ASC, is_primary DESC;

-- name: ListImagesByVariant :many
SELECT * FROM product_images
WHERE variant_id = $1
ORDER BY sort_order ASC;

-- name: GetPrimaryImage :one
SELECT * FROM product_images
WHERE product_id = $1 AND is_primary = TRUE
LIMIT 1;

-- name: CreateProductImage :one
INSERT INTO product_images (product_id, variant_id, url, alt_text, sort_order, is_primary)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: SetPrimaryImage :exec
UPDATE product_images SET is_primary = (id = $2)
WHERE product_id = $1;

-- name: DeleteProductImage :exec
DELETE FROM product_images WHERE id = $1;

-- name: DeleteImagesByProduct :exec
DELETE FROM product_images WHERE product_id = $1;
