-- ============================================================
-- ShipAny logistics gateway: site settings (HK only, v1)
-- ============================================================
-- Credentials and warehouse origin live in site_settings, edited from
-- the admin Settings UI (Commerce tab). Mirrors the Stripe key pattern
-- in 015_payment.sql.
--
-- Endpoint conventions (reverse-engineered from the WC plugin):
--   - shipany_api_key: paste the token from portal.shipany.io > Settings.
--     Env-prefixed keys (SHIPANYDEV / SHIPANYSBX1 / etc.) are auto-routed
--     to the matching subdomain by the Go client.
--   - shipany_region: subdomain suffix. "" = Hong Kong, "-tw" = Taiwan,
--     "-sg" = Singapore, "-th" = Thailand. v1 ships HK only.

INSERT INTO site_settings (key, value, description) VALUES
    -- Master toggle + credentials
    ('shipany_enabled',              'false', 'Master switch for ShipAny rate quoting and shipment creation'),
    ('shipany_user_id',              '',      'ShipAny merchant user ID (from portal.shipany.io — informational; the API uses api_key alone)'),
    ('shipany_api_key',              '',      'ShipAny API access token (from portal.shipany.io > Settings, header: api-tk)'),
    ('shipany_webhook_secret',       '',      'Shared secret for verifying tracking-update callbacks (HMAC-SHA256, untested — ShipAny mostly uses polling)'),
    ('shipany_region',               '',      'API subdomain suffix: "" (HK), "-sg", "-tw", or "-th"'),

    -- Pickup origin (sender contact). v1 falls back to merchants/self values
    -- when these are blank, but explicit values give faster checkout quotes.
    ('shipany_origin_name',          '',      'Pickup origin: contact name'),
    ('shipany_origin_phone',         '',      'Pickup origin: contact phone'),
    ('shipany_origin_line1',         '',      'Pickup origin: street address'),
    ('shipany_origin_line2',         '',      'Pickup origin: building / floor / unit'),
    ('shipany_origin_district',      '',      'Pickup origin: HK district (e.g. 觀塘區)'),
    ('shipany_origin_city',          'Hong Kong', 'Pickup origin: city'),
    ('shipany_origin_postal',        '',      'Pickup origin: postal code (HK has none — leave blank)'),

    -- Quote / shipment defaults
    ('shipany_default_weight_grams', '500',   'Fallback parcel weight (g) when a variant has no weight metadata'),
    ('shipany_default_courier',      '',      'Default courier UID for "Create Shipment" when admin does not pick one (cour_uid from /couriers/)'),
    ('shipany_default_service',      '',      'Default courier service plan (cour_svc_pl) for the default courier'),
    ('shipany_default_storage_type', 'Normal','Default item storage temperature: Normal / Cold / Frozen'),
    ('shipany_paid_by_receiver',     'false', 'Bill the recipient (paid_by_rcvr) by default'),
    ('shipany_self_drop_off',        'false', 'Sender drops parcels at courier counter instead of door pickup'),
    ('shipany_order_ref_suffix',     '',      'Optional suffix appended to ext_order_ref (e.g. "-GYE")'),
    ('shipany_show_courier_tracking_number', 'true', 'Show the courier-side tracking number in customer notifications')
ON CONFLICT (key) DO NOTHING;
