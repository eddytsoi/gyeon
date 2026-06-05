-- 123_dedup_addresses.sql
-- Collapse duplicate customer addresses + prevent future duplicates.
--
-- The `addresses` table backs BOTH order shipping snapshots
-- (orders.shipping_address_id) AND the storefront address book
-- (customers.ListAddresses: SELECT ... WHERE customer_id=$1). The WC order
-- importer (snapshotOrderAddress, internal/importer/orders.go) inserted a fresh
-- address row on EVERY order upsert, so a customer with N orders accumulated N
-- near-identical address-book rows (re-imports added more). This migration
-- collapses identical addresses per customer to one canonical row, repoints
-- orders to it, deletes the duplicates, restores at-most-one-default integrity,
-- then adds a partial expression unique index so new duplicates cannot be
-- written.
--
--   Signature  = normalized (line1, line2, city, state, postal_code, country)
--                scoped per customer_id. Names/phone are intentionally NOT in
--                the key: they drift across orders and would re-fragment the
--                same physical address.
--   Survivor   = is_default DESC, created_at ASC, id ASC  (per group).
--   Guest rows = customer_id IS NULL are never touched.
--
-- orders has NO denormalized shipping-address columns; the displayed address
-- comes entirely from the FK join, and the FK is ON DELETE SET NULL — so we
-- MUST repoint orders to the survivor BEFORE deleting duplicates, otherwise an
-- order would silently lose its address. Survivor and duplicate share identical
-- normalized location fields, so the displayed address text is unchanged.
--
-- Order matters: the cleanup runs and COMMITs first; the unique index is built
-- afterwards, since it would fail on the pre-existing duplicates. The data step
-- is a one-time collapse (re-running is a harmless no-op once deduped); the
-- index uses IF NOT EXISTS and is safe to re-run.

BEGIN;

-- 1 + 2. Repoint orders from every duplicate to its group's survivor.
WITH ranked AS (
    SELECT
        id,
        first_value(id) OVER (
            PARTITION BY
                customer_id,
                lower(btrim(coalesce(line1,       ''))),
                lower(btrim(coalesce(line2,       ''))),
                lower(btrim(coalesce(city,        ''))),
                lower(btrim(coalesce(state,       ''))),
                lower(btrim(coalesce(postal_code, ''))),
                upper(btrim(coalesce(country,     'HK')))
            ORDER BY is_default DESC, created_at ASC, id ASC
        ) AS survivor_id
    FROM addresses
    WHERE customer_id IS NOT NULL
),
dupes AS (
    SELECT id AS dup_id, survivor_id
    FROM ranked
    WHERE id <> survivor_id
)
UPDATE orders o
   SET shipping_address_id = d.survivor_id
  FROM dupes d
 WHERE o.shipping_address_id = d.dup_id;

-- 3. Delete the non-canonical rows (orders already repointed, no FK dangles).
WITH ranked AS (
    SELECT
        id,
        first_value(id) OVER (
            PARTITION BY
                customer_id,
                lower(btrim(coalesce(line1,       ''))),
                lower(btrim(coalesce(line2,       ''))),
                lower(btrim(coalesce(city,        ''))),
                lower(btrim(coalesce(state,       ''))),
                lower(btrim(coalesce(postal_code, ''))),
                upper(btrim(coalesce(country,     'HK')))
            ORDER BY is_default DESC, created_at ASC, id ASC
        ) AS survivor_id
    FROM addresses
    WHERE customer_id IS NOT NULL
)
DELETE FROM addresses a
 USING ranked r
 WHERE a.id = r.id
   AND r.id <> r.survivor_id;

-- 4. is_default integrity: keep at most one default per customer.
WITH default_rank AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY customer_id
               ORDER BY created_at ASC, id ASC
           ) AS rn
    FROM addresses
    WHERE customer_id IS NOT NULL AND is_default = TRUE
)
UPDATE addresses a
   SET is_default = FALSE
  FROM default_rank dr
 WHERE a.id = dr.id AND dr.rn > 1;

-- 5. Promote a default for customers who have addresses but now none flagged.
WITH need_default AS (
    SELECT customer_id
    FROM addresses
    WHERE customer_id IS NOT NULL
    GROUP BY customer_id
    HAVING count(*) FILTER (WHERE is_default) = 0
),
pick AS (
    SELECT DISTINCT ON (a.customer_id) a.id
    FROM addresses a
    JOIN need_default n ON n.customer_id = a.customer_id
    ORDER BY a.customer_id, a.created_at ASC, a.id ASC
)
UPDATE addresses a
   SET is_default = TRUE
  FROM pick p
 WHERE a.id = p.id;

COMMIT;

-- 6. Backstop unique index. Built AFTER the cleanup COMMIT so it isn't applied
--    while duplicates still exist. Partial (customer_id IS NOT NULL) so guest
--    snapshots are exempt; expression-based with the exact normalization used
--    by findAddressID / FindOrCreateAddress in internal/customers.
CREATE UNIQUE INDEX IF NOT EXISTS uq_addresses_customer_signature
    ON addresses (
        customer_id,
        lower(btrim(coalesce(line1,       ''))),
        lower(btrim(coalesce(line2,       ''))),
        lower(btrim(coalesce(city,        ''))),
        lower(btrim(coalesce(state,       ''))),
        lower(btrim(coalesce(postal_code, ''))),
        upper(btrim(coalesce(country,     'HK')))
    )
    WHERE customer_id IS NOT NULL;
