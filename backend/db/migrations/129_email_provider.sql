-- Choose the outgoing email transport. SMTP (Gmail) is the legacy default; the
-- per-second rate gate added in 128 was the first half of the Gmail→Resend move,
-- and this seeds the actual provider switch. Other email settings (from address,
-- public base URL, admin alert, log retention, rate limits) are shared across
-- both providers — only the connection credentials differ. A missing
-- email_provider row defaults to smtp in code, so existing installs are unaffected.
INSERT INTO site_settings (key, value, description) VALUES
    ('email_provider', 'smtp', 'Outgoing email transport: "smtp" or "resend". Other email settings (from address, rate limits) are shared.'),
    ('resend_api_key', '', 'Resend API key (https://resend.com/api-keys). Used only when email_provider = resend.')
ON CONFLICT (key) DO NOTHING;
