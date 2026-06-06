package orders

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// TestMarkPaidByPaymentIntentStoresTransactionID proves a native Stripe payment
// records the Charge id (ch_…) into orders.transaction_id (and that GetByID
// returns it), idempotently. The order is seeded already-paid so the call skips
// the pending-only side effects (status flip, confirmation email, loyalty),
// keeping the test hermetic with a nil emailSvc.
func TestMarkPaidByPaymentIntentStoresTransactionID(t *testing.T) {
	db := dialOrdersTestDB(t)
	t.Cleanup(func() { db.Close() }) // LIFO: runs after the row-delete cleanup below
	ctx := context.Background()
	svc := &OrderService{db: db}

	pi := fmt.Sprintf("pi_test_%d", time.Now().UnixNano())
	var oid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO orders (subtotal, total, status, payment_status, payment_intent_id)
		 VALUES (0,0,'paid','succeeded',$1) RETURNING id`, pi).Scan(&oid); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM orders WHERE id=$1`, oid) })

	const chargeID = "ch_test_abc123"
	if err := svc.MarkPaidByPaymentIntent(ctx, pi, "card", "visa", "4242", chargeID); err != nil {
		t.Fatalf("MarkPaidByPaymentIntent: %v", err)
	}

	var txn sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT transaction_id FROM orders WHERE id=$1`, oid).Scan(&txn); err != nil {
		t.Fatalf("read back transaction_id: %v", err)
	}
	if txn.String != chargeID {
		t.Errorf("transaction_id column = %q, want %q", txn.String, chargeID)
	}

	o, err := svc.GetByID(ctx, oid)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if o.TransactionID == nil || *o.TransactionID != chargeID {
		t.Errorf("GetByID TransactionID = %v, want %q", o.TransactionID, chargeID)
	}

	// Idempotent: a later sparse event (empty charge id) must NOT wipe it.
	if err := svc.MarkPaidByPaymentIntent(ctx, pi, "", "", "", ""); err != nil {
		t.Fatalf("MarkPaidByPaymentIntent (sparse): %v", err)
	}
	o2, err := svc.GetByID(ctx, oid)
	if err != nil {
		t.Fatalf("GetByID after sparse: %v", err)
	}
	if o2.TransactionID == nil || *o2.TransactionID != chargeID {
		t.Errorf("sparse re-call wiped transaction_id: got %v, want %q", o2.TransactionID, chargeID)
	}
}
