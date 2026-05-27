-- ============================================================
-- Installer-tier free-shipping threshold (parallel to migration 074)
-- ============================================================
-- The original threshold (free_shipping_threshold_hkd +
-- free_shipping_threshold_enabled) applies to guests and customers
-- (role = customer). 施工店 customers (role = installer) need their
-- own independent threshold so admins can give them a different free-
-- shipping bar without affecting the default behaviour.
--
-- No fallback: when the installer toggle is off OR the amount is 0,
-- installers always pay shipping — they do NOT inherit the default
-- threshold.
INSERT INTO site_settings (key, value, description) VALUES
    ('free_shipping_threshold_installer_enabled', 'false',
     'Master switch for the installer (施工店) free-shipping threshold. When off, installers always pay SF freight-collect regardless of subtotal.'),
    ('free_shipping_threshold_installer_hkd', '0',
     'Order subtotal (HKD) at or above which shipping is free for installer (施工店) customers. 0 = disabled.')
ON CONFLICT (key) DO NOTHING;
