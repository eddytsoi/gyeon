-- P2 #22 follow-up: wildcard redirects. A row with match_type='wildcard' uses
-- a trailing /* on from_path to capture any suffix; if to_path also ends in
-- /* the captured suffix is appended, otherwise every match collapses to a
-- fixed destination.
ALTER TABLE redirects
    ADD COLUMN match_type TEXT NOT NULL DEFAULT 'exact'
        CHECK (match_type IN ('exact', 'wildcard'));

-- Allow exact and wildcard rules to share a prefix (e.g. /foo + /foo/*).
ALTER TABLE redirects DROP CONSTRAINT redirects_from_path_key;
ALTER TABLE redirects
    ADD CONSTRAINT redirects_from_path_match_type_key UNIQUE (from_path, match_type);

-- Split the partial index so each match type lookup hits a tight index.
DROP INDEX IF EXISTS idx_redirects_from_active;
CREATE INDEX idx_redirects_exact_active
    ON redirects (from_path)
    WHERE is_active = TRUE AND match_type = 'exact';
CREATE INDEX idx_redirects_wildcard_active
    ON redirects (from_path)
    WHERE is_active = TRUE AND match_type = 'wildcard';
