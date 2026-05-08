-- Master switch for outgoing emails. When 'false', the email service skips
-- all transactional / notification sends. SendTest (the admin "Test Email"
-- button) bypasses this flag so SMTP credentials can still be validated.
INSERT INTO site_settings (key, value, description) VALUES
    ('email_enabled', 'true', 'Master switch for outgoing emails. When false, the system stops sending all transactional and notification emails.')
ON CONFLICT (key) DO NOTHING;
