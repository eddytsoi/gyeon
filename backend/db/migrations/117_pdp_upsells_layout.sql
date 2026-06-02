-- ============================================================
-- 117: display style for the PDP up-sells (追加銷售) section.
-- "normal" (default) renders the full-width UpsellGrid below the product.
-- "mini" renders compact horizontal cards (120×120 image left; name /
-- subtitle / price / quick-add right) stacked in the product-info right
-- column, below the product info. Default 'normal' preserves prior behaviour.
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_upsells_layout', 'normal', 'Display style for the 追加銷售 (up-sells) PDP section: "normal" = full-width up-sells grid below the product, "mini" = compact horizontal cards in the right column under the product info.')
ON CONFLICT (key) DO NOTHING;
