-- 114: WooCommerce cross-sells. Merchant-curated complements promoted in the
-- CART based on its contents. Imported from WC product.cross_sell_ids (an
-- ordered list) by the importer's reconcile pass; position preserves the WC
-- array order. Separate from product_copurchase (FBT) and product_promo_bundles.
BEGIN;

CREATE TABLE IF NOT EXISTS product_cross_sells (
    product_id            UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    cross_sell_product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    position              INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, cross_sell_product_id),
    CHECK (product_id <> cross_sell_product_id)
);

CREATE INDEX IF NOT EXISTS idx_product_cross_sells_order
    ON product_cross_sells (product_id, position ASC);

COMMIT;
