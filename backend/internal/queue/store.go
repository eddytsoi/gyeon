package queue

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrNoJob is returned by Claim when no eligible row exists.
var ErrNoJob = errors.New("queue: no eligible job")

// Service is the public enqueue + claim API.
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

// Enqueue inserts a new pending row and returns its id.
func (s *Service) Enqueue(ctx context.Context, jobType string, payload []byte, opts ...EnqueueOptions) (string, error) {
	o := EnqueueOptions{}
	if len(opts) > 0 {
		o = opts[0]
	}
	maxAttempts := 5
	if o.MaxAttempts > 0 {
		maxAttempts = o.MaxAttempts
	}
	runAfter := time.Now()
	if !o.RunAfter.IsZero() {
		runAfter = o.RunAfter
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO queue_jobs (type, payload, max_attempts, run_after, scheduled_at)
		 VALUES ($1, $2, $3, $4, $4)
		 RETURNING id`,
		jobType, payload, maxAttempts, runAfter,
	).Scan(&id)
	return id, err
}

// Claim atomically picks the next eligible pending job and marks it
// processing. Returns ErrNoJob when nothing is available.
func (s *Service) Claim(ctx context.Context, workerID string) (*Job, error) {
	var j Job
	err := s.db.QueryRowContext(ctx,
		`UPDATE queue_jobs
		    SET status='processing', locked_at=NOW(), locked_by=$1,
		        attempts=attempts+1, updated_at=NOW()
		  WHERE id = (
		    SELECT id FROM queue_jobs
		     WHERE status='pending' AND run_after <= NOW()
		     ORDER BY run_after ASC, created_at ASC
		     FOR UPDATE SKIP LOCKED
		     LIMIT 1
		  )
		  RETURNING id, type, payload, attempts, max_attempts`,
		workerID,
	).Scan(&j.ID, &j.Type, &j.Payload, &j.Attempts, &j.MaxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoJob
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// Complete marks the job succeeded.
func (s *Service) Complete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE queue_jobs
		    SET status='succeeded', completed_at=NOW(), locked_at=NULL, locked_by=NULL,
		        last_error=NULL, updated_at=NOW()
		  WHERE id=$1`, id)
	return err
}

// Fail reschedules a failed job for another attempt, or moves to a terminal
// state when out of attempts (or err is permanent).
func (s *Service) Fail(ctx context.Context, j *Job, err error) error {
	msg := err.Error()
	if IsPermanent(err) || j.Attempts >= j.MaxAttempts {
		terminal := "failed"
		if IsPermanent(err) {
			terminal = "dead"
		}
		_, derr := s.db.ExecContext(ctx,
			`UPDATE queue_jobs
			    SET status=$2, last_error=$3, completed_at=NOW(),
			        locked_at=NULL, locked_by=NULL, updated_at=NOW()
			  WHERE id=$1`, j.ID, terminal, msg)
		return derr
	}
	runAfter := time.Now().Add(Next(j.Attempts))
	_, derr := s.db.ExecContext(ctx,
		`UPDATE queue_jobs
		    SET status='pending', last_error=$2, run_after=$3,
		        locked_at=NULL, locked_by=NULL, updated_at=NOW()
		  WHERE id=$1`, j.ID, msg, runAfter)
	return derr
}

// ReapStale returns processing rows that have been locked too long to
// pending. Called periodically by the worker.
func (s *Service) ReapStale(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE queue_jobs
		    SET status='pending', locked_at=NULL, locked_by=NULL,
		        last_error='reaped: stale lock', updated_at=NOW()
		  WHERE status='processing' AND locked_at < NOW() - INTERVAL '5 minutes'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Row is a public-facing job description for the admin viewer.
type Row struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Payload     string  `json:"payload"`
	Status      string  `json:"status"`
	Attempts    int     `json:"attempts"`
	MaxAttempts int     `json:"max_attempts"`
	LastError   *string `json:"last_error,omitempty"`
	RunAfter    string  `json:"run_after"`
	ScheduledAt string  `json:"scheduled_at"`
	LockedAt    *string `json:"locked_at,omitempty"`
	LockedBy    *string `json:"locked_by,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

type ListFilter struct {
	Status string
	Type   string
	From   string
	To     string
	Limit  int
	Offset int
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]Row, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	args := []any{}
	where := []string{"TRUE"}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if f.Type != "" {
		add("type = ?", f.Type)
	}
	if f.From != "" {
		add("created_at >= ?", f.From)
	}
	if f.To != "" {
		add("created_at <= ?", f.To)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM queue_jobs WHERE `+whereSQL, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	q := `SELECT id, type, payload::text, status, attempts, max_attempts, last_error,
	             run_after, scheduled_at, locked_at, locked_by, created_at, updated_at, completed_at
	        FROM queue_jobs
	       WHERE ` + whereSQL + `
	       ORDER BY created_at DESC
	       LIMIT $` + strconv.Itoa(limitIdx) + ` OFFSET $` + strconv.Itoa(offsetIdx)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Row, 0)
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.Type, &r.Payload, &r.Status, &r.Attempts, &r.MaxAttempts,
			&r.LastError, &r.RunAfter, &r.ScheduledAt, &r.LockedAt, &r.LockedBy,
			&r.CreatedAt, &r.UpdatedAt, &r.CompletedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// Retry resets a non-pending job back to pending so it runs again on the next
// claim. Used from the admin viewer to recover dead/failed jobs after a fix.
func (s *Service) Retry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE queue_jobs
		    SET status='pending', attempts=0, last_error=NULL, run_after=NOW(),
		        locked_at=NULL, locked_by=NULL, completed_at=NULL, updated_at=NOW()
		  WHERE id=$1`, id)
	return err
}
