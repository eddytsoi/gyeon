-- ============================================================
-- Customer auth: add password hash + customer JWT secret
-- ============================================================

ALTER TABLE customers ADD COLUMN password_hash VARCHAR(255);

-- index for login lookups already exists on email
