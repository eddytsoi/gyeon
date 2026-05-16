-- Add pwa_enabled to site settings. When 'false', the storefront omits PWA
-- tags (manifest, theme-color, mobile-web-app-capable, apple-mobile-web-app-*,
-- apple-touch-icon fallback) and unregisters the service worker on next load.
INSERT INTO site_settings (key, value, description)
VALUES ('pwa_enabled', 'true',
        'Progressive Web App features — when off, manifest/iOS meta tags are hidden and the service worker is unregistered')
ON CONFLICT (key) DO NOTHING;
