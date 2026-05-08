-- Track when the WC-import setup-password email was sent so subsequent
-- imports under "passwordless" mode don't re-spam customers who already
-- received one. Force mode also writes this timestamp; mode 2 reads it.
ALTER TABLE customers ADD COLUMN IF NOT EXISTS setup_email_sent_at TIMESTAMPTZ NULL;
