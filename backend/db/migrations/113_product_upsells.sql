-- 113: WooCommerce up-sells. Merchant-curated "buy this instead" alternatives
-- shown on the PDP. Imported from WC product.upsell_ids (an ordered list) by
-- the importer's reconcile pass; position preserves the WC array order.
-- Deliberately separate from product_copurchase (algorithmic FBT) and
-- product_promo_bundles — these are hand-picked merchant lists.
BEGIN;

CREATE TABLE IF NOT EXISTS product_upsells (
    product_id        UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    upsell_product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    position          INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, upsell_product_id),
    CHECK (product_id <> upsell_product_id)
);

CREATE INDEX IF NOT EXISTS idx_product_upsells_order
    ON product_upsells (product_id, position ASC);

COMMIT;
