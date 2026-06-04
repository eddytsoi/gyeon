package importer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestLooksLikeFakeAccount pins the conservative fake-account rule: a customer
// is fake only when it is not a paying customer AND has no name AND no billing
// phone. Any single real signal flips it to real. Cases mirror real rows pulled
// from the gyeon.hk WooCommerce store during planning.
func TestLooksLikeFakeAccount(t *testing.T) {
	withPhone := func(p string) wcCustomerAddress { return wcCustomerAddress{Phone: p} }

	tests := []struct {
		name string
		c    wcCustomer
		want bool
	}{
		{
			name: "paying customer with no name/phone is real",
			c:    wcCustomer{Email: "p@x.com", IsPayingCustomer: true},
			want: false,
		},
		{
			name: "non-paying but has first name is real",
			c:    wcCustomer{Email: "carl@x.com", FirstName: "Carl"},
			want: false,
		},
		{
			name: "non-paying but has last name is real",
			c:    wcCustomer{Email: "ah@x.com", LastName: "Donn"},
			want: false,
		},
		{
			name: "non-paying but has billing phone is real",
			c:    wcCustomer{Email: "ah@x.com", Billing: withPhone("91678029")},
			want: false,
		},
		{
			name: "real buyer sample (Jaytsoi)",
			c:    wcCustomer{Email: "magic18729@hotmail.com", FirstName: "Ho Nam", LastName: "Tsoi", Username: "Jaytsoi", IsPayingCustomer: true},
			want: false,
		},
		{
			name: "bot signup: no name, no phone, not paying, random username",
			c:    wcCustomer{Email: "romanaschuerz@gmail.com", Username: "RNZkPfFNerJfSu"},
			want: true,
		},
		{
			name: "bot signup with whitespace-only name/phone",
			c:    wcCustomer{Email: "x@x.com", FirstName: "  ", LastName: "\t", Billing: withPhone("  ")},
			want: true,
		},
		{
			name: "empty customer is fake",
			c:    wcCustomer{Email: "blank@x.com"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeFakeAccount(tt.c); got != tt.want {
				t.Errorf("looksLikeFakeAccount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// dialImporterTestDB connects to the local dev database, skipping the test when
// no DB is reachable (e.g. CI without Postgres). Mirrors the gated live-test
// convention used in internal/customers/service_oauth_test.go.
func dialImporterTestDB(t *testing.T) *sql.DB {
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

// TestPurgeFakeCustomer exercises the destructive cleanup path against the local
// DB: a fake row with no engagement is deleted (with its addresses), while a row
// that has an order or a password is protected and kept.
func TestPurgeFakeCustomer(t *testing.T) {
	db := dialImporterTestDB(t)
	defer db.Close()
	s := &Service{db: db}
	ctx := context.Background()
	base := int(time.Now().UnixNano() % 1_000_000_000)

	seed := func(t *testing.T, wcID int, email string, withPassword bool) string {
		t.Helper()
		pw := sql.NullString{}
		if withPassword {
			pw = sql.NullString{String: "$2a$10$abcdefghijklmnopqrstuv", Valid: true}
		}
		var id string
		if err := db.QueryRowContext(ctx,
			`INSERT INTO customers (email, first_name, last_name, wc_customer_id, password_hash)
			 VALUES ($1, '', '', $2, $3) RETURNING id`, email, wcID, pw).Scan(&id); err != nil {
			t.Fatalf("seed customer: %v", err)
		}
		return id
	}
	cleanup := func(id string) {
		db.Exec(`DELETE FROM orders WHERE customer_id=$1`, id)
		db.Exec(`DELETE FROM addresses WHERE customer_id=$1`, id)
		db.Exec(`DELETE FROM carts WHERE customer_id=$1`, id)
		db.Exec(`DELETE FROM customers WHERE id=$1`, id)
	}
	exists := func(id string) bool {
		var n int
		db.QueryRowContext(ctx, `SELECT count(*) FROM customers WHERE id=$1`, id).Scan(&n)
		return n > 0
	}

	t.Run("deletes a fake with no engagement (and its addresses)", func(t *testing.T) {
		wcID := base + 1
		email := fmt.Sprintf("fake+%d@example.com", wcID)
		id := seed(t, wcID, email, false)
		t.Cleanup(func() { cleanup(id) })
		if _, err := db.ExecContext(ctx,
			`INSERT INTO addresses (customer_id, first_name, last_name, line1, city, postal_code, country)
			 VALUES ($1, '', '', 'x', 'x', '000', 'HK')`, id); err != nil {
			t.Fatalf("seed address: %v", err)
		}
		removed, reason, err := s.purgeFakeCustomer(ctx, wcCustomer{ID: wcID, Email: email})
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if !removed || reason != "" {
			t.Fatalf("want removed=true, reason=''; got removed=%v reason=%q", removed, reason)
		}
		if exists(id) {
			t.Fatal("customer row still present after purge")
		}
		var addrN int
		db.QueryRowContext(ctx, `SELECT count(*) FROM addresses WHERE customer_id=$1`, id).Scan(&addrN)
		if addrN != 0 {
			t.Fatalf("addresses left orphaned: %d", addrN)
		}
	})

	t.Run("keeps a fake that has an order", func(t *testing.T) {
		wcID := base + 2
		email := fmt.Sprintf("fakeorder+%d@example.com", wcID)
		id := seed(t, wcID, email, false)
		t.Cleanup(func() { cleanup(id) })
		if _, err := db.ExecContext(ctx,
			`INSERT INTO orders (customer_id, subtotal, total) VALUES ($1, 0, 0)`, id); err != nil {
			t.Fatalf("seed order: %v", err)
		}
		removed, reason, err := s.purgeFakeCustomer(ctx, wcCustomer{ID: wcID, Email: email})
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if removed {
			t.Fatal("must not delete a customer that has orders")
		}
		if reason != "has orders" {
			t.Fatalf("reason = %q, want 'has orders'", reason)
		}
		if !exists(id) {
			t.Fatal("customer with an order was wrongly deleted")
		}
	})

	t.Run("keeps a fake that set a password", func(t *testing.T) {
		wcID := base + 3
		email := fmt.Sprintf("fakepw+%d@example.com", wcID)
		id := seed(t, wcID, email, true)
		t.Cleanup(func() { cleanup(id) })
		removed, reason, err := s.purgeFakeCustomer(ctx, wcCustomer{ID: wcID, Email: email})
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if removed || reason != "has a password" {
			t.Fatalf("got removed=%v reason=%q, want kept with 'has a password'", removed, reason)
		}
		if !exists(id) {
			t.Fatal("customer with a password was wrongly deleted")
		}
	})

	t.Run("matches by email when wc_customer_id differs", func(t *testing.T) {
		wcID := base + 4
		email := fmt.Sprintf("fakeemail+%d@example.com", wcID)
		id := seed(t, wcID, email, false)
		t.Cleanup(func() { cleanup(id) })
		// WC payload carries a different/zero ID but the same email — must still match.
		removed, reason, err := s.purgeFakeCustomer(ctx, wcCustomer{ID: base + 9999, Email: email})
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if !removed || reason != "" {
			t.Fatalf("want removed via email; got removed=%v reason=%q", removed, reason)
		}
		if exists(id) {
			t.Fatal("customer not removed on email match")
		}
	})

	t.Run("no-op when the account was never imported", func(t *testing.T) {
		removed, reason, err := s.purgeFakeCustomer(ctx, wcCustomer{
			ID:    base + 8888,
			Email: fmt.Sprintf("ghost+%d@example.com", base),
		})
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if removed || reason != "" {
			t.Fatalf("want no-op; got removed=%v reason=%q", removed, reason)
		}
	})
}
