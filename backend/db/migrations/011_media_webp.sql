-- ============================================================
-- WebP support for media_files
-- ============================================================

ALTER TABLE media_files
    ADD COLUMN webp_filename   VARCHAR(255),
    ADD COLUMN webp_url        VARCHAR(1024),
    ADD COLUMN webp_size_bytes BIGINT;
