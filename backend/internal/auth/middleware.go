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
			ctx := context.WithValue(r.Context(), customerIDKey, claims.CustomerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CustomerIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(customerIDKey).(string)
	return id
}

// AdminMiddleware validates the admin JWT and exposes the admin user ID via
// context. Use AdminIDFromContext to retrieve it inside handlers/services that
// want to record the actor (audit log, inventory history, etc.).
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
			next.ServeHTTP(w, r)
		})
	}
}
