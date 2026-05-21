-- Stock Management module: batch stock-change documents (mutations).
-- Each mutation is a draft-first document that contains many variant line
-- items, all going the same direction (all "in" or all "out" — never mixed).
-- On Execute, the variant stock_qty values are updated atomically and each
-- change is logged to inventory_history (reason = "mutation.execute") with
-- a back-reference to the mutation via the new stock_mutation_id column.

CREATE TYPE stock_mutation_type   AS ENUM ('in', 'out');
CREATE TYPE stock_mutation_status AS ENUM ('draft', 'executed');

CREATE TABLE stock_mutations (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number                BIGSERIAL NOT NULL UNIQUE,
    mutation_number       VARCHAR(64) NOT NULL UNIQUE,          -- e.g. MUT-0001
    type                  stock_mutation_type   NOT NULL,
    status                stock_mutation_status NOT NULL DEFAULT 'draft',
    note                  TEXT,
    created_by_admin_id   UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    executed_by_admin_id  UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at           TIMESTAMPTZ
);

CREATE INDEX idx_stock_mutations_status_created
    ON stock_mutations (status, created_at DESC);
CREATE INDEX idx_stock_mutations_type
    ON stock_mutations (type);

CREATE TABLE stock_mutation_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mutation_id   UUID NOT NULL REFERENCES stock_mutations(id) ON DELETE CASCADE,
    variant_id    UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity      INT  NOT NULL CHECK (quantity > 0),           -- always positive; signed by parent type
    before_qty    INT,                                           -- snapshot filled on execute
    after_qty     INT,                                           -- snapshot filled on execute
    position      INT  NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (mutation_id, variant_id)                             -- one row per variant per mutation
);

CREATE INDEX idx_stock_mutation_items_mutation
    ON stock_mutation_items (mutation_id);
CREATE INDEX idx_stock_mutation_items_variant
    ON stock_mutation_items (variant_id);

-- Cross-link inventory_history rows to the mutation that produced them. NULL
-- for every existing row (sales / manual adjustments / wc imports) and for
-- future non-mutation changes. The Stock History UI uses this column to
-- render a link back to the source mutation.
ALTER TABLE inventory_history
    ADD COLUMN stock_mutation_id UUID REFERENCES stock_mutations(id) ON DELETE SET NULL;

CREATE INDEX idx_inv_hist_mutation
    ON inventory_history (stock_mutation_id)
    WHERE stock_mutation_id IS NOT NULL;

-- Configurable prefix for the customer-facing mutation_number (mirrors the
-- existing order_number_prefix setting). Default "MUT" → MUT-0001, MUT-0002…
INSERT INTO site_settings (key, value, description) VALUES
    ('mutation_number_prefix', 'MUT', 'Prefix for stock mutation numbers, e.g. "MUT" → MUT-0001')
ON CONFLICT (key) DO NOTHING;
