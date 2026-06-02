-- ============================================================
-- 115: storefront visibility + label overrides for the WooCommerce
-- up-sells (PDP) and cross-sells (cart) sections. Mirrors the existing
-- FBT/related toggles. `*_show_*` default 'true'; the kicker/heading
-- overrides default '' so the storefront falls back to the i18n strings
-- (m.product_detail_upsells_*, m.cart_cross_sells_*).
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_show_upsells',         'true', 'PDP up-sells (WooCommerce alternatives) section: show when the product has up-sells. When false, hidden.'),
    ('pdp_upsells_kicker',       '',     'Override for the PDP up-sells kicker. Empty falls back to m.product_detail_upsells_kicker().'),
    ('pdp_upsells_heading',      '',     'Override for the PDP up-sells heading. Empty falls back to m.product_detail_upsells_heading().'),
    ('cart_show_cross_sells',    'true', 'Cart cross-sells (WooCommerce complements) section: show when the cart products have cross-sells. When false, hidden.'),
    ('cart_cross_sells_kicker',  '',     'Override for the cart cross-sells kicker. Empty falls back to m.cart_cross_sells_kicker().'),
    ('cart_cross_sells_heading', '',     'Override for the cart cross-sells heading. Empty falls back to m.cart_cross_sells_heading().')
ON CONFLICT (key) DO NOTHING;
