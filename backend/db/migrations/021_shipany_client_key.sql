-- ShipAny added a `client-key` header requirement on certain endpoints
-- (couriers/, etc.) that the older WooCommerce plugin in source/shipany
-- doesn't expose. Operators paste the value from portal.shipany.io into
-- this setting; the Go client sends it on every request when non-empty.
INSERT INTO site_settings (key, value, description) VALUES
    ('shipany_client_key', '', 'ShipAny client-key header value (paste from portal.shipany.io if `couriers/` etc. return 401 client-key missing)')
ON CONFLICT (key) DO NOTHING;
