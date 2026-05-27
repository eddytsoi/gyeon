-- ============================================================
-- Promotions: guest toggle + multi-select category/product targets
-- ============================================================
-- Two changes:
--
-- 1. `allow_guests boolean` lets admins decide whether anonymous
--    shoppers see a campaign / can redeem a coupon. Previously
--    guests were treated as role=customer at the application
--    layer; now eligibility for guests is explicit, independent
--    of customer/installer scoping. Default TRUE preserves the
--    prior behaviour for every existing row.
--
-- 2. discount_campaigns gains `target_ids uuid[]` so a single
--    campaign can apply to multiple categories or multiple
--    products (instead of exactly one). `target_id` is backfilled
--    into the new array and then dropped. `target_type` still
--    distinguishes 'all' / 'category' / 'product' — only the
--    scope-vs-single-id mapping changes.

ALTER TABLE discount_campaigns
    ADD COLUMN IF NOT EXISTS allow_guests boolean NOT NULL DEFAULT true;

ALTER TABLE coupon_codes
    ADD COLUMN IF NOT EXISTS allow_guests boolean NOT NULL DEFAULT true;

-- Backfill: guests were previously treated as customer-role. Preserve that:
-- a campaign/coupon that excludes customers should also exclude guests.
UPDATE discount_campaigns
   SET allow_guests = ('customer'::customer_role = ANY(allowed_roles))
 WHERE allow_guests = true
   AND NOT ('customer'::customer_role = ANY(allowed_roles));

UPDATE coupon_codes
   SET allow_guests = ('customer'::customer_role = ANY(allowed_roles))
 WHERE allow_guests = true
   AND NOT ('customer'::customer_role = ANY(allowed_roles));

ALTER TABLE discount_campaigns
    ADD COLUMN IF NOT EXISTS target_ids uuid[] NOT NULL DEFAULT '{}'::uuid[];

UPDATE discount_campaigns
   SET target_ids = ARRAY[target_id]
 WHERE target_id IS NOT NULL
   AND cardinality(target_ids) = 0;

ALTER TABLE discount_campaigns
    DROP COLUMN IF EXISTS target_id;
