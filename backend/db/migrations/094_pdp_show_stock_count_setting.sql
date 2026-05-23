-- ============================================================
-- PDP stock-count visibility site setting.
-- Default 'true' preserves the pre-existing storefront behavior
-- (show "尚餘 N 件" / "尚餘 N 套" with the exact count).
-- When 'false', the PDP renders a generic "尚有存貨" / "In stock"
-- indicator without revealing the quantity.
-- ============================================================

INSERT INTO site_settings (key, value, description) VALUES
    ('pdp_show_stock_count', 'true', 'Show exact stock count on the PDP (e.g. "尚餘 13 件"). When false, the PDP shows a generic "尚有存貨" / "In stock" indicator without revealing the quantity.')
ON CONFLICT (key) DO NOTHING;
