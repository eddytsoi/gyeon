-- ============================================================
-- Navigation schema
-- ============================================================

CREATE TABLE cms_nav_menus (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle      VARCHAR(100) NOT NULL UNIQUE,  -- e.g. "header", "footer"
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cms_nav_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    menu_id     UUID NOT NULL REFERENCES cms_nav_menus(id) ON DELETE CASCADE,
    parent_id   UUID REFERENCES cms_nav_items(id) ON DELETE CASCADE,
    label       VARCHAR(255) NOT NULL,
    url         VARCHAR(1024) NOT NULL,
    target      VARCHAR(10) NOT NULL DEFAULT '_self',  -- '_self' | '_blank'
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cms_nav_items_menu_id   ON cms_nav_items(menu_id);
CREATE INDEX idx_cms_nav_items_parent_id ON cms_nav_items(parent_id);

-- Seed default menus
INSERT INTO cms_nav_menus (handle, name) VALUES
    ('header', 'Header Navigation'),
    ('footer', 'Footer Navigation');

CREATE TRIGGER trg_cms_nav_menus_updated_at
    BEFORE UPDATE ON cms_nav_menus
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
