-- 023: replace products.is_active boolean with status text
-- Idempotent: safe to re-run, and safe whether the DB still has is_active
-- (production prior to this migration) or already has status applied ad-hoc.
BEGIN;

-- 1. Add status column if missing.
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

-- 2. Backfill from is_active when it still exists.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'products' AND column_name = 'is_active') THEN
        UPDATE products
           SET status = CASE WHEN is_active THEN 'active' ELSE 'inactive' END;
    END IF;
END $$;

-- 3. Constrain allowed values.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'products_status_check') THEN
        ALTER TABLE products
            ADD CONSTRAINT products_status_check
            CHECK (status IN ('active', 'inactive', 'hidden'));
    END IF;
END $$;

-- 4. Drop the old boolean column and its index.
DROP INDEX IF EXISTS idx_products_is_active;
ALTER TABLE products DROP COLUMN IF EXISTS is_active;

-- 5. Index the new column.
CREATE INDEX IF NOT EXISTS idx_products_status ON products(status);

COMMIT;
