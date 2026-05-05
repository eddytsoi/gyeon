package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gyeon/backend/internal/auth"
)

// Entry describes a single audit-loggable event. Pass before/after as any
// JSON-serializable value (typically the full struct or a partial diff).
type Entry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

// Record persists an audit row. Failure is logged and swallowed — audit must
// never block the underlying business operation. Pull admin ID + IP + UA from
// the request context (set by auth.AdminMiddleware + RequestInfoMiddleware).
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, e Entry) {
	adminID, _ := auth.AdminIDFromContext(ctx)
	ip, _ := IPFromContext(ctx)
	ua, _ := UserAgentFromContext(ctx)

	var beforeJSON, afterJSON []byte
	if e.Before != nil {
		if b, err := json.Marshal(e.Before); err == nil {
			beforeJSON = b
		}
	}
	if e.After != nil {
		if b, err := json.Marshal(e.After); err == nil {
			afterJSON = b
		}
	}

	var adminIDArg any
	if adminID != "" {
		adminIDArg = adminID
	}
	var entityIDArg any
	if e.EntityID != "" {
		entityIDArg = e.EntityID
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_audit_log (admin_user_id, action, entity_type, entity_id, before, after, ip, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		adminIDArg, e.Action, e.EntityType, entityIDArg,
		nullableJSON(beforeJSON), nullableJSON(afterJSON),
		ip, ua,
	); err != nil {
		log.Printf("audit: record %s/%s: %v", e.Action, e.EntityType, err)
	}
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// ── Listing (admin UI) ─────────────────────────────────────────────────────

type Row struct {
	ID         string  `json:"id"`
	AdminID    *string `json:"admin_user_id,omitempty"`
	AdminEmail *string `json:"admin_email,omitempty"`
	Action     string  `json:"action"`
	EntityType string  `json:"entity_type"`
	EntityID   *string `json:"entity_id,omitempty"`
	Before     *string `json:"before,omitempty"`
	After      *string `json:"after,omitempty"`
	IP         *string `json:"ip,omitempty"`
	UserAgent  *string `json:"user_agent,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type ListFilter struct {
	Action     string
	EntityType string
	AdminID    string
	From       string
	To         string
	Limit      int
	Offset     int
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]Row, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	args := []any{}
	where := []string{"TRUE"}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.Action != "" {
		add("a.action = ?", f.Action)
	}
	if f.EntityType != "" {
		add("a.entity_type = ?", f.EntityType)
	}
	if f.AdminID != "" {
		add("a.admin_user_id = ?", f.AdminID)
	}
	if f.From != "" {
		add("a.created_at >= ?", f.From)
	}
	if f.To != "" {
		add("a.created_at <= ?", f.To)
	}

	whereSQL := strings.Join(where, " AND ")

	// total
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_audit_log a WHERE `+whereSQL, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	q := `SELECT a.id, a.admin_user_id, u.email, a.action, a.entity_type, a.entity_id,
	             a.before::text, a.after::text, a.ip, a.user_agent, a.created_at
	        FROM admin_audit_log a
	        LEFT JOIN admin_users u ON u.id = a.admin_user_id
	       WHERE ` + whereSQL + `
	       ORDER BY a.created_at DESC
	       LIMIT $` + strconv.Itoa(limitIdx) + ` OFFSET $` + strconv.Itoa(offsetIdx)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Row, 0)
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.AdminID, &r.AdminEmail, &r.Action, &r.EntityType,
			&r.EntityID, &r.Before, &r.After, &r.IP, &r.UserAgent, &r.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// Sentinel for tests / callers that want to detect a missing service.
var ErrNotConfigured = errors.New("audit service not configured")

// ── Request info middleware (IP / User-Agent) ──────────────────────────────

type ctxKey string

const (
	ipKey ctxKey = "audit_ip"
	uaKey ctxKey = "audit_ua"
)

// RequestInfoMiddleware captures IP + User-Agent for downstream audit.Record
// calls. Mount it once on the admin route group, after AdminMiddleware.
func RequestInfoMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, ipKey, clientIP(r))
			ctx = context.WithValue(ctx, uaKey, r.UserAgent())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func clientIP(r *http.Request) string {
	// X-Forwarded-For wins when present (RealIP middleware already populates
	// RemoteAddr too, but XFF carries the real client behind a proxy chain).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if r.RemoteAddr == "" {
		return ""
	}
	if i := strings.LastIndexByte(r.RemoteAddr, ':'); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

func IPFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ipKey).(string)
	return v, ok
}

func UserAgentFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(uaKey).(string)
	return v, ok
}
