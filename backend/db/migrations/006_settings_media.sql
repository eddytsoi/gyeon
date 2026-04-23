-- ============================================================
-- Site settings (key-value store) + media library
-- ============================================================

CREATE TABLE site_settings (
    key         VARCHAR(255) PRIMARY KEY,
    value       TEXT,
    description VARCHAR(500),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default settings
INSERT INTO site_settings (key, value, description) VALUES
    ('site_name',        'Gyeon',       'Display name of the site'),
    ('site_description', '',            'Short site description / tagline'),
    ('contact_email',    '',            'Public contact email address'),
    ('currency',         'HKD',         'Default currency code (ISO 4217)'),
    ('timezone',         'Asia/Hong_Kong', 'Site timezone');

-- ============================================================
-- Media library
-- ============================================================

CREATE TABLE media_files (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename    VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type   VARCHAR(100) NOT NULL,
    size_bytes  BIGINT NOT NULL,
    url         VARCHAR(1024) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_files_created_at ON media_files(created_at DESC);
