-- ============================================================
-- Stripe Customer linkage + saved payment methods
-- ============================================================

ALTER TABLE customers ADD COLUMN stripe_customer_id VARCHAR(255);

CREATE INDEX idx_customers_stripe_customer_id ON customers(stripe_customer_id);

CREATE TABLE saved_payment_methods (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id  UUID        NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    stripe_pm_id VARCHAR(255) NOT NULL UNIQUE,
    brand        VARCHAR(50),
    last4        VARCHAR(4),
    exp_month    INT,
    exp_year     INT,
    is_default   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_saved_pm_customer ON saved_payment_methods(customer_id);

-- Update description now that the setting is fully implemented
UPDATE site_settings
SET description = 'Allow customers to save cards for future purchases'
WHERE key = 'stripe_save_cards';
