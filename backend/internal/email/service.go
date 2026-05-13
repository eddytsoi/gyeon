package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"gyeon/backend/internal/settings"
)

var ErrNotConfigured = errors.New("smtp is not configured")
var ErrDisabled = errors.New("email sending is disabled")

type Service struct {
	settings  *settings.Service
	tmplStore *Store // optional DB-backed override layer (P2 #20). nil = compiled defaults only.
}

func NewService(s *settings.Service) *Service {
	return &Service{settings: s}
}

type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	BaseURL   string
}

func (s *Service) loadConfig(ctx context.Context) (Config, error) {
	if !s.isEnabled(ctx) {
		return Config{}, ErrDisabled
	}
	return s.loadSMTPConfig(ctx)
}

// loadSMTPConfig reads the SMTP credentials without consulting the
// email_enabled master switch. SendTest uses this so the admin can validate
// SMTP settings even when outgoing email is globally disabled.
func (s *Service) loadSMTPConfig(ctx context.Context) (Config, error) {
	c := Config{
		Host:      s.read(ctx, "smtp_host"),
		Username:  s.read(ctx, "smtp_username"),
		Password:  s.read(ctx, "smtp_password"),
		FromEmail: s.read(ctx, "smtp_from_email"),
		FromName:  s.read(ctx, "smtp_from_name"),
		BaseURL:   s.read(ctx, "public_base_url"),
	}
	port, err := strconv.Atoi(s.read(ctx, "smtp_port"))
	if err != nil || port == 0 {
		port = 587
	}
	c.Port = port
	if c.FromName == "" {
		c.FromName = "Gyeon"
	}
	if c.Host == "" || c.Username == "" || c.Password == "" || c.FromEmail == "" {
		return c, ErrNotConfigured
	}
	return c, nil
}

// isEnabled returns true unless the email_enabled site setting is explicitly
// "false". A missing row (fresh install before migration 055) defaults to
// enabled so existing behaviour is preserved.
func (s *Service) isEnabled(ctx context.Context) bool {
	v := strings.TrimSpace(s.read(ctx, "email_enabled"))
	return !strings.EqualFold(v, "false")
}

func (s *Service) PublicBaseURL(ctx context.Context) string {
	v := s.read(ctx, "public_base_url")
	if v == "" {
		return "http://localhost:5173"
	}
	return strings.TrimRight(v, "/")
}

type OrderEmailItem struct {
	Name      string
	SKU       string
	Quantity  int
	UnitPrice float64
	LineTotal float64
}

type OrderEmailParams struct {
	OrderID         string
	OrderNumber     string // customer-facing, e.g. ORD-0001
	CustomerName    string
	CustomerEmail   string
	Items           []OrderEmailItem
	Subtotal        float64
	ShippingFee     float64
	DiscountAmount  float64
	TaxAmount       float64
	TaxLabel        string
	Total           float64
	Currency        string
	ShippingLine1   string
	ShippingLine2   string
	ShippingCity    string
	ShippingPostal  string
	ShippingCountry string
	SetupURL        string // empty unless guest
}

type PaymentLinkParams struct {
	OrderID       string
	OrderNumber   string // customer-facing, e.g. ORD-0001
	CustomerName  string
	CustomerEmail string
	Items         []OrderEmailItem
	Total         float64
	Currency      string
	PaymentURL    string
}

type PasswordResetParams struct {
	CustomerName  string
	CustomerEmail string
	ResetURL      string
	ExpiryHours   int
}

type AdminMessageParams struct {
	To           string
	CustomerName string
	OrderNumber  string
	OrderURL     string // links the customer back to /account/orders/{id}
	Body         string
}

type ShippedEmailParams struct {
	OrderID        string
	OrderNumber    string
	CustomerName   string
	CustomerEmail  string
	Carrier        string
	Service        string
	TrackingNumber string
	TrackingURL    string
	OrderURL       string
}

type RefundEmailParams struct {
	OrderID       string
	OrderNumber   string
	CustomerName  string
	CustomerEmail string
	Currency      string
	RefundAmount  float64
	OrderTotal    float64
	Reason        string
	IsFullRefund  bool
	OrderURL      string
}

