-- Backfill missing shipping phone for WooCommerce-imported data.
--
-- WC's shipping address has no phone field, so imports left orders.ship_phone
-- and the shipping addresses.phone empty while the contact number lived only on
-- the billing side (orders.customer_phone / customers.phone). ShipAny rejects a
-- waybill with no recipient phone, so backfill the empties from the billing-side
-- number. Only fills blanks — rows that already have a phone are untouched.

-- Order shipping snapshot: fill from the order's own customer_phone.
UPDATE orders
   SET ship_phone = customer_phone
 WHERE (ship_phone IS NULL OR btrim(ship_phone) = '')
   AND customer_phone IS NOT NULL
   AND btrim(customer_phone) <> '';

-- Address book: fill from the owning customer's profile phone.
UPDATE addresses a
   SET phone = c.phone
  FROM customers c
 WHERE a.customer_id = c.id
   AND (a.phone IS NULL OR btrim(a.phone) = '')
   AND c.phone IS NOT NULL
   AND btrim(c.phone) <> '';
