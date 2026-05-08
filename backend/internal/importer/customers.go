package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	"gyeon/backend/internal/email"
)

// CustomersImportRequest mirrors ImportRequest but is scoped to the
// /wc/v3/customers endpoint. Only upsert mode is supported — replace
// would orphan order rows (orders.customer_id ON DELETE SET NULL) and
// destroy any storefront accounts customers created post-import.
type CustomersImportRequest struct {
	WCURL    string `json:"wc_url"`
	WCKey    string `json:"wc_key"`
	WCSecret string `json:"wc_secret"`
	// Limit caps the run. 0 = no cap (full sync).
	Limit int `json:"limit"`
	// SendSetupEmail, when true, fires off a setup-password email to each
	// newly-inserted customer (not to ones that already existed). Re-runs
	// won't re-spam imported rows because the email is gated on insert.
	SendSetupEmail bool `json:"send_setup_email"`
}

// CustomersProgressUpdate is streamed once per processed customer plus a
// final Done frame. Mirrors the shape of ProgressUpdate so the frontend's
// SSE plumbing can be largely re-used.
type CustomersProgressUpdate struct {
	TotalCustomers     int      `json:"total_customers"`
	ProcessedCustomers int      `json:"processed_customers"`
	ImportedCustomers  int      `json:"imported_customers"` // newly inserted
	UpdatedCustomers   int      `json:"updated_customers"`  // matched (by wc_customer_id or email), updated in place
	ImportedAddresses  int      `json:"imported_addresses"` // billing + shipping rows added on first import
	SetupEmailsQueued  int      `json:"setup_emails_queued"` // setup-password emails kicked off (async; failures logged server-side)
	Failed             int      `json:"failed"`
	CurrentCustomer    string   `json:"current_customer,omitempty"`
	Done               bool     `json:"done"`
	Errors             []string `json:"errors"`
}

// CustomerTotal returns the WC store's total customer count via the
// X-WP-Total header. 0 on error — the test endpoint already validated
// connectivity, so a missing total is just a UX nicety.
func (s *Service) CustomerTotal(req CustomersImportRequest) int {
	return newWCClient(req.WCURL, req.WCKey, req.WCSecret).fetchCustomerTotal()
}

// RunCustomersStreaming pages through /wc/v3/customers and upserts each
// row into local customers + addresses, calling send() with progress after
// each customer. The final call always has Done = true.
func (s *Service) RunCustomersStreaming(ctx context.Context, req CustomersImportRequest, send func(CustomersProgressUpdate)) {
	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := CustomersProgressUpdate{Errors: []string{}}

	p.TotalCustomers = wc.fetchCustomerTotal()
	if req.Limit > 0 && (p.TotalCustomers == 0 || p.TotalCustomers > req.Limit) {
		p.TotalCustomers = req.Limit
	}
	send(p)

	// Counter for setup-password emails kicked off via the goroutines below.
	// Atomic because the dispatch is async; we read it from the import loop
	// to push the latest value out via the SSE frame.
	var emailsQueued int64

pages:
	for page := 1; ; page++ {
		batch, err := wc.fetchCustomers(page)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch customers page %d: %v", page, err))
			break
		}
		if len(batch) == 0 {
			break
		}
		for _, c := range batch {
			if req.Limit > 0 && p.ProcessedCustomers >= req.Limit {
				break pages
			}
			// Skip rows we cannot key on. WC will sometimes emit user accounts
			// without an email (deleted users, role-only entries) — there is
			// no useful local identity for those.
			if strings.TrimSpace(c.Email) == "" {
				p.Failed++
				p.Errors = append(p.Errors, fmt.Sprintf("customer id=%d skipped: missing email", c.ID))
				p.ProcessedCustomers++
				send(p)
				continue
			}
			p.CurrentCustomer = displayName(c)
			send(p)

			newID, err := s.upsertCustomer(ctx, c, &p)
			if err != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("customer %s: %v", c.Email, err))
				p.Failed++
			} else if req.SendSetupEmail && newID != "" {
				// Fire-and-forget: SMTP is slow and we don't want to block
				// the import. Failures get logged server-side, not surfaced
				// in the SSE stream — emailsQueued counts dispatches, not
				// deliveries. Re-runs of the import never re-send because
				// upsertCustomer returns "" for already-existing rows.
				go s.sendSetupPasswordEmail(c, newID, &emailsQueued)
			}
			p.SetupEmailsQueued = int(atomic.LoadInt64(&emailsQueued))
			p.ProcessedCustomers++
			p.CurrentCustomer = ""
			send(p)
		}
	}

	// Final flush of the emails counter — late dispatches that happened
	// after the loop exited are still reflected.
	p.SetupEmailsQueued = int(atomic.LoadInt64(&emailsQueued))
	p.Done = true
	send(p)
}

// sendSetupPasswordEmail mints a 7-day setup token via the customers
// service and sends the standard password-reset email. Used by the
// customers import when SendSetupEmail is set. Errors are logged but
// never surface to the importer caller — best-effort delivery.
func (s *Service) sendSetupPasswordEmail(wc wcCustomer, customerID string, counter *int64) {
	if s.customerSvc == nil || s.emailSvc == nil {
		return
	}
	// Use a fresh context so the request that initiated the import being
	// cancelled (e.g. admin closes the page) doesn't truncate emails that
	// have already been queued.
	ctx := context.Background()
	token, err := s.customerSvc.CreateSetupToken(ctx, customerID)
	if err != nil {
		log.Printf("import: setup token for %s: %v", wc.Email, err)
		return
	}
	resetURL := strings.TrimRight(s.emailSvc.PublicBaseURL(ctx), "/") + "/account/reset-password?token=" + token
	first, last := resolveName(wc)
	name := strings.TrimSpace(first + " " + last)
	if name == "" {
		name = wc.Email
	}
	if err := s.emailSvc.SendPasswordResetEmail(ctx, email.PasswordResetParams{
		CustomerName:  name,
		CustomerEmail: wc.Email,
		ResetURL:      resetURL,
		ExpiryHours:   7 * 24, // matches CreateSetupToken's 7-day expiry
	}); err != nil {
		log.Printf("import: setup email for %s: %v", wc.Email, err)
		return
	}
	atomic.AddInt64(counter, 1)
}

