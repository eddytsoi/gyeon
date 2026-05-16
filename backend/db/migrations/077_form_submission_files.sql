-- File uploads for CF7-style contact forms. Each row links one stored file
-- to a single form submission. The actual bytes live on disk under
-- ./uploads/forms/<submission_id>/<stored_filename>; this table is the
-- authoritative index used by the admin submissions UI to render download
-- links. CASCADE on submission deletion takes care of the DB rows; the
-- forms service additionally removes the on-disk directory.
CREATE TABLE form_submission_files (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id    UUID NOT NULL REFERENCES form_submissions(id) ON DELETE CASCADE,
    field_name       TEXT NOT NULL,
    original_name    TEXT NOT NULL,
    stored_filename  TEXT NOT NULL,
    mime_type        TEXT NOT NULL,
    size_bytes       BIGINT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_form_submission_files_submission ON form_submission_files(submission_id);

-- Server-side ceiling for per-file uploads on contact forms. Any [file]
-- shortcode with `limit:` above this cap is clamped down server-side; fields
-- without an explicit `limit:` get this value as their effective max.
INSERT INTO site_settings (key, value, description) VALUES
    ('form_upload_hard_cap_mb', '25',
     'Server-side ceiling (MB) on per-file uploads accepted by contact forms.')
ON CONFLICT (key) DO NOTHING;
