-- 115: Per-association variant pin for up-sells. The admin up-sell editor can
-- now pin a specific variant (單品 or 套裝) per association; the storefront shows
-- that variant's price/image. variant_id NULL preserves the old behaviour ("use
-- the target's default variant at render time") and keeps the WooCommerce
-- importer — which only knows product ids — working unchanged.
--
-- The original PK (product_id, upsell_product_id) forbade the same product
-- appearing twice, so it is dropped in favour of two partial unique indexes:
--   * variant_id IS NULL  → one row per (product, target)  [importer, idempotent]
--   * variant_id NOT NULL → one row per (product, target, variant) [admin pins]
BEGIN;

ALTER TABLE product_upsells
    ADD COLUMN IF NOT EXISTS variant_id UUID REFERENCES product_variants(id) ON DELETE CASCADE;

ALTER TABLE product_upsells DROP CONSTRAINT IF EXISTS product_upsells_pkey;

CREATE UNIQUE INDEX IF NOT EXISTS product_upsells_uq_default
    ON product_upsells (product_id, upsell_product_id) WHERE variant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS product_upsells_uq_variant
    ON product_upsells (product_id, upsell_product_id, variant_id) WHERE variant_id IS NOT NULL;

COMMIT;
