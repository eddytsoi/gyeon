package email

import (
	"context"
	"fmt"
	htmltmpl "html/template"
	"regexp"
	"strings"
	texttmpl "text/template"
)

// ContactFormParams carries every value needed to render a contact-form
// notification or auto-reply email. Subject/From/ReplyTo/Body all come from
// the admin-configured form definition; FormTitle and Fields drive
// substitution and a default-rendered HTML fallback when the body itself is
// plain text.
type ContactFormParams struct {
	FormTitle string
	To        string
	From      string // when blank, the SMTP `from_email` is used.
	ReplyTo   string // already-substituted email address.
	Subject   string // CF7 [field-name] placeholders allowed
	Body      string // CF7 [field-name] placeholders allowed; plain text
	Fields    map[string]string
}

// SendContactFormNotification renders + sends the per-form admin
// notification. The body and subject carry CF7 [field-name] placeholders
// which are rewritten into Go text/template syntax just before execution.
// Errors are returned to the caller; the forms service logs them as
// non-fatal (the submission row is still kept).
func (s *Service) SendContactFormNotification(ctx context.Context, p ContactFormParams) error {
	return s.sendContactFormMail(ctx, p)
}

// SendContactFormAutoReply renders + sends the per-form auto-reply to the
// submitter. Same template machinery as the notification.
func (s *Service) SendContactFormAutoReply(ctx context.Context, p ContactFormParams) error {
	return s.sendContactFormMail(ctx, p)
}

func (s *Service) sendContactFormMail(ctx context.Context, p ContactFormParams) error {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if p.To == "" {
		return fmt.Errorf("contact form mail: empty recipient")
	}

	subject, err := renderContactFormPart("subject", p.Subject, p.Fields)
	if err != nil {
		return fmt.Errorf("contact form subject: %w", err)
	}
	text, err := renderContactFormPart("body", p.Body, p.Fields)
	if err != nil {
		return fmt.Errorf("contact form body: %w", err)
	}
	html, err := renderContactFormHTML(p.FormTitle, text)
	if err != nil {
		return fmt.Errorf("contact form html: %w", err)
	}

	from := cfg
	if p.From != "" {
		from.FromEmail = p.From
	}
	return s.sendWithReplyTo(from, p.To, p.ReplyTo, subject, text, html)
}

// cf7FieldRE matches a CF7 placeholder like `[your-name]` or `[your_email]`
// inside the subject/body so we can rewrite it into Go-template syntax.
// Brackets around plain text that doesn't look like a field name are left
// alone (e.g. `[Important]` stays literal).
var cf7FieldRE = regexp.MustCompile(`\[([a-zA-Z][a-zA-Z0-9_-]*)\]`)

// rewriteCF7Placeholders rewrites `[field-name]` into `{{ index .Fields "field-name" }}`
// which Go text/template can execute against ContactFormParams.Fields.
// Hyphens in CF7 names aren't valid Go identifiers, so we use the `index`
// builtin rather than `.Fields.field-name`.
func rewriteCF7Placeholders(s string) string {
	return cf7FieldRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := cf7FieldRE.FindStringSubmatch(m)
		return fmt.Sprintf(`{{ index .Fields %q }}`, sub[1])
	})
}

func renderContactFormPart(name, body string, fields map[string]string) (string, error) {
	tmpl, err := texttmpl.New(name).Parse(rewriteCF7Placeholders(body))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, struct {
		Fields map[string]string
	}{Fields: fields}); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// contactFormHTMLTemplate wraps the plain text body in a minimal HTML
// document so MIME multipart/alternative has both parts. The body is
// escaped — admins write plain text in the form editor, not raw HTML.
var contactFormHTMLTemplate = htmltmpl.Must(htmltmpl.New("cf-html").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#111827">
  <div style="max-width:560px;margin:0 auto;padding:32px 16px">
    <div style="background:#fff;border-radius:16px;padding:32px;border:1px solid #e5e7eb">
      <h1 style="margin:0 0 16px;font-size:18px">{{.Title}}</h1>
      <pre style="white-space:pre-wrap;font:14px/1.6 -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#374151;margin:0">{{.Body}}</pre>
    </div>
    <p style="text-align:center;color:#9ca3af;font-size:12px;margin:24px 0 0">— Gyeon</p>
  </div>
</body></html>`))

func renderContactFormHTML(title, body string) (string, error) {
	var sb strings.Builder
	if err := contactFormHTMLTemplate.Execute(&sb, struct {
		Title string
		Body  string
	}{Title: title, Body: body}); err != nil {
		return "", err
	}
	return sb.String(), nil
}
