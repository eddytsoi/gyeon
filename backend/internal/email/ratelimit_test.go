package email

import (
	"testing"
	"time"
)

// fakeClock drives the bucket deterministically. windowSeconds picks the cap's
// refill window: 1 = per-second, 60 = per-minute.
func newBucketAt(t time.Time, windowSeconds float64) (*tokenBucket, *time.Time) {
	now := t
	b := newTokenBucket(windowSeconds)
	b.now = func() time.Time { return now }
	return b, &now
}

func TestTokenBucket_DisabledWhenLimitNonPositive(t *testing.T) {
	b, _ := newBucketAt(time.Unix(0, 0), 1)
	if w := b.reserve(0); w != 0 {
		t.Fatalf("limit 0 should not throttle, got %v", w)
	}
	if w := b.reserve(-5); w != 0 {
		t.Fatalf("negative limit should not throttle, got %v", w)
	}
}

// startsFullThenThrottles asserts a cold bucket serves `limit` immediate
// reservations then makes the next one wait ~one token's worth of refill.
func startsFullThenThrottles(t *testing.T, windowSeconds float64, limit int, want time.Duration) {
	t.Helper()
	b, _ := newBucketAt(time.Unix(0, 0), windowSeconds)

	// Cold bucket starts full: the first `limit` reservations are immediate.
	for i := 0; i < limit; i++ {
		if w := b.reserve(limit); w != 0 {
			t.Fatalf("reservation %d should be immediate, got %v", i, w)
		}
	}

	// The next reservation (no time elapsed) must wait ~1 token's worth.
	w := b.reserve(limit)
	if w <= 0 {
		t.Fatalf("expected a positive wait once the bucket is empty, got %v", w)
	}
	if w < want-20*time.Millisecond || w > want+20*time.Millisecond {
		t.Fatalf("expected wait ~%v, got %v", want, w)
	}
}

func TestTokenBucket_PerSecond_StartsFullThenThrottles(t *testing.T) {
	// 2 per second → 1s / 2 = 500ms per token.
	startsFullThenThrottles(t, 1, 2, 500*time.Millisecond)
}

func TestTokenBucket_PerMinute_StartsFullThenThrottles(t *testing.T) {
	// 30 per minute → 60s / 30 = 2s per token (Gmail path).
	startsFullThenThrottles(t, 60, 30, 2*time.Second)
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	const limit = 10 // 10 tokens per second
	b, now := newBucketAt(time.Unix(0, 0), 1)

	// Drain the full bucket.
	for i := 0; i < limit; i++ {
		b.reserve(limit)
	}
	// Empty now: a reservation should require a wait.
	if w := b.reserve(limit); w <= 0 {
		t.Fatalf("expected wait on empty bucket, got %v", w)
	}

	// Advance 1s → 10 tokens refilled (minus the 1 we just reserved into the
	// negative, and staying under the limit cap). 9 immediate reservations
	// should be available.
	*now = now.Add(1 * time.Second)
	immediate := 0
	for i := 0; i < 20; i++ {
		if b.reserve(limit) == 0 {
			immediate++
		} else {
			break
		}
	}
	if immediate < 8 || immediate > 10 {
		t.Fatalf("after 1s refill expected ~9 immediate sends, got %d", immediate)
	}
}
