-- Company Logo (Footer) site settings: URL of an image picked from the Media
-- module shown in the storefront footer brand area, plus its render height in
-- px. Empty URL falls back to the text logo rendered via m.footer_logo().
INSERT INTO site_settings (key, value, description) VALUES
    ('company_logo_footer_url', '', 'Storefront footer company logo. Pick an image from the Media module; empty falls back to the text logo.'),
    ('company_logo_footer_height_px', '40', 'Render height of the storefront footer company logo in pixels. Width scales automatically.')
ON CONFLICT (key) DO NOTHING;
