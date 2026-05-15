-- Add blog_enabled to site settings
-- When 'false', the storefront /blog routes 404 and nav links to /blog are hidden.
-- Admin can still manage posts via /admin/cms/posts regardless of this flag.
INSERT INTO site_settings (key, value, description)
VALUES ('blog_enabled', 'true', 'Public blog visibility — when off, /blog routes 404 and blog nav links are hidden')
ON CONFLICT (key) DO NOTHING;
