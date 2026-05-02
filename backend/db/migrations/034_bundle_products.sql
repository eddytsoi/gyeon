ALTER TABLE products ADD COLUMN kind TEXT NOT NULL DEFAULT 'simple' CHECK (kind IN ('simple', 'bundle'));
CREATE INDEX idx_products_kind ON products(kind);

CREATE TABLE bundle_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bundle_product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    component_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    sort_order INT NOT NULL DEFAULT 0,
    display_name_override VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (bundle_product_id, component_variant_id)
);

ALTER TABLE order_items ADD COLUMN parent_item_id UUID REFERENCES order_items(id) ON DELETE CASCADE;
