-- Per-outcome response behaviour for forms. Each form picks independently
-- for success and error whether to show an inline message (the historical
-- default — `success_message` / `error_message`) or redirect to a CMS page.
-- New columns default to 'message' so existing rows keep current behaviour.
ALTER TABLE forms
    ADD COLUMN IF NOT EXISTS success_mode    TEXT NOT NULL DEFAULT 'message',
    ADD COLUMN IF NOT EXISTS error_mode      TEXT NOT NULL DEFAULT 'message',
    ADD COLUMN IF NOT EXISTS success_page_id UUID REFERENCES cms_pages(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS error_page_id   UUID REFERENCES cms_pages(id) ON DELETE SET NULL;
