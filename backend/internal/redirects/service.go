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
	ErrInvalidWild   = errors.New("wildcard from_path must end with /* and have no other *; to_path may end with /* or have no *")
)

const (
	MatchExact    = "exact"
	MatchWildcard = "wildcard"
)

type Redirect struct {
	ID        string  `json:"id"`
	FromPath  string  `json:"from_path"`
	ToPath    string  `json:"to_path"`
	Code      int     `json:"code"`
	IsActive  bool    `json:"is_active"`
	Note      *string `json:"note,omitempty"`
	MatchType string  `json:"match_type"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Input struct {
	FromPath  string  `json:"from_path"`
	ToPath    string  `json:"to_path"`
	Code      int     `json:"code"`
	IsActive  bool    `json:"is_active"`
	Note      *string `json:"note"`
	MatchType string  `json:"match_type"`
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

func normalizeMatchType(t string) string {
	t = strings.TrimSpace(t)
	if t == MatchWildcard {
		return MatchWildcard
	}
	return MatchExact
}

func validate(in Input) error {
	from := normalize(in.FromPath)
	to := normalize(in.ToPath)
	mt := normalizeMatchType(in.MatchType)
	if !strings.HasPrefix(from, "/") {
		return ErrInvalidPath
	}
	if from == to {
		return ErrSelfRedirect
	}
	if in.Code != 301 && in.Code != 302 {
		return errors.New("code must be 301 or 302")
	}
	switch mt {
	case MatchExact:
		if strings.Contains(from, "*") || strings.Contains(to, "*") {
			return ErrInvalidWild
		}
	case MatchWildcard:
		// from_path must end with /*, and that's the only * allowed.
		if !strings.HasSuffix(from, "/*") || strings.Count(from, "*") != 1 {
			return ErrInvalidWild
		}
		// to_path either ends with /* (suffix substitution) or contains no *.
		if strings.Contains(to, "*") && (!strings.HasSuffix(to, "/*") || strings.Count(to, "*") != 1) {
			return ErrInvalidWild
		}
	}
	return nil
}

// checkCycle rejects single-hop cycles: if to_path matches an existing active
// from_path, following the chain leads back into another redirect. We don't
// resolve full chains — keeping the rule "to_path must be a final destination."
// Skipped for wildcard rows because suffix substitution makes a literal match
// against another from_path structurally unlikely.
func (s *Service) checkCycle(ctx context.Context, toPath string, excludeID string) error {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM redirects WHERE from_path = $1 AND match_type = 'exact' AND is_active = TRUE AND ($2 = '' OR id::text <> $2))`,
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
		`SELECT id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at
		   FROM redirects ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Redirect, 0)
	for rows.Next() {
		var r Redirect
		if err := rows.Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (*Redirect, error) {
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at
		   FROM redirects WHERE id = $1`, id).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt)
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
	mt := normalizeMatchType(in.MatchType)
	if in.IsActive && mt == MatchExact {
		if err := s.checkCycle(ctx, to, ""); err != nil {
			return nil, err
		}
	}
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO redirects (from_path, to_path, code, is_active, note, match_type)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at`,
		from, to, in.Code, in.IsActive, in.Note, mt).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt)
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
	mt := normalizeMatchType(in.MatchType)
	if in.IsActive && mt == MatchExact {
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
		    SET from_path=$2, to_path=$3, code=$4, is_active=$5, note=$6, match_type=$7, updated_at=NOW()
		  WHERE id=$1
		 RETURNING id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at`,
		id, from, to, in.Code, in.IsActive, in.Note, mt).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt)
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

// MatchActive returns the redirect for an incoming path. Exact matches win
// over wildcards; among wildcards, the longest from_path wins. The returned
// Redirect has ToPath already resolved (wildcard suffix substituted) so the
// caller can serve it verbatim.
func (s *Service) MatchActive(ctx context.Context, fromPath string) (*Redirect, error) {
	var r Redirect
	err := s.db.QueryRowContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at
		   FROM redirects
		  WHERE from_path = $1 AND match_type = 'exact' AND is_active = TRUE`, fromPath).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt)
	if err == nil {
		return &r, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Wildcard pass: pick the most specific (longest from_path) active wildcard
	// whose stripped prefix matches the incoming path. We strip the literal
	// trailing "/*" (validated at write time) and require either an exact
	// prefix-only hit or that the next char of the incoming path is "/", so
	// /product-category-x does NOT match /product-category/*.
	err = s.db.QueryRowContext(ctx,
		`SELECT id, from_path, to_path, code, is_active, note, match_type, created_at, updated_at
		   FROM redirects
		  WHERE match_type = 'wildcard'
		    AND is_active  = TRUE
		    AND from_path LIKE '%/*'
		    AND (
		         $1 = substr(from_path, 1, length(from_path) - 2)
		      OR $1 LIKE substr(from_path, 1, length(from_path) - 2) || '/%'
		    )
		  ORDER BY length(from_path) DESC
		  LIMIT 1`, fromPath).
		Scan(&r.ID, &r.FromPath, &r.ToPath, &r.Code, &r.IsActive, &r.Note, &r.MatchType, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ToPath = resolveWildcardTarget(r.FromPath, r.ToPath, fromPath)
	return &r, nil
}

// resolveWildcardTarget strips the trailing /* from the rule's from_path to
// derive the captured suffix, then substitutes it into to_path. If to_path
// has no /*, the suffix is dropped and to_path is used verbatim.
func resolveWildcardTarget(ruleFrom, ruleTo, incoming string) string {
	prefix := strings.TrimSuffix(ruleFrom, "/*")
	suffix := strings.TrimPrefix(incoming, prefix) // "", "/foo", "/foo/bar"
	if strings.HasSuffix(ruleTo, "/*") {
		return strings.TrimSuffix(ruleTo, "/*") + suffix
	}
	return ruleTo
}

// Postgres unique-violation SQLSTATE 23505. We sniff the error text to avoid a
// dependency on the lib/pq error package — keeps the package import-free.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
