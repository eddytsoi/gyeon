-- Toggle for whether the storefront wraps the Page content in vertical padding
-- (py-12 sm:py-16). Defaults TRUE so existing pages keep their current spacing.
-- Authors can switch it off when shortcodes like [hero] / [banner] need to
-- bleed all the way to the header / footer edges.
ALTER TABLE cms_pages
    ADD COLUMN content_padded BOOLEAN NOT NULL DEFAULT TRUE;