// upsertCustomer inserts a new customer (with billing + shipping addresses)
// or, if a row already exists either by wc_customer_id or email, updates the
// names / phone in place. Existing rows keep their addresses untouched —
// re-running the import never overwrites address edits an admin (or the
// customer themself, post-import) made on storefront.
//
// Returns the local customer.id only when a brand-new row was inserted; an
// empty string means the WC customer was matched against an existing row,
// so the caller knows not to send a setup-password email (avoiding spam on
// re-runs).
func (s *Service) upsertCustomer(ctx context.Context, wc wcCustomer, p *CustomersProgressUpdate) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	first, last := resolveName(wc)
	phone := nullableString(wc.Billing.Phone)

	var customerID string
	existed := false

	// 1) Match by wc_customer_id (idempotent re-run).
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE wc_customer_id=$1`, wc.ID).Scan(&customerID)
	if err == nil {
		existed = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("lookup by wc_customer_id: %w", err)
	} else {
		// 2) Fall back to email so a manually-created storefront account adopts
		//    its WC counterpart instead of failing the unique email constraint.
		err = tx.QueryRowContext(ctx,
			`SELECT id FROM customers WHERE email=$1`, wc.Email).Scan(&customerID)
		if err == nil {
			existed = true
			if _, err := tx.ExecContext(ctx,
				`UPDATE customers SET wc_customer_id=$2 WHERE id=$1`,
				customerID, wc.ID); err != nil {
				return "", fmt.Errorf("backfill wc_customer_id: %w", err)
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("lookup by email: %w", err)
		}
	}

	if existed {
		if _, err := tx.ExecContext(ctx,
			`UPDATE customers SET first_name=$2, last_name=$3, phone=$4 WHERE id=$1`,
			customerID, first, last, phone); err != nil {
			return "", fmt.Errorf("update customer: %w", err)
		}
		p.UpdatedCustomers++
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return "", nil
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name, phone, wc_customer_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		wc.Email, first, last, phone, wc.ID).Scan(&customerID); err != nil {
		return "", fmt.Errorf("insert customer: %w", err)
	}
	p.ImportedCustomers++

	// Addresses are only inserted on first import. Re-runs leave the
	// existing rows alone so admin-edited billing details stick.
	if hasAddress(wc.Billing) {
		if err := insertAddress(ctx, tx, customerID, wc.Billing, true); err != nil {
			return "", fmt.Errorf("insert billing address: %w", err)
		}
		p.ImportedAddresses++
	}
	if hasAddress(wc.Shipping) && !sameAddress(wc.Billing, wc.Shipping) {
		if err := insertAddress(ctx, tx, customerID, wc.Shipping, !hasAddress(wc.Billing)); err != nil {
			return "", fmt.Errorf("insert shipping address: %w", err)
		}
		p.ImportedAddresses++
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return customerID, nil
}

// resolveName fills empty first/last_name fallbacks from the WC username
// or email-local-part. The local schema marks both NOT NULL, so we cannot
// pass through empty strings.
func resolveName(c wcCustomer) (first, last string) {
	first = strings.TrimSpace(c.FirstName)
	last = strings.TrimSpace(c.LastName)
	if first != "" || last != "" {
		return first, last
	}
	if c.Username != "" {
		return c.Username, ""
	}
	if at := strings.IndexByte(c.Email, '@'); at > 0 {
		return c.Email[:at], ""
	}
	return "Customer", ""
}

func displayName(c wcCustomer) string {
	first, last := resolveName(c)
	if last != "" {
		return strings.TrimSpace(first + " " + last)
	}
	return first
}

func hasAddress(a wcCustomerAddress) bool {
	return strings.TrimSpace(a.Address1) != "" || strings.TrimSpace(a.City) != "" || strings.TrimSpace(a.Postcode) != ""
}

func sameAddress(a, b wcCustomerAddress) bool {
	return strings.EqualFold(a.Address1, b.Address1) &&
		strings.EqualFold(a.Address2, b.Address2) &&
		strings.EqualFold(a.City, b.City) &&
		strings.EqualFold(a.Postcode, b.Postcode) &&
		strings.EqualFold(a.Country, b.Country)
}

// insertAddress inserts a billing / shipping row from a WC address payload.
// Uses the customer's first/last name as a fallback when the address itself
// has none (some WC stores collect the name only at the customer level).
func insertAddress(ctx context.Context, tx *sql.Tx, customerID string, a wcCustomerAddress, isDefault bool) error {
	first := strings.TrimSpace(a.FirstName)
	last := strings.TrimSpace(a.LastName)
	if first == "" && last == "" {
		// Pull names from the customer row we just inserted.
		_ = tx.QueryRowContext(ctx,
			`SELECT first_name, last_name FROM customers WHERE id=$1`, customerID).
			Scan(&first, &last)
	}

	country := strings.ToUpper(strings.TrimSpace(a.Country))
	if len(country) != 2 {
		country = "HK"
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		customerID,
		first, last,
		nullableString(a.Phone),
		strings.TrimSpace(a.Address1),
		nullableString(a.Address2),
		strings.TrimSpace(a.City),
		nullableString(a.State),
		strings.TrimSpace(a.Postcode),
		country,
		isDefault,
	)
	return err
}

func nullableString(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
