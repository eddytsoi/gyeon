-- Per-(customer_role, nav_item) visibility rules for storefront CMS nav.
--
-- Same negative-rule shape as customer_role_category_rules (096): row
-- existence means "hide this nav item from this role". Missing row = the
-- item is visible to that role (the default). Anonymous storefront
-- visitors are treated as 'customer' upstream, so they pick up the same
-- rules as logged-in customers.
--
-- Only one dimension (visibility) — no purchase axis — so we skip the
-- boolean columns from 096 and let row existence itself carry the signal.

CREATE TABLE IF NOT EXISTS customer_role_nav_item_rules (
    role         customer_role NOT NULL,
    nav_item_id  UUID NOT NULL REFERENCES cms_nav_items(id) ON DELETE CASCADE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role, nav_item_id)
);

CREATE INDEX IF NOT EXISTS idx_crnir_item
    ON customer_role_nav_item_rules (nav_item_id);
