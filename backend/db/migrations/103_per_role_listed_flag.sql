-- Per-role "listed" flag for the category-rules matrix.
--
-- Replaces the global hidden_category_ids site setting with a per-(role,
-- category_id) `is_listed` flag on customer_role_category_rules. Same
-- semantics as the old setting (FALSE = hide from product listings / category
-- nav / search; PDP-by-slug still resolves so "private link" sales work) but
-- now expressible per storefront role — so an installer-only catalog is just
-- "is_listed=FALSE for role='customer', TRUE for role='installer'".
--
-- Why a third column instead of overloading can_view: can_view=FALSE 404s the
-- PDP too. The old hidden_category_ids deliberately kept PDP-by-slug working
-- (for direct-link sales), and we need to preserve that distinction.
--
-- The DEFAULT TRUE preserves "out of the box every role can see and list and
-- buy every category", matching the matrix UX where the absence of a row
-- means "fully allowed".

ALTER TABLE customer_role_category_rules
    ADD COLUMN IF NOT EXISTS is_listed BOOLEAN NOT NULL DEFAULT TRUE;

-- Backfill: every UUID listed in the existing hidden_category_ids site
-- setting gets one row per customer_role enum value with is_listed=FALSE,
-- preserving today's behaviour exactly until an admin explicitly edits.
-- The CROSS JOIN over enum_range means future roles added to customer_role
-- before this migration runs would also be backfilled (no impact for the
-- current 'customer' | 'installer' set).
INSERT INTO customer_role_category_rules (role, category_id, can_view, is_listed, can_purchase)
SELECT r.role, c.id, TRUE, FALSE, TRUE
FROM (SELECT unnest(enum_range(NULL::customer_role)) AS role) r
CROSS JOIN LATERAL (
    SELECT id FROM categories
    WHERE id::text = ANY (
        SELECT jsonb_array_elements_text(
            COALESCE(
                (SELECT value::jsonb FROM site_settings WHERE key='hidden_category_ids'),
                '[]'::jsonb
            )
        )
    )
) c
ON CONFLICT (role, category_id) DO UPDATE
    SET is_listed = EXCLUDED.is_listed;

-- Partial index mirroring the existing can_view / can_purchase indexes —
-- BlockedListCategoryIDs only ever looks at FALSE rows.
CREATE INDEX IF NOT EXISTS idx_crcr_unlisted
    ON customer_role_category_rules (role) WHERE is_listed = FALSE;

-- The setting is now fully expressed in the matrix; drop it so the admin
-- UI has one source of truth.
DELETE FROM site_settings WHERE key = 'hidden_category_ids';
