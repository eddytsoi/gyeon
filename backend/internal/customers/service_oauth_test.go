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

// dialTestDB connects to the local dev database, skipping the test when no DB
// is reachable (e.g. CI without Postgres). Mirrors the gated live-test
// convention used elsewhere in the repo.
func dialTestDB(t *testing.T) *sql.DB {
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

func TestFindOrCreateByOAuth(t *testing.T) {
	db := dialTestDB(t)
	defer db.Close()
	svc := NewService(db)
	ctx := context.Background()

	uniq := time.Now().UnixNano()
	email := fmt.Sprintf("oauthtest+%d@example.com", uniq)
	googleSub := fmt.Sprintf("google-sub-%d", uniq)
	appleSub := fmt.Sprintf("apple-sub-%d", uniq)

	var customerID string
	t.Cleanup(func() {
		if customerID != "" {
			db.Exec(`DELETE FROM customer_oauth_identities WHERE customer_id=$1`, customerID)
			db.Exec(`DELETE FROM customers WHERE id=$1`, customerID)
		}
	})

	// 1. First Google login → brand-new customer + identity.
	c, err := svc.FindOrCreateByOAuth(ctx, "google", googleSub, email, "Ada", "Lovelace")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	customerID = c.ID
	if c.Email != email || c.FirstName != "Ada" || c.LastName != "Lovelace" {
		t.Fatalf("unexpected customer: %+v", c)
	}
	if c.Role != RoleCustomer {
		t.Fatalf("role = %q, want %q", c.Role, RoleCustomer)
	}

	// 2. Returning Google login (same provider+subject) → same customer.
	c2, err := svc.FindOrCreateByOAuth(ctx, "google", googleSub, email, "", "")
	if err != nil {
		t.Fatalf("returning: %v", err)
	}
	if c2.ID != customerID {
		t.Fatalf("returning login created a new customer %s (want %s)", c2.ID, customerID)
	}

	// 3. Apple login with the SAME email → auto-links to the existing customer.
	c3, err := svc.FindOrCreateByOAuth(ctx, "apple", appleSub, email, "", "")
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if c3.ID != customerID {
		t.Fatalf("auto-link created a new customer %s (want %s)", c3.ID, customerID)
	}

	// Exactly one customer for this email, two linked identities.
	var nCustomers, nIdentities int
	db.QueryRow(`SELECT count(*) FROM customers WHERE email=$1`, email).Scan(&nCustomers)
	db.QueryRow(`SELECT count(*) FROM customer_oauth_identities WHERE customer_id=$1`, customerID).Scan(&nIdentities)
	if nCustomers != 1 {
		t.Fatalf("customers with email = %d, want 1", nCustomers)
	}
	if nIdentities != 2 {
		t.Fatalf("linked identities = %d, want 2 (google+apple)", nIdentities)
	}
}

func TestFindOrCreateByOAuthRequiresEmailAndSubject(t *testing.T) {
	svc := NewService(nil) // never reaches the DB — validation happens first
	if _, err := svc.FindOrCreateByOAuth(context.Background(), "google", "", "a@b.com", "", ""); err == nil {
		t.Fatal("expected error for empty subject")
	}
	if _, err := svc.FindOrCreateByOAuth(context.Background(), "google", "sub", "", "", ""); err == nil {
		t.Fatal("expected error for empty email")
	}
}
