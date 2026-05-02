-- 032_wc_import_keys.sql
-- Stable identifiers for WooCommerce-imported records so re-import becomes
-- an upsert instead of a destructive clear-and-reimport. Admin work
-- (translations, manual images, manual variants) survives a re-sync.
--
-- All new columns are NULLABLE so manually-created products / variants /
-- media files coexist with imported ones. PostgreSQL treats NULLs as
-- distinct under UNIQUE, so many manual rows can sit alongside one mapped
-- WC record.
--
-- Idempotent: safe to re-run.
BEGIN;

-- products.wc_product_id ---------------------------------------------------
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS wc_product_id INT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'products_wc_product_id_key') THEN
        ALTER TABLE products
            ADD CONSTRAINT products_wc_product_id_key UNIQUE (wc_product_id);
    END IF;
END $$;

-- product_variants.wc_variation_id ----------------------------------------
-- For variable products: stores the WC variation ID (globally unique in WC).
-- For simple products: NULL — the (product_id, wc_variation_id IS NULL)
-- predicate uniquely identifies the simple-product fallback variant.
ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS wc_variation_id INT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'product_variants_wc_variation_id_key') THEN
        ALTER TABLE product_variants
            ADD CONSTRAINT product_variants_wc_variation_id_key UNIQUE (wc_variation_id);
    END IF;
END $$;

-- media_files.source_url --------------------------------------------------
-- Original URL the file was downloaded from. Set when the importer fetches
-- a WC product image; left NULL for admin uploads. Allows the importer to
-- reuse an existing media row instead of re-downloading on every re-sync.
ALTER TABLE media_files
    ADD COLUMN IF NOT EXISTS source_url VARCHAR(2048);

CREATE INDEX IF NOT EXISTS idx_media_files_source_url
    ON media_files(source_url) WHERE source_url IS NOT NULL;

COMMIT;
