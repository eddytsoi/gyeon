-- Phase 6: tax MVP. Single global rate stored in site_settings; tax_amount
-- computed at checkout and persisted on the order row for audit trail.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS tax_amount NUMERIC(10, 2) NOT NULL DEFAULT 0;

INSERT INTO site_settings (key, value, description) VALUES
    ('tax_enabled',   'false',     'Toggle to apply tax at checkout. Disabled by default.'),
    ('tax_rate',      '0',         'Tax rate as a decimal fraction (e.g. 0.05 = 5%).'),
    ('tax_label',     'Sales Tax', 'Label shown to customers on cart, checkout, invoices.'),
    ('tax_inclusive', 'false',     'When true, the displayed price includes tax (back-calculated).')
ON CONFLICT (key) DO NOTHING;
