-- 086: pre-aggregated "frequently bought together" pairs.
-- Populated by ProductService.RebuildCopurchase (see backend/internal/shop/product_service.go),
-- invoked by POST /admin/products/recommendations/rebuild — wire up an external
-- cron (same shape as abandoned-cart) to keep this fresh.
-- Storefront PDP reads are a single indexed lookup ordered by together_order_count.
BEGIN;

CREATE TABLE IF NOT EXISTS product_copurchase (
    product_id           UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    related_product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    together_order_count INT  NOT NULL,
    PRIMARY KEY (product_id, related_product_id)
);

CREATE INDEX IF NOT EXISTS idx_product_copurchase_rank
    ON product_copurchase (product_id, together_order_count DESC);

COMMIT;
