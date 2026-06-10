// Package wcshim impersonates the parts of the WooCommerce REST API that
// ShipAny calls to push shipment status updates back to a merchant site.
//
// After Gyeon takes over a customer's WC domain, ShipAny continues sending
// PUT /wp-json/wc/v3/orders/{id} with the consumer_key/secret it received
// during the original WC OAuth handshake. We accept those credentials by
// re-implementing WC's wc_api_hash() authentication, with keys migrated
// one-shot from the legacy WC site's wp_woocommerce_api_keys table.
//
// Scope is deliberately narrow: only the endpoint(s) ShipAny actually hits.
// See plan: /Users/eddytsoi/.claude/plans/shipany-woocommerce-shipany-memoized-popcorn.md
package wcshim

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"gyeon/backend/internal/respond"
)

// wcAPIHash mirrors WooCommerce's wc_api_hash() (wc-core-functions.php):
//
//	return hash_hmac( 'sha256', $data, 'wc-api' );
//
// Used both for storage (we migrate the already-hashed value out of WC's
// wp_woocommerce_api_keys table) and for validating incoming requests: we
// hash the consumer_key the caller presents and look it up.
func wcAPIHash(consumerKey string) string {
	mac := hmac.New(sha256.New, []byte("wc-api"))
	mac.Write([]byte(consumerKey))
	return hex.EncodeToString(mac.Sum(nil))
}

// ckPrefix returns a short, non-sensitive prefix of a consumer key for logs.
// WC consumer keys start with a "ck_" prefix followed by 40 hex chars; the
// first few are plenty to correlate against the ShipAny portal without
// logging anything reusable.
func ckPrefix(ck string) string {
	if len(ck) <= 10 {
		return ck
	}
	return ck[:10]
}

type apiKeyRow struct {
	KeyID          int64
	ConsumerSecret string
	Permissions    string
}

var (
	errMissingCreds = errors.New("missing consumer credentials")
	errBadCreds     = errors.New("invalid consumer credentials")
)

// extractCreds reads consumer_key/secret from either HTTP Basic Auth
// (the path ShipAny uses over HTTPS) or — as a fallback that matches
// WC's own behaviour — the consumer_key / consumer_secret query params.
func extractCreds(r *http.Request) (ck, cs string, ok bool) {
	if user, pass, basicOK := r.BasicAuth(); basicOK && user != "" && pass != "" {
		return user, pass, true
	}
	q := r.URL.Query()
	ck = q.Get("consumer_key")
	cs = q.Get("consumer_secret")
	if ck == "" || cs == "" {
		return "", "", false
	}
	return ck, cs, true
}

// authenticate looks up a key and validates the secret in constant time.
// It does NOT enforce permission scope — that's the caller's job because
// the required scope depends on the HTTP method.
func authenticate(ctx context.Context, db *sql.DB, ck, cs string) (*apiKeyRow, error) {
	hashed := wcAPIHash(ck)
	var row apiKeyRow
	err := db.QueryRowContext(ctx,
		`SELECT key_id, consumer_secret, permissions
		   FROM legacy_wc_api_keys
		  WHERE consumer_key = $1 AND revoked_at IS NULL`,
		hashed).Scan(&row.KeyID, &row.ConsumerSecret, &row.Permissions)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errBadCreds
	}
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(row.ConsumerSecret), []byte(cs)) {
		return nil, errBadCreds
	}
	return &row, nil
}

// hasWritePerm returns true for "write" or "read_write" — both grant
// PUT/POST/DELETE access on the WC REST API.
func hasWritePerm(p string) bool {
	return p == "write" || p == "read_write"
}

// touchLastAccess updates last_access asynchronously. Best effort —
// failures are logged but never propagate to the caller.
func touchLastAccess(db *sql.DB, keyID int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx,
			`UPDATE legacy_wc_api_keys SET last_access = NOW() WHERE key_id = $1`,
			keyID); err != nil {
			log.Printf("wcshim: touch last_access %d: %v", keyID, err)
		}
	}()
}

// BasicAuthMiddleware authenticates incoming requests against
// legacy_wc_api_keys. For non-GET methods, it also enforces that the
// key has write permission. Failures return WC-shaped error JSON so the
// caller (ShipAny) sees something recognisably WC-ish.
func BasicAuthMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ck, cs, ok := extractCreds(r)
			if !ok {
				// Previously silent — a 401 here showed only as a bare status
				// code in the access log, with no way to tell auth failures
				// apart. Log it so missing-credential pushes are diagnosable.
				log.Printf("wcshim auth: missing consumer credentials method=%s path=%s from=%s",
					r.Method, r.URL.Path, r.RemoteAddr)
				respond.Error(w, http.StatusUnauthorized, "Consumer key/secret missing.")
				return
			}
			row, err := authenticate(r.Context(), db, ck, cs)
			if err != nil {
				if errors.Is(err, errBadCreds) {
					log.Printf("wcshim auth: invalid credentials ck=%s… method=%s from=%s",
						ckPrefix(ck), r.Method, r.RemoteAddr)
				} else {
					log.Printf("wcshim: auth lookup: %v", err)
				}
				respond.Error(w, http.StatusUnauthorized, "Consumer key/secret invalid.")
				return
			}
			if r.Method != http.MethodGet && !hasWritePerm(row.Permissions) {
				log.Printf("wcshim auth: key %d lacks write permission (have %q) method=%s from=%s",
					row.KeyID, row.Permissions, r.Method, r.RemoteAddr)
				respond.Error(w, http.StatusForbidden, "The API key provided does not have write permissions.")
				return
			}
			touchLastAccess(db, row.KeyID)
			next.ServeHTTP(w, r)
		})
	}
}
