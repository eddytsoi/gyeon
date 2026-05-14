-- Announcement strip styling controls — toggle, colors, and text size.
-- Defaults match the previous hard-coded look (cream bg + ink-900 text, 16px).
INSERT INTO site_settings (key, value, description) VALUES
    ('site_notice_enabled',    'true',    'Show the announcement strip above the storefront header. When off, the strip is hidden regardless of site_notice content.'),
    ('site_notice_bg_color',   '#EDE9E1', 'Background color of the announcement strip (CSS color, e.g. #EDE9E1).'),
    ('site_notice_text_color', '#1A1A1A', 'Text color of the announcement strip (CSS color, e.g. #1A1A1A).'),
    ('site_notice_text_size',  '16',      'Font size of the announcement strip text in pixels (sm breakpoint and up).')
ON CONFLICT (key) DO NOTHING;
