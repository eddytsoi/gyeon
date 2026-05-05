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

type Service struct {
	settings *settings.Service
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

// SendTest sends a plain test email to verify SMTP configuration.
func (s *Service) SendTest(ctx context.Context, to string) error {
	cfg, err := s.loadConfig(ctx)
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
	subject := fmt.Sprintf("完成付款 — %s", orderRef(p.OrderNumber, p.OrderID))
	html := renderPaymentLinkHTML(p)
	text := renderPaymentLinkText(p)
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
	subject := "重設您的 Gyeon 帳戶密碼"
	html := renderPasswordResetHTML(p)
	text := renderPasswordResetText(p)
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
	subject := fmt.Sprintf("店家回覆 — %s", orderRef(p.OrderNumber, ""))
	html := renderAdminMessageHTML(p)
	text := renderAdminMessageText(p)
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

	subject := fmt.Sprintf("訂單確認 — %s", orderRef(p.OrderNumber, p.OrderID))
	html := renderOrderHTML(p)
	text := renderOrderText(p)

	return s.send(cfg, p.CustomerEmail, subject, text, html)
}

// SendOrderShipped notifies the customer that their order has shipped, with
// optional carrier / tracking info.
func (s *Service) SendOrderShipped(ctx context.Context, p ShippedEmailParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("您的訂單已寄出 — %s", orderRef(p.OrderNumber, p.OrderID))
	html := renderShippedHTML(p)
	text := renderShippedText(p)
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
	subject := fmt.Sprintf("退款通知 — %s", orderRef(p.OrderNumber, p.OrderID))
	html := renderRefundHTML(p)
	text := renderRefundText(p)
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
	subject := "您的購物車還在等您 — Gyeon"
	html := renderAbandonedCartHTML(p)
	text := renderAbandonedCartText(p)
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
	subject := fmt.Sprintf("低庫存警示 — %s", p.ProductName)
	html := renderLowStockHTML(p)
	text := renderLowStockText(p)
	return s.send(cfg, to, subject, text, html)
}

func (s *Service) send(cfg Config, to, subject, text, html string) error {
	from := mime.QEncoding.Encode("utf-8", cfg.FromName) + " <" + cfg.FromEmail + ">"
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)

	boundary := "gyeon-mime-boundary-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", from)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
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

func renderOrderText(p OrderEmailParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	fmt.Fprintf(&b, "感謝您的訂購！我們已收到您的付款，訂單編號：%s\n\n", orderRef(p.OrderNumber, p.OrderID))
	b.WriteString("──────── 訂單明細 ────────\n")
	for _, it := range p.Items {
		fmt.Fprintf(&b, "%s × %d   %s %.2f\n", it.Name, it.Quantity, p.Currency, it.LineTotal)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "小計：     %s %.2f\n", p.Currency, p.Subtotal)
	if p.DiscountAmount > 0 {
		fmt.Fprintf(&b, "折扣：    -%s %.2f\n", p.Currency, p.DiscountAmount)
	}
	if p.TaxAmount > 0 {
		label := p.TaxLabel
		if label == "" {
			label = "稅金"
		}
		fmt.Fprintf(&b, "%s：     %s %.2f\n", label, p.Currency, p.TaxAmount)
	}
	fmt.Fprintf(&b, "運費：     %s %.2f\n", p.Currency, p.ShippingFee)
	fmt.Fprintf(&b, "總額：     %s %.2f\n\n", p.Currency, p.Total)

	if p.ShippingLine1 != "" {
		b.WriteString("──────── 送貨地址 ────────\n")
		fmt.Fprintf(&b, "%s\n", p.ShippingLine1)
		if p.ShippingLine2 != "" {
			fmt.Fprintf(&b, "%s\n", p.ShippingLine2)
		}
		fmt.Fprintf(&b, "%s %s\n%s\n\n", p.ShippingCity, p.ShippingPostal, p.ShippingCountry)
	}

	if p.SetupURL != "" {
		b.WriteString("──────── 完成註冊 ────────\n")
		b.WriteString("您是以訪客身份下單。建立帳戶後可隨時查看訂單狀態與歷史訂單：\n")
		fmt.Fprintf(&b, "%s\n\n", p.SetupURL)
		b.WriteString("此連結將於 7 日後失效。\n\n")
	}

	b.WriteString("如有任何疑問，歡迎回覆此電郵。\n\n— Gyeon")
	return b.String()
}

