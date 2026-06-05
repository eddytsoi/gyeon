package customers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// dialDedupTestDB mirrors the gated live-DB convention in
// service_oauth_test.go: skip when no Postgres is reachable.
func dialDedupTestDB(t *testing.T) *sql.DB {
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

func TestFindOrCreateAddress(t *testing.T) {
	db := dialDedupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	email := fmt.Sprintf("dedup-%d@test.hk", time.Now().UnixNano())
	var cid string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name) VALUES ($1,'T','T') RETURNING id`,
		email).Scan(&cid); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	var guestIDs []string
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM addresses WHERE customer_id=$1`, cid)
		for _, g := range guestIDs {
			db.ExecContext(ctx, `DELETE FROM addresses WHERE id=$1`, g)
		}
		db.ExecContext(ctx, `DELETE FROM customers WHERE id=$1`, cid)
	})

	f := AddressFields{FirstName: "T", LastName: "T", Line1: "1 Test Rd", City: "TC", PostalCode: "000", Country: "HK"}

	id1, err := FindOrCreateAddress(ctx, db, &cid, f, true)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Identical payload must reuse the existing row.
	id2, err := FindOrCreateAddress(ctx, db, &cid, f, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("identical address should reuse %s, got %s", id1, id2)
	}

	// Case / whitespace variant must also reuse (same normalization as index).
	fv := f
	fv.Line1, fv.City, fv.Country = "  1 TEST RD ", "tc", "hk"
	id3, err := FindOrCreateAddress(ctx, db, &cid, fv, false)
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	if id3 != id1 {
		t.Fatalf("case/space variant should reuse %s, got %s", id1, id3)
	}

	// A genuinely different address inserts a new row.
	fd := f
	fd.Line1 = "2 Other Rd"
	id4, err := FindOrCreateAddress(ctx, db, &cid, fd, false)
	if err != nil {
		t.Fatalf("different: %v", err)
	}
	if id4 == id1 {
		t.Fatalf("different address must not reuse")
	}

	// Guest (nil customer) always inserts a fresh, unshared row.
	g1, err := FindOrCreateAddress(ctx, db, nil, f, false)
	if err != nil {
		t.Fatalf("guest1: %v", err)
	}
	g2, err := FindOrCreateAddress(ctx, db, nil, f, false)
	if err != nil {
		t.Fatalf("guest2: %v", err)
	}
	guestIDs = append(guestIDs, g1, g2)
	if g1 == "" || g2 == "" || g1 == g2 {
		t.Fatalf("guest snapshots must be distinct, got %q and %q", g1, g2)
	}

	// The customer should hold exactly 2 distinct addresses.
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM addresses WHERE customer_id=$1`, cid).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 customer addresses, got %d", n)
	}
}
