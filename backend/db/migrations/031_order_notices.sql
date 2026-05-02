-- ============================================================
-- Order Notices  (system events + admin/customer messaging)
-- ============================================================
-- Three notice roles share one timeline per order:
--   system   — admin-only audit (status transitions, internal notes)
--   admin    — admin → customer message (visible to both, customer gets email)
--   customer — customer → admin message (visible to both)
--
-- read_at semantics: set when the *recipient* first viewed the order page.
--   role='admin'    → set when customer opens /account/orders/{id}
--   role='customer' → set when admin opens /admin/orders/{id}
--   role='system'   → stays NULL (no recipient)
-- ------------------------------------------------------------

CREATE TYPE notice_role AS ENUM ('system', 'admin', 'customer');

CREATE TABLE order_notices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    role        notice_role NOT NULL,
    status      order_status,        -- only set on role='system' rows produced by a status transition
    body        TEXT NOT NULL,
    author_id   UUID,                 -- admin_users.id for role='admin'; customers.id for role='customer'; NULL for role='system'
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_notices_order_id     ON order_notices(order_id);
CREATE INDEX idx_order_notices_order_role   ON order_notices(order_id, role);
CREATE INDEX idx_order_notices_unread_admin ON order_notices(order_id) WHERE role = 'admin'    AND read_at IS NULL;
CREATE INDEX idx_order_notices_unread_cust  ON order_notices(order_id) WHERE role = 'customer' AND read_at IS NULL;
