// Package loyalty implements the points balance + ledger MVP from P3 #24.
//
// The program is intentionally minimal:
//   - Earn: triggered by orders.MarkPaidByPaymentIntent when an order's
//     payment is confirmed. Points = floor(subtotal * loyalty_points_per_hkd).
//   - Adjust: admin manual delta with a reason note (positive or negative).
//   - Redeem: not yet integrated with checkout — only Adjust("redeem", -N) is
//     supported by admins on behalf of the customer.
//
// All mutations write to loyalty_ledger inside the same tx that updates
// loyalty_balance, so the running balance is always reproducible.
package loyalty

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrCustomerMissing = errors.New("customer required")
	ErrInsufficient    = errors.New("insufficient points")
	ErrNotFound        = errors.New("not found")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// EarnRate reads loyalty_enabled + loyalty_points_per_hkd. Returns 0 if
// disabled or the setting is unparseable.
func (s *Service) EarnRate(ctx context.Context) float64 {
	var enabled string
	if err := s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = 'loyalty_enabled'`).Scan(&enabled); err != nil {
		return 0
	}
	if strings.TrimSpace(enabled) != "true" {
		return 0
	}
	var raw string
	if err := s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = 'loyalty_points_per_hkd'`).Scan(&raw); err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// EarnFromOrder credits points for one paid order. Idempotent: if a ledger
// row already exists for this orderID it skips. Caller (orders pkg) invokes
// this after marking the order paid; failures are logged, not propagated.
func (s *Service) EarnFromOrder(ctx context.Context, customerID, orderID string, subtotal float64) error {
	if customerID == "" {
		return ErrCustomerMissing
	}
	rate := s.EarnRate(ctx)
	if rate <= 0 || subtotal <= 0 {
		return nil
	}
	delta := int(subtotal * rate) // floor — fractional points are dropped
	if delta <= 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Idempotency guard: bail if we've already credited this order.
	var already bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM loyalty_ledger WHERE order_id = $1 AND reason = 'order.earn')`,
		orderID).Scan(&already); err != nil {
		return err
	}
	if already {
		return nil
	}

	balance, err := upsertBalance(ctx, tx, customerID, delta)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO loyalty_ledger (customer_id, delta, balance_after, reason, order_id)
		 VALUES ($1, $2, $3, 'order.earn', $4)`,
		customerID, delta, balance, orderID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// Adjust applies a manual delta (admin tool). actorID may be empty for
// system-driven adjustments. Negative delta below current balance returns
// ErrInsufficient.
func (s *Service) Adjust(ctx context.Context, customerID string, delta int, reason, note, actorID string) (int, error) {
	if customerID == "" {
		return 0, ErrCustomerMissing
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var current int
	err = tx.QueryRowContext(ctx,
		`SELECT points FROM loyalty_balance WHERE customer_id = $1 FOR UPDATE`, customerID,
	).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		current = 0
	} else if err != nil {
		return 0, err
	}
	next := current + delta
	if next < 0 {
		return 0, ErrInsufficient
	}

	balance, err := upsertBalance(ctx, tx, customerID, delta)
	if err != nil {
		return 0, err
	}

	var noteArg, actorArg any
	if note != "" {
		noteArg = note
	}
	if actorID != "" {
		actorArg = actorID
	}
	if reason == "" {
		reason = "admin.adjust"
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO loyalty_ledger (customer_id, delta, balance_after, reason, note, actor_user_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		customerID, delta, balance, reason, noteArg, actorArg,
	); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return balance, nil
}

// upsertBalance applies a delta and returns the resulting balance. Caller is
// responsible for the tx lifecycle.
func upsertBalance(ctx context.Context, tx *sql.Tx, customerID string, delta int) (int, error) {
	var balance int
	err := tx.QueryRowContext(ctx,
		`INSERT INTO loyalty_balance (customer_id, points) VALUES ($1, $2)
		 ON CONFLICT (customer_id) DO UPDATE SET
		   points = loyalty_balance.points + EXCLUDED.points,
		   updated_at = NOW()
		 RETURNING points`,
		customerID, delta).Scan(&balance)
	return balance, err
}

func (s *Service) GetBalance(ctx context.Context, customerID string) (int, error) {
	var p int
	err := s.db.QueryRowContext(ctx,
		`SELECT points FROM loyalty_balance WHERE customer_id = $1`, customerID,
	).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return p, err
}

type LedgerRow struct {
	ID           string  `json:"id"`
	Delta        int     `json:"delta"`
	BalanceAfter int     `json:"balance_after"`
	Reason       string  `json:"reason"`
	OrderID      *string `json:"order_id,omitempty"`
	ActorEmail   *string `json:"actor_email,omitempty"`
	Note         *string `json:"note,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func (s *Service) Ledger(ctx context.Context, customerID string, limit int) ([]LedgerRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.id, l.delta, l.balance_after, l.reason, l.order_id, u.email, l.note, l.created_at
		   FROM loyalty_ledger l
		   LEFT JOIN admin_users u ON u.id = l.actor_user_id
		  WHERE l.customer_id = $1
		  ORDER BY l.created_at DESC
		  LIMIT $2`,
		customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LedgerRow, 0)
	for rows.Next() {
		var r LedgerRow
		if err := rows.Scan(&r.ID, &r.Delta, &r.BalanceAfter, &r.Reason, &r.OrderID, &r.ActorEmail, &r.Note, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
