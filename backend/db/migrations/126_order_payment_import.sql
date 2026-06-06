-- Capture WooCommerce payment info on order import. paid_at had no column
-- (PaidAt was computed from order_status_history, which imported orders lack);
-- transaction_id is stored for records only. Additive + nullable, so old
-- readers and native Stripe/bank-transfer orders are unaffected. GetByID
-- prefers paid_at, falling back to the history computation.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS paid_at        TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS transaction_id VARCHAR(255);
