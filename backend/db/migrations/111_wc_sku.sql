-- 111_wc_sku.sql
-- Capture the original WooCommerce SKU alongside Gyeon's own generated `sku`.
-- The importer auto-generates `product_variants.sku` (NOT NULL UNIQUE) from the
-- product slug; `wc_sku` is a separate informational mirror of WC's own SKU so
-- merchants can cross-reference Gyeon records against the WooCommerce/Taobao
-- catalog. The generated `sku` is untouched.
--
-- Both columns are NULLABLE and NOT UNIQUE: manually-created rows have no WC
-- SKU, WC permits empty/duplicate SKUs, and a simple product's products.wc_sku
-- equals its single variant's product_variants.wc_sku. Empty WC SKUs are stored
-- as NULL by the importer.
--
-- Idempotent: safe to re-run.
BEGIN;

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS wc_sku VARCHAR(255);

ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS wc_sku VARCHAR(255);

COMMIT;
