-- 025_variant_name.sql
-- Optional human-readable label for a variant (e.g. "Navy / Medium").
-- Nullable; when null the UI falls back to the SKU.

ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS name TEXT;
