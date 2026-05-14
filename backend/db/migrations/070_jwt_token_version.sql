-- 070_jwt_token_version.sql
--
-- Per-user counter that backs JWT revocation ("sign out everywhere").
--
-- Mechanism: every issued admin/customer JWT carries a `tv` claim with the
-- value of this column at issue time. Auth middleware looks up the user's
-- current token_version (cached) on each request and rejects tokens whose
-- claim doesn't match. The sign-out-everywhere endpoint bumps the column,
-- invalidating every previously-issued token for that user.
--
-- Default 0 — existing tokens have no `tv` claim, which decodes to 0,
-- which matches the column default, so live sessions keep working through
-- the rollout. The first revocation event bumps to 1 and onwards.

ALTER TABLE admin_users
    ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0;
