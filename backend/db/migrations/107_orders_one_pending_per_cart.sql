-- 107_orders_one_pending_per_cart.sql
-- Enforce at most one in-flight (pending) order per cart, so a shopper who
-- closes the checkout tab after a failed payment and reopens cart/checkout in a
-- new tab continues the SAME order instead of creating a duplicate. The
-- application also dedups proactively in OrderService.Checkout(); this partial
-- unique index is the concurrency backstop for racing checkout requests.
--
-- Imported orders (WooCommerce etc.) carry cart_id = NULL and are unaffected.

-- 1) Clean up any duplicates this bug already produced, otherwise the unique
--    index below would fail to build. For each cart with >1 pending order keep
--    the newest and cancel the rest. Restock their items the same way the app's
--    cancel path does (UpdateStatus -> restockOrderItemsTx restocks every
--    order_item with a non-null variant_id), so inventory matches a manual
--    admin cancel.
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY cart_id ORDER BY created_at DESC) AS rn
    FROM orders
    WHERE status = 'pending' AND cart_id IS NOT NULL
),
to_cancel AS (
    SELECT id FROM ranked WHERE rn > 1
),
restock AS (
    SELECT oi.variant_id AS vid, SUM(oi.quantity) AS qty
    FROM order_items oi
    JOIN to_cancel tc ON tc.id = oi.order_id
    WHERE oi.variant_id IS NOT NULL
    GROUP BY oi.variant_id
)
UPDATE product_variants pv
SET stock_qty = pv.stock_qty + r.qty
FROM restock r
WHERE pv.id = r.vid;

-- Record the cancel in the status history, then flip the duplicates to
-- cancelled. (Re-derives the same rn>1 set; restock above did not change status.)
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY cart_id ORDER BY created_at DESC) AS rn
    FROM orders
    WHERE status = 'pending' AND cart_id IS NOT NULL
)
INSERT INTO order_status_history (order_id, status)
SELECT id, 'cancelled' FROM ranked WHERE rn > 1;

WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY cart_id ORDER BY created_at DESC) AS rn
    FROM orders
    WHERE status = 'pending' AND cart_id IS NOT NULL
)
UPDATE orders
SET status = 'cancelled', updated_at = now()
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

-- 2) The backstop.
CREATE UNIQUE INDEX IF NOT EXISTS orders_one_pending_per_cart
    ON orders (cart_id)
    WHERE status = 'pending' AND cart_id IS NOT NULL;
