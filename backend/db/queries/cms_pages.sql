-- name: ListPages :many
SELECT * FROM cms_pages ORDER BY created_at DESC;

-- name: ListPublishedPages :many
SELECT * FROM cms_pages WHERE is_published = TRUE ORDER BY title ASC;

-- name: GetPageByID :one
SELECT * FROM cms_pages WHERE id = $1;

-- name: GetPageBySlug :one
SELECT * FROM cms_pages WHERE slug = $1 AND is_published = TRUE;

-- name: CreatePage :one
INSERT INTO cms_pages (slug, title, content, meta_title, meta_desc)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdatePage :one
UPDATE cms_pages
SET slug=$2, title=$3, content=$4, meta_title=$5, meta_desc=$6, is_published=$7
WHERE id = $1
RETURNING *;

-- name: DeletePage :exec
DELETE FROM cms_pages WHERE id = $1;
