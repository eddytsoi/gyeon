-- P2 Phase 5: admin-editable email templates (P2 #20). Each row is an override
-- for one of the hardcoded templates in backend/internal/email/service.go. When
-- a row is missing or `is_enabled=FALSE`, the service falls back to the
-- compiled-in default — admins can always recover by deleting the row.
CREATE TABLE email_templates (
    key         TEXT PRIMARY KEY,         -- e.g. "order_confirmation", "order_shipped"
    subject     TEXT NOT NULL,
    html        TEXT NOT NULL,
    text        TEXT NOT NULL,
    is_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  UUID REFERENCES admin_users(id) ON DELETE SET NULL
);
