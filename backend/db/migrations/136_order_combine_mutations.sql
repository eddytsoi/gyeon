-- Combine executed out-mutations (出貨單) into a single sales order (賬面單).
--
-- An admin can multi-select several already-executed stock-out mutations and
-- roll their contents into one new order assigned to a customer / installer /
-- installer_v2. The physical stock already left when those mutations were
-- executed, so this order must NEVER touch inventory — not at create time, and
-- not when it is later cancelled/refunded.
--
-- stock_managed:
--   true  (default) — normal order; checkout/admin-create deduct stock and
--                      cancel/refund restock it. All existing orders stay true.
--   false           — accounting-only order; the restock paths skip it so a
--                      cancellation can never conjure stock that was already
--                      shipped out.
ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS stock_managed BOOLEAN NOT NULL DEFAULT TRUE;

-- consumed_by_order_id links a source out-mutation to the order it was rolled
-- into, and acts as the lock that prevents the same physical shipment from
-- being billed twice: an atomic `... WHERE consumed_by_order_id IS NULL`
-- conditional update claims the mutations inside the order-create transaction.
-- ON DELETE SET NULL releases the mutations again if the order is later deleted
-- (only cancelled/refunded orders are deletable), so they can be re-combined.
ALTER TABLE stock_mutations
  ADD COLUMN IF NOT EXISTS consumed_by_order_id UUID
    REFERENCES orders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_stock_mutations_consumed
  ON stock_mutations (consumed_by_order_id)
  WHERE consumed_by_order_id IS NOT NULL;
