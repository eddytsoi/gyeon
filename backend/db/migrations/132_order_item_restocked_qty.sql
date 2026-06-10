-- Per-line restocked quantity, for selective restock on refund.
--
-- Refunds can now restock each order line individually (admin picks how many
-- units of each item go back to sellable stock — damaged goods stay out).
-- restocked_qty tracks how many units of the line have already been returned to
-- inventory so repeated/partial refunds (and the legacy full-refund auto-restock
-- path) never double-count, and the admin UI can cap the picker at the remaining
-- amount (quantity - restocked_qty) and show "restocked X/Y".
ALTER TABLE order_items
  ADD COLUMN IF NOT EXISTS restocked_qty INT NOT NULL DEFAULT 0;
