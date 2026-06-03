-- Bank-transfer (銀行轉賬 / BACS) payment settings.
--
-- Bank transfer is the only payment method available to installer / installer_v2
-- customers (regular customers + guests always pay by Stripe). The dynamic
-- account details below are admin-editable; the static labels/copy live in the
-- storefront i18n messages. All six keys are non-sensitive and are exposed via
-- the public settings endpoint so the storefront can render the notice.
INSERT INTO site_settings (key, value, description) VALUES
    ('bank_transfer_enabled', 'true',
     'Master toggle for the bank-transfer payment method shown to installer / installer_v2 customers.'),
    ('bank_transfer_account_name', 'Miracle Trading International Limited',
     'Bank-transfer payee / account holder name (名稱) shown on checkout for installer customers.'),
    ('bank_transfer_bank_name', 'HSBC',
     'Bank-transfer bank name (銀行) shown on checkout for installer customers.'),
    ('bank_transfer_account_number', '747-242725-838',
     'Bank-transfer account number (賬戶) shown on checkout for installer customers.'),
    ('bank_transfer_whatsapp_display', '3468 0832',
     'Human-readable WhatsApp number shown in the "send your transfer record" instruction.'),
    ('bank_transfer_whatsapp_url', 'https://wa.me/85234680832',
     'wa.me link target for the WhatsApp number shown in the bank-transfer instruction.')
ON CONFLICT (key) DO NOTHING;
