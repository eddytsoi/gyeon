package email

import (
	"sync"
	"time"
)

// tokenBucket is a small, dependency-free rate limiter used by the email queue
// worker to smooth bursts of outbound SMTP. It refills continuously at
// limit-per-minute and caps the standing balance at `limit` so an idle period
// can't bank an unbounded burst. The configured limit is passed on every call
// (it comes from a site setting), so an admin change applies on the next send
// without restarting the worker.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	now        func() time.Time // injectable for tests; defaults to time.Now
}

func newTokenBucket() *tokenBucket {
	return &tokenBucket{now: time.Now}
}

// reserve consumes one token for an immediate send. It returns how long the
// caller should wait before sending: zero means a token was available now, a
// positive duration means the bucket was empty and the caller should sleep
// that long (the token is reserved, so the balance goes negative and future
// callers wait their turn). A limit <= 0 disables throttling (always zero).
func (b *tokenBucket) reserve(limit int) time.Duration {
	if limit <= 0 {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	perSecond := float64(limit) / 60.0
	if b.lastRefill.IsZero() {
		// First use: start full so a cold worker isn't throttled.
		b.tokens = float64(limit)
	} else {
		b.tokens += now.Sub(b.lastRefill).Seconds() * perSecond
		if b.tokens > float64(limit) {
			b.tokens = float64(limit)
		}
	}
	b.lastRefill = now

	b.tokens-- // reserve this send (may go negative)
	if b.tokens >= 0 {
		return 0
	}
	// Tokens needed back to zero, converted to wait time.
	return time.Duration((-b.tokens / perSecond) * float64(time.Second))
}
