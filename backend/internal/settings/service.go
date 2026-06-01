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
	"free_shipping_threshold_enabled", // master switch (paired with the amount above)
	"free_shipping_threshold_installer_hkd",     // installer (施工店) tier override, read by storefront when customer.role = installer
	"free_shipping_threshold_installer_enabled", // master switch for the installer threshold (no fallback to the default when off)
	"favicon_url",                // injected into <svelte:head> on storefront + admin
	"company_logo_url",           // storefront header logo image URL
	"company_logo_height_px",     // storefront header logo render height (px)
	"company_logo_footer_url",       // storefront footer logo image URL
	"company_logo_footer_height_px", // storefront footer logo render height (px)
	"site_notice",                // storefront announcement strip copy
	"site_notice_enabled",        // storefront announcement strip on/off toggle
	"site_notice_bg_color",       // storefront announcement strip background color
	"site_notice_text_color",     // storefront announcement strip text color
	"site_notice_text_size",      // storefront announcement strip font size (px)
	"shipping_notice_bg_color",   // storefront shipping notice strip background color
	"shipping_notice_text_color", // storefront shipping notice strip text color
	"shipping_notice_text_size",  // storefront shipping notice strip font size (px)
	"shipping_notice_eligible_bg_color",   // strip background once cart subtotal hits the free-shipping threshold
	"shipping_notice_eligible_text_color", // strip text color once cart subtotal hits the free-shipping threshold
	"recaptcha_enabled",          // forms shortcode reads this to decide whether to load grecaptcha
	"recaptcha_site_key",         // public reCAPTCHA v3 site key — loaded by storefront
	"google_oauth_enabled",       // storefront login/register reads this to show the "Sign in with Google" button
	"apple_oauth_enabled",        // storefront login/register reads this to show the "Sign in with Apple" button
	"homepage_page_id",           // CMS page id used as the storefront homepage; empty = default template
	"blog_enabled",               // when 'false', storefront /blog routes 404 and nav links to /blog are hidden
	"pwa_enabled",                // when 'false', storefront hides PWA tags and unregisters the service worker
	"shipany_enabled",            // master toggle that drives the storefront logistics card (default-courier mode)
	"pdp_taobao_layout_enabled",  // site default for the taobao-style PDP modal; product-level use_taobao_layout overrides
	"social_media",               // JSON-encoded array of {icon, url, label?, customSvgPath?} rendered in the storefront footer
	"website_slogan",             // storefront footer slogan text; empty falls back to the localized m.footer_tagline() message
	"pdp_show_specs_strip",       // storefront PDP toggle: dark-blue 4-points specs strip
	"pdp_show_complete_set",      // storefront PDP toggle: "相關產品" related-products BundleComposer
	"pdp_complete_set_kicker",    // storefront PDP override for the related-products kicker; empty falls back to i18n
	"pdp_complete_set_heading",   // storefront PDP override for the related-products heading; empty falls back to i18n
	"pdp_show_fbt",               // storefront PDP toggle: "其他客人都會買埋呢啲" frequently-bought-together BundleComposer
	"pdp_fbt_kicker",             // storefront PDP override for the FBT kicker; empty falls back to i18n
	"pdp_fbt_heading",            // storefront PDP override for the FBT heading; empty falls back to i18n
	"pdp_fbt_preselect_all",      // storefront PDP: FBT bundle starts all-selected (true) vs none (false)
	"pdp_complete_set_preselect_all", // storefront PDP: related-products bundle starts all-selected (true) vs none (false)
	"pdp_content_layout",         // storefront PDP layout for 內容 / 使用方法 / 適用表面: "tabs" or "nav-list"
	"pdp_navlist_show_nav",       // storefront PDP (nav-list mode): show/hide the section anchor-nav bar
	"pdp_navlist_show_titles",    // storefront PDP (nav-list mode): show/hide each section's title heading
	"pdp_show_stock_count",       // storefront PDP toggle: show exact stock count vs generic "in stock" indicator
	"account_page_layout",        // storefront 我的帳戶 shell: "classic" (sidebar) or "modern" (top tab bar)
	"checkout_page_layout",       // storefront checkout: "classic" (current WooCommerce-style) or "modern" (stepped + single-step payment)
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

// TTLHours reads a setting value as integer hours and returns it as time.Duration.
// Falls back to fallbackHours if the key is missing or the value is not a positive integer.
func (s *Service) TTLHours(ctx context.Context, key string, fallbackHours int) time.Duration {
	st, err := s.Get(ctx, key)
	if err != nil {
		return time.Duration(fallbackHours) * time.Hour
	}
	n, err := strconv.Atoi(st.Value)
	if err != nil || n <= 0 {
		return time.Duration(fallbackHours) * time.Hour
	}
	return time.Duration(n) * time.Hour
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
