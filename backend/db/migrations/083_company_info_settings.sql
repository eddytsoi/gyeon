-- Company info shown on the PDF receipt header. Free-text site_settings,
-- editable in admin Settings → Company info. Empty values render as blank
-- lines on the receipt rather than failing the request.
INSERT INTO site_settings (key, value, description) VALUES
    ('company_address_line1',   '', 'Company street address line 1, shown on PDF receipts.'),
    ('company_address_line2',   '', 'Company street address line 2 (optional).'),
    ('company_city',            '', 'Company city, shown on PDF receipts.'),
    ('company_postal_code',     '', 'Company postal/zip code.'),
    ('company_country',         '', 'Company country.'),
    ('company_phone',           '', 'Company phone number shown on PDF receipts.'),
    ('company_registration_no', '', 'Company registration / business reg. no. (e.g. HK BR no), shown on PDF receipts.')
ON CONFLICT (key) DO NOTHING;
