-- Favicon site setting: URL of an image picked from the Media module. Empty
-- value falls back to the static /icon.svg shipped in app.html.
INSERT INTO site_settings (key, value, description) VALUES
    ('favicon_url', '', 'Storefront/admin favicon URL. Pick an image from the Media module; empty falls back to the bundled icon.')
ON CONFLICT (key) DO NOTHING;
