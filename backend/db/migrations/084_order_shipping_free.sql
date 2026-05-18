-- 084_order_shipping_free.sql
-- Persist whether an order shipped under the merchant-paid free-shipping
-- promo (true) or as SF freight-collect / pay-on-delivery (false). Today the
-- numeric orders.shipping_fee is always 0 in both cases, so receipts, emails
-- and the account order page cannot tell the two apart and end up showing
-- "HK$0.00" everywhere. We freeze the decision at checkout so later changes
-- to the free_shipping_threshold_* settings don't retroactively rewrite the
-- label on historical orders.

ALTER TABLE orders ADD COLUMN shipping_free BOOLEAN NOT NULL DEFAULT FALSE;

-- Best-effort backfill for existing orders, using the *current* threshold
-- setting. We accept some drift on historical orders — there is no way to
-- recover the threshold value at the time of those orders, and showing the
-- correct label on the bulk of past orders is strictly better than the
-- current behavior of always printing "HK$0.00".
UPDATE orders o
SET shipping_free = TRUE
WHERE COALESCE(
        (SELECT value FROM site_settings WHERE key = 'free_shipping_threshold_enabled'),
        'false'
      ) = 'true'
  AND (o.subtotal - COALESCE(o.discount_amount, 0)) >= COALESCE(
        NULLIF((SELECT value FROM site_settings WHERE key = 'free_shipping_threshold_hkd'), '')::numeric,
        1e18
      );
