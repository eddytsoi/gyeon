-- Eligible-state colors for the shipping notice strip — applied when the
-- cart subtotal has reached the free-shipping threshold and the strip text
-- flips to "訂單已免運費...". Defaults match the threshold-state defaults
-- so behavior is unchanged until admin sets distinct colors.
INSERT INTO site_settings (key, value, description) VALUES
    ('shipping_notice_eligible_bg_color',   '#1F4E3D', 'Background color of the shipping notice strip once the cart subtotal reaches the free-shipping threshold.'),
    ('shipping_notice_eligible_text_color', '#FFFFFF', 'Text color of the shipping notice strip once the cart subtotal reaches the free-shipping threshold.')
ON CONFLICT (key) DO NOTHING;
