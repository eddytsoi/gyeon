-- 029_variant_weight.sql
-- Per-variant shipping weight in grams. Nullable: when blank, the
-- ShipAny quote / shipment falls back to shipany_default_weight_grams
-- so existing data keeps working unchanged.

ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS weight_grams INT;
