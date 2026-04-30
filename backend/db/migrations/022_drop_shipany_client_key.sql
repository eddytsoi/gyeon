-- Removes shipany_client_key. The header isn't required by the live ShipAny
-- API in our region — couriers/, orders/, query-rate/ all work with just
-- api-tk + order-from + order-from-ver. The setting was added speculatively
-- in 021 and is now dead.
DELETE FROM site_settings WHERE key = 'shipany_client_key';
