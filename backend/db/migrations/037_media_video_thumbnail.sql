-- ============================================================
-- Video first-frame thumbnail support for media_files
-- ============================================================

ALTER TABLE media_files
    ADD COLUMN thumbnail_filename   VARCHAR(255),
    ADD COLUMN thumbnail_url        VARCHAR(1024),
    ADD COLUMN thumbnail_size_bytes BIGINT;
