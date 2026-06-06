-- Rate-limit knobs for the email queue worker so outbound SMTP stays under
-- Gmail's caps (~500/day for a free account, plus short-term burst throttling).
-- The worker counts successfully-sent rows in smtp_log over a rolling 24h
-- window; once email_daily_limit is reached it defers further sends via the
-- queue's run_after instead of dropping them. email_rate_per_minute smooths
-- bursts so a flood of enqueued jobs doesn't trip Gmail's short-term limit.
-- Set either to 0 (or below) to disable that gate.
INSERT INTO site_settings (key, value, description) VALUES
    ('email_daily_limit', '450', 'Max emails the queue worker will send in a rolling 24h window before deferring. Keep below your SMTP provider''s daily cap (free Gmail ~500). 0 disables the daily limit.'),
    ('email_rate_per_minute', '30', 'Max emails the queue worker will send per minute (smooths bursts to avoid SMTP rate rejections). 0 disables the per-minute limit.')
ON CONFLICT (key) DO NOTHING;
