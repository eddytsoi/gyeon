package customers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gyeon/backend/internal/util"
)

var ErrNotFound = errors.New("customer not found")
var ErrEmailTaken = errors.New("email already registered")
var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrInvalidToken = errors.New("invalid or expired token")

type Customer struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     *string `json:"phone,omitempty"`
	IsActive  bool    `json:"is_active"`
	Role      string  `json:"role"` // customer_role enum: 'customer' | 'installer'
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// RoleCustomer is the storefront default and what anonymous visitors are
// treated as. RoleInstaller is the elevated tier admins can grant.
const (
	RoleCustomer  = "customer"
	RoleInstaller = "installer"
)

// NormalizeRole maps any incoming role string (admin form value, WC role, …)
// to a canonical enum value. Unknown values fall through to "customer" so a
// typo never escalates privileges or breaks a column NOT NULL constraint.
func NormalizeRole(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case RoleInstaller, "installerv2":
		return RoleInstaller
	default:
		return RoleCustomer
	}
}

type Address struct {
	ID         string  `json:"id"`
	CustomerID *string `json:"customer_id,omitempty"`
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Phone      *string `json:"phone,omitempty"`
	Line1      string  `json:"line1"`
	Line2      *string `json:"line2,omitempty"`
	City       string  `json:"city"`
	State      *string `json:"state,omitempty"`
	PostalCode string  `json:"postal_code"`
	Country    string  `json:"country"`
	IsDefault  bool    `json:"is_default"`
	CreatedAt  string  `json:"created_at"`
}

type RegisterRequest struct {
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     *string `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     *string `json:"phone"`
}

type CreateAddressRequest struct {
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Phone      *string `json:"phone"`
	Line1      string  `json:"line1"`
	Line2      *string `json:"line2"`
	City       string  `json:"city"`
	State      *string `json:"state"`
	PostalCode string  `json:"postal_code"`
	Country    string  `json:"country"`
	IsDefault  bool    `json:"is_default"`
}

// AuditRecorder is the minimal interface this service needs from the audit
// package. Decoupled to avoid an import cycle.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry)
}

type AuditEntry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

type Service struct {
	db    *sql.DB
	audit AuditRecorder
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetAudit wires an optional audit recorder. Call from main during setup.
func (s *Service) SetAudit(rec AuditRecorder) { s.audit = rec }

func (s *Service) record(ctx context.Context, action, entityType, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: entityType, EntityID: entityID,
		Before: before, After: after,
	})
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*Customer, error) {
	var exists bool
	s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM customers WHERE email=$1)`, req.Email).Scan(&exists)
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var c Customer
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO customers (email, password_hash, first_name, last_name, phone)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at`,
		req.Email, string(hash), req.FirstName, req.LastName, req.Phone).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*Customer, error) {
	var c Customer
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, first_name, last_name, phone, is_active, role::text, created_at, updated_at
		 FROM customers WHERE email=$1 AND is_active=TRUE`, req.Email).
		Scan(&c.ID, &c.Email, &hash, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	// Customers imported from WooCommerce (or guest-checkout rows that never
	// finished setup) have password_hash IS NULL. Treat that the same as a
	// wrong-password attempt so we don't leak whether a row exists, and the
	// customer gets pushed toward the "Forgot password?" link.
	if !hash.Valid || hash.String == "" {
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash.String), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return &c, nil
}

