-- ============================================================
-- Per-product "How to Use" and "Compatible Surfaces" content
-- ============================================================
-- Both render in the storefront product detail page tabs alongside
-- the existing "Content" (description) tab. how_to_use is markdown.
-- compatible_surfaces is a small set of admin-toggled keys whose
-- icons + labels are rendered by the frontend; unknown keys are
-- ignored, so the column is intentionally unconstrained.

ALTER TABLE products
    ADD COLUMN how_to_use          TEXT,
    ADD COLUMN compatible_surfaces TEXT[] NOT NULL DEFAULT '{}';