type AbandonedCartItem struct {
	Name      string
	Quantity  int
	UnitPrice float64
}

type AbandonedCartParams struct {
	CustomerName  string
	CustomerEmail string
	Items         []AbandonedCartItem
	Subtotal      float64
	Currency      string
	ResumeURL     string
}

type LowStockParams struct {
	To              string
	ProductName     string
	VariantName     string
	SKU             string
	StockQty        int
	Threshold       int
	AdminProductURL string
}

// SendTest sends a plain test email to verify SMTP configuration. Bypasses
// the email_enabled master switch so the admin can validate credentials even
// when outgoing email is globally disabled.
func (s *Service) SendTest(ctx context.Context, to string) error {
	cfg, err := s.loadSMTPConfig(ctx)
	if err != nil {
		return err
	}
	subject := "SMTP Configuration Test — Gyeon"
	text := "Hello,\n\nThis is a test email sent from Gyeon to verify your SMTP configuration is working correctly.\n\nIf you received this message, your email settings are configured properly and outgoing mail is functioning as expected.\n\nNo action is required.\n\n— Gyeon Admin"
	html := `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>SMTP Test</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 8px;font-size:20px">SMTP Configuration Test</h1>
      <p style="margin:0 0 16px;color:#6b7280;font-size:14px;line-height:1.6">This is a test email sent from Gyeon to verify your SMTP configuration is working correctly.</p>
      <p style="margin:0 0 16px;color:#374151;font-size:14px;line-height:1.6">If you received this message, your email settings are configured properly and outgoing mail is functioning as expected.</p>
      <p style="margin:0;color:#6b7280;font-size:14px">No action is required.</p>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon Admin</p>
  </div>
</body></html>`
	return s.send(cfg, to, subject, text, html)
}

