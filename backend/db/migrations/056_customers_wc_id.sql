-- 056_customers_wc_id.sql
-- Stable WooCommerce identifier on customers so re-importing the WC store
-- becomes an idempotent upsert (matched by wc_customer_id, falls back to
-- email). Manually-created customers (wc_customer_id IS NULL) coexist
-- with imported ones; PostgreSQL treats NULLs as distinct under UNIQUE.
--
-- Idempotent: safe to re-run.
BEGIN;

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS wc_customer_id INT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'customers_wc_customer_id_key') THEN
        ALTER TABLE customers
            ADD CONSTRAINT customers_wc_customer_id_key UNIQUE (wc_customer_id);
    END IF;
END $$;

COMMIT;
