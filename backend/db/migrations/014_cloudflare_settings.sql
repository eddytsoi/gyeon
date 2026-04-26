-- Add Cloudflare zone and token to site settings for cache purging on media delete
INSERT INTO site_settings (key, value, description)
VALUES
  ('cloudflare_zone_id',   '', 'Cloudflare Zone ID used to purge cached uploads on delete'),
  ('cloudflare_api_token', '', 'Cloudflare API Token used to purge cached uploads on delete')
ON CONFLICT (key) DO NOTHING;
