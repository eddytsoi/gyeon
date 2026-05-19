-- ============================================================
-- PDP "taobao" layout: add-to-cart opens a modal that surfaces
-- both variants ("基本裝") and curated bundle products ("優惠套裝").
-- ============================================================
-- The original PDP layout stays the default. A site setting flips
-- the default; products.use_taobao_layout is a per-product override
-- (NULL = follow site default, TRUE = force taobao, FALSE = force
-- classic).

INSERT INTO site_settings (key, value, description) VALUES
  ('pdp_taobao_layout_enabled', 'false',
   'When true, the storefront PDP uses the taobao-style add-to-cart modal site-wide. Each product can override via products.use_taobao_layout.')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE products
    ADD COLUMN use_taobao_layout BOOLEAN;

-- product_promo_bundles links a parent product to the bundle products
-- shown as "優惠套裝" inside the taobao modal. The bundle products are
-- themselves rows in `products` with kind = 'bundle' — the same model
-- the existing BundleComposer surfaces below the classic PDP. Cascade
-- on delete from either side: removing the parent or the bundle nukes
-- the association without leaving a dangling row.
CREATE TABLE product_promo_bundles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    bundle_product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (parent_product_id, bundle_product_id)
);
CREATE INDEX idx_promo_bundles_parent ON product_promo_bundles(parent_product_id, sort_order);
