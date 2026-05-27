-- ============================================================
-- Role targeting for discount campaigns and coupon codes
-- ============================================================
-- Admins can now scope each campaign / coupon to one or more
-- customer_role values (today: 'customer' | 'installer'). A
-- campaign or coupon is eligible iff the shopper's role appears
-- in allowed_roles.
--
-- The DEFAULT '{customer,installer}' preserves current behaviour
-- for every existing row (both roles eligible). Guests are
-- treated as 'customer' in the call-sites, matching the existing
-- free-shipping convention (see migration 100).
--
-- Empty array means "no one" — enforcement of "at least one role
-- selected" lives in the admin layer (handler + Sveltekit forms);
-- the DB does not enforce it so future scripts can intentionally
-- park a row.
--
-- New roles added later via ALTER TYPE ADD VALUE will NOT
-- retroactively apply to existing campaigns/coupons — admins must
-- opt them in by editing each row.

ALTER TABLE discount_campaigns
    ADD COLUMN IF NOT EXISTS allowed_roles customer_role[]
    NOT NULL DEFAULT '{customer,installer}'::customer_role[];

ALTER TABLE coupon_codes
    ADD COLUMN IF NOT EXISTS allowed_roles customer_role[]
    NOT NULL DEFAULT '{customer,installer}'::customer_role[];
