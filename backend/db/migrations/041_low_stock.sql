-- Phase 3: per-variant low-stock threshold + global default + alert toggle.
ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS low_stock_threshold INT;

INSERT INTO site_settings (key, value, description) VALUES
    ('low_stock_threshold_default', '5',    'Default low-stock threshold applied when a variant has no override'),
    ('low_stock_alert_enabled',     'true', 'Send admin email when a variant crosses its low-stock threshold')
ON CONFLICT (key) DO NOTHING;