// FindOrCreateByOAuth resolves the customer behind a social login. Resolution
// order, all in one transaction:
//  1. an existing (provider, subject) identity → returning user;
//  2. an existing customer with the same email → auto-link a new identity
//     (the provider has verified the email, so this safely merges WooCommerce
//     imports and password accounts into one login);
//  3. otherwise create a fresh customer and identity.
//
// email must be the provider's verified email (caller's responsibility).
func (s *Service) FindOrCreateByOAuth(ctx context.Context, provider, subject, email, firstName, lastName string) (*Customer, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if subject == "" || email == "" {
		return nil, errors.New("oauth: missing subject or email")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const cols = `id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at`
	scan := func(row interface{ Scan(...any) error }, c *Customer) error {
		return row.Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	}

	// 1. Returning user — match by provider identity. Columns are qualified
	// because customer_oauth_identities also has an `email` column.
	const colsC = `c.id, c.email, c.first_name, c.last_name, c.phone, c.is_active, c.role::text, c.created_at, c.updated_at`
	var c Customer
	err = scan(tx.QueryRowContext(ctx,
		`SELECT `+colsC+` FROM customers c
		   JOIN customer_oauth_identities i ON i.customer_id = c.id
		  WHERE i.provider=$1 AND i.subject=$2`, provider, subject), &c)
	if err == nil {
		return &c, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// 2. Existing account with the same email — auto-link.
	err = scan(tx.QueryRowContext(ctx,
		`SELECT `+cols+` FROM customers WHERE email=$1`, email), &c)
	if err == nil {
		if _, ierr := tx.ExecContext(ctx,
			`INSERT INTO customer_oauth_identities (customer_id, provider, subject, email)
			 VALUES ($1, $2, $3, $4)`, c.ID, provider, subject, email); ierr != nil {
			return nil, ierr
		}
		return &c, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// 3. Brand-new customer (no password — social login only until they set one).
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if first == "" && last == "" {
		first = emailLocalPart(email)
	}
	err = scan(tx.QueryRowContext(ctx,
		`INSERT INTO customers (email, first_name, last_name)
		 VALUES ($1, $2, $3)
		 RETURNING `+cols, email, first, last), &c)
	if err != nil {
		return nil, err
	}
	if _, ierr := tx.ExecContext(ctx,
		`INSERT INTO customer_oauth_identities (customer_id, provider, subject, email)
		 VALUES ($1, $2, $3, $4)`, c.ID, provider, subject, email); ierr != nil {
		return nil, ierr
	}
	return &c, tx.Commit()
}

func emailLocalPart(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

// TokenVersion returns the current JWT revocation counter for the customer.
// Issued tokens carry this value in a `tv` claim; the middleware rejects
// tokens whose claim doesn't match the live value here.
func (s *Service) TokenVersion(ctx context.Context, customerID string) (int, error) {
	var tv int
	err := s.db.QueryRowContext(ctx,
		`SELECT token_version FROM customers WHERE id=$1`, customerID).Scan(&tv)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return tv, err
}

// IncrementTokenVersion bumps the counter, instantly invalidating every
// previously-issued JWT for this customer. Used by sign-out-everywhere.
// Returns the new value so the caller can mint a fresh token if it wants
// to keep the current session alive.
func (s *Service) IncrementTokenVersion(ctx context.Context, customerID string) (int, error) {
	var tv int
	err := s.db.QueryRowContext(ctx,
		`UPDATE customers SET token_version = token_version + 1 WHERE id=$1 RETURNING token_version`,
		customerID).Scan(&tv)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return tv, err
}

func (s *Service) GetByID(ctx context.Context, id string) (*Customer, error) {
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at
		 FROM customers WHERE id=$1`, id).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// GetRole returns just the role for a customer ID. Used by the storefront
// product / cart queries to apply per-role visibility & purchase rules
// without pulling the full Customer row on every request. Returns
// RoleCustomer on lookup failure (anonymous / deleted customers) so the
// caller never has to special-case errors.
func (s *Service) GetRole(ctx context.Context, customerID string) string {
	if customerID == "" {
		return RoleCustomer
	}
	var role string
	if err := s.db.QueryRowContext(ctx,
		`SELECT role::text FROM customers WHERE id=$1`, customerID).Scan(&role); err != nil {
		return RoleCustomer
	}
	return NormalizeRole(role)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*Customer, error) {
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at
		 FROM customers WHERE email=$1`, email).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Service) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*Customer, error) {
	var before *Customer
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id); err == nil {
			before = prev
		}
	}
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`UPDATE customers SET first_name=$2, last_name=$3, phone=$4
		 WHERE id=$1
		 RETURNING id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at`,
		id, req.FirstName, req.LastName, req.Phone).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.record(ctx, "customer.update_profile", "customer", c.ID, before, c)
	return &c, nil
}

// UpdateRole sets the customer's role to the given value. Caller is expected
// to have passed the new role through NormalizeRole so invalid input is
// rejected at the API boundary rather than caught by the column's enum
// constraint here. Audited under a distinct action so role escalations stand
// out in the log.
func (s *Service) UpdateRole(ctx context.Context, id, role string) (*Customer, error) {
	role = NormalizeRole(role)
	var before *Customer
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id); err == nil {
			before = prev
		}
	}
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`UPDATE customers SET role = $2::customer_role
		 WHERE id = $1
		 RETURNING id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at`,
		id, role).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.record(ctx, "customer.update_role", "customer", c.ID, before, c)
	return &c, nil
}

// customerSearchFields are matched by the optional admin `search` param on List.
// The trailing concatenated full-name expression makes "Mary Tam" match.
var customerSearchFields = []string{
	"email", "first_name", "last_name", "phone",
	"(first_name || ' ' || last_name)",
}

// ListFilters captures the optional admin-list filters applied on top of
// the free-text search. Empty fields mean "no filter on this dimension".
type ListFilters struct {
	// Active filters by is_active. "active" → is_active=true,
	// "inactive" → is_active=false, "" → no filter.
	Active string
	// Role filters by the customer_role enum. "" → no filter.
	Role string
}

