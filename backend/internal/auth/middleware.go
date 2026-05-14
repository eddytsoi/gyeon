package auth

import (
	"context"
	"net/http"
	"strings"

	"gyeon/backend/internal/respond"
)

type contextKey string

const (
	customerIDKey contextKey = "customer_id"
	adminUserIDKey contextKey = "admin_user_id"
)

// TokenVersionStore exposes the live token_version counter that backs the
// "sign out everywhere" feature. Implementations are expected to cache;
// AdminMiddleware/CustomerMiddleware call this on every request.
//
// Invalidate* must drop the cached entry — without it, revocation has the
// cache TTL of latency before old tokens stop being accepted.
type TokenVersionStore interface {
	AdminVersion(ctx context.Context, userID string) (int, error)
	CustomerVersion(ctx context.Context, customerID string) (int, error)
	InvalidateAdmin(userID string)
	InvalidateCustomer(customerID string)
}

// versionStore is set once at startup via SetVersionStore. Nil → tv checks
// are skipped (fail-open). This keeps tests and bootstrap simple — set it
// only when the DB-backed store is wired in main.
var versionStore TokenVersionStore

func SetVersionStore(s TokenVersionStore) { versionStore = s }

// InvalidateAdminVersion drops any cached token_version for the given
// admin user. Call this right after Service.IncrementTokenVersion so
// revoked tokens stop being accepted on the next request (not after the
// cache TTL).
func InvalidateAdminVersion(userID string) {
	if versionStore != nil {
		versionStore.InvalidateAdmin(userID)
	}
}

// InvalidateCustomerVersion is the customer-side counterpart.
func InvalidateCustomerVersion(customerID string) {
	if versionStore != nil {
		versionStore.InvalidateCustomer(customerID)
	}
}

// checkTokenVersion rejects tokens whose claim doesn't match the user's
// current token_version. claimTV=0 + storeTV=0 matches by default, so
// existing pre-revocation tokens stay valid until the user signs out
// everywhere (which bumps storeTV to 1).
func checkTokenVersion(ctx context.Context, isAdmin bool, subject string, claimTV int) bool {
	if versionStore == nil || subject == "" {
		return true
	}
	var (
		current int
		err     error
	)
	if isAdmin {
		current, err = versionStore.AdminVersion(ctx, subject)
	} else {
		current, err = versionStore.CustomerVersion(ctx, subject)
	}
	if err != nil {
		// Fail-closed on lookup errors: a stale-cache read or DB blip
		// shouldn't open the door to revoked tokens. Auth-protected pages
		// will surface a 401 and prompt re-login.
		return false
	}
	return current == claimTV
}

func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				respond.Error(w, http.StatusUnauthorized, "missing or invalid token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if _, err := ValidateToken(tokenStr, secret); err != nil {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CustomerMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				respond.Error(w, http.StatusUnauthorized, "missing or invalid token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := ValidateToken(tokenStr, secret)
			if err != nil || claims.Role != "customer" {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if !checkTokenVersion(r.Context(), false, claims.CustomerID, claims.TokenVersion) {
				respond.Error(w, http.StatusUnauthorized, "session revoked")
				return
			}
			ctx := context.WithValue(r.Context(), customerIDKey, claims.CustomerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CustomerIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(customerIDKey).(string)
	return id
}

// adminRoles are the role claim values accepted by AdminMiddleware. RequireRole
// can narrow this down further per route (e.g. super_admin only).
var adminRoles = map[string]struct{}{
	"admin":       {}, // legacy single-password token issued by GenerateToken
	"super_admin": {},
	"editor":      {},
	"viewer":      {},
}

// AdminMiddleware validates the admin JWT and exposes the admin user ID via
// context. Use AdminIDFromContext to retrieve it inside handlers/services that
// want to record the actor (audit log, inventory history, etc.).
//
// Role check is defense in depth — admin and customer JWTs are signed with
// different secrets, so a customer token would already fail signature
// verification. Rejecting unknown roles still hardens against:
//   - tokens issued with an unexpected role (typo, future migration)
//   - tokens whose role field was tampered before re-signing under a leaked
//     admin secret — at least the role envelope must be known
func AdminMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				respond.Error(w, http.StatusUnauthorized, "missing or invalid token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := ValidateToken(tokenStr, secret)
			if err != nil {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if _, ok := adminRoles[claims.Role]; !ok {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if !checkTokenVersion(r.Context(), true, claims.Subject, claims.TokenVersion) {
				respond.Error(w, http.StatusUnauthorized, "session revoked")
				return
			}
			ctx := r.Context()
			if claims.Subject != "" {
				ctx = context.WithValue(ctx, adminUserIDKey, claims.Subject)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminIDFromContext returns the admin user ID set by AdminMiddleware. Empty
// string + ok=false when the request did not pass through AdminMiddleware (or
// the token had no subject claim — legacy admin tokens).
func AdminIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(adminUserIDKey).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// RequireRole returns middleware that allows the request through only if the
// admin JWT's role claim matches one of `roles`. Mount strictly inside an
// AdminMiddleware group — RequireRole re-parses the bearer token but doesn't
// signal-check the rest of the validity envelope (expiry, signing alg) on its
// own. The admin secret is required because the role claim is signed.
func RequireRole(secret string, roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				respond.Error(w, http.StatusUnauthorized, "missing or invalid token")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := ValidateToken(tokenStr, secret)
			if err != nil {
				respond.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				respond.Error(w, http.StatusForbidden, "insufficient role")
				return
			}
			if !checkTokenVersion(r.Context(), true, claims.Subject, claims.TokenVersion) {
				respond.Error(w, http.StatusUnauthorized, "session revoked")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
