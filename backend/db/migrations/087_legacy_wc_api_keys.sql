-- 087: Receive ShipAny status updates by impersonating the WooCommerce REST API.
--
-- ShipAny pushes shipment status updates via WC's standard REST endpoint
-- (PUT /wp-json/wc/v3/orders/{id}) using consumer_key/secret obtained
-- via the WC OAuth handshake. To stay compatible after Gyeon takes over
-- the WC domain, we re-implement WC's authentication scheme:
--   - consumer_key stored as HMAC-SHA256(key, "wc-api") in hex
--   - consumer_secret stored as plaintext (`cs_<32 hex>`)
--   - request validated by hash-lookup + constant-time secret compare
--
-- Keys are migrated one-shot from the legacy WC site's
-- wp_woocommerce_api_keys table at cutover — see
-- scripts/migrate-shipany-keys.sh. New keys (e.g. when onboarding a
-- second merchant) would require implementing /wc-auth/v1/authorize,
-- which is deliberately out of scope for the first cutover.
BEGIN;

CREATE TABLE IF NOT EXISTS legacy_wc_api_keys (
    key_id          BIGINT      PRIMARY KEY,
    user_id         BIGINT,
    description     TEXT,
    permissions     VARCHAR(16) NOT NULL,
    consumer_key    CHAR(64)    NOT NULL UNIQUE,  -- HMAC-SHA256(key, "wc-api") hex
    consumer_secret VARCHAR(64) NOT NULL,         -- cs_<32 hex>, plaintext
    truncated_key   CHAR(7),                       -- last 7 chars of original key, admin-UI hint
    last_access     TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);

-- Active-only partial index so lookups skip revoked rows.
CREATE INDEX IF NOT EXISTS idx_legacy_wc_api_keys_active
    ON legacy_wc_api_keys (consumer_key) WHERE revoked_at IS NULL;

COMMIT;
