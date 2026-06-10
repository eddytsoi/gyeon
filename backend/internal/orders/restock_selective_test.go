package orders

import (
	"context"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestRestockSpecificItemsTx is the core proof for selective refund restock:
// the admin can return some lines to stock while leaving others out (damaged
// goods), restock partial quantities, and never double-count across repeated
// refunds. Gated on a reachable local Postgres (see dialOrdersTestDB).
func TestRestockSpecificItemsTx(t *testing.T) {
	db := dialOrdersTestDB(t)
	defer db.Close()
	ctx := context.Background()
	svc := &OrderService{db: db}

	// Seed a product with two variants at a known stock level.
	slug := fmt.Sprintf("restock-test-%d", time.Now().UnixNano())
	var pid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO products (slug, name) VALUES ($1, 'Restock Test') RETURNING id`, slug).Scan(&pid); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM products WHERE id=$1`, pid) })

	mkVariant := func(sku string, stock int) string {
		var vid string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO product_variants (product_id, sku, price, stock_qty) VALUES ($1, $2, 100, $3) RETURNING id`,
			pid, sku, stock).Scan(&vid); err != nil {
			t.Fatalf("seed variant %s: %v", sku, err)
		}
		return vid
	}
	goodVar := mkVariant(slug+"-good", 5)    // will be restocked
	dmgVar := mkVariant(slug+"-damaged", 5)  // damaged — must NOT be restocked

	// Seed an order with one line per variant, qty 3 each.
	var oid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO orders (subtotal, total) VALUES (600, 600) RETURNING id`).Scan(&oid); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	t.Cleanup(func() { db.ExecContext(ctx, `DELETE FROM orders WHERE id=$1`, oid) })

	mkItem := func(vid string, qty int) string {
		var iid string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO order_items (order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total)
			 VALUES ($1, $2, 'Restock Test', 'sku', 100, $3, $4) RETURNING id`,
			oid, vid, qty, 100*qty).Scan(&iid); err != nil {
			t.Fatalf("seed order_item: %v", err)
		}
		return iid
	}
	goodItem := mkItem(goodVar, 3)
	dmgItem := mkItem(dmgVar, 3)

	stockOf := func(vid string) int {
		var q int
		if err := db.QueryRowContext(ctx, `SELECT stock_qty FROM product_variants WHERE id=$1`, vid).Scan(&q); err != nil {
			t.Fatalf("read stock: %v", err)
		}
		return q
	}
	restockedOf := func(iid string) int {
		var q int
		if err := db.QueryRowContext(ctx, `SELECT restocked_qty FROM order_items WHERE id=$1`, iid).Scan(&q); err != nil {
			t.Fatalf("read restocked_qty: %v", err)
		}
		return q
	}
	histCount := func(vid string) int {
		var n int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM inventory_history WHERE order_id=$1 AND variant_id=$2`, oid, vid).Scan(&n); err != nil {
			t.Fatalf("count history: %v", err)
		}
		return n
	}

	run := func(items []RestockItem) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		note := "damaged-return"
		if err := svc.restockSpecificItemsTx(ctx, tx, oid, items, "order.refund", &note); err != nil {
			tx.Rollback()
			t.Fatalf("restockSpecificItemsTx: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	// (1) Restock 2 of the good line; leave the damaged line out entirely.
	run([]RestockItem{
		{OrderItemID: goodItem, Quantity: 2},
		{OrderItemID: dmgItem, Quantity: 0}, // qty 0 — skipped
	})
	if got := stockOf(goodVar); got != 7 {
		t.Fatalf("good variant stock = %d, want 7 (5 + 2 restocked)", got)
	}
	if got := stockOf(dmgVar); got != 5 {
		t.Fatalf("damaged variant stock = %d, want 5 (untouched)", got)
	}
	if got := restockedOf(goodItem); got != 2 {
		t.Fatalf("good item restocked_qty = %d, want 2", got)
	}
	if got := restockedOf(dmgItem); got != 0 {
		t.Fatalf("damaged item restocked_qty = %d, want 0", got)
	}
	if got := histCount(goodVar); got != 1 {
		t.Fatalf("good variant inventory_history rows = %d, want 1", got)
	}
	if got := histCount(dmgVar); got != 0 {
		t.Fatalf("damaged variant inventory_history rows = %d, want 0", got)
	}

	// (2) Over-restock the good line (ask for 5, only 1 remains): clamps to 1,
	//     never exceeding the ordered quantity. No double-count.
	run([]RestockItem{{OrderItemID: goodItem, Quantity: 5}})
	if got := stockOf(goodVar); got != 8 {
		t.Fatalf("good variant stock = %d, want 8 (clamped to remaining 1)", got)
	}
	if got := restockedOf(goodItem); got != 3 {
		t.Fatalf("good item restocked_qty = %d, want 3 (== ordered qty, capped)", got)
	}

	// (3) Fully restocked line is a no-op now.
	run([]RestockItem{{OrderItemID: goodItem, Quantity: 1}})
	if got := stockOf(goodVar); got != 8 {
		t.Fatalf("good variant stock = %d, want 8 (already fully restocked, no-op)", got)
	}
}
