-- Add shipping_countries setting (JSON array of ISO 3166-1 alpha-2 codes)
INSERT INTO site_settings (key, value, description)
VALUES ('shipping_countries', '["HK"]', 'Countries available at checkout (ISO 3166-1 alpha-2 codes, JSON array)')
ON CONFLICT (key) DO NOTHING;
