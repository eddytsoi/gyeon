-- 028_order_number.sql
-- Customer-facing formatted order number ({prefix}-{padded sequence}).
-- The numeric sequence still comes from orders.number (BIGSERIAL); this
-- column just persists the rendered string so admin UI / emails / SSE
-- show a stable, prefix-configurable value.

INSERT INTO site_settings (key, value, description) VALUES
    ('order_number_prefix', 'ORD', 'Prefix for the customer-facing order number, e.g. "ORD" → ORD-0001')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS order_number VARCHAR(64);

-- Backfill existing rows with the legacy ORD-{n} format so the new
-- column is non-empty for all historical orders. Pad to 4 digits to
-- match the new format going forward.
UPDATE orders
   SET order_number = 'ORD-' || LPAD(number::text, 4, '0')
 WHERE order_number IS NULL;

-- Enforce uniqueness once backfilled.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
         WHERE schemaname = 'public'
           AND indexname  = 'idx_orders_order_number_unique'
    ) THEN
        CREATE UNIQUE INDEX idx_orders_order_number_unique ON orders(order_number);
    END IF;
END$$;
