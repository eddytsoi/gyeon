-- Hidden category IDs: products in these categories are excluded from
-- storefront listings, search, and category nav. Direct product URLs still
-- resolve so admin can sell to specific customers via URL/QR-code "special
-- UI" without exposing the products in the public catalog.
INSERT INTO site_settings (key, value, description) VALUES
    ('hidden_category_ids', '[]', 'JSON array of category UUIDs hidden from storefront product listings, search, and category navigation. Direct /products/<slug> URLs still resolve so the products remain purchasable via private links.')
ON CONFLICT (key) DO NOTHING;
