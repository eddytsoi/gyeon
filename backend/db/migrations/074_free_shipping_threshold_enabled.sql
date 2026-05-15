-- ============================================================
-- Master switch for the free-shipping threshold
-- ============================================================
-- Companion to free_shipping_threshold_hkd (migration 050). The threshold
-- amount alone could not express "no free shipping ever, always SF
-- freight-collect" cleanly (relying on '0' was implicit), and admins need
-- to lock the behaviour deliberately. This explicit flag also drives the
-- ShipAny paid_by_rcvr field per shipment in backend/internal/shipany.
--
-- Off (default): every SF Express shipment is booked paid_by_rcvr=true.
-- On: paid_by_rcvr is computed per order — false when subtotal reaches
-- free_shipping_threshold_hkd, otherwise true.

INSERT INTO site_settings (key, value, description) VALUES
    ('free_shipping_threshold_enabled', 'false',
     'Master switch for the free-shipping threshold. When off, shipping is always SF freight-collect (paid by receiver) regardless of subtotal.')
ON CONFLICT (key) DO NOTHING;
