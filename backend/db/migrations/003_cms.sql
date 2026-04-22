-- ============================================================
-- CMS schema  (pages + posts)
-- ============================================================

-- ------------------------------------------------------------
-- Pages  (static content: About, Contact, custom landing pages)
-- ------------------------------------------------------------
CREATE TABLE cms_pages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(255) NOT NULL UNIQUE,
    title           VARCHAR(255) NOT NULL,
    content         TEXT NOT NULL DEFAULT '',      -- Markdown
    meta_title      VARCHAR(255),
    meta_desc       VARCHAR(500),
    is_published    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cms_pages_slug ON cms_pages(slug);
CREATE INDEX idx_cms_pages_is_published ON cms_pages(is_published);

-- ------------------------------------------------------------
-- Post Categories
-- ------------------------------------------------------------
CREATE TABLE cms_post_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0
);

-- ------------------------------------------------------------
-- Posts  (blog articles)
-- ------------------------------------------------------------
CREATE TABLE cms_posts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID REFERENCES cms_post_categories(id) ON DELETE SET NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    title           VARCHAR(255) NOT NULL,
    excerpt         TEXT,                          -- short summary
    content         TEXT NOT NULL DEFAULT '',      -- Markdown
    cover_image_url VARCHAR(1024),
    is_published    BOOLEAN NOT NULL DEFAULT FALSE,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cms_posts_slug ON cms_posts(slug);
CREATE INDEX idx_cms_posts_is_published ON cms_posts(is_published);
CREATE INDEX idx_cms_posts_published_at ON cms_posts(published_at DESC);
CREATE INDEX idx_cms_posts_category_id ON cms_posts(category_id);

-- ------------------------------------------------------------
-- updated_at triggers
-- ------------------------------------------------------------
CREATE TRIGGER trg_cms_pages_updated_at
    BEFORE UPDATE ON cms_pages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_cms_posts_updated_at
    BEFORE UPDATE ON cms_posts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
