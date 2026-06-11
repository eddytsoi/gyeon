-- Record WHO performed each order status change so the admin order-detail
-- "狀態變更記錄" (status-change log) section can show an 操作者 (operator)
-- column. NULL means a system / automation actor (ShipAny status webhook,
-- the auto-shipment paid→processing job, or any non-admin caller) — rendered
-- as 系統 in the UI. Mirrors inventory_history.actor_user_id.
--
-- Nullable + no backfill: existing history rows keep a NULL operator and
-- render as 系統 / —.
ALTER TABLE order_status_history
    ADD COLUMN IF NOT EXISTS actor_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL;
