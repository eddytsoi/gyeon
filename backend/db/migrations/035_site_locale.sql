INSERT INTO site_settings (key, value) VALUES ('site_locale', 'en')
ON CONFLICT (key) DO NOTHING;
