// Package queue provides an in-process, Postgres-backed job queue used to
// decouple slow side-effects (SMTP, ShipAny API) from the request path.
// Handlers register against a job type; the worker claims pending rows via
// SELECT ... FOR UPDATE SKIP LOCKED and runs them with exponential backoff.
package queue

import (
	"context"
	"errors"
	"time"
)

// Job types used across the codebase. Centralized here so callers and the
// worker registration site share string constants.
const (
	JobTypeSendEmail              = "send_email"
	JobTypeSendEmailRaw           = "send_email_raw"
	JobTypeCreateShipanyShipment  = "create_shipany_shipment"
)

// HandlerFunc processes one job payload. Returning nil marks the job
// succeeded. Returning a NonRetryable error marks it `dead` immediately.
// Any other error counts against attempts and re-schedules via backoff.
type HandlerFunc func(ctx context.Context, payload []byte) error

// Job is the in-memory shape of a claimed queue row.
type Job struct {
	ID          string
	Type        string
	Payload     []byte
	Attempts    int
	MaxAttempts int
}

// NonRetryable wraps an error to indicate the job should not be retried.
// Used for e.g. email_enabled=false or a missing shipment configuration —
// repeated attempts will never succeed until an operator intervenes.
type NonRetryable struct{ Err error }

func (n *NonRetryable) Error() string { return n.Err.Error() }
func (n *NonRetryable) Unwrap() error { return n.Err }

// Permanent returns an error that the worker will treat as terminal.
func Permanent(err error) error { return &NonRetryable{Err: err} }

// IsPermanent reports whether err is (or wraps) NonRetryable.
func IsPermanent(err error) bool {
	var nr *NonRetryable
	return errors.As(err, &nr)
}

// EnqueueOptions tune scheduling. Zero values apply sensible defaults.
type EnqueueOptions struct {
	// MaxAttempts overrides the default (5). Use 1 for fire-and-forget jobs.
	MaxAttempts int
	// RunAfter delays the first attempt. Zero = run immediately.
	RunAfter time.Time
}
