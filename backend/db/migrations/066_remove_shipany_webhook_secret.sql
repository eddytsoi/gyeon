-- ShipAny mostly uses polling, not webhooks. The webhook_secret setting was
-- seeded by 019_shipany_settings.sql but never wired up in the admin UI.
-- Drop the row so a fresh install doesn't carry a dead key.
DELETE FROM settings WHERE key = 'shipany_webhook_secret';
