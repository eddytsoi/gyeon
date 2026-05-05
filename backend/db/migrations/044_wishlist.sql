-- P1 Phase 2: customer wishlist. One row per (customer, product). The product
-- is the right granularity for the storefront UI (variant selection happens on
-- PDP). UNIQUE constraint makes the toggle endpoint idempotent.
CREATE TABLE IF NOT EXISTS wishlist_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    product_id  UUID NOT NULL REFERENCES products(id)  ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (customer_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_wishlist_items_customer
    ON wishlist_items (customer_id, created_at DESC);
