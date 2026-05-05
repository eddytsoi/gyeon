-- P2 Phase 3: admin audit log (P2 #17). Captures who did what, when, and the
-- before/after JSON snapshot for high-sensitivity admin operations. Insertion
-- is fire-and-forget — recording failures must not block the underlying business
-- operation.
CREATE TABLE admin_audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,           -- e.g. "order.refund", "settings.bulk_update"
    entity_type   TEXT NOT NULL,           -- e.g. "order", "settings", "redirect"
    entity_id     TEXT,                    -- UUID or string key (settings keys, etc.)
    before        JSONB,
    after         JSONB,
    ip            TEXT,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_admin       ON admin_audit_log (admin_user_id, created_at DESC);
CREATE INDEX idx_audit_entity      ON admin_audit_log (entity_type, entity_id);
CREATE INDEX idx_audit_action_time ON admin_audit_log (action, created_at DESC);
