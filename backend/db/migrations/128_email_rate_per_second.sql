-- Add a per-second burst gate alongside the existing per-minute one so the
-- email queue worker can serve both providers around the Gmail→Resend move.
-- email_rate_per_minute (seeded in 127) suits Gmail SMTP's per-minute caps;
-- email_rate_per_second suits Resend's per-second rate limit. The two gates are
-- independent token buckets and a send must clear both, so set either to 0 to
-- disable that gate. Seeded at 0 (off) — we're still on Gmail today; an admin
-- raises it when cutting over to Resend.
INSERT INTO site_settings (key, value, description) VALUES
    ('email_rate_per_second', '0', 'Max emails the queue worker will send per second (suits Resend''s rate limit; runs alongside the per-minute gate). 0 disables the per-second limit.')
ON CONFLICT (key) DO NOTHING;
