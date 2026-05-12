-- 062_variant_sort_order.sql
-- Adds explicit display ordering for product variants so admins can drag-
-- reorder them on the product detail page. Before this, ListVariants
-- relied on created_at, which made re-arranging variants impossible
-- without rewriting timestamps.
--
-- Existing rows are seeded with sort_order = row_number() over created_at
-- (per product), so the current visible order is preserved on first deploy.

ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_product_variants_product_id_sort_order
    ON product_variants(product_id, sort_order);

UPDATE product_variants pv
SET sort_order = sub.rn
FROM (
    SELECT id, row_number() OVER (PARTITION BY product_id ORDER BY created_at) AS rn
    FROM product_variants
) sub
WHERE pv.id = sub.id AND pv.sort_order = 0;
