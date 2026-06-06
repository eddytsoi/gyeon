package email

import (
	"testing"
	"time"
)

// fakeClock drives the bucket deterministically.
func newBucketAt(t time.Time) (*tokenBucket, *time.Time) {
	now := t
	b := newTokenBucket()
	b.now = func() time.Time { return now }
	return b, &now
}

func TestTokenBucket_DisabledWhenLimitNonPositive(t *testing.T) {
	b, _ := newBucketAt(time.Unix(0, 0))
	if w := b.reserve(0); w != 0 {
		t.Fatalf("limit 0 should not throttle, got %v", w)
	}
	if w := b.reserve(-5); w != 0 {
		t.Fatalf("negative limit should not throttle, got %v", w)
	}
}

func TestTokenBucket_StartsFullThenThrottles(t *testing.T) {
	const limit = 30
	b, _ := newBucketAt(time.Unix(0, 0))

	// Cold bucket starts full: the first `limit` reservations are immediate.
	for i := 0; i < limit; i++ {
		if w := b.reserve(limit); w != 0 {
			t.Fatalf("reservation %d should be immediate, got %v", i, w)
		}
	}

	// The next reservation (no time elapsed) must wait ~1 token's worth:
	// 60s / 30 per min = 2s per token.
	w := b.reserve(limit)
	if w <= 0 {
		t.Fatalf("expected a positive wait once the bucket is empty, got %v", w)
	}
	want := 2 * time.Second
	if w < want-50*time.Millisecond || w > want+50*time.Millisecond {
		t.Fatalf("expected wait ~%v, got %v", want, w)
	}
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	const limit = 60 // 1 token per second
	b, now := newBucketAt(time.Unix(0, 0))

	// Drain the full bucket.
	for i := 0; i < limit; i++ {
		b.reserve(limit)
	}
	// Empty now: a reservation should require a wait.
	if w := b.reserve(limit); w <= 0 {
		t.Fatalf("expected wait on empty bucket, got %v", w)
	}

	// Advance 10s → 10 tokens refilled (minus the 1 we just reserved into the
	// negative). 9 immediate reservations should be available.
	*now = now.Add(10 * time.Second)
	immediate := 0
	for i := 0; i < 20; i++ {
		if b.reserve(limit) == 0 {
			immediate++
		} else {
			break
		}
	}
	if immediate < 8 || immediate > 10 {
		t.Fatalf("after 10s refill expected ~9 immediate sends, got %d", immediate)
	}
}
