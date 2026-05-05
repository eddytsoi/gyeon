-- P1 Phase 4: abandoned cart recovery. One reminder per cart, tracked via
-- abandoned_email_sent_at so the cron is idempotent.
ALTER TABLE carts
    ADD COLUMN IF NOT EXISTS abandoned_email_sent_at TIMESTAMPTZ;

INSERT INTO site_settings (key, value, description) VALUES
    ('abandoned_cart_enabled',         'false', 'Send abandoned-cart recovery emails on a schedule'),
    ('abandoned_cart_threshold_hours', '24',    'Hours of inactivity before a cart is treated as abandoned')
ON CONFLICT (key) DO NOTHING;
