-- i18n translation tables for CMS pages, posts, and products.
-- The base tables retain their original columns (serves as default/fallback language).
-- Translation tables store locale-specific overrides; queries COALESCE translation over base.

CREATE TABLE cms_page_translations (
    page_id UUID  NOT NULL REFERENCES cms_pages(id) ON DELETE CASCADE,
    locale  VARCHAR(10) NOT NULL,  -- BCP 47 tag, e.g. 'en', 'zh-HK', 'ja'
    title       VARCHAR(255) NOT NULL,
    content     TEXT NOT NULL,
    meta_title  VARCHAR(255),
    meta_desc   TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (page_id, locale)
);

CREATE TABLE cms_post_translations (
    post_id UUID  NOT NULL REFERENCES cms_posts(id) ON DELETE CASCADE,
    locale  VARCHAR(10) NOT NULL,
    title   VARCHAR(255) NOT NULL,
    excerpt TEXT,
    content TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_id, locale)
);

CREATE TABLE product_translations (
    product_id  UUID  NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    locale      VARCHAR(10) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, locale)
);