func renderOrderHTML(p OrderEmailParams) string {
	var rows strings.Builder
	for _, it := range p.Items {
		fmt.Fprintf(&rows,
			`<tr><td style="padding:8px 0;">%s <span style="color:#9ca3af">× %d</span></td>`+
				`<td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">%s %.2f</td></tr>`,
			htmlEscape(it.Name), it.Quantity, p.Currency, it.LineTotal)
	}

	var address strings.Builder
	if p.ShippingLine1 != "" {
		address.WriteString(`<h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">送貨地址</h3>`)
		address.WriteString(`<p style="margin:0;color:#374151;line-height:1.6">`)
		address.WriteString(htmlEscape(p.ShippingLine1))
		if p.ShippingLine2 != "" {
			address.WriteString("<br>" + htmlEscape(p.ShippingLine2))
		}
		fmt.Fprintf(&address, "<br>%s %s<br>%s",
			htmlEscape(p.ShippingCity), htmlEscape(p.ShippingPostal), htmlEscape(p.ShippingCountry))
		address.WriteString(`</p>`)
	}

	var setupBlock string
	if p.SetupURL != "" {
		setupBlock = fmt.Sprintf(`
<div style="margin-top:32px;padding:20px;background:#f9fafb;border-radius:12px;border:1px solid #e5e7eb">
  <h3 style="margin:0 0 8px;font-size:15px;color:#111827">完成註冊以追蹤訂單</h3>
  <p style="margin:0 0 16px;color:#6b7280;font-size:14px;line-height:1.6">您是以訪客身份下單。建立帳戶後即可隨時查看訂單狀態與歷史訂單，並更快完成下次結帳。</p>
  <a href="%s" style="display:inline-block;padding:10px 20px;background:#111827;color:#fff;text-decoration:none;border-radius:8px;font-size:14px;font-weight:500">設定密碼</a>
  <p style="margin:12px 0 0;color:#9ca3af;font-size:12px">此連結將於 7 日後失效，且只可使用一次。</p>
</div>`, p.SetupURL)
	}

	discountRow := ""
	if p.DiscountAmount > 0 {
		discountRow = fmt.Sprintf(
			`<tr><td style="padding:4px 0;color:#059669">折扣</td><td style="padding:4px 0;text-align:right;color:#059669">-%s %.2f</td></tr>`,
			p.Currency, p.DiscountAmount)
	}
	taxRow := ""
	if p.TaxAmount > 0 {
		label := p.TaxLabel
		if label == "" {
			label = "稅金"
		}
		taxRow = fmt.Sprintf(
			`<tr><td style="padding:4px 0;color:#6b7280">%s</td><td style="padding:4px 0;text-align:right">%s %.2f</td></tr>`,
			htmlEscape(label), p.Currency, p.TaxAmount)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>訂單確認</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">感謝您的訂購</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">您好 %s，我們已收到您的付款。訂單編號 <strong style="color:#111827">%s</strong></p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">訂單明細</h3>
      <table style="width:100%%;border-collapse:collapse;font-size:14px">%s</table>

      <table style="width:100%%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:4px 0;color:#6b7280">小計</td><td style="padding:4px 0;text-align:right">%s %.2f</td></tr>
        %s
        %s
        <tr><td style="padding:4px 0;color:#6b7280">運費</td><td style="padding:4px 0;text-align:right">%s %.2f</td></tr>
        <tr><td style="padding:8px 0 0;font-weight:600;border-top:1px solid #e5e7eb">總額</td><td style="padding:8px 0 0;text-align:right;font-weight:600;border-top:1px solid #e5e7eb">%s %.2f</td></tr>
      </table>

      %s
      %s
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`,
		htmlEscape(p.CustomerName), htmlEscape(orderRef(p.OrderNumber, p.OrderID)),
		rows.String(),
		p.Currency, p.Subtotal,
		discountRow,
		taxRow,
		p.Currency, p.ShippingFee,
		p.Currency, p.Total,
		address.String(),
		setupBlock,
	)
}

func renderPaymentLinkText(p PaymentLinkParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	fmt.Fprintf(&b, "您的訂單 %s 已建立，請按以下連結完成付款：\n\n", orderRef(p.OrderNumber, p.OrderID))
	fmt.Fprintf(&b, "%s\n\n", p.PaymentURL)
	b.WriteString("──────── 訂單明細 ────────\n")
	for _, it := range p.Items {
		fmt.Fprintf(&b, "%s × %d   %s %.2f\n", it.Name, it.Quantity, p.Currency, it.LineTotal)
	}
	fmt.Fprintf(&b, "\n總額：%s %.2f\n\n", p.Currency, p.Total)
	b.WriteString("此付款連結將於 24 小時後失效。付款成功後您會收到正式訂單確認電郵。\n\n— Gyeon")
	return b.String()
}

func renderPaymentLinkHTML(p PaymentLinkParams) string {
	var rows strings.Builder
	for _, it := range p.Items {
		fmt.Fprintf(&rows,
			`<tr><td style="padding:8px 0;">%s <span style="color:#9ca3af">× %d</span></td>`+
				`<td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">%s %.2f</td></tr>`,
			htmlEscape(it.Name), it.Quantity, p.Currency, it.LineTotal)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>完成付款</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">請完成付款</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">您好 %s，您的訂單 <strong style="color:#111827">%s</strong> 已建立，請按下方按鈕完成付款。</p>

      <div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">立即完成付款</a>
      </div>
      <p style="text-align:center;margin:0 0 24px;color:#9ca3af;font-size:12px">此連結將於 24 小時後失效</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">訂單明細</h3>
      <table style="width:100%%;border-collapse:collapse;font-size:14px">%s</table>

      <table style="width:100%%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:8px 0 0;font-weight:600">總額</td><td style="padding:8px 0 0;text-align:right;font-weight:600">%s %.2f</td></tr>
      </table>

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px;line-height:1.6">付款成功後您會收到正式訂單確認電郵。如連結無法開啟，請複製貼上至瀏覽器：<br><span style="word-break:break-all;color:#6b7280">%s</span></p>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`,
		htmlEscape(p.CustomerName), htmlEscape(orderRef(p.OrderNumber, p.OrderID)),
		p.PaymentURL,
		rows.String(),
		p.Currency, p.Total,
		htmlEscape(p.PaymentURL),
	)
}

