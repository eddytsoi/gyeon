-- Storefront homepage source. Empty value keeps the default Home template
-- (hero + featured products). A page UUID makes the storefront root render
-- that published CMS page instead, using the same MarkdownContent renderer
-- as /pages/[slug].
INSERT INTO site_settings (key, value, description) VALUES
    ('homepage_page_id', '', 'CMS page id used as the storefront homepage. Empty = use the default Home template (hero + featured products).')
ON CONFLICT (key) DO NOTHING;
