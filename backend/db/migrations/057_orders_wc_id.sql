-- 057_orders_wc_id.sql
-- Stable WooCommerce identifier on orders so re-importing the WC store
-- becomes an idempotent upsert keyed by WC order ID. Manually-created
-- orders (wc_order_id IS NULL) coexist with imported ones; PostgreSQL
-- treats NULLs as distinct under UNIQUE so many manual orders sit
-- alongside one mapped WC record.
--
-- Idempotent: safe to re-run.
BEGIN;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS wc_order_id INT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'orders_wc_order_id_key') THEN
        ALTER TABLE orders
            ADD CONSTRAINT orders_wc_order_id_key UNIQUE (wc_order_id);
    END IF;
END $$;

COMMIT;