func renderPasswordResetText(p PasswordResetParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	b.WriteString("我們收到了重設您 Gyeon 帳戶密碼的請求。請按以下連結設定新密碼：\n\n")
	fmt.Fprintf(&b, "%s\n\n", p.ResetURL)
	fmt.Fprintf(&b, "此連結將於 %d 小時後失效，且只可使用一次。\n\n", p.ExpiryHours)
	b.WriteString("如非本人要求，請忽略此電郵，您的密碼不會被更改。\n\n— Gyeon")
	return b.String()
}

func renderPasswordResetHTML(p PasswordResetParams) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>重設密碼</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">重設密碼</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px;line-height:1.6">您好 %s，我們收到了重設您 Gyeon 帳戶密碼的請求。請按下方按鈕設定新密碼。</p>

      <div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">重設密碼</a>
      </div>
      <p style="text-align:center;margin:0 0 24px;color:#9ca3af;font-size:12px">此連結將於 %d 小時後失效，且只可使用一次</p>

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px;line-height:1.6">如連結無法開啟，請複製貼上至瀏覽器：<br><span style="word-break:break-all;color:#6b7280">%s</span></p>

      <div style="margin-top:24px;padding-top:16px;border-top:1px solid #e5e7eb">
        <p style="margin:0;color:#9ca3af;font-size:12px;line-height:1.6">如非本人要求，請忽略此電郵，您的密碼不會被更改。</p>
      </div>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`,
		htmlEscape(p.CustomerName),
		p.ResetURL,
		p.ExpiryHours,
		htmlEscape(p.ResetURL),
	)
}

func renderAdminMessageText(p AdminMessageParams) string {
	var b strings.Builder
	if p.CustomerName != "" {
		fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	} else {
		b.WriteString("您好，\n\n")
	}
	fmt.Fprintf(&b, "您的訂單 %s 收到一則新訊息：\n\n", orderRef(p.OrderNumber, ""))
	b.WriteString("──────── 訊息內容 ────────\n")
	b.WriteString(p.Body)
	b.WriteString("\n──────────────────────────\n\n")
	if p.OrderURL != "" {
		fmt.Fprintf(&b, "查看訂單詳情或回覆：\n%s\n\n", p.OrderURL)
	}
	b.WriteString("— Gyeon")
	return b.String()
}

func renderAdminMessageHTML(p AdminMessageParams) string {
	greeting := "您好，"
	if p.CustomerName != "" {
		greeting = "您好 " + htmlEscape(p.CustomerName) + "，"
	}
	bodyHTML := strings.ReplaceAll(htmlEscape(p.Body), "\n", "<br>")
	cta := ""
	if p.OrderURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單並回覆</a>
      </div>`, p.OrderURL)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>店家回覆</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">店家回覆</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">%s 您的訂單 <strong style="color:#111827">%s</strong> 收到一則新訊息。</p>

      <div style="padding:16px;background:#f9fafb;border-left:3px solid #111827;border-radius:6px;color:#374151;font-size:14px;line-height:1.7;white-space:pre-wrap">%s</div>

      %s
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎登入帳戶回覆 — Gyeon</p>
  </div>
