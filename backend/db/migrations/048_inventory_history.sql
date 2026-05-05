-- P2 Phase 4: variant inventory history (P2 #23). Every stock change writes one
-- row. warehouse_id is reserved for future multi-warehouse work — always NULL
-- for now (no warehouses table created in this migration).
CREATE TABLE inventory_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id      UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    delta           INT NOT NULL,                      -- can be negative (sale) or positive (restock)
    before_qty      INT NOT NULL,
    after_qty       INT NOT NULL,
    reason          TEXT NOT NULL,                     -- "order.checkout" | "admin.adjust" | "admin.variant_update" | "wc.import"
    actor_user_id   UUID REFERENCES admin_users(id) ON DELETE SET NULL, -- NULL for customer-driven (checkout)
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    note            TEXT,
    warehouse_id    UUID,                              -- reserved for multi-warehouse; always NULL today
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inv_hist_variant_time ON inventory_history (variant_id, created_at DESC);
CREATE INDEX idx_inv_hist_order        ON inventory_history (order_id) WHERE order_id IS NOT NULL;
