package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Worker drains the queue. N goroutines poll for jobs at pollInterval and
// dispatch to type-specific handlers. A separate reaper goroutine returns
// stale locked rows to pending every minute.
type Worker struct {
	svc          *Service
	handlers     map[string]HandlerFunc
	concurrency  int
	pollInterval time.Duration
	timeouts     map[string]time.Duration
	defaultTO    time.Duration
	wg           sync.WaitGroup
}

// NewWorker constructs a worker. Set concurrency to the number of in-flight
// jobs allowed at once and pollInterval to how often to look for new work.
func NewWorker(svc *Service, concurrency int, pollInterval time.Duration) *Worker {
	if concurrency < 1 {
		concurrency = 1
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	return &Worker{
		svc:          svc,
		handlers:     map[string]HandlerFunc{},
		concurrency:  concurrency,
		pollInterval: pollInterval,
		timeouts:     map[string]time.Duration{},
		defaultTO:    60 * time.Second,
	}
}

// Register attaches a handler for a job type. Call before Start.
func (w *Worker) Register(jobType string, h HandlerFunc) {
	w.handlers[jobType] = h
}

// SetTimeout overrides the per-job execution timeout for a type.
func (w *Worker) SetTimeout(jobType string, d time.Duration) {
	w.timeouts[jobType] = d
}

// Start spawns the worker goroutines. They exit when ctx is cancelled. Stop
// blocks until all goroutines have returned.
func (w *Worker) Start(ctx context.Context) {
	host, _ := os.Hostname()
	for i := 0; i < w.concurrency; i++ {
		id := fmt.Sprintf("%s/%d", host, i)
		w.wg.Add(1)
		go w.runWorker(ctx, id)
	}
	w.wg.Add(1)
	go w.runReaper(ctx)
}

// Stop waits for all goroutines to finish. The caller is expected to cancel
// the context passed to Start first.
func (w *Worker) Stop() {
	w.wg.Wait()
}

func (w *Worker) runWorker(ctx context.Context, workerID string) {
	defer w.wg.Done()
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	for {
		// Drain quickly: keep claiming as long as the previous claim succeeded.
		for {
			if ctx.Err() != nil {
				return
			}
			more := w.drainOne(ctx, workerID)
			if !more {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// drainOne claims a single job and runs its handler. Returns true if a job
// was processed (so the caller should try again before sleeping).
func (w *Worker) drainOne(ctx context.Context, workerID string) bool {
	job, err := w.svc.Claim(ctx, workerID)
	if errors.Is(err, ErrNoJob) {
		return false
	}
	if err != nil {
		log.Printf("queue: claim: %v", err)
		return false
	}

	handler, ok := w.handlers[job.Type]
	if !ok {
		// Unknown job type — dead-letter it so it doesn't loop forever.
		_ = w.svc.Fail(ctx, job, Permanent(fmt.Errorf("no handler registered for job type %q", job.Type)))
		return true
	}

	timeout := w.defaultTO
	if t, ok := w.timeouts[job.Type]; ok {
		timeout = t
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = runWithRecover(handler, jobCtx, job.Payload)
	if err != nil {
		if ferr := w.svc.Fail(ctx, job, err); ferr != nil {
			log.Printf("queue: fail %s: %v (handler err: %v)", job.ID, ferr, err)
		}
		return true
	}
	if ferr := w.svc.Complete(ctx, job.ID); ferr != nil {
		log.Printf("queue: complete %s: %v", job.ID, ferr)
	}
	return true
}

func runWithRecover(h HandlerFunc, ctx context.Context, payload []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return h(ctx, payload)
}

func (w *Worker) runReaper(ctx context.Context) {
	defer w.wg.Done()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := w.svc.ReapStale(ctx)
			if err != nil {
				log.Printf("queue: reap: %v", err)
			} else if n > 0 {
				log.Printf("queue: reaped %d stale rows", n)
			}
		}
	}
}
