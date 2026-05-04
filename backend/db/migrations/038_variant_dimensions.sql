ALTER TABLE product_variants
    ADD COLUMN IF NOT EXISTS length_mm INT,
    ADD COLUMN IF NOT EXISTS width_mm  INT,
    ADD COLUMN IF NOT EXISTS height_mm INT;
