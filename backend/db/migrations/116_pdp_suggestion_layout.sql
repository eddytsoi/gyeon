-- ============================================================
-- 116: per-section display style for the two PDP suggestion blocks.
-- Each section can render as the BundleComposer ("amazon style" / 現代 /
-- "modern" — selectable mini-cards + running total + "Add all") or the
-- UpsellGrid ("up-sells style" / 經典 / "classic" — plain product grid with
-- a per-card quick-add button). Default 'modern' preserves prior behaviour
-- (both sections rendered with the BundleComposer).
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_fbt_layout',          'modern', 'Display style for the 經常一起購買 (frequently-bought-together) PDP section: "modern" = bundle composer (amazon style), "classic" = up-sells product grid.'),
    ('pdp_complete_set_layout', 'modern', 'Display style for the 同類相關 (related / complete-the-set) PDP section: "modern" = bundle composer (amazon style), "classic" = up-sells product grid.')
ON CONFLICT (key) DO NOTHING;
