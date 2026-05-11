-- 061: multi-category support for products and posts.
--
-- Each product/post still has a "primary" category_id (used for canonical
-- URLs, breadcrumbs, default display). These join tables hold the full
-- set of categories — including the primary — so listing filters can use
-- a single EXISTS check against the link table without an OR on category_id.
--
-- Idempotent: safe to re-run.
BEGIN;

CREATE TABLE IF NOT EXISTS product_category_links (
    product_id  UUID NOT NULL REFERENCES products(id)   ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_product_category_links_category_id
    ON product_category_links(category_id);

CREATE TABLE IF NOT EXISTS cms_post_category_links (
    post_id     UUID NOT NULL REFERENCES cms_posts(id)           ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES cms_post_categories(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_cms_post_category_links_category_id
    ON cms_post_category_links(category_id);

-- Backfill: every existing (product, category_id) and (post, category_id)
-- becomes a link row so the new EXISTS-based filter returns the same set
-- the old `category_id = X` filter did.
INSERT INTO product_category_links (product_id, category_id)
SELECT id, category_id FROM products WHERE category_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO cms_post_category_links (post_id, category_id)
SELECT id, category_id FROM cms_posts WHERE category_id IS NOT NULL
ON CONFLICT DO NOTHING;

COMMIT;
