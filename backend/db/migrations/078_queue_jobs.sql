-- v0.9.136: in-process queue for async work (emails, shipany shipment creation).
-- The worker process polls queue_jobs with FOR UPDATE SKIP LOCKED and runs
-- type-specific handlers. Status flow:
--   pending -> processing (claim) -> succeeded | failed | dead (terminal)
-- A "failed" job has exhausted max_attempts; "dead" means a non-retryable
-- error was returned by the handler (e.g. email_enabled=false).
CREATE TABLE queue_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type          TEXT NOT NULL,
    payload       JSONB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    attempts      INT  NOT NULL DEFAULT 0,
    max_attempts  INT  NOT NULL DEFAULT 5,
    last_error    TEXT,
    run_after     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at     TIMESTAMPTZ,
    locked_by     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX idx_queue_jobs_claim
    ON queue_jobs (run_after)
    WHERE status = 'pending';

CREATE INDEX idx_queue_jobs_locked
    ON queue_jobs (locked_at)
    WHERE status = 'processing';

CREATE INDEX idx_queue_jobs_status_created
    ON queue_jobs (status, created_at DESC);
