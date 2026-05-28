INSERT INTO site_settings (key, value, description) VALUES
    ('admin_token_ttl_hours', '24', 'Admin login session length in hours (default: 24)'),
    ('customer_token_ttl_hours', '720', 'Customer login session length in hours (default: 720 = 30 days)')
ON CONFLICT (key) DO NOTHING;
