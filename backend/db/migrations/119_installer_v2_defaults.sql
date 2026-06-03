-- Settings + promotion defaults for the installer_v2 (施工店_v2) role.
-- Runs AFTER 118 (which adds the enum value in its own transaction) so the
-- 'installer_v2' literal is safe to reference here.

-- Independent free-shipping threshold for installer_v2 (parallel to migration 100).
-- No fallback: when the toggle is off OR the amount is 0, installer_v2 always
-- pays shipping — it does NOT inherit the default threshold.
INSERT INTO site_settings (key, value, description) VALUES
    ('free_shipping_threshold_installer_v2_enabled', 'false',
     'Master switch for the installer_v2 (施工店_v2) free-shipping threshold. When off, installer_v2 always pays shipping regardless of subtotal.'),
    ('free_shipping_threshold_installer_v2_hkd', '0',
     'Order subtotal (HKD) at or above which shipping is free for installer_v2 (施工店_v2) customers. 0 = disabled.')
ON CONFLICT (key) DO NOTHING;

-- New campaigns/coupons default to including installer_v2. Existing rows keep
-- their arrays unchanged — admins opt them in by editing each row, matching
-- migration 101's stated policy.
ALTER TABLE discount_campaigns
    ALTER COLUMN allowed_roles SET DEFAULT '{customer,installer,installer_v2}'::customer_role[];
ALTER TABLE coupon_codes
    ALTER COLUMN allowed_roles SET DEFAULT '{customer,installer,installer_v2}'::customer_role[];
