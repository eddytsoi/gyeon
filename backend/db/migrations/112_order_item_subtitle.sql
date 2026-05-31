-- 112_order_item_subtitle.sql
-- Snapshot the product subtitle (副標題) onto each order line, alongside the
-- existing product_name snapshot. Captured at order-creation time so the
-- subtitle shown on the order-confirmed page, confirmation email and account
-- order history stays stable even if the product's subtitle later changes or
-- the product is deleted.
--
-- Nullable: pre-existing orders (and bundle component sub-rows, which don't
-- carry a subtitle) stay NULL and simply render no subtitle line.
--
-- Idempotent: safe to re-run.
BEGIN;

ALTER TABLE order_items
    ADD COLUMN IF NOT EXISTS product_subtitle VARCHAR(255);

COMMIT;
