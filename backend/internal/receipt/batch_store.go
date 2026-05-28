package receipt

import (
	"context"
	"database/sql"
	"encoding/json"
)

// BatchError records one order that could not be included in the ZIP, with a
// machine-readable reason the frontend maps to a localized label.
//
//	not_receiptable — unpaid / cancelled / refunded
//	generation_failed — PDF render failed
//	not_found — order vanished between request and processing
type BatchError struct {
	OrderID     string `json:"order_id"`
	OrderNumber string `json:"order_number"`
	Reason      string `json:"reason"`
}

// Batch is the in-memory shape of a receipt_batch_jobs row.
type Batch struct {
	ID             string       `json:"id"`
	Status         string       `json:"status"`
	Locale         string       `json:"locale"`
	OrderIDs       []string     `json:"order_ids"`
	Total          int          `json:"total"`
	SucceededCount int          `json:"succeeded_count"`
	Errors         []BatchError `json:"errors"`
	ZipPath        string       `json:"-"`
}

// BatchStore persists receipt batch requests + their results. Raw SQL mirrors
// the queue.Service style — this is queue-adjacent infrastructure, not a
// domain entity, so it stays out of sqlc.
type BatchStore struct {
	db *sql.DB
}

func NewBatchStore(db *sql.DB) *BatchStore { return &BatchStore{db: db} }

// CreateBatch inserts a pending batch and returns its id.
func (s *BatchStore) CreateBatch(ctx context.Context, locale string, orderIDs []string) (string, error) {
	idsJSON, err := json.Marshal(orderIDs)
	if err != nil {
		return "", err
	}
	var id string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO receipt_batch_jobs (locale, order_ids, total)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		locale, idsJSON, len(orderIDs),
	).Scan(&id)
	return id, err
}

// GetBatch loads a batch by id. Returns sql.ErrNoRows when missing.
func (s *BatchStore) GetBatch(ctx context.Context, id string) (*Batch, error) {
	var b Batch
	var idsJSON, errsJSON []byte
	var zipPath sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, status, locale, order_ids, total, succeeded_count, errors, zip_path
		   FROM receipt_batch_jobs WHERE id = $1`, id,
	).Scan(&b.ID, &b.Status, &b.Locale, &idsJSON, &b.Total, &b.SucceededCount, &errsJSON, &zipPath)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(idsJSON, &b.OrderIDs)
	b.Errors = []BatchError{}
	_ = json.Unmarshal(errsJSON, &b.Errors)
	b.ZipPath = zipPath.String
	return &b, nil
}

// MarkProcessing flips a batch into the processing state when the worker
// picks it up.
func (s *BatchStore) MarkProcessing(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE receipt_batch_jobs SET status='processing' WHERE id=$1`, id)
	return err
}

// CompleteBatch records the result of a finished batch. zipPath may be empty
// when no order produced a receipt (all skipped).
func (s *BatchStore) CompleteBatch(ctx context.Context, id, zipPath string, succeededCount int, errs []BatchError) error {
	errsJSON, err := json.Marshal(errs)
	if err != nil {
		return err
	}
	var zip sql.NullString
	if zipPath != "" {
		zip = sql.NullString{String: zipPath, Valid: true}
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE receipt_batch_jobs
		    SET status='succeeded', succeeded_count=$2, errors=$3, zip_path=$4, completed_at=NOW()
		  WHERE id=$1`,
		id, succeededCount, errsJSON, zip)
	return err
}

// FailBatch marks a batch failed (a crash, not a per-order skip).
func (s *BatchStore) FailBatch(ctx context.Context, id, msg string) error {
	errsJSON, _ := json.Marshal([]BatchError{{Reason: msg}})
	_, err := s.db.ExecContext(ctx,
		`UPDATE receipt_batch_jobs
		    SET status='failed', errors=$2, completed_at=NOW()
		  WHERE id=$1`, id, errsJSON)
	return err
}
