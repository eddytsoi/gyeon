-- 027_orders_cart_id.sql
-- Persist the source cart_id on each order so the webhook handler can
-- empty the cart only after Stripe confirms payment_intent.succeeded.
-- Nullable: ON DELETE SET NULL preserves orders if the cart row is
-- ever deleted, and lets us still create orders for callers (e.g.
-- imports) that don't have a cart context.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS cart_id UUID
        REFERENCES carts(id) ON DELETE SET NULL;
