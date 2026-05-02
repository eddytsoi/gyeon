-- name: GetBundleItems :many
SELECT
    bi.id,
    bi.bundle_product_id,
    bi.component_variant_id,
    bi.quantity,
    bi.sort_order,
    bi.display_name_override,
    p.name  AS component_product_name,
    pv.name AS component_variant_name,
    pv.sku  AS component_sku,
    pv.stock_qty AS component_stock_qty,
    pv.price     AS component_price,
    bi.created_at
FROM bundle_items bi
JOIN product_variants pv ON pv.id = bi.component_variant_id
JOIN products p ON p.id = pv.product_id
WHERE bi.bundle_product_id = $1
ORDER BY bi.sort_order ASC, bi.created_at ASC;

-- name: CreateBundleItem :one
INSERT INTO bundle_items (bundle_product_id, component_variant_id, quantity, sort_order, display_name_override)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateBundleItem :one
UPDATE bundle_items
SET quantity = $2, sort_order = $3, display_name_override = $4
WHERE id = $1
RETURNING *;

-- name: DeleteBundleItem :exec
DELETE FROM bundle_items WHERE id = $1;

-- name: DeleteAllBundleItems :exec
DELETE FROM bundle_items WHERE bundle_product_id = $1;

-- name: GetDerivedBundleStock :one
SELECT COALESCE(MIN(FLOOR(pv.stock_qty::float / bi.quantity)), 0)::int AS derived_stock
FROM bundle_items bi
JOIN product_variants pv ON pv.id = bi.component_variant_id
WHERE bi.bundle_product_id = $1;
