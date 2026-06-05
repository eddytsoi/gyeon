-- 124_order_shipping_snapshot.sql
-- Freeze each order's shipping address as an immutable on-order snapshot.
--
-- orders.shipping_address_id is a live FK into `addresses`, which (post-123) is
-- a deduped, customer-editable address book. Reading the address via that join
-- means editing/deleting a book entry mutates placed-order history, and
-- ON DELETE SET NULL wipes the order's address entirely. These ship_* columns
-- freeze the address at write time, like an invoice — mirroring the existing
-- customer_email/customer_phone/customer_name snapshot pattern (migration 015).
--
-- shipping_address_id is KEPT (records which book row was used; powers "use
-- saved address" and the resume COALESCE) but is no longer the display source
-- of truth. Types mirror the source `addresses` columns (migration 002).
--
-- Idempotent: ADD COLUMN IF NOT EXISTS + a backfill that only fills NULLs.

BEGIN;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS ship_first_name  VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ship_last_name   VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ship_phone       VARCHAR(50),
    ADD COLUMN IF NOT EXISTS ship_line1       VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ship_line2       VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ship_city        VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ship_state       VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ship_postal_code VARCHAR(20),
    ADD COLUMN IF NOT EXISTS ship_country     CHAR(2);

-- Backfill from the current FK join. 123 already deduped + repointed every
-- order to its surviving book row, so this join yields the correct,
-- currently-displayed address. Guests / SET-NULL rows have no match and keep
-- NULL ship_* (GetByID returns nil ShippingAddress for them — unchanged).
-- Only touch rows not already snapshotted, so re-running is a no-op.
UPDATE orders o
   SET ship_first_name  = a.first_name,
       ship_last_name   = a.last_name,
       ship_phone       = a.phone,
       ship_line1       = a.line1,
       ship_line2       = a.line2,
       ship_city        = a.city,
       ship_state       = a.state,
       ship_postal_code = a.postal_code,
       ship_country     = a.country
  FROM addresses a
 WHERE o.shipping_address_id = a.id
   AND o.ship_line1 IS NULL;

COMMIT;
