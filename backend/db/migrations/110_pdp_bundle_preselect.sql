-- ============================================================
-- PDP bundle "default all-selected" site settings.
-- Both the 經常一齊購買 (FBT) and 相關產品 (related-products /
-- complete-the-set) widgets share BundleComposer, whose checkboxes
-- start pre-checked for every in-stock, purchasable item.
-- Default 'true' preserves that behavior. When 'false', every item
-- in that section starts unchecked and the customer opts in.
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_fbt_preselect_all', 'true', 'PDP frequently-bought-together: pre-check all items by default. When false, items start unchecked.'),
    ('pdp_complete_set_preselect_all', 'true', 'PDP related-products bundle: pre-check all items by default. When false, items start unchecked.')
ON CONFLICT (key) DO NOTHING;