</body></html>`,
		greeting, htmlEscape(orderRef(p.OrderNumber, "")), bodyHTML, cta)
}

func renderShippedText(p ShippedEmailParams) string {
	var b strings.Builder
	if p.CustomerName != "" {
		fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	} else {
		b.WriteString("您好，\n\n")
	}
	fmt.Fprintf(&b, "您的訂單 %s 已經寄出！\n\n", orderRef(p.OrderNumber, p.OrderID))
	if p.Carrier != "" || p.TrackingNumber != "" {
		b.WriteString("──────── 物流資訊 ────────\n")
		if p.Carrier != "" {
			fmt.Fprintf(&b, "物流公司：%s", p.Carrier)
			if p.Service != "" {
				fmt.Fprintf(&b, "（%s）", p.Service)
			}
			b.WriteString("\n")
		}
		if p.TrackingNumber != "" {
			fmt.Fprintf(&b, "追蹤編號：%s\n", p.TrackingNumber)
		}
		if p.TrackingURL != "" {
			fmt.Fprintf(&b, "追蹤連結：%s\n", p.TrackingURL)
		}
		b.WriteString("\n")
	}
	if p.OrderURL != "" {
		fmt.Fprintf(&b, "查看訂單詳情：\n%s\n\n", p.OrderURL)
	}
	b.WriteString("如有任何疑問，歡迎回覆此電郵。\n\n— Gyeon")
	return b.String()
}

func renderShippedHTML(p ShippedEmailParams) string {
	greeting := "您好，"
	if p.CustomerName != "" {
		greeting = "您好 " + htmlEscape(p.CustomerName) + "，"
	}

	var tracking strings.Builder
	if p.Carrier != "" || p.TrackingNumber != "" {
		tracking.WriteString(`<h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:24px 0 8px">物流資訊</h3>`)
		tracking.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:14px">`)
		if p.Carrier != "" {
			carrier := htmlEscape(p.Carrier)
			if p.Service != "" {
				carrier += `<span style="color:#9ca3af"> · ` + htmlEscape(p.Service) + `</span>`
			}
			fmt.Fprintf(&tracking, `<tr><td style="padding:4px 0;color:#6b7280">物流公司</td><td style="padding:4px 0;text-align:right">%s</td></tr>`, carrier)
		}
		if p.TrackingNumber != "" {
			fmt.Fprintf(&tracking, `<tr><td style="padding:4px 0;color:#6b7280">追蹤編號</td><td style="padding:4px 0;text-align:right;font-family:ui-monospace,SFMono-Regular,Menlo,monospace">%s</td></tr>`, htmlEscape(p.TrackingNumber))
		}
		tracking.WriteString(`</table>`)
	}

	cta := ""
	if p.TrackingURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">追蹤包裹</a>
      </div>`, p.TrackingURL)
	} else if p.OrderURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單</a>
      </div>`, p.OrderURL)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>已寄出</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">您的訂單已寄出</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">%s 您的訂單 <strong style="color:#111827">%s</strong> 已交付物流公司寄出。</p>
      %s
      %s
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`,
		greeting, htmlEscape(orderRef(p.OrderNumber, p.OrderID)),
		tracking.String(), cta)
}

func renderRefundText(p RefundEmailParams) string {
	var b strings.Builder
	if p.CustomerName != "" {
		fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	} else {
		b.WriteString("您好，\n\n")
	}
	if p.IsFullRefund {
		fmt.Fprintf(&b, "您的訂單 %s 已全額退款。\n\n", orderRef(p.OrderNumber, p.OrderID))
	} else {
		fmt.Fprintf(&b, "您的訂單 %s 已部分退款。\n\n", orderRef(p.OrderNumber, p.OrderID))
	}
	b.WriteString("──────── 退款明細 ────────\n")
	fmt.Fprintf(&b, "退款金額：%s %.2f\n", p.Currency, p.RefundAmount)
	if !p.IsFullRefund {
		fmt.Fprintf(&b, "訂單總額：%s %.2f\n", p.Currency, p.OrderTotal)
	}
	if p.Reason != "" {
		fmt.Fprintf(&b, "原因：%s\n", p.Reason)
	}
	b.WriteString("\n款項將於 5–10 個工作天內退回您原本的付款方式。\n\n")
	if p.OrderURL != "" {
		fmt.Fprintf(&b, "查看訂單詳情：\n%s\n\n", p.OrderURL)
	}
	b.WriteString("如有任何疑問，歡迎回覆此電郵。\n\n— Gyeon")
	return b.String()
}

func renderRefundHTML(p RefundEmailParams) string {
	greeting := "您好，"
	if p.CustomerName != "" {
		greeting = "您好 " + htmlEscape(p.CustomerName) + "，"
	}
	heading := "退款已處理"
	intro := "您的訂單 <strong style=\"color:#111827\">" + htmlEscape(orderRef(p.OrderNumber, p.OrderID)) + "</strong> 已部分退款。"
	if p.IsFullRefund {
		intro = "您的訂單 <strong style=\"color:#111827\">" + htmlEscape(orderRef(p.OrderNumber, p.OrderID)) + "</strong> 已全額退款。"
	}

	reasonRow := ""
	if p.Reason != "" {
		reasonRow = fmt.Sprintf(`<tr><td style="padding:4px 0;color:#6b7280">原因</td><td style="padding:4px 0;text-align:right">%s</td></tr>`, htmlEscape(p.Reason))
	}

	totalRow := ""
	if !p.IsFullRefund {
		totalRow = fmt.Sprintf(`<tr><td style="padding:4px 0;color:#6b7280">訂單總額</td><td style="padding:4px 0;text-align:right">%s %.2f</td></tr>`, p.Currency, p.OrderTotal)
	}

	cta := ""
	if p.OrderURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">查看訂單</a>
      </div>`, p.OrderURL)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>退款通知</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">%s</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">%s %s</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">退款明細</h3>
      <table style="width:100%%;border-collapse:collapse;font-size:14px">
        <tr><td style="padding:4px 0;color:#6b7280">退款金額</td><td style="padding:4px 0;text-align:right;font-weight:600;color:#059669">%s %.2f</td></tr>
        %s
        %s
      </table>

      <p style="margin:24px 0 0;color:#6b7280;font-size:13px;line-height:1.6">款項將於 5–10 個工作天內退回您原本的付款方式。</p>

      %s
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">如有疑問，歡迎回覆此電郵 — Gyeon</p>
  </div>
