-- 026_stripe_country.sql
-- Stripe account country / region (ISO 3166-1 alpha-2). Drives the
-- default country shown in Stripe Elements address fields.

INSERT INTO site_settings (key, value, description) VALUES
    ('stripe_country', 'HK', 'Stripe account country / region (ISO 3166-1 alpha-2 code)')
ON CONFLICT (key) DO NOTHING;
