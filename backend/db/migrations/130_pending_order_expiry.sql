-- Auto-expiry policy for unpaid pending orders.
-- A background sweep cancels (and restocks) pending orders that stay unpaid past
-- the configured age. Card/Stripe and bank-transfer orders get separate
-- thresholds because a wire transfer legitimately takes days. A value of 0
-- disables auto-expiry for that category. Admin-only — NOT exposed via the
-- public settings allowlist.
--
-- Ships DISABLED (0/0): the first production sweep would otherwise cancel and
-- email every already-stale pending order at once (incl. imported legacy
-- orders). An admin enables it by entering hours in the settings page —
-- recommended 24 (card) / 168 (bank transfer) — after reviewing the backlog
-- and while able to watch a controlled "Run now".
INSERT INTO site_settings (key, value, description) VALUES
  ('pending_order_expiry_hours',               '0', 'Hours before an unpaid card/Stripe order is auto-cancelled and restocked (0 = disabled; recommended 24)'),
  ('pending_order_expiry_bank_transfer_hours', '0', 'Hours before an unpaid bank-transfer order is auto-cancelled and restocked (0 = disabled; recommended 168)')
ON CONFLICT (key) DO NOTHING;
