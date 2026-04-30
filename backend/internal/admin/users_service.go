package admin

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"gyeon/backend/internal/util"
)

// adminUserSearchFields are matched by the optional `search` param on List.
var adminUserSearchFields = []string{"email", "name"}

var ErrUserNotFound = errors.New("admin user not found")
var ErrEmailTaken = errors.New("email already registered")
var ErrInvalidCredentials = errors.New("invalid email or password")

type AdminUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateAdminUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type UpdateAdminUserRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
}

type AdminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) Login(ctx context.Context, req AdminLoginRequest) (*AdminUser, error) {
	var u AdminUser
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, role, is_active, created_at, updated_at
		 FROM admin_users WHERE email=$1 AND is_active=TRUE`, req.Email).
		Scan(&u.ID, &u.Email, &hash, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

func (s *UserService) List(ctx context.Context, search string) ([]AdminUser, error) {
	args := []any{}
	query := `SELECT id, email, name, role, is_active, created_at, updated_at
		 FROM admin_users ORDER BY created_at ASC`
	if clause, arg := util.BuildSearchClause(search, adminUserSearchFields, 1); clause != "" {
		query = `SELECT id, email, name, role, is_active, created_at, updated_at
		 FROM admin_users WHERE ` + clause + ` ORDER BY created_at ASC`
		args = append(args, arg)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role,
			&u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserService) Create(ctx context.Context, req CreateAdminUserRequest) (*AdminUser, error) {
	var exists bool
	s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_users WHERE email=$1)`, req.Email).Scan(&exists)
	if exists {
		return nil, ErrEmailTaken
	}

	if req.Role == "" {
		req.Role = "editor"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var u AdminUser
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO admin_users (email, password_hash, name, role)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, email, name, role, is_active, created_at, updated_at`,
		req.Email, string(hash), req.Name, req.Role).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserService) Update(ctx context.Context, id string, req UpdateAdminUserRequest) (*AdminUser, error) {
	var u AdminUser
	err := s.db.QueryRowContext(ctx,
		`UPDATE admin_users SET name=$2, role=$3, is_active=$4
		 WHERE id=$1
		 RETURNING id, email, name, role, is_active, created_at, updated_at`,
		id, req.Name, req.Role, req.IsActive).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM admin_users WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SeedSuperAdmin creates the first super_admin if no admin users exist.
func (s *UserService) SeedSuperAdmin(ctx context.Context, email, password string) error {
	var count int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO admin_users (email, password_hash, name, role) VALUES ($1,$2,$3,'super_admin')`,
		email, string(hash), "Super Admin")
	return err
}
