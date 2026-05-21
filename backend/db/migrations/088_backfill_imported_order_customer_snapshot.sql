-- 088: Backfill orders.customer_email/phone/name for imported WC orders.
--
-- The WC → Gyeon order importer (backend/internal/importer/orders.go) never
-- populated the orders snapshot columns customer_email / customer_phone /
-- customer_name; it only wrote contact info into the joined customers row.
-- The rest of the codebase (admin search, analytics, order emails, list/detail
-- queries) reads these snapshot columns directly, so imported orders appeared
-- to have no billing contact info.
--
-- This migration copies values from the joined customers row into the
-- snapshot columns for imported orders only (wc_order_id IS NOT NULL).
-- Native-checkout orders already populate these columns at creation time and
-- are untouched. Guest imports with customer_id IS NULL cannot be recovered
-- from local state — those need a re-import while DNS still resolves to the
-- legacy WP site.

UPDATE orders o
   SET customer_email = COALESCE(NULLIF(o.customer_email, ''), c.email),
       customer_phone = COALESCE(NULLIF(o.customer_phone, ''), c.phone),
       customer_name  = COALESCE(NULLIF(o.customer_name,  ''),
                                  NULLIF(TRIM(COALESCE(c.first_name,'') || ' ' || COALESCE(c.last_name,'')), ''))
  FROM customers c
 WHERE o.customer_id = c.id
   AND o.wc_order_id IS NOT NULL
   AND (o.customer_email IS NULL OR o.customer_email = ''
     OR o.customer_phone IS NULL OR o.customer_phone = ''
     OR o.customer_name  IS NULL OR o.customer_name  = '');
