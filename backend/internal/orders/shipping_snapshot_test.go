package orders

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// dialOrdersTestDB mirrors the gated live-DB convention used elsewhere
// (customers/address_dedup_test.go): skip when no Postgres is reachable.
func dialOrdersTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://gyeon:gyeon@localhost:5432/gyeon?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("no test DB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("test DB unreachable: %v", err)
	}
	return db
}

// TestOrderShippingAddressSnapshot is the core proof that an order's shipping
// address is frozen at write time: editing or deleting the customer's
// address-book row must never alter (or wipe) a placed order's address.
func TestOrderShippingAddressSnapshot(t *testing.T) {
	db := dialOrdersTestDB(t)
	defer db.Close()
	ctx := context.Background()
	svc := &OrderService{db: db}

	email := fmt.Sprintf("ordersnap-%d@test.hk", time.Now().UnixNano())
	var cid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name) VALUES ($1,'Snap','Test') RETURNING id`,
		email).Scan(&cid); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	var aid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default)
		 VALUES ($1,'Snap','Test','111','100 Original Rd','Unit 1','Kowloon',NULL,'000','HK',TRUE) RETURNING id`,
		cid).Scan(&aid); err != nil {
		t.Fatalf("seed address: %v", err)
	}

	// loadShipSnapshot is the exact helper checkout/admin use to freeze the
	// address. Insert an order mirroring that write path (FK + ship_* copy).
	snap, err := svc.loadShipSnapshot(ctx, &aid)
	if err != nil {
		t.Fatalf("loadShipSnapshot: %v", err)
	}
	if snap.Line1 != "100 Original Rd" {
		t.Fatalf("loadShipSnapshot line1 = %q, want 100 Original Rd", snap.Line1)
	}

	var oid string
	insertArgs := append([]any{cid, aid, 0.0, 0.0}, snap.args()...)
	if err := db.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, shipping_address_id, subtotal, total,
		    ship_first_name, ship_last_name, ship_phone, ship_line1, ship_line2, ship_city, ship_state, ship_postal_code, ship_country)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		insertArgs...).Scan(&oid); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM orders WHERE id=$1`, oid)
		db.ExecContext(ctx, `DELETE FROM addresses WHERE customer_id=$1`, cid)
		db.ExecContext(ctx, `DELETE FROM customers WHERE id=$1`, cid)
	})

	o1, err := svc.GetByID(ctx, oid)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if o1.ShippingAddress == nil || o1.ShippingAddress.Line1 != "100 Original Rd" || o1.ShippingAddress.City != "Kowloon" {
		t.Fatalf("expected frozen snapshot, got %+v", o1.ShippingAddress)
	}

	// (a) EDIT the book address — order must be unchanged.
	if _, err := db.ExecContext(ctx, `UPDATE addresses SET line1='999 Changed Ave', city='Central' WHERE id=$1`, aid); err != nil {
		t.Fatalf("edit address: %v", err)
	}
	o2, err := svc.GetByID(ctx, oid)
	if err != nil {
		t.Fatalf("GetByID after edit: %v", err)
	}
	if o2.ShippingAddress == nil || o2.ShippingAddress.Line1 != "100 Original Rd" || o2.ShippingAddress.City != "Kowloon" {
		t.Fatalf("order address changed after book EDIT: %+v", o2.ShippingAddress)
	}

	// (b) DELETE the book address — FK SET NULL must not wipe the snapshot.
	if _, err := db.ExecContext(ctx, `DELETE FROM addresses WHERE id=$1`, aid); err != nil {
		t.Fatalf("delete address: %v", err)
	}
	o3, err := svc.GetByID(ctx, oid)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if o3.ShippingAddress == nil || o3.ShippingAddress.Line1 != "100 Original Rd" {
		t.Fatalf("order lost its address after book DELETE: %+v", o3.ShippingAddress)
	}
	if o3.ShippingAddressID != nil {
		t.Fatalf("expected shipping_address_id NULL after FK SET NULL, got %q", *o3.ShippingAddressID)
	}
}
