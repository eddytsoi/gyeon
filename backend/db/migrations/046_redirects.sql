-- P2 Phase 1: URL redirects (P2 #22). Used when CMS slugs change so old links
-- don't 404. Resolved by the SvelteKit hook before the storefront route handlers,
-- so it covers both CMS pages and arbitrary product/category URLs.
CREATE TABLE redirects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_path   TEXT NOT NULL UNIQUE,
    to_path     TEXT NOT NULL,
    code        SMALLINT NOT NULL DEFAULT 301 CHECK (code IN (301, 302)),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    note        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial index — only active rows are queried by the public match endpoint.
CREATE INDEX idx_redirects_from_active ON redirects (from_path) WHERE is_active = TRUE;
