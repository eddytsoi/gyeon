-- 024: drop 'hidden' from products.status allowed values.
-- Existing 'hidden' rows are migrated to 'inactive' (same storefront effect:
-- the public list filters status='active' so both are invisible to customers).
-- Idempotent: safe to re-run.
BEGIN;

-- 1. Backfill any existing 'hidden' rows.
UPDATE products SET status = 'inactive' WHERE status = 'hidden';

-- 2. Replace the CHECK constraint with the narrower set.
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_status_check;
ALTER TABLE products
    ADD CONSTRAINT products_status_check
    CHECK (status IN ('active', 'inactive'));

COMMIT;