// SendPaymentLink renders and sends a "complete payment" email containing a
// link the customer can click to finish Stripe payment in their browser.
// Used when checkout is initiated via MCP (no inline Stripe Element flow).
func (s *Service) SendPaymentLink(ctx context.Context, p PaymentLinkParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if p.Currency == "" {
		p.Currency = "HKD"
	}
	subject, html, text := s.applyTemplate(ctx, "payment_link", p, func() (string, string, string) {
		return renderDefault("subject:payment_link", paymentLinkSubject, p),
			renderDefault("html:payment_link", paymentLinkHTML, p),
			renderDefault("text:payment_link", paymentLinkText, p)
	})
	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendPasswordResetEmail sends a one-time password-reset link to the customer.
// Triggered from the admin customer detail page; the token is generated by
// customers.Service before this is called.
func (s *Service) SendPasswordResetEmail(ctx context.Context, p PasswordResetParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if p.ExpiryHours == 0 {
		p.ExpiryHours = 24
	}
	subject, html, text := s.applyTemplate(ctx, "password_reset", p, func() (string, string, string) {
		return renderDefault("subject:password_reset", passwordResetSubject, p),
			renderDefault("html:password_reset", passwordResetHTML, p),
			renderDefault("text:password_reset", passwordResetText, p)
	})
	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendAdminMessageNotification emails the customer when an admin posts a
// reply to their order. Best-effort — caller should not fail the request on
// SMTP errors.
func (s *Service) SendAdminMessageNotification(ctx context.Context, p AdminMessageParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	subject, html, text := s.applyTemplate(ctx, "admin_message", p, func() (string, string, string) {
		return renderDefault("subject:admin_message", adminMessageSubject, p),
			renderDefault("html:admin_message", adminMessageHTML, p),
			renderDefault("text:admin_message", adminMessageText, p)
	})
	return s.send(cfg, p.To, subject, text, html)
}

// SendOrderConfirmation renders and sends the order confirmation email.
// Returns ErrNotConfigured if SMTP credentials are missing — caller may treat as warning.
func (s *Service) SendOrderConfirmation(ctx context.Context, p OrderEmailParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}

	if p.Currency == "" {
		p.Currency = "HKD"
	}

	subject, html, text := s.applyTemplate(ctx, "order_confirmation", p, func() (string, string, string) {
		return renderDefault("subject:order_confirmation", orderConfirmationSubject, p),
			renderDefault("html:order_confirmation", orderConfirmationHTML, p),
			renderDefault("text:order_confirmation", orderConfirmationText, p)
	})

	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendOrderShipped notifies the customer that their order has shipped, with
// optional carrier / tracking info.
func (s *Service) SendOrderShipped(ctx context.Context, p ShippedEmailParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	subject, html, text := s.applyTemplate(ctx, "order_shipped", p, func() (string, string, string) {
		return renderDefault("subject:order_shipped", orderShippedSubject, p),
			renderDefault("html:order_shipped", orderShippedHTML, p),
			renderDefault("text:order_shipped", orderShippedText, p)
	})
	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendOrderRefunded notifies the customer that their order has been (partially
// or fully) refunded.
func (s *Service) SendOrderRefunded(ctx context.Context, p RefundEmailParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if p.Currency == "" {
		p.Currency = "HKD"
	}
	subject, html, text := s.applyTemplate(ctx, "order_refunded", p, func() (string, string, string) {
		return renderDefault("subject:order_refunded", orderRefundedSubject, p),
			renderDefault("html:order_refunded", orderRefundedHTML, p),
			renderDefault("text:order_refunded", orderRefundedText, p)
	})
	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendAbandonedCart reminds a logged-in customer about a cart they left
// without checking out. Best-effort — errors are logged by the caller.
func (s *Service) SendAbandonedCart(ctx context.Context, p AbandonedCartParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if p.Currency == "" {
		p.Currency = "HKD"
	}
	subject, html, text := s.applyTemplate(ctx, "abandoned_cart", p, func() (string, string, string) {
		return renderDefault("subject:abandoned_cart", abandonedCartSubject, p),
			renderDefault("html:abandoned_cart", abandonedCartHTML, p),
			renderDefault("text:abandoned_cart", abandonedCartText, p)
	})
	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendLowStockAlert notifies the configured admin alert email address that a
// variant has crossed its low-stock threshold.
func (s *Service) SendLowStockAlert(ctx context.Context, p LowStockParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	to := p.To
	if to == "" {
		to = s.read(ctx, "admin_alert_email")
	}
	if to == "" {
		to = cfg.FromEmail
	}
	if to == "" {
		return ErrNotConfigured
	}
	subject, html, text := s.applyTemplate(ctx, "low_stock_alert", p, func() (string, string, string) {
		return renderDefault("subject:low_stock_alert", lowStockAlertSubject, p),
			renderDefault("html:low_stock_alert", lowStockAlertHTML, p),
			renderDefault("text:low_stock_alert", lowStockAlertText, p)
	})
	return s.send(cfg, to, subject, text, html)
}

func (s *Service) send(cfg Config, to, subject, text, html string) error {
	return s.sendWithReplyTo(cfg, to, "", subject, text, html)
}

// sendWithReplyTo is the variant used by contact-form mail where the admin
// configures a Reply-To header (typically `[your-email]` resolved to the
// submitter's address) so replying in the inbox goes back to the customer.
func (s *Service) sendWithReplyTo(cfg Config, to, replyTo, subject, text, html string) error {
	from := mime.QEncoding.Encode("utf-8", cfg.FromName) + " <" + cfg.FromEmail + ">"
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)

	boundary := "gyeon-mime-boundary-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	if replyTo != "" {
		fmt.Fprintf(&msg, "Reply-To: %s\r\n", replyTo)
	}
	fmt.Fprintf(&msg, "Subject: %s\r\n", encodedSubject)
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)

	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	fmt.Fprintf(&msg, "Content-Type: text/plain; charset=\"utf-8\"\r\n")
	fmt.Fprintf(&msg, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(text)
	msg.WriteString("\r\n\r\n")

	fmt.Fprintf(&msg, "--%s\r\n", boundary)
	fmt.Fprintf(&msg, "Content-Type: text/html; charset=\"utf-8\"\r\n")
	fmt.Fprintf(&msg, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(html)
	msg.WriteString("\r\n\r\n")

	fmt.Fprintf(&msg, "--%s--\r\n", boundary)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	return smtp.SendMail(addr, auth, cfg.FromEmail, []string{to}, msg.Bytes())
}

// orderRef prefers the customer-facing number; falls back to the
// truncated UUID when the number isn't available (e.g. legacy callers).
func orderRef(orderNumber, orderID string) string {
	if orderNumber != "" {
		return orderNumber
	}
	if len(orderID) > 8 {
		return orderID[:8]
	}
	return orderID
}

// ── Compiled-in template defaults (Go text/template syntax) ────────────────
//
// Each Send* method uses these as fallback when no DB override exists.
// `defaultsFor()` in handler.go also returns these raw strings so admins see
// `{{.X}}` syntax in the editor when they "Reset to defaults".
//
// FuncMap (template_store.go: emailFuncs):
//   {{.X | esc}}     — HTML-escape user-controlled strings
//   {{orderref .OrderNumber .OrderID}}  — order number with UUID fallback
//   {{printf "%.2f" .X}}  — 2-decimal money format (built-in)
//   {{$.X}}          — access outer scope inside {{range}}

// password_reset ────────────────────────────────────────────────────────────
const passwordResetSubject = `重設您的 Gyeon 帳戶密碼`

const passwordResetText = `您好 {{.CustomerName}}，

我們收到了重設您 Gyeon 帳戶密碼的請求。請按以下連結設定新密碼：

{{.ResetURL}}

此連結將於 {{.ExpiryHours}} 小時後失效，且只可使用一次。

如非本人要求，請忽略此電郵，您的密碼不會被更改。

— Gyeon`

const passwordResetHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>重設密碼</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">重設密碼</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px;line-height:1.6">您好 {{.CustomerName | esc}}，我們收到了重設您 Gyeon 帳戶密碼的請求。請按下方按鈕設定新密碼。</p>

      <div style="text-align:center;margin:24px 0 8px">
        <a href="{{.ResetURL}}" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">重設密碼</a>
      </div>
      <p style="text-align:center;margin:0 0 24px;color:#9ca3af;font-size:12px">此連結將於 {{.ExpiryHours}} 小時後失效，且只可使用一次</p>

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px;line-height:1.6">如連結無法開啟，請複製貼上至瀏覽器：<br><span style="word-break:break-all;color:#6b7280">{{.ResetURL | esc}}</span></p>

      <div style="margin-top:24px;padding-top:16px;border-top:1px solid #e5e7eb">
        <p style="margin:0;color:#9ca3af;font-size:12px;line-height:1.6">如非本人要求，請忽略此電郵，您的密碼不會被更改。</p>
      </div>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`

// low_stock_alert ──────────────────────────────────────────────────────────
const lowStockAlertSubject = `低庫存警示 — {{.ProductName}}`

const lowStockAlertText = `低庫存警示

商品：{{.ProductName}}{{if .VariantName}}（{{.VariantName}}）{{end}}
{{if .SKU}}SKU：{{.SKU}}
{{end}}目前庫存：{{.StockQty}}
閾值：{{.Threshold}}

{{if .AdminProductURL}}前往補貨：
{{.AdminProductURL}}

{{end}}— Gyeon`

const lowStockAlertHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>低庫存警示</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">低庫存警示</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">以下商品的庫存已跌至閾值或以下。</p>

      <table style="width:100%;border-collapse:collapse;font-size:14px">
        <tr><td style="padding:4px 0;color:#6b7280">商品</td><td style="padding:4px 0;text-align:right">{{.ProductName | esc}}{{if .VariantName}}<span style="color:#9ca3af"> · {{.VariantName | esc}}</span>{{end}}</td></tr>
        {{if .SKU}}<tr><td style="padding:4px 0;color:#6b7280">SKU</td><td style="padding:4px 0;text-align:right;font-family:ui-monospace,SFMono-Regular,Menlo,monospace">{{.SKU | esc}}</td></tr>{{end}}
        <tr><td style="padding:4px 0;color:#6b7280">目前庫存</td><td style="padding:4px 0;text-align:right;font-weight:600;color:#dc2626">{{.StockQty}}</td></tr>
        <tr><td style="padding:4px 0;color:#6b7280">閾值</td><td style="padding:4px 0;text-align:right">{{.Threshold}}</td></tr>
      </table>

      {{if .AdminProductURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.AdminProductURL}}" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">前往補貨</a>
      </div>{{end}}
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon Admin Alert</p>
  </div>
</body></html>`

// admin_message ────────────────────────────────────────────────────────────
const adminMessageSubject = `店家回覆 — {{orderref .OrderNumber ""}}`

const adminMessageText = `{{if .CustomerName}}您好 {{.CustomerName}}，{{else}}您好，{{end}}

您的訂單 {{orderref .OrderNumber ""}} 收到一則新訊息：

──────── 訊息內容 ────────
{{.Body}}
──────────────────────────

{{if .OrderURL}}查看訂單詳情或回覆：
{{.OrderURL}}

{{end}}— Gyeon`

// adminMessageHTML preserves the original whitespace-pre-wrap behaviour by
// printing Body unchanged inside a pre-wrap container; admins can still edit
// the wrapper to taste. Note Body is escaped to prevent HTML injection.
const adminMessageHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>店家回覆</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">店家回覆</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">{{if .CustomerName}}您好 {{.CustomerName | esc}}，{{else}}您好，{{end}} 您的訂單 <strong style="color:#111827">{{orderref .OrderNumber "" | esc}}</strong> 收到一則新訊息。</p>

      <div style="padding:16px;background:#f9fafb;border-left:3px solid #111827;border-radius:6px;color:#374151;font-size:14px;line-height:1.7;white-space:pre-wrap">{{.Body | esc}}</div>

      {{if .OrderURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.OrderURL}}" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單並回覆</a>
      </div>{{end}}
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎登入帳戶回覆 — Gyeon</p>
  </div>
</body></html>`

// order_shipped ────────────────────────────────────────────────────────────
const orderShippedSubject = `您的訂單已寄出 — {{orderref .OrderNumber .OrderID}}`

const orderShippedText = `{{if .CustomerName}}您好 {{.CustomerName}}，{{else}}您好，{{end}}

您的訂單 {{orderref .OrderNumber .OrderID}} 已經寄出！

{{if or .Carrier .TrackingNumber}}──────── 物流資訊 ────────
{{if .Carrier}}物流公司：{{.Carrier}}{{if .Service}}（{{.Service}}）{{end}}
{{end}}{{if .TrackingNumber}}追蹤編號：{{.TrackingNumber}}
{{end}}{{if .TrackingURL}}追蹤連結：{{.TrackingURL}}
{{end}}
{{end}}{{if .OrderURL}}查看訂單詳情：
{{.OrderURL}}

{{end}}如有任何疑問，歡迎回覆此電郵。

— Gyeon`

const orderShippedHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>已寄出</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">您的訂單已寄出</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">{{if .CustomerName}}您好 {{.CustomerName | esc}}，{{else}}您好，{{end}} 您的訂單 <strong style="color:#111827">{{orderref .OrderNumber .OrderID | esc}}</strong> 已交付物流公司寄出。</p>
      {{if or .Carrier .TrackingNumber}}<h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">物流資訊</h3>
      <table style="width:100%;border-collapse:collapse;font-size:14px">
        {{if .Carrier}}<tr><td style="padding:4px 0;color:#6b7280">物流公司</td><td style="padding:4px 0;text-align:right">{{.Carrier | esc}}{{if .Service}}<span style="color:#9ca3af"> · {{.Service | esc}}</span>{{end}}</td></tr>{{end}}
        {{if .TrackingNumber}}<tr><td style="padding:4px 0;color:#6b7280">追蹤編號</td><td style="padding:4px 0;text-align:right;font-family:ui-monospace,SFMono-Regular,Menlo,monospace">{{.TrackingNumber | esc}}</td></tr>{{end}}
      </table>{{end}}
      {{if .TrackingURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.TrackingURL}}" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">追蹤包裹</a>
      </div>{{else if .OrderURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.OrderURL}}" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單</a>
      </div>{{end}}
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`

// order_refunded ───────────────────────────────────────────────────────────
const orderRefundedSubject = `退款通知 — {{orderref .OrderNumber .OrderID}}`

const orderRefundedText = `{{if .CustomerName}}您好 {{.CustomerName}}，{{else}}您好，{{end}}

{{if .IsFullRefund}}您的訂單 {{orderref .OrderNumber .OrderID}} 已全額退款。{{else}}您的訂單 {{orderref .OrderNumber .OrderID}} 已部分退款。{{end}}

──────── 退款明細 ────────
退款金額：{{.Currency}} {{printf "%.2f" .RefundAmount}}
{{if not .IsFullRefund}}訂單總額：{{.Currency}} {{printf "%.2f" .OrderTotal}}
{{end}}{{if .Reason}}原因：{{.Reason}}
{{end}}
款項將於 5–10 個工作天內退回您原本的付款方式。

{{if .OrderURL}}查看訂單詳情：
{{.OrderURL}}

{{end}}如有任何疑問，歡迎回覆此電郵。

— Gyeon`

const orderRefundedHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>退款通知</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">退款已處理</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">{{if .CustomerName}}您好 {{.CustomerName | esc}}，{{else}}您好，{{end}} 您的訂單 <strong style="color:#111827">{{orderref .OrderNumber .OrderID | esc}}</strong> {{if .IsFullRefund}}已全額退款。{{else}}已部分退款。{{end}}</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">退款明細</h3>
      <table style="width:100%;border-collapse:collapse;font-size:14px">
        <tr><td style="padding:4px 0;color:#6b7280">退款金額</td><td style="padding:4px 0;text-align:right;font-weight:600;color:#059669">{{.Currency}} {{printf "%.2f" .RefundAmount}}</td></tr>
        {{if not .IsFullRefund}}<tr><td style="padding:4px 0;color:#6b7280">訂單總額</td><td style="padding:4px 0;text-align:right">{{.Currency}} {{printf "%.2f" .OrderTotal}}</td></tr>{{end}}
        {{if .Reason}}<tr><td style="padding:4px 0;color:#6b7280">原因</td><td style="padding:4px 0;text-align:right">{{.Reason | esc}}</td></tr>{{end}}
      </table>

      <p style="margin:24px 0 0;color:#6b7280;font-size:13px;line-height:1.6">款項將於 5–10 個工作天內退回您原本的付款方式。</p>

      {{if .OrderURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.OrderURL}}" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單</a>
      </div>{{end}}
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`

// payment_link ─────────────────────────────────────────────────────────────
const paymentLinkSubject = `完成付款 — {{orderref .OrderNumber .OrderID}}`

const paymentLinkText = `您好 {{.CustomerName}}，

您的訂單 {{orderref .OrderNumber .OrderID}} 已建立，請按以下連結完成付款：

{{.PaymentURL}}

──────── 訂單明細 ────────
{{range .Items}}{{.Name}} × {{.Quantity}}   {{$.Currency}} {{printf "%.2f" .LineTotal}}
{{end}}
總額：{{.Currency}} {{printf "%.2f" .Total}}

此付款連結將於 24 小時後失效。付款成功後您會收到正式訂單確認電郵。

— Gyeon`

const paymentLinkHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>完成付款</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">請完成付款</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">您好 {{.CustomerName | esc}}，您的訂單 <strong style="color:#111827">{{orderref .OrderNumber .OrderID | esc}}</strong> 已建立，請按下方按鈕完成付款。</p>

      <div style="text-align:center;margin:24px 0 8px">
        <a href="{{.PaymentURL}}" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">立即完成付款</a>
      </div>
      <p style="text-align:center;margin:0 0 24px;color:#9ca3af;font-size:12px">此連結將於 24 小時後失效</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">訂單明細</h3>
      <table style="width:100%;border-collapse:collapse;font-size:14px">{{range .Items}}<tr><td style="padding:8px 0;">{{.Name | esc}} <span style="color:#9ca3af">× {{.Quantity}}</span></td><td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">{{$.Currency}} {{printf "%.2f" .LineTotal}}</td></tr>{{end}}</table>

      <table style="width:100%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:8px 0 0;font-weight:600">總額</td><td style="padding:8px 0 0;text-align:right;font-weight:600">{{.Currency}} {{printf "%.2f" .Total}}</td></tr>
      </table>

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px;line-height:1.6">付款成功後您會收到正式訂單確認電郵。如連結無法開啟，請複製貼上至瀏覽器：<br><span style="word-break:break-all;color:#6b7280">{{.PaymentURL | esc}}</span></p>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`

// abandoned_cart ───────────────────────────────────────────────────────────
const abandonedCartSubject = `您的購物車還在等您 — Gyeon`

const abandonedCartText = `{{if .CustomerName}}您好 {{.CustomerName}}，{{else}}您好，{{end}}

您之前選購的商品仍在購物車中。為您保留如下：

{{range .Items}}{{.Name}} × {{.Quantity}}   {{$.Currency}} {{printf "%.2f" (mul .UnitPrice .Quantity)}}
{{end}}
小計：{{.Currency}} {{printf "%.2f" .Subtotal}}

{{if .ResumeURL}}點此繼續結帳：
{{.ResumeURL}}

{{end}}如已下單可忽略此電郵。

— Gyeon`

const abandonedCartHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>購物車提醒</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">您的購物車還在等您</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">{{if .CustomerName}}您好 {{.CustomerName | esc}}，{{else}}您好，{{end}} 我們為您保留了以下商品。</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">購物車內容</h3>
      <table style="width:100%;border-collapse:collapse;font-size:14px">{{range .Items}}<tr><td style="padding:8px 0;">{{.Name | esc}} <span style="color:#9ca3af">× {{.Quantity}}</span></td><td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">{{$.Currency}} {{printf "%.2f" (mul .UnitPrice .Quantity)}}</td></tr>{{end}}</table>

      <table style="width:100%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:8px 0 0;font-weight:600">小計</td><td style="padding:8px 0 0;text-align:right;font-weight:600">{{.Currency}} {{printf "%.2f" .Subtotal}}</td></tr>
      </table>

      {{if .ResumeURL}}<div style="text-align:center;margin:24px 0 8px">
        <a href="{{.ResumeURL}}" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">繼續結帳</a>
      </div>{{end}}

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px">如已下單可忽略此電郵。</p>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon</p>
  </div>
</body></html>`

// order_confirmation ───────────────────────────────────────────────────────
const orderConfirmationSubject = `訂單確認 — {{orderref .OrderNumber .OrderID}}`

const orderConfirmationText = `您好 {{.CustomerName}}，

感謝您的訂購！我們已收到您的付款，訂單編號：{{orderref .OrderNumber .OrderID}}

──────── 訂單明細 ────────
{{range .Items}}{{.Name}} × {{.Quantity}}   {{$.Currency}} {{printf "%.2f" .LineTotal}}
{{end}}
小計：     {{.Currency}} {{printf "%.2f" .Subtotal}}
{{if gt .DiscountAmount 0.0}}折扣：    -{{.Currency}} {{printf "%.2f" .DiscountAmount}}
{{end}}{{if gt .TaxAmount 0.0}}{{if .TaxLabel}}{{.TaxLabel}}{{else}}稅金{{end}}：     {{.Currency}} {{printf "%.2f" .TaxAmount}}
{{end}}運費：     {{.Currency}} {{printf "%.2f" .ShippingFee}}
總額：     {{.Currency}} {{printf "%.2f" .Total}}

{{if .ShippingLine1}}──────── 送貨地址 ────────
{{.ShippingLine1}}
{{if .ShippingLine2}}{{.ShippingLine2}}
{{end}}{{.ShippingCity}} {{.ShippingPostal}}
{{.ShippingCountry}}

{{end}}{{if .SetupURL}}──────── 完成註冊 ────────
您是以訪客身份下單。建立帳戶後可隨時查看訂單狀態與歷史訂單：
{{.SetupURL}}

此連結將於 7 日後失效。

{{end}}如有任何疑問，歡迎回覆此電郵。

— Gyeon`

const orderConfirmationHTML = `<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>訂單確認</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">感謝您的訂購</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">您好 {{.CustomerName | esc}}，我們已收到您的付款。訂單編號 <strong style="color:#111827">{{orderref .OrderNumber .OrderID | esc}}</strong></p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">訂單明細</h3>
      <table style="width:100%;border-collapse:collapse;font-size:14px">{{range .Items}}<tr><td style="padding:8px 0;">{{.Name | esc}} <span style="color:#9ca3af">× {{.Quantity}}</span></td><td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">{{$.Currency}} {{printf "%.2f" .LineTotal}}</td></tr>{{end}}</table>

      <table style="width:100%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:4px 0;color:#6b7280">小計</td><td style="padding:4px 0;text-align:right">{{.Currency}} {{printf "%.2f" .Subtotal}}</td></tr>
        {{if gt .DiscountAmount 0.0}}<tr><td style="padding:4px 0;color:#059669">折扣</td><td style="padding:4px 0;text-align:right;color:#059669">-{{.Currency}} {{printf "%.2f" .DiscountAmount}}</td></tr>{{end}}
        {{if gt .TaxAmount 0.0}}<tr><td style="padding:4px 0;color:#6b7280">{{if .TaxLabel}}{{.TaxLabel | esc}}{{else}}稅金{{end}}</td><td style="padding:4px 0;text-align:right">{{.Currency}} {{printf "%.2f" .TaxAmount}}</td></tr>{{end}}
        <tr><td style="padding:4px 0;color:#6b7280">運費</td><td style="padding:4px 0;text-align:right">{{.Currency}} {{printf "%.2f" .ShippingFee}}</td></tr>
        <tr><td style="padding:8px 0 0;font-weight:600;border-top:1px solid #e5e7eb">總額</td><td style="padding:8px 0 0;text-align:right;font-weight:600;border-top:1px solid #e5e7eb">{{.Currency}} {{printf "%.2f" .Total}}</td></tr>
      </table>

      {{if .ShippingLine1}}<h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">送貨地址</h3>
      <p style="margin:0;color:#374151;line-height:1.6">{{.ShippingLine1 | esc}}{{if .ShippingLine2}}<br>{{.ShippingLine2 | esc}}{{end}}<br>{{.ShippingCity | esc}} {{.ShippingPostal | esc}}<br>{{.ShippingCountry | esc}}</p>{{end}}
      {{if .SetupURL}}
<div style="margin-top:32px;padding:20px;background:#f9fafb;border-radius:12px;border:1px solid #e5e7eb">
  <h3 style="margin:0 0 8px;font-size:15px;color:#111827">完成註冊以追蹤訂單</h3>
  <p style="margin:0 0 16px;color:#6b7280;font-size:14px;line-height:1.6">您是以訪客身份下單。建立帳戶後即可隨時查看訂單狀態與歷史訂單，並更快完成下次結帳。</p>
  <a href="{{.SetupURL}}" style="display:inline-block;padding:10px 20px;background:#111827;color:#fff;text-decoration:none;border-radius:8px;font-size:14px;font-weight:500">設定密碼</a>
  <p style="margin:12px 0 0;color:#9ca3af;font-size:12px">此連結將於 7 日後失效，且只可使用一次。</p>
</div>{{end}}
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`

// renderDefault parses and executes a compiled-in default template against
// `params`. On parse/exec failure (which should be impossible since defaults
// are covered by tests) it logs and returns an empty string so the send still
// attempts to deliver whatever rendered.
func renderDefault(name, body string, params any) string {
	out, _ := executeTemplate(name, body, params)
	return out
}

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}
