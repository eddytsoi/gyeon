-- Social login (Google / Apple) for storefront customers.
--
-- customer_oauth_identities links a customer to one or more OAuth providers.
-- A customer may have both a Google and an Apple identity (plus an optional
-- password), so identities live in their own table rather than as columns on
-- customers. UNIQUE(provider, subject) lets the callback resolve a returning
-- user by their provider-issued id.
CREATE TABLE customer_oauth_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    provider    VARCHAR(20)  NOT NULL,   -- 'google' | 'apple'
    subject     VARCHAR(255) NOT NULL,   -- provider's stable user id ("sub")
    email       VARCHAR(255),            -- email snapshot at link time
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, subject)
);
CREATE INDEX idx_oauth_identities_customer ON customer_oauth_identities(customer_id);

-- oauth_login_states holds the short-lived per-request data for an in-flight
-- authorization-code handshake. Keyed by the random `state` the provider
-- echoes back, so the callback can resolve the request without relying on a
-- cookie (Apple posts the callback cross-site, where SameSite=Lax cookies are
-- dropped). Rows are single-use: consumed (deleted) on callback, and any
-- leftover expired rows are swept opportunistically.
CREATE TABLE oauth_login_states (
    state         VARCHAR(64) PRIMARY KEY,
    provider      VARCHAR(20)  NOT NULL,
    code_verifier VARCHAR(128),          -- Google PKCE verifier
    nonce         VARCHAR(64),           -- Apple id_token nonce
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_oauth_login_states_expires ON oauth_login_states(expires_at);