</body></html>`,
		heading, greeting, intro,
		p.Currency, p.RefundAmount,
		totalRow, reasonRow, cta)
}

func renderAbandonedCartText(p AbandonedCartParams) string {
	var b strings.Builder
	if p.CustomerName != "" {
		fmt.Fprintf(&b, "您好 %s，\n\n", p.CustomerName)
	} else {
		b.WriteString("您好，\n\n")
	}
	b.WriteString("您之前選購的商品仍在購物車中。為您保留如下：\n\n")
	for _, it := range p.Items {
		fmt.Fprintf(&b, "%s × %d   %s %.2f\n", it.Name, it.Quantity, p.Currency, it.UnitPrice*float64(it.Quantity))
	}
	fmt.Fprintf(&b, "\n小計：%s %.2f\n\n", p.Currency, p.Subtotal)
	if p.ResumeURL != "" {
		fmt.Fprintf(&b, "點此繼續結帳：\n%s\n\n", p.ResumeURL)
	}
	b.WriteString("如已下單可忽略此電郵。\n\n— Gyeon")
	return b.String()
}

func renderAbandonedCartHTML(p AbandonedCartParams) string {
	greeting := "您好，"
	if p.CustomerName != "" {
		greeting = "您好 " + htmlEscape(p.CustomerName) + "，"
	}
	var rows strings.Builder
	for _, it := range p.Items {
		fmt.Fprintf(&rows,
			`<tr><td style="padding:8px 0;">%s <span style="color:#9ca3af">× %d</span></td>`+
				`<td style="padding:8px 0;text-align:right;font-variant-numeric:tabular-nums">%s %.2f</td></tr>`,
			htmlEscape(it.Name), it.Quantity, p.Currency, it.UnitPrice*float64(it.Quantity))
	}
	cta := ""
	if p.ResumeURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:14px 32px;background:#111827;color:#fff;text-decoration:none;border-radius:12px;font-size:15px;font-weight:600">繼續結帳</a>
      </div>`, p.ResumeURL)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>購物車提醒</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">您的購物車還在等您</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">%s 我們為您保留了以下商品。</p>

      <h3 style="font-size:13px;color:#6b7280;text-transform:uppercase;letter-spacing:.05em;margin:0 0 8px">購物車內容</h3>
      <table style="width:100%%;border-collapse:collapse;font-size:14px">%s</table>

      <table style="width:100%%;border-collapse:collapse;font-size:14px;margin-top:16px;border-top:1px solid #e5e7eb;padding-top:12px">
        <tr><td style="padding:8px 0 0;font-weight:600">小計</td><td style="padding:8px 0 0;text-align:right;font-weight:600">%s %.2f</td></tr>
      </table>

      %s

      <p style="margin:24px 0 0;color:#9ca3af;font-size:12px">如已下單可忽略此電郵。</p>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon</p>
  </div>
