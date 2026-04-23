package customers

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrNotFound = errors.New("customer not found")
var ErrEmailTaken = errors.New("email already registered")
var ErrInvalidCredentials = errors.New("invalid email or password")

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

func (s *Service) List(ctx context.Context, limit, offset int) ([]Customer, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email, first_name, last_name, phone, is_active, created_at, updated_at
		 FROM customers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
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

type OrderSummary struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Total     float64 `json:"total"`
	CreatedAt string  `json:"created_at"`
}

func (s *Service) ListOrders(ctx context.Context, customerID string, limit, offset int) ([]OrderSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, status, total, created_at FROM orders
		 WHERE customer_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]OrderSummary, 0)
	for rows.Next() {
		var o OrderSummary
		if err := rows.Scan(&o.ID, &o.Status, &o.Total, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
