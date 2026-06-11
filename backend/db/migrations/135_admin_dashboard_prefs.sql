-- Per-admin dashboard customisation. Each admin can keep multiple named layout
-- presets (which KPI cards are visible, their order, and section collapse state,
-- stored opaquely in `layout` JSONB). The currently-selected preset and the
-- global compare mode live on admin_users so switching presets / compare doesn't
-- rewrite a blob. A separate table (rather than one JSONB column on admin_users)
-- models the 1-admin → N-presets relation cleanly.

CREATE TABLE admin_dashboard_layouts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id     UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    is_default   BOOLEAN NOT NULL DEFAULT FALSE,
    layout       JSONB NOT NULL DEFAULT '{}',
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_dashboard_layouts_admin ON admin_dashboard_layouts(admin_id);

CREATE TRIGGER trg_admin_dashboard_layouts_updated_at
    BEFORE UPDATE ON admin_dashboard_layouts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE admin_users
    ADD COLUMN IF NOT EXISTS active_layout_id UUID
        REFERENCES admin_dashboard_layouts(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS dashboard_compare_mode VARCHAR(20) NOT NULL DEFAULT 'prev_month';
