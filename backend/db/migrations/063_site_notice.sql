-- Site-wide announcement bar copy. Rendered verbatim by AnnouncementStrip.svelte
-- when non-empty; hidden when empty. Editable from admin Settings.
INSERT INTO site_settings (key, value, description) VALUES
    ('site_notice', '訂單滿 HK$500 免運費 · 即日訂購次日送達', 'Text displayed in the storefront announcement strip above the header. Leave empty to hide the strip.')
ON CONFLICT (key) DO NOTHING;
