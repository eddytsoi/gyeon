-- Contact Form 7-style forms. Admins build a form in the back office with
-- CF7-style inline shortcodes (`[text* your-name placeholder "Your name"]`
-- etc.) and embed it on any page via `[contact-form id="..."]`. The CF7
-- parser lives in backend/internal/forms — `markup` is the editable source
-- of truth; `fields` is the canonical parsed JSON used for validation and
-- frontend rendering.
CREATE TABLE forms (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT NOT NULL UNIQUE,
    title            TEXT NOT NULL,
    markup           TEXT NOT NULL DEFAULT '',
    fields           JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Admin notification mail (required path)
    mail_to          TEXT NOT NULL,
    mail_from        TEXT NOT NULL DEFAULT '',
    mail_subject     TEXT NOT NULL,
    mail_body        TEXT NOT NULL,
    mail_reply_to    TEXT NOT NULL DEFAULT '',

    -- Optional auto-reply to submitter
    reply_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    reply_to_field   TEXT NOT NULL DEFAULT '',
    reply_from       TEXT NOT NULL DEFAULT '',
    reply_subject    TEXT NOT NULL DEFAULT '',
    reply_body       TEXT NOT NULL DEFAULT '',

    success_message  TEXT NOT NULL DEFAULT 'Thank you for your message.',
    error_message    TEXT NOT NULL DEFAULT 'There was an error. Please try again.',

    recaptcha_action TEXT NOT NULL DEFAULT 'contact_form',

    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE form_submissions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id          UUID NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
    data             JSONB NOT NULL,
    ip               INET,
    user_agent       TEXT NOT NULL DEFAULT '',
    recaptcha_score  NUMERIC(3,2),
    mail_sent        BOOLEAN NOT NULL DEFAULT FALSE,
    mail_error       TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_form_submissions_form ON form_submissions(form_id, created_at DESC);

-- reCAPTCHA v3 site settings. `recaptcha_site_key` is public (exposed via
-- /api/v1/settings/); `recaptcha_secret_key` stays admin-only and is read by
-- backend/internal/recaptcha when verifying tokens. `recaptcha_enabled`
-- defaults to false so the verifier is a no-op until an admin configures it.
INSERT INTO site_settings (key, value, description) VALUES
    ('recaptcha_enabled',    'false', 'Master switch for Google reCAPTCHA v3 verification on form submissions.'),
    ('recaptcha_site_key',   '',      'Google reCAPTCHA v3 site key (public). Empty disables client-side script injection.'),
    ('recaptcha_secret_key', '',      'Google reCAPTCHA v3 secret key (server-only). Used to verify tokens with Google.'),
    ('recaptcha_min_score',  '0.5',   'Minimum score (0.0–1.0) below which a submission is rejected as bot-like.')
ON CONFLICT (key) DO NOTHING;
