-- ============================================================
-- eShop schema
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ------------------------------------------------------------
-- Categories (hierarchical, supports subcategories)
-- ------------------------------------------------------------
CREATE TABLE categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id       UUID REFERENCES categories(id) ON DELETE SET NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    image_url       VARCHAR(1024),
    sort_order      INT NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_slug ON categories(slug);

-- ------------------------------------------------------------
-- Products
-- ------------------------------------------------------------
CREATE TABLE products (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID REFERENCES categories(id) ON DELETE SET NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'inactive', 'hidden')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_slug ON products(slug);
CREATE INDEX idx_products_status ON products(status);

-- ------------------------------------------------------------
-- Product Attributes  (e.g. "Color", "Size")
-- ------------------------------------------------------------
CREATE TABLE product_attributes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_product_attributes_product_id ON product_attributes(product_id);

-- ------------------------------------------------------------
-- Product Attribute Values  (e.g. "Red", "Blue", "S", "M", "L")
-- ------------------------------------------------------------
CREATE TABLE product_attribute_values (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id    UUID NOT NULL REFERENCES product_attributes(id) ON DELETE CASCADE,
    value           VARCHAR(100) NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_product_attribute_values_attribute_id ON product_attribute_values(attribute_id);

-- ------------------------------------------------------------
-- Product Variants  (each unique combination of attribute values)
-- ------------------------------------------------------------
CREATE TABLE product_variants (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id          UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku                 VARCHAR(255) NOT NULL UNIQUE,
    price               NUMERIC(12, 2) NOT NULL,
    compare_at_price    NUMERIC(12, 2),           -- original price, shown as strikethrough
    stock_qty           INT NOT NULL DEFAULT 0,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_product_variants_sku ON product_variants(sku);

-- ------------------------------------------------------------
-- Variant ↔ Attribute Value  (M2M join)
-- ------------------------------------------------------------
CREATE TABLE product_variant_attribute_values (
    variant_id              UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    attribute_value_id      UUID NOT NULL REFERENCES product_attribute_values(id) ON DELETE CASCADE,
    PRIMARY KEY (variant_id, attribute_value_id)
);

-- ------------------------------------------------------------
-- Product Images
-- ------------------------------------------------------------
CREATE TABLE product_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id  UUID REFERENCES product_variants(id) ON DELETE SET NULL,  -- optional: variant-specific image
    url         VARCHAR(1024) NOT NULL,
    alt_text    VARCHAR(255),
    sort_order  INT NOT NULL DEFAULT 0,
    is_primary  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_product_images_product_id ON product_images(product_id);
CREATE INDEX idx_product_images_variant_id ON product_images(variant_id);

-- ------------------------------------------------------------
-- updated_at auto-update trigger
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_categories_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_products_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_product_variants_updated_at
    BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
