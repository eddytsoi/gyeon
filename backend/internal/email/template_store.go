package email

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"html"
	"log"
	"strings"
	texttmpl "text/template"
)

// emailFuncs is the FuncMap available to every email template (compiled-in
// defaults AND admin-edited DB overrides). `esc` HTML-escapes user-controlled
// strings to prevent XSS; `orderref` mirrors the Go-side helper that prefers
// the customer-facing order number and falls back to a truncated UUID;
// `mul` multiplies a float by an int (used for line-total calc when the item
// struct only has UnitPrice + Quantity).
var emailFuncs = texttmpl.FuncMap{
	"esc":      html.EscapeString,
	"orderref": orderRef,
	"mul":      func(a float64, b int) float64 { return a * float64(b) },
}

// Template is a DB-stored override for one of the compiled-in email templates.
type Template struct {
	Key       string  `json:"key"`
	Subject   string  `json:"subject"`
	HTML      string  `json:"html"`
	Text      string  `json:"text"`
	IsEnabled bool    `json:"is_enabled"`
	UpdatedAt string  `json:"updated_at"`
	UpdatedBy *string `json:"updated_by,omitempty"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (st *Store) Get(ctx context.Context, key string) (*Template, error) {
	var t Template
	err := st.db.QueryRowContext(ctx,
		`SELECT key, subject, html, text, is_enabled, updated_at, updated_by
		   FROM email_templates WHERE key = $1`, key).
		Scan(&t.Key, &t.Subject, &t.HTML, &t.Text, &t.IsEnabled, &t.UpdatedAt, &t.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (st *Store) List(ctx context.Context) ([]Template, error) {
	rows, err := st.db.QueryContext(ctx,
		`SELECT key, subject, html, text, is_enabled, updated_at, updated_by
		   FROM email_templates ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Template, 0)
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.Key, &t.Subject, &t.HTML, &t.Text, &t.IsEnabled, &t.UpdatedAt, &t.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type UpsertInput struct {
	Subject   string  `json:"subject"`
	HTML      string  `json:"html"`
	Text      string  `json:"text"`
	IsEnabled bool    `json:"is_enabled"`
	UpdatedBy *string `json:"-"`
}

