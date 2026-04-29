-- Add sequential display numbers to key entities.
-- UUIDs remain the primary keys; these numbers are human-readable display IDs only.
ALTER TABLE products   ADD COLUMN number BIGSERIAL NOT NULL UNIQUE;
ALTER TABLE orders     ADD COLUMN number BIGSERIAL NOT NULL UNIQUE;
ALTER TABLE cms_pages  ADD COLUMN number BIGSERIAL NOT NULL UNIQUE;
ALTER TABLE cms_posts  ADD COLUMN number BIGSERIAL NOT NULL UNIQUE;