</body></html>`,
		greeting, rows.String(), p.Currency, p.Subtotal, cta)
}

func renderLowStockText(p LowStockParams) string {
	var b strings.Builder
	b.WriteString("低庫存警示\n\n")
	fmt.Fprintf(&b, "商品：%s", p.ProductName)
	if p.VariantName != "" {
		fmt.Fprintf(&b, "（%s）", p.VariantName)
	}
	b.WriteString("\n")
	if p.SKU != "" {
		fmt.Fprintf(&b, "SKU：%s\n", p.SKU)
	}
	fmt.Fprintf(&b, "目前庫存：%d\n", p.StockQty)
	fmt.Fprintf(&b, "閾值：%d\n\n", p.Threshold)
	if p.AdminProductURL != "" {
		fmt.Fprintf(&b, "前往補貨：\n%s\n\n", p.AdminProductURL)
	}
	b.WriteString("— Gyeon")
	return b.String()
}

func renderLowStockHTML(p LowStockParams) string {
	productLine := htmlEscape(p.ProductName)
	if p.VariantName != "" {
		productLine += `<span style="color:#9ca3af"> · ` + htmlEscape(p.VariantName) + `</span>`
	}
	skuRow := ""
	if p.SKU != "" {
		skuRow = fmt.Sprintf(`<tr><td style="padding:4px 0;color:#6b7280">SKU</td><td style="padding:4px 0;text-align:right;font-family:ui-monospace,SFMono-Regular,Menlo,monospace">%s</td></tr>`, htmlEscape(p.SKU))
	}
	cta := ""
	if p.AdminProductURL != "" {
		cta = fmt.Sprintf(`<div style="text-align:center;margin:24px 0 8px">
        <a href="%s" style="display:inline-block;padding:12px 24px;background:#111827;color:#fff;text-decoration:none;border-radius:10px;font-size:14px;font-weight:600">前往補貨</a>
      </div>`, p.AdminProductURL)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-HK"><head><meta charset="utf-8"><title>低庫存警示</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Noto Sans TC',sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 4px;font-size:22px">低庫存警示</h1>
      <p style="margin:0 0 24px;color:#6b7280;font-size:14px">以下商品的庫存已跌至閾值或以下。</p>

      <table style="width:100%%;border-collapse:collapse;font-size:14px">
        <tr><td style="padding:4px 0;color:#6b7280">商品</td><td style="padding:4px 0;text-align:right">%s</td></tr>
        %s
        <tr><td style="padding:4px 0;color:#6b7280">目前庫存</td><td style="padding:4px 0;text-align:right;font-weight:600;color:#dc2626">%d</td></tr>
        <tr><td style="padding:4px 0;color:#6b7280">閾值</td><td style="padding:4px 0;text-align:right">%d</td></tr>
      </table>

      %s
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon Admin Alert</p>
  </div>
</body></html>`,
		productLine, skuRow, p.StockQty, p.Threshold, cta)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
	)
	return r.Replace(s)
}

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}
