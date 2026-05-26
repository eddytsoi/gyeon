-- Customer-facing roles + per-(role, category) restriction rules.
--
-- customer_role is intentionally separate from admin_users.admin_role: the
-- former gates storefront visibility / cart eligibility, the latter gates
-- back-office capabilities. Today only "customer" (default) and "installer"
-- exist; new roles require ALTER TYPE ADD VALUE.
--
-- customer_role_category_rules stores only the *negative* cases (can_view = FALSE
-- or can_purchase = FALSE). A missing (role, category_id) row means "allowed by
-- default", which matches the admin matrix UX: out-of-the-box every role can
-- see and buy every category.

DO $$ BEGIN
    CREATE TYPE customer_role AS ENUM ('customer', 'installer');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS role customer_role NOT NULL DEFAULT 'customer';

CREATE TABLE IF NOT EXISTS customer_role_category_rules (
    role         customer_role NOT NULL,
    category_id  UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    can_view     BOOLEAN NOT NULL DEFAULT TRUE,
    can_purchase BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role, category_id)
);

-- Partial indexes: the filtering queries only ever look at the FALSE rows.
CREATE INDEX IF NOT EXISTS idx_crcr_blocked_view
    ON customer_role_category_rules (role) WHERE can_view = FALSE;
CREATE INDEX IF NOT EXISTS idx_crcr_blocked_purchase
    ON customer_role_category_rules (role) WHERE can_purchase = FALSE;
