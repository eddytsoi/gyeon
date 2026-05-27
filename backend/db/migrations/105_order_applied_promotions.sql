-- ============================================================
-- orders.applied_promotions snapshot
-- ============================================================
-- Captures which campaign(s) + coupon actually contributed to the
-- order's discount_amount, alongside each promotion's name +
-- description as they were at checkout time. Lets the
-- success page and customer-account order detail render "你獲得了 X
-- 折扣 — <說明>" without re-resolving (and surviving subsequent
-- edits or deletions of the underlying campaign/coupon row).
--
-- Shape: a JSON array of
--   { kind: "campaign" | "coupon",
--     id: uuid string,
--     name: string,         -- campaign name OR coupon code
--     description: string?, -- admin-authored, optional
--     amount: number }
--
-- Default '[]' keeps every existing imported / pre-migration order
-- valid; the frontend treats an empty array as "render nothing".

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS applied_promotions JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(applied_promotions) = 'array');
