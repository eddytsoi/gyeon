package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"

	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/email"
)

// Setup-email modes for the customers import. The field on
// CustomersImportRequest carries one of these strings.
const (
	SetupEmailModeSkip         = "skip"         // never email
	SetupEmailModePasswordless = "passwordless" // email customers without a password yet, only if not already emailed
	SetupEmailModeForce        = "force"        // email every imported customer, regardless of state
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
	// CustomerID, when > 0, limits the run to a single WooCommerce customer
	// (fetched via /wc/v3/customers/{id}). Skips the page loop and the
	// CustomerTotal call entirely. Mutually exclusive with Limit — when
	// both are set CustomerID wins.
	CustomerID int `json:"customer_id"`
	// SetupEmailMode controls who receives the setup-password email:
	//   "skip"         — never send (silent import).
	//   "passwordless" — send iff password_hash IS NULL AND setup_email_sent_at IS NULL.
	//                    A row imported earlier under "skip" therefore still becomes
	//                    eligible the first time the admin runs "passwordless".
	//   "force"        — send to every imported customer. Useful for re-onboarding
	//                    campaigns or post-incident resets.
	// Empty / unrecognized values fall back to "skip" (safest default).
	SetupEmailMode string `json:"setup_email_mode"`
}

// CustomersProgressUpdate is streamed once per processed customer plus a
// final Done frame. Mirrors the shape of ProgressUpdate so the frontend's
// SSE plumbing can be largely re-used.
type CustomersProgressUpdate struct {
	TotalCustomers     int      `json:"total_customers"`
	ProcessedCustomers int      `json:"processed_customers"`
	ImportedCustomers  int      `json:"imported_customers"`  // newly inserted
	UpdatedCustomers   int      `json:"updated_customers"`   // matched (by wc_customer_id or email), updated in place
	ImportedAddresses  int      `json:"imported_addresses"`  // billing + shipping rows added on first import
	SetupEmailsQueued  int      `json:"setup_emails_queued"` // setup-password emails kicked off (async; failures logged server-side)
	SkippedFakeCustomers   int  `json:"skipped_fake"`        // suspected-fake accounts not imported (and not emailed)
	RemovedFakeCustomers   int  `json:"removed_fake"`        // previously-imported fakes deleted from the Gyeon DB
	ProtectedFakeCustomers int  `json:"protected_fake"`      // fakes that matched an existing row but were kept (had orders/password/login)
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
// each customer. The final call always has Done = true. When req.CustomerID
// is set, the page loop is skipped and just that one customer is fetched
// + upserted.
func (s *Service) RunCustomersStreaming(ctx context.Context, req CustomersImportRequest, send func(CustomersProgressUpdate)) {
	wc := newWCClient(req.WCURL, req.WCKey, req.WCSecret)
	p := CustomersProgressUpdate{Errors: []string{}}

	if req.CustomerID > 0 {
		// Single-customer run: denominator is always 1, no count call.
		p.TotalCustomers = 1
	} else {
		p.TotalCustomers = wc.fetchCustomerTotal()
		if req.Limit > 0 && (p.TotalCustomers == 0 || p.TotalCustomers > req.Limit) {
			p.TotalCustomers = req.Limit
		}
	}
	send(p)

	// Counter for setup-password emails kicked off via the goroutines below.
	// Atomic because the dispatch is async; we read it from the import loop
	// to push the latest value out via the SSE frame.
	var emailsQueued int64

	// processOne owns the per-customer work — email gate, upsert, setup-email
	// dispatch, progress counters. Shared between the single-id branch and
	// the paginated loop so the two paths stay byte-for-byte identical from
	// the SSE consumer's point of view.
	processOne := func(c wcCustomer) {
		if strings.TrimSpace(c.Email) == "" {
			p.Failed++
			p.Errors = append(p.Errors, fmt.Sprintf("customer id=%d skipped: missing email", c.ID))
			p.ProcessedCustomers++
			send(p)
			return
		}

		// Fake-account gate. Runs for every email mode (skip/passwordless/force)
		// and before the upsert, so a suspected bot signup is never imported and
		// never emailed. If it was imported in a prior run, purgeFakeCustomer
		// deletes it — unless it has since gained real engagement (orders, a
		// password, or a linked login), in which case it's kept untouched.
		if looksLikeFakeAccount(c) {
			p.SkippedFakeCustomers++
			removed, protectedReason, err := s.purgeFakeCustomer(ctx, c)
			switch {
			case err != nil:
				p.Errors = append(p.Errors, fmt.Sprintf("purge fake %s: %v", c.Email, err))
			case removed:
				p.RemovedFakeCustomers++
			case protectedReason != "":
				p.ProtectedFakeCustomers++
				p.Errors = append(p.Errors, fmt.Sprintf("kept %s (suspected fake but %s)", c.Email, protectedReason))
			}
			p.ProcessedCustomers++
			send(p)
			return
		}

		p.CurrentCustomer = displayName(c)
		send(p)

		customerID, err := s.upsertCustomer(ctx, c, &p)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("customer %s: %v", c.Email, err))
			p.Failed++
		} else if customerID != "" && s.shouldSendSetupEmail(ctx, customerID, req.SetupEmailMode) {
			if _, uerr := s.db.ExecContext(ctx,
				`UPDATE customers SET setup_email_sent_at = NOW() WHERE id=$1`, customerID); uerr != nil {
				p.Errors = append(p.Errors, fmt.Sprintf("mark setup email %s: %v", c.Email, uerr))
			} else {
				go s.sendSetupPasswordEmail(c, customerID, &emailsQueued)
			}
		}
		p.SetupEmailsQueued = int(atomic.LoadInt64(&emailsQueued))
		p.ProcessedCustomers++
		p.CurrentCustomer = ""
		send(p)
	}

	if req.CustomerID > 0 {
		cust, err := wc.fetchCustomer(req.CustomerID)
		if err != nil {
			p.Errors = append(p.Errors, fmt.Sprintf("fetch customer %d: %v", req.CustomerID, err))
		} else {
			processOne(cust)
		}
	} else {
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
				processOne(c)
			}
		}
	}

	// Final flush of the emails counter — late dispatches that happened
	// after the loop exited are still reflected.
	p.SetupEmailsQueued = int(atomic.LoadInt64(&emailsQueued))
	p.Done = true
	send(p)
}

