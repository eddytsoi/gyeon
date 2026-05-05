-- Phase 2: extra settings for transactional + alert emails.
INSERT INTO site_settings (key, value, description) VALUES
    ('admin_alert_email', '', 'Recipient for low-stock and other admin alerts (falls back to smtp_from_email when empty)')
ON CONFLICT (key) DO NOTHING;
