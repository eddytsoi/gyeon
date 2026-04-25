-- Add maintenance_mode to site settings
INSERT INTO site_settings (key, value, description)
VALUES ('maintenance_mode', 'false', 'When true, non-admin visitors are redirected to the maintenance page')
ON CONFLICT (key) DO NOTHING;
