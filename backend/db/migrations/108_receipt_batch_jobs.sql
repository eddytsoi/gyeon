-- Batch receipt download: one row per "download receipts for N orders" request.
-- The actual work runs on the queue worker (job type generate_receipt_batch);
-- this table holds the request, its progress/result, and the path to the built
-- ZIP so the admin UI can poll for completion and trigger the download.
--
-- We need our own table (rather than reading queue_jobs) because queue_jobs has
-- no result column and its rows are pruned — we must retain the zip_path and the
-- per-order skip/error report until the admin has downloaded it.
--   status flow: pending -> processing -> succeeded | failed
--   (partial success — some orders skipped — is still 'succeeded'; the UI keys
--    off succeeded_count / zip_path. 'failed' is reserved for a crashed job.)
CREATE TABLE receipt_batch_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status          TEXT NOT NULL DEFAULT 'pending',
    locale          TEXT NOT NULL,
    order_ids       JSONB NOT NULL,                    -- requested order UUIDs
    total           INT  NOT NULL DEFAULT 0,
    succeeded_count INT  NOT NULL DEFAULT 0,
    errors          JSONB NOT NULL DEFAULT '[]',       -- [{order_id, order_number, reason}]
    zip_path        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
