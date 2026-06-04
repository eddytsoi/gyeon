-- 121_product_custom_order.sql
-- Per-product manual sort order (自訂次序) for storefront product listings.
--
-- NULLABLE INT: most products have no manual order. The default storefront
-- ordering ("featured") sorts products WITH a custom_order first, descending
-- (NULLS LAST), then falls back to updated_at DESC for the rest. Used by the
-- category page, the main /products listing, and the [products categories="…"]
-- shortcode. Explicit [products ids="…"] keeps its author-typed order.
--
-- Idempotent: safe to re-run.
BEGIN;

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS custom_order INT;

COMMIT;
