// Package ratelimit implements a tiny in-memory sliding-window rate limiter
// suitable for protecting login, password-reset, and checkout endpoints
// against credential stuffing and abuse from a single host.
//
// Trade-offs are explicit and small on purpose:
//   - Per-process: a horizontally scaled deployment must front the API with a
//     shared limiter (Cloudflare, nginx, Redis-backed) or sticky sessions —
//     the counts here aren't shared across instances.
//   - Bucket keyed by RemoteAddr's host portion, which chi's RealIP
//     middleware fills in from X-Forwarded-For / X-Real-IP.
//   - No persistence: an API restart zeroes everyone's counters. Fine for
//     auth abuse (each attempt is still capped per window).
package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gyeon/backend/internal/respond"
)

type limiter struct {
	max    int
	window time.Duration
	mu     sync.Mutex
	hits   map[string][]time.Time
}

// Middleware returns chi middleware that allows up to `max` requests per
// `window` per client IP. Excess requests get HTTP 429 with a Retry-After
// header. Construct one limiter per protected endpoint group — sharing one
// across unrelated routes would let traffic on a busy endpoint starve a
// quiet one of its budget.
func Middleware(max int, window time.Duration) func(http.Handler) http.Handler {
	l := &limiter{
		max:    max,
		window: window,
		hits:   make(map[string][]time.Time),
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()
			wait, ok := l.allow(ip, now)
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
				respond.Error(w, http.StatusTooManyRequests, "too many requests; please try again shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// allow returns (waitUntilNextSlot, true) if the request is allowed, or
// (waitDuration, false) if not.
func (l *limiter) allow(key string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	hits := l.hits[key]
	// Drop expired entries. The slice is append-only and chronological, so
	// the first non-expired index is the new start.
	i := 0
	for i < len(hits) && hits[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		hits = hits[i:]
	}
	if len(hits) >= l.max {
		next := hits[0].Add(l.window).Sub(now)
		l.hits[key] = hits
		return next, false
	}
	l.hits[key] = append(hits, now)
	return 0, true
}

func clientIP(r *http.Request) string {
	// chi's middleware.RealIP rewrites r.RemoteAddr to the upstream client
	// IP when X-Forwarded-For / X-Real-IP is present. Strip the port so the
	// key isn't different for every connection.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
