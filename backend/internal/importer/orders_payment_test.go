package importer

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// dialImporterTestDB is the gated live-DB helper shared with customers_test.go.

// TestUpsertOrderPaymentFields is the core proof that the WC importer now
// captures payment info: a paid order gets payment_method (human title),
// payment_status='succeeded' and paid_at from date_paid_gmt; an unpaid order
// leaves status/paid_at NULL (NOT stamped NOW); re-import is idempotent.
func TestUpsertOrderPaymentFields(t *testing.T) {
	db := dialImporterTestDB(t)
	// Register Close via Cleanup (not defer) so it runs LIFO-last, after the
	// row-delete cleanup below — a deferred Close fires on test return, before
	// any t.Cleanup, leaving the deletes to hit a closed pool.
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	svc := &Service{db: db}

	// High sentinel wc_order_ids so cleanup only ever touches our own rows.
	const paidWC, unpaidWC, longWC = 990000001, 990000002, 990000003
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE wc_order_id IN ($1,$2,$3))`, paidWC, unpaidWC, longWC)
		db.ExecContext(ctx, `DELETE FROM orders WHERE wc_order_id IN ($1,$2,$3)`, paidWC, unpaidWC, longWC)
	})

	// --- Paid order: guest (no billing email), no line items. ---
	paid := wcOrder{
		ID:                 paidWC,
		Number:             "TST-PAID",
		PaymentMethod:      "ppcp-gateway",
		PaymentMethodTitle: "PayPal",
		TransactionID:      "TXN-ABC-123",
		DatePaidGMT:        "2025-03-04T08:30:00",
		Total:              "100.00",
	}
	var p OrdersProgressUpdate
	if err := svc.upsertOrder(ctx, paid, "paid", "ORD", &p); err != nil {
		t.Fatalf("upsert paid order: %v", err)
	}

	var pm, ps, txn sql.NullString
	var paidAt sql.NullTime
	if err := db.QueryRowContext(ctx,
		`SELECT payment_method, payment_status, paid_at, transaction_id FROM orders WHERE wc_order_id=$1`,
		paidWC).Scan(&pm, &ps, &paidAt, &txn); err != nil {
		t.Fatalf("read back paid order: %v", err)
	}
	if pm.String != "PayPal" {
		t.Errorf("payment_method = %q, want PayPal (human title preferred over slug)", pm.String)
	}
	if ps.String != "succeeded" {
		t.Errorf("payment_status = %q, want succeeded", ps.String)
	}
	if txn.String != "TXN-ABC-123" {
		t.Errorf("transaction_id = %q, want TXN-ABC-123", txn.String)
	}
	if !paidAt.Valid {
		t.Fatalf("paid_at is NULL, want 2025-03-04T08:30:00Z")
	}
	if got := paidAt.Time.UTC().Format(time.RFC3339); got != "2025-03-04T08:30:00Z" {
		t.Errorf("paid_at = %s, want 2025-03-04T08:30:00Z", got)
	}

	// --- Unpaid order: no date_paid → status/paid_at must stay NULL. ---
	unpaid := wcOrder{
		ID:                 unpaidWC,
		Number:             "TST-UNPAID",
		PaymentMethod:      "bacs",
		PaymentMethodTitle: "銀行轉帳",
		Total:              "50.00",
	}
	if err := svc.upsertOrder(ctx, unpaid, "pending", "ORD", &p); err != nil {
		t.Fatalf("upsert unpaid order: %v", err)
	}
	var pm2, ps2, txn2 sql.NullString
	var paidAt2 sql.NullTime
	if err := db.QueryRowContext(ctx,
		`SELECT payment_method, payment_status, paid_at, transaction_id FROM orders WHERE wc_order_id=$1`,
		unpaidWC).Scan(&pm2, &ps2, &paidAt2, &txn2); err != nil {
		t.Fatalf("read back unpaid order: %v", err)
	}
	if pm2.String != "銀行轉帳" {
		t.Errorf("unpaid payment_method = %q, want 銀行轉帳", pm2.String)
	}
	if ps2.Valid {
		t.Errorf("unpaid payment_status = %q, want NULL", ps2.String)
	}
	if paidAt2.Valid {
		t.Errorf("unpaid paid_at = %v, want NULL (must not be stamped NOW)", paidAt2.Time)
	}
	if txn2.Valid {
		t.Errorf("unpaid transaction_id = %q, want NULL", txn2.String)
	}

	// --- Idempotency: re-import the paid order, values stable, still one row. ---
	if err := svc.upsertOrder(ctx, paid, "paid", "ORD", &p); err != nil {
		t.Fatalf("re-upsert paid order: %v", err)
	}
	var cnt int
	var paidAt3 sql.NullTime
	if err := db.QueryRowContext(ctx,
		`SELECT count(*), max(paid_at) FROM orders WHERE wc_order_id=$1`, paidWC).Scan(&cnt, &paidAt3); err != nil {
		t.Fatalf("count after re-import: %v", err)
	}
	if cnt != 1 {
		t.Errorf("re-import produced %d rows for wc_order_id=%d, want 1", cnt, paidWC)
	}
	if !paidAt3.Valid || paidAt3.Time.UTC().Format(time.RFC3339) != "2025-03-04T08:30:00Z" {
		t.Errorf("paid_at drifted after re-import: %v", paidAt3.Time)
	}
	if p.UpdatedOrders < 1 {
		t.Errorf("re-import should count as an update, UpdatedOrders=%d", p.UpdatedOrders)
	}

	// --- Rune-cap: a >50-rune multibyte title must store exactly 50 chars
	// without a VARCHAR(50) overflow error. ---
	long := make([]rune, 60)
	for i := range long {
		long[i] = '銀'
	}
	longOrder := wcOrder{
		ID:                 longWC,
		Number:             "TST-LONG",
		PaymentMethodTitle: string(long),
		Total:              "1.00",
	}
	if err := svc.upsertOrder(ctx, longOrder, "pending", "ORD", &p); err != nil {
		t.Fatalf("upsert long-title order: %v", err)
	}
	var charLen int
	if err := db.QueryRowContext(ctx,
		`SELECT char_length(payment_method) FROM orders WHERE wc_order_id=$1`, longWC).Scan(&charLen); err != nil {
		t.Fatalf("read back long order: %v", err)
	}
	if charLen != 50 {
		t.Errorf("long title char_length = %d, want 50 (rune-capped)", charLen)
	}
}
