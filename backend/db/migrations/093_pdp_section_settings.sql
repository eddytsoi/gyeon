-- ============================================================
-- PDP section visibility / labels / layout site settings.
-- All defaults preserve the pre-existing storefront behavior
-- (sections visible, labels fall back to i18n, layout = tabs).
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_show_specs_strip',     'true', 'Show the dark-blue 4-points specs strip on the product detail page.'),
    ('pdp_show_complete_set',    'true', 'Show the "完成整套保護" (Complete the Set) related-products BundleComposer on the PDP.'),
    ('pdp_complete_set_kicker',  '',     'Override for the related-products kicker (small caps line above the heading). Empty falls back to the localized m.product_detail_related_kicker() message.'),
    ('pdp_complete_set_heading', '',     'Override for the related-products heading. Empty falls back to the localized m.product_detail_related_heading() message.'),
    ('pdp_show_fbt',             'true', 'Show the "其他客人都會買埋呢啲" (Frequently Bought Together) BundleComposer on the PDP.'),
    ('pdp_fbt_kicker',           '',     'Override for the FBT kicker. Empty falls back to the localized m.product_detail_fbt_kicker() message.'),
    ('pdp_fbt_heading',          '',     'Override for the FBT heading. Empty falls back to the localized m.product_detail_fbt_heading() message.'),
    ('pdp_content_layout',       'tabs', 'Layout for the 內容 / 使用方法 / 適用表面 block on the PDP. Values: "tabs" (current tab page view) or "nav-list" (anchor nav with sections stacked vertically).')
ON CONFLICT (key) DO NOTHING;
