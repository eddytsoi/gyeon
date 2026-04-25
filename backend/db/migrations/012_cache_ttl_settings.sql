-- Cache TTL settings (seconds) — configurable from the admin panel
INSERT INTO site_settings (key, value, description) VALUES
    ('cache_ttl_shop', '300',  'Product & category cache TTL in seconds (default: 300)'),
    ('cache_ttl_cms',  '300',  'CMS pages & posts cache TTL in seconds (default: 300)'),
    ('cache_ttl_nav',  '900',  'Navigation cache TTL in seconds (default: 900)');