// buildListWhere assembles the WHERE clause and positional args for List.
// Placeholders are numbered starting at startIdx so the caller can reserve
// earlier slots for LIMIT/OFFSET.
func buildListWhere(search string, filters ListFilters, startIdx int) (string, []any) {
	var parts []string
	var args []any
	idx := startIdx
	if clause, arg := util.BuildSearchClause(search, customerSearchFields, idx); clause != "" {
		parts = append(parts, clause)
		args = append(args, arg)
		idx++
	}
	if filters.Active == "active" || filters.Active == "inactive" {
		parts = append(parts, fmt.Sprintf("is_active = $%d", idx))
		args = append(args, filters.Active == "active")
		idx++
	}
	if filters.Role != "" {
		parts = append(parts, fmt.Sprintf("role = $%d::customer_role", idx))
		args = append(args, NormalizeRole(filters.Role))
		idx++
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func (s *Service) List(ctx context.Context, search string, filters ListFilters, limit, offset int) ([]Customer, int, error) {
	countWhere, countArgs := buildListWhere(search, filters, 1)
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM customers`+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// SELECT reserves $1=limit, $2=offset and renumbers filter placeholders
	// from $3 onwards. countArgs and selectArgs hold the same values; only
	// the placeholder numbering inside the SQL differs.
	selectWhere, selectArgs := buildListWhere(search, filters, 3)
	query := `SELECT id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at
		 FROM customers` + selectWhere + ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	args := append([]any{limit, offset}, selectArgs...)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone,
			&c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		customers = append(customers, c)
	}
	return customers, total, rows.Err()
}

func (s *Service) ListAddresses(ctx context.Context, customerID string) ([]Address, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default, created_at
		 FROM addresses WHERE customer_id=$1 ORDER BY is_default DESC, created_at ASC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addrs := make([]Address, 0)
	for rows.Next() {
		var a Address
		if err := rows.Scan(&a.ID, &a.CustomerID, &a.FirstName, &a.LastName, &a.Phone,
			&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.Country,
			&a.IsDefault, &a.CreatedAt); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

func (s *Service) CreateAddress(ctx context.Context, customerID string, req CreateAddressRequest) (*Address, error) {
	if req.Country == "" {
		req.Country = "HK"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if req.IsDefault {
		tx.ExecContext(ctx, `UPDATE addresses SET is_default=FALSE WHERE customer_id=$1`, customerID)
	}

	var a Address
	err = tx.QueryRowContext(ctx,
		`INSERT INTO addresses (customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING id, customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default, created_at`,
		customerID, req.FirstName, req.LastName, req.Phone, req.Line1, req.Line2,
		req.City, req.State, req.PostalCode, req.Country, req.IsDefault).
		Scan(&a.ID, &a.CustomerID, &a.FirstName, &a.LastName, &a.Phone,
			&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.Country,
			&a.IsDefault, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.record(ctx, "customer.address.create", "customer_address", a.ID, nil, a)
	return &a, nil
}

// getAddress returns a single address scoped to its owning customer. Used as
// a before-snapshot for audit on update/delete; ErrNoRows is swallowed by
// callers via the audit nil-check pattern.
func (s *Service) getAddress(ctx context.Context, customerID, addressID string) (*Address, error) {
	var a Address
	err := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default, created_at
		 FROM addresses WHERE id=$1 AND customer_id=$2`, addressID, customerID).
		Scan(&a.ID, &a.CustomerID, &a.FirstName, &a.LastName, &a.Phone,
			&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.Country,
			&a.IsDefault, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &a, err
}

func (s *Service) UpdateAddress(ctx context.Context, customerID, addressID string, req CreateAddressRequest) (*Address, error) {
	var before *Address
	if s.audit != nil {
		if prev, err := s.getAddress(ctx, customerID, addressID); err == nil {
			before = prev
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if req.IsDefault {
		tx.ExecContext(ctx, `UPDATE addresses SET is_default=FALSE WHERE customer_id=$1`, customerID)
	}

	var a Address
	err = tx.QueryRowContext(ctx,
		`UPDATE addresses SET first_name=$3, last_name=$4, phone=$5, line1=$6, line2=$7,
		 city=$8, state=$9, postal_code=$10, country=$11, is_default=$12
		 WHERE id=$1 AND customer_id=$2
		 RETURNING id, customer_id, first_name, last_name, phone, line1, line2, city, state, postal_code, country, is_default, created_at`,
		addressID, customerID, req.FirstName, req.LastName, req.Phone, req.Line1, req.Line2,
		req.City, req.State, req.PostalCode, req.Country, req.IsDefault).
		Scan(&a.ID, &a.CustomerID, &a.FirstName, &a.LastName, &a.Phone,
			&a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.Country,
			&a.IsDefault, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.record(ctx, "customer.address.update", "customer_address", a.ID, before, a)
	return &a, nil
}

func (s *Service) DeleteAddress(ctx context.Context, customerID, addressID string) error {
	var before *Address
	if s.audit != nil {
		if prev, err := s.getAddress(ctx, customerID, addressID); err == nil {
			before = prev
		}
	}
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM addresses WHERE id=$1 AND customer_id=$2`, addressID, customerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	s.record(ctx, "customer.address.delete", "customer_address", addressID, before, nil)
	return nil
}

// UpsertGuest finds-or-creates a customer row by email for guest checkouts.
// Returns isGuest=true when the row currently has no password_hash (i.e. has
// never registered) — caller can decide to send a setup-password link.
func (s *Service) UpsertGuest(ctx context.Context, email, firstName, lastName string, phone *string) (*Customer, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	var c Customer
	var pwHash sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, email, password_hash, first_name, last_name, phone, is_active, role::text, created_at, updated_at
		 FROM customers WHERE email=$1`, email).
		Scan(&c.ID, &c.Email, &pwHash, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx,
			`INSERT INTO customers (email, first_name, last_name, phone)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, email, first_name, last_name, phone, is_active, role::text, created_at, updated_at`,
			email, firstName, lastName, phone).
			Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.Role, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, false, err
		}
		return &c, true, tx.Commit()
	}
	if err != nil {
		return nil, false, err
	}

	isGuest := !pwHash.Valid || pwHash.String == ""
	return &c, isGuest, tx.Commit()
}

// CreateSetupToken generates a one-time URL-safe token (64 hex chars) tied to
// a customer; expires after 7 days. Returns the raw token (caller embeds in URL).
func (s *Service) CreateSetupToken(ctx context.Context, customerID string) (string, error) {
	return s.issueAccountToken(ctx, customerID, 7*24*time.Hour)
}

// IssuePasswordResetToken issues a one-time token (same table as setup tokens,
// same consumption flow) with a shorter 24-hour expiry — used when admin
// triggers a password reset email for an existing customer.
func (s *Service) IssuePasswordResetToken(ctx context.Context, customerID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	token, err := s.issueAccountToken(ctx, customerID, 24*time.Hour)
	if err != nil {
		return "", time.Time{}, err
	}
	s.record(ctx, "customer.password_reset_token.issue", "customer", customerID,
		nil, map[string]any{"customer_id": customerID, "expires_at": expiresAt})
	return token, expiresAt, nil
}

func (s *Service) issueAccountToken(ctx context.Context, customerID string, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	expiresAt := time.Now().Add(ttl)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO account_setup_tokens (token, customer_id, expires_at)
		 VALUES ($1, $2, $3)`, token, customerID, expiresAt)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeSetupToken validates the token, hashes the new password, sets it on
// the customer, and marks the token consumed — all in one transaction.
func (s *Service) ConsumeSetupToken(ctx context.Context, token, password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var customerID string
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT customer_id, expires_at, consumed_at
		 FROM account_setup_tokens WHERE token=$1`, token).
		Scan(&customerID, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if consumedAt.Valid {
		return ErrInvalidToken
	}
	if time.Now().After(expiresAt) {
		return ErrInvalidToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE customers SET password_hash=$2 WHERE id=$1`, customerID, string(hash)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE account_setup_tokens SET consumed_at=NOW() WHERE token=$1`, token); err != nil {
		return err
	}

	return tx.Commit()
}

type OrderSummary struct {
	ID         string  `json:"id"`
	Number     int64   `json:"number"`
	Status     string  `json:"status"`
	Total      float64 `json:"total"`
	CreatedAt  string  `json:"created_at"`
	ItemsCount int64   `json:"items_count"`
}

func (s *Service) ListOrders(ctx context.Context, customerID string, limit, offset int) ([]OrderSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.id, o.number, o.status, o.total, o.created_at,
		        COALESCE(SUM(oi.quantity), 0)::bigint AS items_count
		   FROM orders o
		   LEFT JOIN order_items oi
		     ON oi.order_id = o.id AND oi.parent_item_id IS NULL
		  WHERE o.customer_id=$1
		  GROUP BY o.id
		  ORDER BY o.created_at DESC
		  LIMIT $2 OFFSET $3`,
		customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]OrderSummary, 0)
	for rows.Next() {
		var o OrderSummary
		if err := rows.Scan(&o.ID, &o.Number, &o.Status, &o.Total, &o.CreatedAt, &o.ItemsCount); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetOrderIDByNumber resolves a sequential order display number to the
// underlying UUID, scoped to the given customer. Returns sql.ErrNoRows if
// the number does not exist or belongs to another customer.
func (s *Service) GetOrderIDByNumber(ctx context.Context, customerID string, n int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE number=$1 AND customer_id=$2`,
		n, customerID).Scan(&id)
	return id, err
}
