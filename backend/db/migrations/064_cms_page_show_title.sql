-- Toggle for whether the storefront renders the Page's <h1> title.
-- Defaults TRUE so existing pages keep their current behavior.
ALTER TABLE cms_pages
    ADD COLUMN show_title BOOLEAN NOT NULL DEFAULT TRUE;
