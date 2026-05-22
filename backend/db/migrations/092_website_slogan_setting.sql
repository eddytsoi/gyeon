-- Website slogan site setting: free-text tagline rendered under the company
-- logo in the storefront footer. Empty value falls back to the localized
-- m.footer_tagline() message bundled in the frontend i18n catalog.
INSERT INTO site_settings (key, value, description) VALUES
    ('website_slogan', '', 'Storefront footer slogan text. Empty falls back to the localized footer_tagline message.')
ON CONFLICT (key) DO NOTHING;
