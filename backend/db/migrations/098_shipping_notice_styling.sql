-- Shipping notice strip styling controls — second site-wide notice rendered
-- below the announcement strip. Visibility is inherited from
-- free_shipping_threshold_enabled + free_shipping_threshold_hkd (no separate
-- toggle here). Defaults chosen for a contrast pair below the cream
-- announcement strip.
INSERT INTO site_settings (key, value, description) VALUES
    ('shipping_notice_bg_color',   '#1F4E3D', 'Background color of the shipping notice strip (CSS color, e.g. #1F4E3D).'),
    ('shipping_notice_text_color', '#FFFFFF', 'Text color of the shipping notice strip (CSS color, e.g. #FFFFFF).'),
    ('shipping_notice_text_size',  '14',      'Font size of the shipping notice strip text in pixels.')
ON CONFLICT (key) DO NOTHING;
