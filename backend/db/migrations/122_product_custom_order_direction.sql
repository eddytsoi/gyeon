-- 自訂次序 (custom_order) listing sort direction. Backend-only — read by
-- ListEnrichedFiltered to order the category list + [products] shortcode.
-- Not in publicSettingKeys (storefront never reads it). 'asc' = smaller
-- number first (default); 'desc' = larger number first.
INSERT INTO site_settings (key, value, description) VALUES
    ('product_custom_order_direction', 'asc',
     'Sort direction for product custom-order (自訂次序) in category + [products] listings: asc = smaller number first, desc = larger number first.')
ON CONFLICT (key) DO NOTHING;
