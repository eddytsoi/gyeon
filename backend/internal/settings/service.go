package settings

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/lib/pq"
)

type Setting struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

// publicSettingKeys are the only site_settings keys safe to expose via the
// unauthenticated GET /api/v1/settings/ endpoint. Stripe/SMTP/ShipAny secrets
// stay admin-only. Add a key here only after confirming the storefront needs
// it and the value is non-sensitive.
var publicSettingKeys = []string{
	"maintenance_mode",
	"mcp_enabled",
	"shipping_countries",
	"site_locale",
	"stripe_save_cards",
	"tax_enabled",
	"tax_rate",
	"tax_label",
	"tax_inclusive",
	"public_base_url",
	"ga4_measurement_id",         // P3 #26 — read by storefront tracker
	"meta_pixel_id",              // P3 #26
	"free_shipping_threshold_hkd", // P3 #29 — used by checkout summary + free-ship banner
	"favicon_url",                // injected into <svelte:head> on storefront + admin
	"site_notice",                // storefront announcement strip copy
	"recaptcha_enabled",          // forms shortcode reads this to decide whether to load grecaptcha
	"recaptcha_site_key",         // public reCAPTCHA v3 site key — loaded by storefront
}

// AuditRecorder mirrors the minimal shape of audit.Service.Record, kept local
// to avoid an import cycle.
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

// SetAudit wires an optional audit recorder. Call from main during setup.
func (s *Service) SetAudit(rec AuditRecorder) { s.audit = rec }

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListAll(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, description, updated_at FROM site_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	return scanSettings(rows)
}

func (s *Service) ListPublic(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, description, updated_at FROM site_settings WHERE key = ANY($1) ORDER BY key`,
		pq.Array(publicSettingKeys))
	if err != nil {
		return nil, err
	}
	return scanSettings(rows)
}

func scanSettings(rows *sql.Rows) ([]Setting, error) {
	defer rows.Close()
	settings := make([]Setting, 0)
	for rows.Next() {
		var st Setting
		if err := rows.Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, st)
	}
	return settings, rows.Err()
}

func (s *Service) Get(ctx context.Context, key string) (*Setting, error) {
	var st Setting
	err := s.db.QueryRowContext(ctx,
		`SELECT key, value, description, updated_at FROM site_settings WHERE key=$1`, key).
		Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) Set(ctx context.Context, key, value string) (*Setting, error) {
	var st Setting
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO site_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()
		 RETURNING key, value, description, updated_at`,
		key, value).
		Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// TTL reads a setting value as integer seconds and returns it as time.Duration.
// Falls back to fallbackSecs if the key is missing or the value is not a positive integer.
func (s *Service) TTL(ctx context.Context, key string, fallbackSecs int) time.Duration {
	st, err := s.Get(ctx, key)
	if err != nil {
		return time.Duration(fallbackSecs) * time.Second
	}
	n, err := strconv.Atoi(st.Value)
	if err != nil || n <= 0 {
		return time.Duration(fallbackSecs) * time.Second
	}
	return time.Duration(n) * time.Second
}

func (s *Service) BulkSet(ctx context.Context, updates map[string]string) ([]Setting, error) {
	// Snapshot prior values for audit (only keys being changed, only when audit
	// is wired — otherwise skip to avoid the extra round-trip).
	var before map[string]string
	if s.audit != nil && len(updates) > 0 {
		before = make(map[string]string, len(updates))
		for k := range updates {
			if prev, err := s.Get(ctx, k); err == nil {
				before[k] = prev.Value
			} else {
				before[k] = ""
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for key, value := range updates {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO site_settings (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`,
			key, value)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if s.audit != nil {
		s.audit.Record(ctx, AuditEntry{
			Action: "settings.bulk_update", EntityType: "settings", EntityID: "",
			Before: before, After: updates,
		})
	}

	return s.ListAll(ctx)
}
