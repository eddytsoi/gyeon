-- P3 #24 — loyalty points MVP. Customers earn 1 point per HK$1 of paid order
-- subtotal (excluding shipping + tax). Redemption is admin-driven for now;
-- a future iteration will plumb it into checkout.
CREATE TABLE loyalty_balance (
    customer_id  UUID PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
    points       INT NOT NULL DEFAULT 0 CHECK (points >= 0),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Append-only ledger so the balance is auditable. balance_after is denormalized
-- at insert time so we can reproduce the customer view without recomputing.
CREATE TABLE loyalty_ledger (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id    UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    delta          INT NOT NULL,                -- +earn, -redeem/adjust
    balance_after  INT NOT NULL,
    reason         TEXT NOT NULL,               -- "order.earn" | "admin.adjust" | "redeem" | "expire"
    order_id       UUID REFERENCES orders(id) ON DELETE SET NULL,
    actor_user_id  UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    note           TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loyalty_ledger_customer_time ON loyalty_ledger (customer_id, created_at DESC);
CREATE INDEX idx_loyalty_ledger_order ON loyalty_ledger (order_id) WHERE order_id IS NOT NULL;

-- Earn rate is a single global setting. Sub-1.0 lets operators tune the cost.
INSERT INTO site_settings (key, value, description) VALUES
    ('loyalty_enabled',          'false', 'Master switch for the loyalty program (P3 #24).'),
    ('loyalty_points_per_hkd',   '1',     'Points earned per HK$1 of paid order subtotal (post-discount).'),
    ('loyalty_redeem_rate_hkd',  '100',   'Points needed to redeem HK$1. Reserved for future checkout integration.')
ON CONFLICT (key) DO NOTHING;
