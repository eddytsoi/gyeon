-- Phase 5: refund tracking on orders. Supports partial + full refunds and
-- audits the Stripe refund ID for reconciliation.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS refund_amount    NUMERIC(10, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refund_reason    TEXT,
    ADD COLUMN IF NOT EXISTS refunded_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS stripe_refund_id VARCHAR(128);
