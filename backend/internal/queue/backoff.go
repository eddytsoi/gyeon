package queue

import (
	"math/rand"
	"time"
)

// Next computes the wait before the next attempt. Exponential with jitter,
// capped at 1h. attempts is 1-indexed (the attempt that just failed).
func Next(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	base := time.Duration(30) * time.Second
	d := base * (1 << (attempts - 1))
	cap := time.Hour
	if d > cap {
		d = cap
	}
	jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
	return d + jitter
}
