package mcp

import (
	"net/http"
	"strings"
)

// apiKeyMiddleware guards the SSE connect endpoint with a bearer token.
// Only /sse needs guarding — without a session, /message POSTs are rejected
// by the MCP server itself (unknown session ID → 404).
func apiKeyMiddleware(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sse") {
			bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if bearer != key {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
