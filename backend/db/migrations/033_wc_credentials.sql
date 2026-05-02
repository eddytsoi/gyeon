-- 033_wc_credentials.sql
-- Persist WooCommerce REST API credentials so the admin import page can
-- pre-fill them on subsequent visits and run scheduled re-syncs without
-- re-entering keys. Stored in site_settings alongside other admin-only
-- secrets (Stripe / SMTP / ShipAny). Idempotent.

INSERT INTO site_settings (key, value, description) VALUES
    ('wc_url',             '', 'WooCommerce store base URL (e.g. https://shop.example.com) for the import module'),
    ('wc_consumer_key',    '', 'WooCommerce REST API consumer key (ck_...)'),
    ('wc_consumer_secret', '', 'WooCommerce REST API consumer secret (cs_...)')
ON CONFLICT (key) DO NOTHING;
