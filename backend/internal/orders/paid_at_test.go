package orders

import (
	"context"
	"testing"
	"time"
)

// TestGetByIDPaidAtColumnPreference proves GetByID prefers the orders.paid_at
// column (set by the WC importer, which writes no status-history rows) and
// falls back to the earliest 'paid' status-history row for native orders that
// leave the column NULL.
func TestGetByIDPaidAtColumnPreference(t *testing.T) {
	db := dialOrdersTestDB(t)
	defer db.Close()
	ctx := context.Background()
	svc := &OrderService{db: db}

	mkOrder := func(t *testing.T) string {
		t.Helper()
		var oid string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO orders (subtotal, total, status) VALUES (0,0,'paid') RETURNING id`).Scan(&oid); err != nil {
			t.Fatalf("seed order: %v", err)
		}
		t.Cleanup(func() {
			db.ExecContext(ctx, `DELETE FROM order_status_history WHERE order_id=$1`, oid)
			db.ExecContext(ctx, `DELETE FROM orders WHERE id=$1`, oid)
		})
		return oid
	}

	// (a) Column set → PaidAt comes from the column, ignoring any history.
	t.Run("prefers column", func(t *testing.T) {
		oid := mkOrder(t)
		if _, err := db.ExecContext(ctx,
			`UPDATE orders SET paid_at='2025-03-04T08:30:00Z' WHERE id=$1`, oid); err != nil {
			t.Fatalf("set paid_at: %v", err)
		}
		// A decoy history row at a different time — must be ignored.
		if _, err := db.ExecContext(ctx,
			`INSERT INTO order_status_history (order_id, status, created_at) VALUES ($1,'paid','2099-01-01T00:00:00Z')`, oid); err != nil {
			t.Fatalf("seed decoy history: %v", err)
		}
		o, err := svc.GetByID(ctx, oid)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if o.PaidAt == nil {
			t.Fatalf("PaidAt nil, want the column value")
		}
		got, _ := time.Parse(time.RFC3339, *o.PaidAt)
		if got.UTC().Format(time.RFC3339) != "2025-03-04T08:30:00Z" {
			t.Errorf("PaidAt = %s, want column 2025-03-04T08:30:00Z (not decoy history)", *o.PaidAt)
		}
	})

	// (b) Column NULL + history present → falls back to earliest 'paid' row.
	t.Run("falls back to history", func(t *testing.T) {
		oid := mkOrder(t)
		if _, err := db.ExecContext(ctx,
			`INSERT INTO order_status_history (order_id, status, created_at) VALUES
			   ($1,'paid','2024-06-06T10:00:00Z'),
			   ($1,'paid','2024-07-07T10:00:00Z')`, oid); err != nil {
			t.Fatalf("seed history: %v", err)
		}
		o, err := svc.GetByID(ctx, oid)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if o.PaidAt == nil {
			t.Fatalf("PaidAt nil, want earliest history row")
		}
		got, _ := time.Parse(time.RFC3339, *o.PaidAt)
		if got.UTC().Format(time.RFC3339) != "2024-06-06T10:00:00Z" {
			t.Errorf("PaidAt = %s, want earliest history 2024-06-06T10:00:00Z", *o.PaidAt)
		}
	})

	// (c) Column NULL + no history → PaidAt nil.
	t.Run("nil when neither", func(t *testing.T) {
		oid := mkOrder(t)
		o, err := svc.GetByID(ctx, oid)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if o.PaidAt != nil {
			t.Errorf("PaidAt = %q, want nil", *o.PaidAt)
		}
	})
}
