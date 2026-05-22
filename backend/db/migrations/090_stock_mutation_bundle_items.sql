-- Stock mutations: bundle product support.
-- A bundle product becomes one parent row (variant_id = bundle's parent
-- variant, parent_item_id NULL) with one child row per bundle component
-- (parent_item_id = parent's id, variant_id = the real stocked variant,
-- quantity already multiplied by parent.qty).
-- Execute touches only the children; parent rows are display-only and leave
-- before_qty / after_qty NULL.

ALTER TABLE stock_mutation_items
    ADD COLUMN parent_item_id UUID
        REFERENCES stock_mutation_items(id) ON DELETE CASCADE;

-- Replace the strict (mutation_id, variant_id) uniqueness with a partial
-- index that applies only to top-level rows. Component rows are allowed to
-- repeat the same variant_id within a mutation — two bundles sharing a
-- component, or a top-level variant that also happens to be inside a bundle.
ALTER TABLE stock_mutation_items
    DROP CONSTRAINT stock_mutation_items_mutation_id_variant_id_key;

CREATE UNIQUE INDEX stock_mutation_items_unique_top_level
    ON stock_mutation_items (mutation_id, variant_id)
    WHERE parent_item_id IS NULL;

CREATE INDEX idx_stock_mutation_items_parent
    ON stock_mutation_items (parent_item_id)
    WHERE parent_item_id IS NOT NULL;
