package customers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
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

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
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
		 RETURNING id, email, first_name, last_name, phone, is_active, created_at, updated_at`,
		req.Email, string(hash), req.FirstName, req.LastName, req.Phone).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*Customer, error) {
	var c Customer
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers WHERE email=$1 AND is_active=TRUE`, req.Email).
		Scan(&c.ID, &c.Email, &hash, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return &c, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*Customer, error) {
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers WHERE id=$1`, id).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*Customer, error) {
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers WHERE email=$1`, email).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Service) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*Customer, error) {
	var c Customer
	err := s.db.QueryRowContext(ctx,
		`UPDATE customers SET first_name=$2, last_name=$3, phone=$4
		 WHERE id=$1
		 RETURNING id, email, first_name, last_name, phone, is_active, created_at, updated_at`,
		id, req.FirstName, req.LastName, req.Phone).
		Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// customerSearchFields are matched by the optional admin `search` param on List.
// The trailing concatenated full-name expression makes "Mary Tam" match.
var customerSearchFields = []string{
	"email", "first_name", "last_name", "phone",
	"(first_name || ' ' || last_name)",
}

func (s *Service) List(ctx context.Context, search string, limit, offset int) ([]Customer, error) {
	args := []any{limit, offset}
	query := `SELECT id, email, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	if clause, arg := util.BuildSearchClause(search, customerSearchFields, 3); clause != "" {
		query = `SELECT id, email, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers WHERE ` + clause + ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = append(args, arg)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone,
			&c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
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
	return &a, tx.Commit()
}

func (s *Service) UpdateAddress(ctx context.Context, customerID, addressID string, req CreateAddressRequest) (*Address, error) {
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
	return &a, tx.Commit()
}

func (s *Service) DeleteAddress(ctx context.Context, customerID, addressID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM addresses WHERE id=$1 AND customer_id=$2`, addressID, customerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
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
		`SELECT id, email, password_hash, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers WHERE email=$1`, email).
		Scan(&c.ID, &c.Email, &pwHash, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx,
			`INSERT INTO customers (email, first_name, last_name, phone)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, email, first_name, last_name, phone, is_active, created_at, updated_at`,
			email, firstName, lastName, phone).
			Scan(&c.ID, &c.Email, &c.FirstName, &c.LastName, &c.Phone, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
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
		   LEFT JOIN order_items oi ON oi.order_id = o.id
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
