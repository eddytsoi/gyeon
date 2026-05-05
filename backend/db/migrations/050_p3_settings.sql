-- P3 settings: tracking IDs (P3 #26) and free-shipping threshold (P3 #29).
-- All public-readable so the SvelteKit storefront can pick them up at SSR.
INSERT INTO site_settings (key, value, description) VALUES
    ('ga4_measurement_id',           '',  'Google Analytics 4 measurement ID (e.g. G-XXXXXXXXXX). Empty disables GA4.'),
    ('meta_pixel_id',                '',  'Meta (Facebook) Pixel ID. Empty disables Meta Pixel.'),
    ('free_shipping_threshold_hkd',  '0', 'Order subtotal (HKD) at or above which shipping is free. 0 = disabled.')
ON CONFLICT (key) DO NOTHING;
