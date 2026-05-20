-- Source-of-truth aggregation that ProductService.RebuildCopurchase runs to
-- repopulate product_copurchase. Kept as a sqlc-style file alongside the
-- other query definitions for documentation; the real execution path is in
-- backend/internal/shop/product_service.go (raw SQL, like the rest of the
-- shop service).

-- name: RebuildProductCopurchase :exec
-- 1) wipe the table, 2) re-aggregate pairs from paid+ orders.
-- HAVING >= 2 drops one-off coincidences (a single shared order isn't a
-- "frequently bought together" signal).
TRUNCATE TABLE product_copurchase;
INSERT INTO product_copurchase (product_id, related_product_id, together_order_count)
SELECT v1.product_id, v2.product_id, COUNT(DISTINCT o.id) AS together
FROM orders o
JOIN order_items oi1 ON oi1.order_id = o.id
JOIN product_variants v1 ON v1.id = oi1.variant_id
JOIN order_items oi2 ON oi2.order_id = o.id AND oi2.id <> oi1.id
JOIN product_variants v2 ON v2.id = oi2.variant_id
WHERE o.status IN ('paid','processing','shipped','delivered')
  AND v1.product_id <> v2.product_id
GROUP BY v1.product_id, v2.product_id
HAVING COUNT(DISTINCT o.id) >= 2;

-- name: ListFrequentlyBoughtTogether :many
-- Used by the storefront PDP; ranks by historical co-purchase count, filters
-- to publicly-visible products only.
SELECT p.*
FROM product_copurchase cp
JOIN products p ON p.id = cp.related_product_id
WHERE cp.product_id = $1
  AND p.status = 'active'
ORDER BY cp.together_order_count DESC, p.created_at DESC
LIMIT $2;
