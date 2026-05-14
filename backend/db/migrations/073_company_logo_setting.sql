-- Company Logo site settings: URL of an image picked from the Media module
-- shown in the storefront header top-left, plus its render height in px.
-- Empty URL falls back to the text logo rendered via m.header_logo().
INSERT INTO site_settings (key, value, description) VALUES
    ('company_logo_url', '', 'Storefront header company logo. Pick an image from the Media module; empty falls back to the text logo.'),
    ('company_logo_height_px', '40', 'Render height of the storefront company logo in pixels. Width scales automatically.')
ON CONFLICT (key) DO NOTHING;
