package auth

import (
	"net/http"
	"strings"

	"gyeon/backend/internal/respond"
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
