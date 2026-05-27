-- ============================================================
-- Promotion max_order_amount upper bound
-- ============================================================
-- Adds an optional upper bound on the cart subtotal at which a
-- campaign / coupon stays eligible. Pairs with the existing
-- min_order_amount column so promotions can be restricted to a
-- subtotal range (e.g. "$500–$2000 only — no whales").
--
-- Nullable, mirroring min_order_amount. NULL = no cap.

ALTER TABLE discount_campaigns
    ADD COLUMN IF NOT EXISTS max_order_amount NUMERIC(12,2);

ALTER TABLE coupon_codes
    ADD COLUMN IF NOT EXISTS max_order_amount NUMERIC(12,2);
