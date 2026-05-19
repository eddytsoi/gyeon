-- name: ListPromoBundles :many
-- Bundle products curated as "優惠套裝" under a parent product. Returns the
-- bundle product's default variant (auto-created, SKU 'BUNDLE-...') so the
-- storefront can show price/compare_at/variant_id without a follow-up query.
SELECT
    ppb.id,
    ppb.parent_product_id,
    ppb.bundle_product_id,
    ppb.sort_order,
    p.slug             AS bundle_slug,
    p.name             AS bundle_name,
    p.excerpt          AS bundle_excerpt,
    p.status           AS bundle_status,
    pv.id              AS bundle_variant_id,
    pv.price           AS bundle_price,
    pv.compare_at_price AS bundle_compare_at_price,
    pv.stock_qty       AS bundle_stock_qty,
    pi.url             AS bundle_primary_image_url,
    ppb.created_at
FROM product_promo_bundles ppb
JOIN products p          ON p.id = ppb.bundle_product_id
JOIN product_variants pv ON pv.product_id = p.id AND pv.is_active = TRUE
LEFT JOIN LATERAL (
    SELECT url FROM product_images
     WHERE product_id = p.id
     ORDER BY is_primary DESC, sort_order ASC, created_at ASC
     LIMIT 1
) pi ON TRUE
WHERE ppb.parent_product_id = $1
ORDER BY ppb.sort_order ASC, ppb.created_at ASC;

-- name: DeleteAllPromoBundles :exec
DELETE FROM product_promo_bundles WHERE parent_product_id = $1;

-- name: InsertPromoBundle :exec
INSERT INTO product_promo_bundles (parent_product_id, bundle_product_id, sort_order)
VALUES ($1, $2, $3);
