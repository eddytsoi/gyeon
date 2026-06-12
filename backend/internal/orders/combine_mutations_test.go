package orders

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gyeon/backend/internal/customers"

	_ "github.com/lib/pq"
)

// TestCreateOrderFromMutations is the core proof for combining executed
// out-mutations (出貨單) into one accounting-only order: it aggregates leaf
// items across mutations, never touches stock, locks the source mutations
// against re-use, and — critically — a later cancellation does NOT restock the
// goods that already shipped. Gated on a reachable local Postgres.
func TestCreateOrderFromMutations(t *testing.T) {
	db := dialOrdersTestDB(t)
	// Close via Cleanup (not defer) so it runs AFTER the t.Cleanup data deletes
	// below — deferred calls fire before Cleanups, which would close the pool
	// out from under them and leak the seeded rows.
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	svc := &OrderService{db: db, customerSvc: customers.NewService(db)}

	uniq := time.Now().UnixNano()

	// --- Seed product + variant. stock_qty=95 simulates the post-out-mutation
	//     on-hand level (the goods already left when the mutations executed). ---
	var pid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO products (slug, name) VALUES ($1, 'Combine Test') RETURNING id`,
		fmt.Sprintf("combine-test-%d", uniq)).Scan(&pid); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM products WHERE id=$1`, pid) })

	var vid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO product_variants (product_id, sku, price, stock_qty) VALUES ($1, $2, 100, 95) RETURNING id`,
		pid, fmt.Sprintf("combine-sku-%d", uniq)).Scan(&vid); err != nil {
		t.Fatalf("seed variant: %v", err)
	}

	stockOf := func() int {
		var q int
		if err := db.QueryRowContext(ctx, `SELECT stock_qty FROM product_variants WHERE id=$1`, vid).Scan(&q); err != nil {
			t.Fatalf("read stock: %v", err)
		}
		return q
	}

	// --- Seed customer + default address. ---
	var cid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name, role) VALUES ($1,'Combine','Buyer','installer_v2') RETURNING id`,
		fmt.Sprintf("combine-%d@test.hk", uniq)).Scan(&cid); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM customers WHERE id=$1`, cid) })

	var aid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, city, postal_code, country, is_default)
		 VALUES ($1,'Combine','Buyer','555','1 Test Rd','Kowloon','000','HK',TRUE) RETURNING id`,
		cid).Scan(&aid); err != nil {
		t.Fatalf("seed address: %v", err)
	}

	// --- Seed two executed out-mutations for the SAME variant (qty 5 and 3) so
	//     the combine must aggregate them into one order line of qty 8. ---
	mkExecutedOutMutation := func(qty int) string {
		var mid string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO stock_mutations (mutation_number, type, status, executed_at)
			 VALUES ($1, 'out', 'executed', NOW()) RETURNING id`,
			fmt.Sprintf("MUT-COMB-%d-%d", uniq, qty)).Scan(&mid); err != nil {
			t.Fatalf("seed mutation: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO stock_mutation_items (mutation_id, variant_id, quantity, before_qty, after_qty)
			 VALUES ($1, $2, $3, 0, 0)`, mid, vid, qty); err != nil {
			t.Fatalf("seed mutation item: %v", err)
		}
		// Delete the mutation (cascades its items) before the product cleanup runs
		// (LIFO order) — stock_mutation_items pins the variant via ON DELETE RESTRICT.
		t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM stock_mutations WHERE id=$1`, mid) })
		return mid
	}
	mutA := mkExecutedOutMutation(5)
	mutB := mkExecutedOutMutation(3)

	// --- Combine both into one order assigned to the customer. ---
	order, err := svc.CreateOrderFromMutations(ctx, CombineMutationsRequest{
		MutationIDs:       []string{mutA, mutB},
		CustomerID:        &cid,
		ShippingAddressID: &aid,
	})
	if err != nil {
		t.Fatalf("CreateOrderFromMutations: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM orders WHERE id=$1`, order.ID) })

	// (1) Stock untouched — no double-deduction.
	if got := stockOf(); got != 95 {
		t.Fatalf("stock_qty = %d, want 95 (combine must not deduct)", got)
	}
	// (2) Order is accounting-only.
	if order.StockManaged {
		t.Fatalf("order.StockManaged = true, want false")
	}
	// (3) No inventory_history written for this order.
	var histN int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM inventory_history WHERE order_id=$1`, order.ID).Scan(&histN); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if histN != 0 {
		t.Fatalf("inventory_history rows = %d, want 0", histN)
	}
	// (4) Leaf items aggregated: one line, qty 5+3=8.
	leaf := 0
	for _, it := range order.Items {
		if it.VariantID != nil && *it.VariantID == vid {
			leaf++
			if it.Quantity != 8 {
				t.Fatalf("order line qty = %d, want 8 (aggregated)", it.Quantity)
			}
		}
	}
	if leaf != 1 {
		t.Fatalf("order lines for variant = %d, want 1 aggregated line", leaf)
	}
	// (5) Both source mutations locked to this order.
	for _, mid := range []string{mutA, mutB} {
		var consumed *string
		if err := db.QueryRowContext(ctx,
			`SELECT consumed_by_order_id FROM stock_mutations WHERE id=$1`, mid).Scan(&consumed); err != nil {
			t.Fatalf("read consumed_by: %v", err)
		}
		if consumed == nil || *consumed != order.ID {
			t.Fatalf("mutation %s consumed_by_order_id = %v, want %s", mid, consumed, order.ID)
		}
	}

	// (6) Re-combining an already-consumed mutation is rejected with a reason.
	_, err = svc.CreateOrderFromMutations(ctx, CombineMutationsRequest{
		MutationIDs:       []string{mutA},
		CustomerID:        &cid,
		ShippingAddressID: &aid,
	})
	var notCombinable *MutationsNotCombinableError
	if !errors.As(err, &notCombinable) {
		t.Fatalf("re-combine error = %v, want *MutationsNotCombinableError", err)
	}
	if len(notCombinable.Problems) != 1 || notCombinable.Problems[0].Reason != "already_consumed" {
		t.Fatalf("problems = %+v, want one already_consumed", notCombinable.Problems)
	}

	// (7) Cancelling the order must NOT restock (goods already shipped out).
	if _, err := svc.UpdateStatus(ctx, order.ID, UpdateStatusRequest{Status: StatusCancelled}); err != nil {
		t.Fatalf("UpdateStatus cancelled: %v", err)
	}
	if got := stockOf(); got != 95 {
		t.Fatalf("stock_qty after cancel = %d, want 95 (no restock for accounting-only order)", got)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM inventory_history WHERE order_id=$1`, order.ID).Scan(&histN); err != nil {
		t.Fatalf("count history after cancel: %v", err)
	}
	if histN != 0 {
		t.Fatalf("inventory_history rows after cancel = %d, want 0", histN)
	}
}
