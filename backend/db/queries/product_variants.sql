-- name: ListVariantsByProduct :many
SELECT * FROM product_variants
WHERE product_id = $1 AND is_active = TRUE
ORDER BY sort_order ASC, created_at ASC;

-- name: ReorderVariants :exec
UPDATE product_variants AS pv
SET sort_order = data.sort_order
FROM (
    SELECT unnest(@ids::uuid[])                        AS id,
           generate_subscripts(@ids::uuid[], 1)::int   AS sort_order
) AS data
WHERE pv.id = data.id AND pv.product_id = @product_id;

-- name: GetVariantByID :one
SELECT * FROM product_variants WHERE id = $1;

-- name: GetVariantBySKU :one
SELECT * FROM product_variants WHERE sku = $1;

-- name: CreateVariant :one
INSERT INTO product_variants (product_id, sku, price, compare_at_price, stock_qty)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateVariant :one
UPDATE product_variants
SET
    sku              = $2,
    price            = $3,
    compare_at_price = $4,
    stock_qty        = $5,
    is_active        = $6
WHERE id = $1
RETURNING *;

-- name: AdjustStock :one
UPDATE product_variants
SET stock_qty = stock_qty + $2
WHERE id = $1
RETURNING *;

-- name: DecrementStock :one
UPDATE product_variants
SET stock_qty = stock_qty - $2
WHERE id = $1 AND stock_qty >= $2
RETURNING *;

-- name: DeleteVariant :exec
DELETE FROM product_variants WHERE id = $1;

-- name: ListVariantAttributeValues :many
SELECT
    pav.id,
    pav.attribute_id,
    pa.name  AS attribute_name,
    pav.value,
    pav.sort_order
FROM product_variant_attribute_values pvav
JOIN product_attribute_values pav ON pav.id = pvav.attribute_value_id
JOIN product_attributes pa ON pa.id = pav.attribute_id
WHERE pvav.variant_id = $1
ORDER BY pa.sort_order, pav.sort_order;

-- name: SetVariantAttributeValue :exec
INSERT INTO product_variant_attribute_values (variant_id, attribute_value_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearVariantAttributeValues :exec
DELETE FROM product_variant_attribute_values WHERE variant_id = $1;
