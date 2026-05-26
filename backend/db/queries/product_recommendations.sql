-- Documentation copy of the queries powering the multi-algorithm FBT mix.
-- The real execution path is in backend/internal/shop/product_service.go
-- (raw SQL, like the rest of the shop service). These sqlc-style stubs
-- exist so a reviewer can find the query shapes alongside the other
-- queries without grepping the Go file.
--
-- Three candidate pools combined in FrequentlyBoughtTogether:
--   A. Co-purchase  (kept in product_copurchase, see product_copurchase.sql)
--   B. Bestsellers  — top sales over the last N days
--   C. Slow-movers  — products with stock but few/no sales over the last N days
--
-- All three pools restrict to `p.kind = 'simple'` with at least one in-stock
-- active variant so the FBT row never offers a bundle or sold-out tile.
--
-- Caller passes:
--   $1 :: text[]  — product IDs to exclude (source product + already-seen pool IDs)
--   $2 :: int     — pool cap (e.g. 20)
--   $3 :: int     — sales window in days (e.g. 30)
--   $4 :: text[]  — category IDs for "same category" filter; empty = storewide
-- The Go layer skips the category EXISTS clause when $4 is empty.

-- name: ListBestsellersPool :many
-- Top sellers over the last $3 days. Counts only top-level order_items
-- (`parent_item_id IS NULL`) so components shipped as part of a bundle don't
-- double-count toward the parent product. Restricted to simple products with
-- at least one in-stock active variant so bundles and sold-out items never
-- enter the pool.
WITH sales AS (
    SELECT pv.product_id, SUM(oi.quantity)::bigint AS qty
    FROM order_items oi
    JOIN orders o          ON o.id  = oi.order_id
    JOIN product_variants pv ON pv.id = oi.variant_id
    WHERE oi.parent_item_id IS NULL
      AND o.status IN ('paid','processing','shipped','delivered')
      AND o.created_at >= NOW() - ($3 || ' days')::interval
    GROUP BY pv.product_id
)
SELECT p.id
FROM products p
JOIN sales s ON s.product_id = p.id
WHERE p.status = 'active'
  AND p.kind = 'simple'
  AND EXISTS (
      SELECT 1 FROM product_variants pv
      WHERE pv.product_id = p.id
        AND pv.is_active = TRUE
        AND pv.stock_qty > 0
  )
  AND p.id <> ALL($1::uuid[])
  AND (cardinality($4::uuid[]) = 0 OR EXISTS (
      SELECT 1 FROM product_category_links pcl
      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($4::uuid[])
  ))
ORDER BY s.qty DESC, p.created_at DESC
LIMIT $2;

-- name: ListSlowMoversPool :many
-- Products with at least one active variant in stock (above the configured
-- low-stock floor) whose 30-day sales are at or below $5 (default: 2).
-- Bundles are excluded — their variant stock is synthetic (always 0) so
-- they'd never qualify naturally and pulling them in would require derived-
-- stock per row. The slow-mover bucket is for surfacing simple SKUs that
-- need a push.
WITH sales AS (
    SELECT pv.product_id, COALESCE(SUM(oi.quantity), 0)::bigint AS qty
    FROM product_variants pv
    LEFT JOIN order_items oi ON oi.variant_id = pv.id AND oi.parent_item_id IS NULL
    LEFT JOIN orders o       ON o.id = oi.order_id
                            AND o.status IN ('paid','processing','shipped','delivered')
                            AND o.created_at >= NOW() - ($3 || ' days')::interval
    WHERE pv.is_active = TRUE
    GROUP BY pv.product_id
),
stock AS (
    SELECT pv.product_id, SUM(pv.stock_qty)::bigint AS total_stock
    FROM product_variants pv
    WHERE pv.is_active = TRUE
    GROUP BY pv.product_id
)
SELECT p.id
FROM products p
JOIN stock st ON st.product_id = p.id
LEFT JOIN sales s ON s.product_id = p.id
WHERE p.status = 'active'
  AND p.kind = 'simple'
  AND st.total_stock > $6       -- min-stock floor (e.g. 5)
  AND COALESCE(s.qty, 0) <= $5  -- max sales over window (e.g. 2)
  AND p.id <> ALL($1::uuid[])
  AND (cardinality($4::uuid[]) = 0 OR EXISTS (
      SELECT 1 FROM product_category_links pcl
      WHERE pcl.product_id = p.id AND pcl.category_id = ANY($4::uuid[])
  ))
ORDER BY st.total_stock DESC, COALESCE(s.qty, 0) ASC, p.created_at DESC
LIMIT $2;

-- name: ListCoPurchasePool :many
-- Same shape as ListFrequentlyBoughtTogether but returns only IDs + accepts
-- an exclude list, so the Go side can dedupe across pools before hydration.
-- Restricted to simple products with at least one in-stock active variant.
SELECT p.id
FROM product_copurchase cp
JOIN products p ON p.id = cp.related_product_id
WHERE cp.product_id = $3                    -- source product
  AND p.status = 'active'
  AND p.kind = 'simple'
  AND EXISTS (
      SELECT 1 FROM product_variants pv
      WHERE pv.product_id = p.id
        AND pv.is_active = TRUE
        AND pv.stock_qty > 0
  )
  AND p.id <> ALL($1::uuid[])
ORDER BY cp.together_order_count DESC, p.created_at DESC
LIMIT $2;
