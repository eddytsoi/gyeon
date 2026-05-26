-- FBT excluded category slugs: products linked to any of these categories
-- (via product_category_links) are filtered out of all three PDP
-- Frequently-Bought-Together pools (co-purchase, bestsellers, slow-movers).
-- Separate from hidden_category_ids — those use UUIDs and scope storewide.
INSERT INTO site_settings (key, value, description) VALUES
    ('fbt_excluded_category_slugs',
     '["coating","ppf-film","installers"]',
     'JSON array of category slugs excluded from PDP Frequently-Bought-Together pools. Products linked (via product_category_links) to any of these categories are filtered out of all three FBT pools (co-purchase, bestsellers, slow-movers). Separate from hidden_category_ids — those use UUIDs and scope storewide.')
ON CONFLICT (key) DO NOTHING;
