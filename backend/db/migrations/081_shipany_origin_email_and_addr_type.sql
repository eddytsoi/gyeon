-- ============================================================
-- ShipAny pickup origin: add email + address type
-- ============================================================
-- The sender contact block (sndr_ctc) in POST orders/ accepts an
-- `email` field, and the `addr.typ` is one of "Residential" /
-- "Commercial". Before this migration we hardcoded "Residential" and
-- never sent email; SF Express in particular charges and validates
-- pickups differently for commercial vs residential addresses.

INSERT INTO site_settings (key, value, description) VALUES
    ('shipany_origin_email',     '',           'Pickup origin: contact email (sndr_ctc.ctc.email)'),
    ('shipany_origin_addr_type', 'Commercial', 'Pickup origin: address type — "Residential" or "Commercial" (sndr_ctc.addr.typ)')
ON CONFLICT (key) DO NOTHING;