// sendSetupPasswordEmail mints a 7-day setup token via the customers
// service and sends the account-setup email (set a password, or sign in
// with Google/Apple using the same email). Caller has
// already gated on SetupEmailMode and stamped customers.setup_email_sent_at,
// so this function just delivers. Errors are logged but never surface to
// the importer caller — best-effort delivery.
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
	if err := s.emailSvc.SendAccountSetupEmail(ctx, email.PasswordResetParams{
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

// shouldSendSetupEmail decides whether the loop should fire a setup-password
// email for the given customer. "skip" never sends. "force" always sends.
// "passwordless" sends only when the customer has no password AND has not
// already been emailed in a prior run.
func (s *Service) shouldSendSetupEmail(ctx context.Context, customerID, mode string) bool {
	switch mode {
	case SetupEmailModeForce:
		return true
	case SetupEmailModePasswordless:
		var passwordSet, alreadyEmailed bool
		err := s.db.QueryRowContext(ctx,
			`SELECT (password_hash IS NOT NULL AND password_hash != ''),
			        (setup_email_sent_at IS NOT NULL)
			   FROM customers WHERE id=$1`, customerID).Scan(&passwordSet, &alreadyEmailed)
		if err != nil {
			return false
		}
		return !passwordSet && !alreadyEmailed
	default:
		return false
	}
}

// looksLikeFakeAccount classifies a WC customer as a suspected bot/spam signup.
// The gyeon.hk WooCommerce store carries thousands of these — accounts that
// registered but never bought anything and left every contact field blank
// (validated against a 500-row live sample: ~61% fake, 0 paying-customer false
// positives, 91% of fakes carrying random gibberish usernames).
//
// The rule is deliberately conservative — a customer is fake only when ALL of:
//   - is_paying_customer is false (never completed a paid order), AND
//   - both first and last name are empty, AND
//   - the billing phone is empty.
//
// Any single real-looking signal (a paid order, a name, or a phone) flips the
// verdict to real, so a genuine customer is never skipped or deleted.
func looksLikeFakeAccount(c wcCustomer) bool {
	if c.IsPayingCustomer {
		return false
	}
	if strings.TrimSpace(c.FirstName) != "" || strings.TrimSpace(c.LastName) != "" {
		return false
	}
	if strings.TrimSpace(c.Billing.Phone) != "" {
		return false
	}
	return true
}

// purgeFakeCustomer removes a suspected-fake customer that was imported in a
// prior run. It locates the existing Gyeon row (by wc_customer_id, then email)
// and deletes it together with its child rows — but only when the row shows no
// real engagement. Returns:
//   - removed=true               when the row was deleted,
//   - protectedReason="..."      when a matching row was found but kept (the
//     reason names the engagement signal that protected it),
//   - removed=false, reason=""   when no matching row existed (nothing to do).
//
// Guards (any one keeps the row): it has at least one order, a password set, or
// a linked OAuth identity. The orders guard is the critical one — a fake-by-WC
// account can still have a pending bank-transfer order in Gyeon
// (is_paying_customer stays false until paid), and orders.customer_id is
// ON DELETE SET NULL, so a blind delete would orphan real order history.
func (s *Service) purgeFakeCustomer(ctx context.Context, wc wcCustomer) (removed bool, protectedReason string, err error) {
	var customerID string
	var passwordSet bool
	row := s.db.QueryRowContext(ctx,
		`SELECT id, (password_hash IS NOT NULL AND password_hash != '')
		   FROM customers WHERE wc_customer_id=$1`, wc.ID)
	if scanErr := row.Scan(&customerID, &passwordSet); errors.Is(scanErr, sql.ErrNoRows) {
		// Fall back to email — a row imported before wc_customer_id was backfilled,
		// or one created storefront-side, still needs to be matched.
		if scanErr = s.db.QueryRowContext(ctx,
			`SELECT id, (password_hash IS NOT NULL AND password_hash != '')
			   FROM customers WHERE email=$1`, wc.Email).Scan(&customerID, &passwordSet); errors.Is(scanErr, sql.ErrNoRows) {
			return false, "", nil // not imported — nothing to remove
		} else if scanErr != nil {
			return false, "", fmt.Errorf("lookup by email: %w", scanErr)
		}
	} else if scanErr != nil {
		return false, "", fmt.Errorf("lookup by wc_customer_id: %w", scanErr)
	}

	if passwordSet {
		return false, "has a password", nil
	}

	var orderCount int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM orders WHERE customer_id=$1`, customerID).Scan(&orderCount); err != nil {
		return false, "", fmt.Errorf("count orders: %w", err)
	}
	if orderCount > 0 {
		return false, "has orders", nil
	}

	var oauthCount int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM customer_oauth_identities WHERE customer_id=$1`, customerID).Scan(&oauthCount); err != nil {
		return false, "", fmt.Errorf("count oauth identities: %w", err)
	}
	if oauthCount > 0 {
		return false, "has a linked login", nil
	}

	// Safe to delete. addresses and carts are ON DELETE SET NULL, so remove
	// them explicitly to avoid leaving orphan junk rows; the remaining
	// dependents (wishlist, loyalty, saved payment methods, setup tokens,
	// oauth — none here) are ON DELETE CASCADE and go with the customers row.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM addresses WHERE customer_id=$1`, customerID); err != nil {
		return false, "", fmt.Errorf("delete addresses: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM carts WHERE customer_id=$1`, customerID); err != nil {
		return false, "", fmt.Errorf("delete carts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM customers WHERE id=$1`, customerID); err != nil {
		return false, "", fmt.Errorf("delete customer: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, "", err
	}
	return true, "", nil
}

// upsertCustomer inserts a new customer (with billing + shipping addresses)
// or, if a row already exists either by wc_customer_id or email, updates the
// names / phone in place. Existing rows keep their addresses untouched —
// re-running the import never overwrites address edits an admin (or the
// customer themself, post-import) made on storefront.
//
// Returns the local customer.id whether the row was newly inserted or
// matched against an existing one. The setup-email gate is decided
// downstream by shouldSendSetupEmail (which checks password_hash and
// setup_email_sent_at), not by insert-vs-update here.
func (s *Service) upsertCustomer(ctx context.Context, wc wcCustomer, p *CustomersProgressUpdate) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	first, last := resolveName(wc)
	phone := nullableString(wc.Billing.Phone)
	// WC role "installer" maps to Gyeon "installer"; WC "installer_v2"
	// (or the legacy condensed "installerv2") maps to the distinct
	// "installer_v2" tier; everything else is a regular customer.
	// See customers.NormalizeRole for the canonical mapping.
	role := customers.NormalizeRole(wc.Role)

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
			`UPDATE customers SET first_name=$2, last_name=$3, phone=$4, role=$5::customer_role WHERE id=$1`,
			customerID, first, last, phone, role); err != nil {
			return "", fmt.Errorf("update customer: %w", err)
		}
		p.UpdatedCustomers++
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return customerID, nil
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name, phone, wc_customer_id, role)
		 VALUES ($1, $2, $3, $4, $5, $6::customer_role)
		 RETURNING id`,
		wc.Email, first, last, phone, wc.ID, role).Scan(&customerID); err != nil {
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

	_, err := customers.FindOrCreateAddress(ctx, tx, &customerID, customers.AddressFields{
		FirstName:  first,
		LastName:   last,
		Phone:      nullableString(a.Phone),
		Line1:      strings.TrimSpace(a.Address1),
		Line2:      nullableString(a.Address2),
		City:       strings.TrimSpace(a.City),
		State:      nullableString(a.State),
		PostalCode: strings.TrimSpace(a.Postcode),
		Country:    country,
	}, isDefault)
	return err
}

func nullableString(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}
