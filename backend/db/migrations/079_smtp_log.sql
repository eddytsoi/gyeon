-- v0.9.136: audit trail for every outbound email attempt. Rows are written
-- by the queue worker after each SMTP attempt. subject/body fields hold the
-- RENDERED output (post-template-execute) so a resend can replay verbatim
-- without re-rendering from the template (which may have been edited since).
-- Access is admin-only via /admin/smtp-log.
CREATE TABLE smtp_log (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    queue_job_id        UUID REFERENCES queue_jobs(id) ON DELETE SET NULL,
    template_key        TEXT,
    trigger_condition   TEXT NOT NULL,
    related_entity_type TEXT,
    related_entity_id   TEXT,
    recipient           TEXT NOT NULL,
    from_email          TEXT NOT NULL,
    from_name           TEXT,
    reply_to            TEXT,
    subject             TEXT NOT NULL,
    body_html           TEXT NOT NULL,
    body_text           TEXT NOT NULL,
    status              TEXT NOT NULL,
    failure_reason      TEXT,
    attempt_number      INT NOT NULL DEFAULT 1,
    resent_from_id      UUID REFERENCES smtp_log(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_smtp_log_created   ON smtp_log (created_at DESC);
CREATE INDEX idx_smtp_log_status    ON smtp_log (status, created_at DESC);
CREATE INDEX idx_smtp_log_recipient ON smtp_log (recipient);
CREATE INDEX idx_smtp_log_template  ON smtp_log (template_key);
CREATE INDEX idx_smtp_log_related   ON smtp_log (related_entity_type, related_entity_id);
