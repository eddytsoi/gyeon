package redirects

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrNotFound      = errors.New("redirect not found")
	ErrInvalidPath   = errors.New("from_path must start with /")
	ErrSelfRedirect  = errors.New("from_path must not equal to_path")
	ErrCycle         = errors.New("redirect would create a cycle")
	ErrDuplicateFrom = errors.New("another redirect already uses this from_path")
)

type Redirect struct {
	ID        string  `json:"id"`
	FromPath  string  `json:"from_path"`
	ToPath    string  `json:"to_path"`
	Code      int     `json:"code"`
	IsActive  bool    `json:"is_active"`
	Note      *string `json:"note,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Input struct {
	FromPath string  `json:"from_path"`
	ToPath   string  `json:"to_path"`
	Code     int     `json:"code"`
	IsActive bool    `json:"is_active"`
	Note     *string `json:"note"`
}

// AuditRecorder is the minimal interface this service needs from the audit
// package. Decoupled to avoid an import cycle and keep the service testable.
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

func (s *Service) record(ctx context.Context, action, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: "redirect", EntityID: entityID,
		Before: before, After: after,
	})
}

func normalize(p string) string { return strings.TrimSpace(p) }

func validate(in Input) error {
	from := normalize(in.FromPath)
	to := normalize(in.ToPath)
	if !strings.HasPrefix(from, "/") {
		return ErrInvalidPath
	}
	if from == to {
		return ErrSelfRedirect
	}
	if in.Code != 301 && in.Code != 302 {
		return errors.New("code must be 301 or 302")
	}
	return nil
}

// checkCycle rejects single-hop cycles: if to_path matches an existing active
// from_path, following the chain leads back into another redirect. We don't
// resolve full chains — keeping the rule "to_path must be a final destination."
func (s *Service) checkCycle(ctx context.Context, toPath string, excludeID string) error {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM redirects WHERE from_path = $1 AND is_active = TRUE AND ($2 = '' OR id::text <> $2))`,
		toPath, excludeID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrCycle
	}
	return nil
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Redirect, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redirects`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, created_at, updated_at
		   FROM redirects ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Redirect, 0)
	for rows.Next() {
		var r Redirect
		if err := rows.Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (*Redirect, error) {
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, created_at, updated_at
		   FROM redirects WHERE id = $1`, id).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

func (s *Service) Create(ctx context.Context, in Input) (*Redirect, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	from := normalize(in.FromPath)
	to := normalize(in.ToPath)
	if in.IsActive {
		if err := s.checkCycle(ctx, to, ""); err != nil {
			return nil, err
		}
	}
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO redirects (from_path, to_path, code, is_active, note)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, from_path, to_path, code, is_active, note, created_at, updated_at`,
		from, to, in.Code, in.IsActive, in.Note).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateFrom
		}
		return nil, err
	}
	s.record(ctx, "redirect.create", r.ID, nil, r)
	return &r, nil
}

func (s *Service) Update(ctx context.Context, id string, in Input) (*Redirect, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	from := normalize(in.FromPath)
	to := normalize(in.ToPath)
	if in.IsActive {
		if err := s.checkCycle(ctx, to, id); err != nil {
			return nil, err
		}
	}
	// Snapshot prior state for audit (best-effort; errors swallowed)
	var before *Redirect
	if s.audit != nil {
		if prev, err := s.Get(ctx, id); err == nil {
			before = prev
		}
	}
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`UPDATE redirects
		    SET from_path=$2, to_path=$3, code=$4, is_active=$5, note=$6, updated_at=NOW()
		  WHERE id=$1
		 RETURNING id, from_path, to_path, code, is_active, note, created_at, updated_at`,
		id, from, to, in.Code, in.IsActive, in.Note).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateFrom
		}
		return nil, err
	}
	s.record(ctx, "redirect.update", r.ID, before, r)
	return &r, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var before *Redirect
	if s.audit != nil {
		if prev, err := s.Get(ctx, id); err == nil {
			before = prev
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM redirects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.record(ctx, "redirect.delete", id, before, nil)
	return nil
}

// MatchActive returns the redirect target for an exact from_path lookup, only
// considering active rows. Returns ErrNotFound if no match.
func (s *Service) MatchActive(ctx context.Context, fromPath string) (*Redirect, error) {
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, created_at, updated_at
		   FROM redirects WHERE from_path = $1 AND is_active = TRUE`, fromPath).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

// Postgres unique-violation SQLSTATE 23505. We sniff the error text to avoid a
// dependency on the lib/pq error package — keeps the package import-free.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
