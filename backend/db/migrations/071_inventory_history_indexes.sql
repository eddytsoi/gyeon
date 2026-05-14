-- 071: indexes to support the admin /admin/stock-history list (進出記錄 module).
-- The existing migration 048 already created (variant_id, created_at DESC) and a
-- partial index on order_id. The new admin list filters on reason / actor /
-- date range across all variants, so we add covering indexes for those.
CREATE INDEX IF NOT EXISTS idx_inv_hist_created_at ON inventory_history (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_inv_hist_reason     ON inventory_history (reason);
CREATE INDEX IF NOT EXISTS idx_inv_hist_actor      ON inventory_history (actor_user_id) WHERE actor_user_id IS NOT NULL;
