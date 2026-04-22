-- name: ListPosts :many
SELECT * FROM cms_posts ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: ListPublishedPosts :many
SELECT * FROM cms_posts
WHERE is_published = TRUE
ORDER BY published_at DESC NULLS LAST
LIMIT $1 OFFSET $2;

-- name: CountPublishedPosts :one
SELECT COUNT(*) FROM cms_posts WHERE is_published = TRUE;

-- name: GetPostByID :one
SELECT * FROM cms_posts WHERE id = $1;

-- name: GetPostBySlug :one
SELECT * FROM cms_posts WHERE slug = $1 AND is_published = TRUE;

-- name: CreatePost :one
INSERT INTO cms_posts (category_id, slug, title, excerpt, content, cover_image_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePost :one
UPDATE cms_posts
SET category_id=$2, slug=$3, title=$4, excerpt=$5, content=$6,
    cover_image_url=$7, is_published=$8,
    published_at = CASE WHEN $8 = TRUE AND published_at IS NULL THEN NOW() ELSE published_at END
WHERE id = $1
RETURNING *;

-- name: DeletePost :exec
DELETE FROM cms_posts WHERE id = $1;