func (st *Store) Upsert(ctx context.Context, key string, in UpsertInput) (*Template, error) {
	// Validate templates parse — admins shouldn't be able to break send by
	// saving a template with `{{.Foo` (missing close brace). Returns an error
	// surfaced as 422 by the handler.
	if _, err := texttmpl.New("subject").Funcs(emailFuncs).Parse(in.Subject); err != nil {
		return nil, errParseFailure("subject", err)
	}
	if _, err := texttmpl.New("html").Funcs(emailFuncs).Parse(in.HTML); err != nil {
		return nil, errParseFailure("html", err)
	}
	if _, err := texttmpl.New("text").Funcs(emailFuncs).Parse(in.Text); err != nil {
		return nil, errParseFailure("text", err)
	}

	var updatedByArg any
	if in.UpdatedBy != nil && *in.UpdatedBy != "" {
		updatedByArg = *in.UpdatedBy
	}
	var t Template
	err := st.db.QueryRowContext(ctx,
		`INSERT INTO email_templates (key, subject, html, text, is_enabled, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (key) DO UPDATE SET
		     subject=EXCLUDED.subject, html=EXCLUDED.html, text=EXCLUDED.text,
		     is_enabled=EXCLUDED.is_enabled, updated_by=EXCLUDED.updated_by, updated_at=NOW()
		 RETURNING key, subject, html, text, is_enabled, updated_at, updated_by`,
		key, in.Subject, in.HTML, in.Text, in.IsEnabled, updatedByArg).
		Scan(&t.Key, &t.Subject, &t.HTML, &t.Text, &t.IsEnabled, &t.UpdatedAt, &t.UpdatedBy)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (st *Store) Reset(ctx context.Context, key string) error {
	_, err := st.db.ExecContext(ctx, `DELETE FROM email_templates WHERE key = $1`, key)
	return err
}

// ── applyTemplate: DB override → Go text/template render → fall back to
// compiled defaults on miss/error ──────────────────────────────────────────

// applyTemplate looks up an override for `key`. If present and enabled, it
// renders the override against `params`. On miss / parse error / render error,
// it returns fallback().
func (s *Service) applyTemplate(ctx context.Context, key string, params any,
	fallback func() (subject, html, text string)) (string, string, string) {

	if s.tmplStore == nil {
		return fallback()
	}
	t, err := s.tmplStore.Get(ctx, key)
	if err != nil || t == nil || !t.IsEnabled {
		if err != nil {
			log.Printf("email: load template %s: %v", key, err)
		}
		return fallback()
	}

	subject, ok1 := executeTemplate("subject:"+key, t.Subject, params)
	html, ok2 := executeTemplate("html:"+key, t.HTML, params)
	text, ok3 := executeTemplate("text:"+key, t.Text, params)
	if !ok1 || !ok2 || !ok3 {
		return fallback()
	}
	return subject, html, text
}

func executeTemplate(name, body string, params any) (string, bool) {
	tmpl, err := texttmpl.New(name).Funcs(emailFuncs).Parse(body)
	if err != nil {
		log.Printf("email: parse %s: %v", name, err)
		return "", false
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		log.Printf("email: exec %s: %v", name, err)
		return "", false
	}
	return buf.String(), true
}

// SetTemplateStore wires an optional DB-backed template store. When nil, the
// service uses compiled-in templates only (P0/P1 behaviour).
func (s *Service) SetTemplateStore(st *Store) {
	s.tmplStore = st
}

// ── Errors ─────────────────────────────────────────────────────────────────

type ParseError struct {
	Field string
	Err   error
}

func (e *ParseError) Error() string {
	return "template parse failed in " + e.Field + ": " + e.Err.Error()
}

func errParseFailure(field string, err error) *ParseError {
	return &ParseError{Field: field, Err: err}
}

// SampleParamsFor returns realistic sample data for a template key — used by
// the Send Test endpoint and the admin preview button.
func SampleParamsFor(key string) any {
	switch key {
	case "order_confirmation":
		return OrderEmailParams{
			OrderID: "00000000-0000-0000-0000-000000000001", OrderNumber: "ORD-0001",
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			Currency: "HKD",
			Subtotal: 100, ShippingFee: 30, DiscountAmount: 0, Total: 135,
			Items: []OrderEmailItem{{Name: "Sample Product", SKU: "SKU-001", Quantity: 1, UnitPrice: 100, LineTotal: 100}},
		}
	case "order_shipped":
		return ShippedEmailParams{
			OrderID: "00000000-0000-0000-0000-000000000001", OrderNumber: "ORD-0001",
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			Carrier: "SF Express", TrackingNumber: "SF1234567890",
		}
	case "order_refunded":
		return RefundEmailParams{
			OrderID: "00000000-0000-0000-0000-000000000001", OrderNumber: "ORD-0001",
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			Currency: "HKD", RefundAmount: 50, OrderTotal: 135, Reason: "Customer requested partial refund",
		}
	case "payment_link":
		return PaymentLinkParams{
			OrderID: "00000000-0000-0000-0000-000000000001", OrderNumber: "ORD-0001",
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			PaymentURL: "https://example.com/pay/00000000", Total: 135, Currency: "HKD",
		}
	case "password_reset":
		return PasswordResetParams{
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			ResetURL: "https://example.com/account/reset-password?token=sampletoken",
			ExpiryHours: 24,
		}
	case "admin_message":
		return AdminMessageParams{
			To: "sample@example.com", OrderNumber: "ORD-0001",
			CustomerName: "Sample Customer",
			OrderURL: "https://example.com/account/orders/00000000",
			Body: "Hello — your order has been received and is being prepared.",
		}
	case "abandoned_cart":
		return AbandonedCartParams{
			CustomerName: "Sample Customer", CustomerEmail: "sample@example.com",
			Currency: "HKD",
			Items: []AbandonedCartItem{{Name: "Sample Product", Quantity: 1, UnitPrice: 100}},
			Subtotal: 100, ResumeURL: "https://example.com/cart",
		}
	case "low_stock_alert":
		return LowStockParams{
			ProductName: "Sample Product", VariantName: "Default", SKU: "SKU-001",
			StockQty: 3, Threshold: 5,
		}
	}
	return map[string]string{}
}

// ── Variable hints surfaced in the admin UI for each template key ──────────

func VariablesFor(key string) []string {
	switch key {
	case "order_confirmation":
		return []string{".OrderNumber", ".CustomerName", ".CustomerEmail", ".Currency",
			".Subtotal", ".ShippingFee", ".DiscountAmount", ".Total", ".Items"}
	case "order_shipped":
		return []string{".OrderNumber", ".CustomerName", ".CustomerEmail",
			".Carrier", ".Service", ".TrackingNumber", ".TrackingURL"}
	case "order_refunded":
		return []string{".OrderNumber", ".CustomerName", ".CustomerEmail",
			".Currency", ".RefundAmount", ".OrderTotal", ".Reason"}
	case "payment_link":
		return []string{".OrderNumber", ".CustomerName", ".CustomerEmail",
			".PaymentURL", ".Total", ".Currency"}
	case "password_reset":
		return []string{".CustomerName", ".CustomerEmail", ".ResetURL", ".ExpiryHours"}
	case "admin_message":
		return []string{".OrderNumber", ".CustomerName", ".OrderURL", ".Body"}
	case "abandoned_cart":
		return []string{".CustomerName", ".CustomerEmail", ".Currency",
			".Subtotal", ".Items", ".ResumeURL"}
	case "low_stock_alert":
		return []string{".ProductName", ".VariantName", ".SKU", ".StockQty", ".Threshold"}
	}
	return nil
}

// AllKeys returns every supported template key in display order.
func AllKeys() []string {
	return []string{
		"order_confirmation", "order_shipped", "order_refunded",
		"payment_link", "password_reset", "admin_message",
		"abandoned_cart", "low_stock_alert",
	}
}

// DisplayName returns a human-friendly label for a template key.
func DisplayName(key string) string {
	switch key {
	case "order_confirmation":
		return "Order confirmation"
	case "order_shipped":
		return "Order shipped"
	case "order_refunded":
		return "Order refunded"
	case "payment_link":
		return "Payment link"
	case "password_reset":
		return "Password reset"
	case "admin_message":
		return "Admin message"
	case "abandoned_cart":
		return "Abandoned cart reminder"
	case "low_stock_alert":
		return "Low stock alert"
	}
	return strings.Title(strings.ReplaceAll(key, "_", " "))
}
