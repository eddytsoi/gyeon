-- v0.9.136: settings for the new automation flow + SMTP log retention.
INSERT INTO site_settings (key, value, description) VALUES
    ('auto_shipany_on_paid_enabled', 'false',
     'When true, paid orders automatically create a Shipany shipment via the queue worker and write a system notice with the result.'),
    ('smtp_log_retention_days', '90',
     'Days to retain SMTP log rows before pruning (0 = keep forever). Pruner is a future task.')
ON CONFLICT (key) DO NOTHING;
