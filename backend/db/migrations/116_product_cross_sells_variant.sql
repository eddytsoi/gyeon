-- 116: Per-association variant pin for cross-sells — the cross-sell counterpart
-- to migration 115. See that file for the rationale behind the nullable
-- variant_id and the two partial unique indexes replacing the composite PK.
BEGIN;

ALTER TABLE product_cross_sells
    ADD COLUMN IF NOT EXISTS variant_id UUID REFERENCES product_variants(id) ON DELETE CASCADE;

ALTER TABLE product_cross_sells DROP CONSTRAINT IF EXISTS product_cross_sells_pkey;

CREATE UNIQUE INDEX IF NOT EXISTS product_cross_sells_uq_default
    ON product_cross_sells (product_id, cross_sell_product_id) WHERE variant_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS product_cross_sells_uq_variant
    ON product_cross_sells (product_id, cross_sell_product_id, variant_id) WHERE variant_id IS NOT NULL;

COMMIT;
