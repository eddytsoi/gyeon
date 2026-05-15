package forms

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"regexp"
	"strings"

	"github.com/lib/pq"
	"gyeon/backend/internal/email"
)

// EmailSender is the slice of email.Service the forms package needs. Defined
// as an interface so tests can stub it cheaply.
type EmailSender interface {
	SendContactFormNotification(ctx context.Context, p email.ContactFormParams) error
	SendContactFormAutoReply(ctx context.Context, p email.ContactFormParams) error
}

// RecaptchaVerifier mirrors recaptcha.Verifier; interface so tests don't need
// HTTP plumbing.
type RecaptchaVerifier interface {
	Verify(ctx context.Context, token, expectedAction string) (float64, error)
	Enabled(ctx context.Context) bool
}

type Service struct {
	db        *sql.DB
	emailSvc  EmailSender
	recaptcha RecaptchaVerifier
}

func NewService(db *sql.DB, emailSvc EmailSender, rc RecaptchaVerifier) *Service {
	return &Service{db: db, emailSvc: emailSvc, recaptcha: rc}
}

// ──────────────────────── Admin CRUD ────────────────────────

// List returns a paginated slice of forms sorted by creation date (newest
// first). The markup column is excluded to keep the payload small; the admin
// detail page re-fetches via GetByID.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Form, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM forms`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		formSelect+` ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Form, 0)
	for rows.Next() {
		f, err := scanForm(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

func (s *Service) GetByID(ctx context.Context, id string) (*Form, error) {
	f, err := scanForm(s.db.QueryRowContext(ctx, formSelect+` WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Form, error) {
	f, err := scanForm(s.db.QueryRowContext(ctx, formSelect+` WHERE slug = $1`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ListBySlugs returns the forms whose slugs are in `slugs`. Used by the
// storefront ref resolver so a page that embeds multiple forms can fetch
// them in one round-trip.
func (s *Service) ListBySlugs(ctx context.Context, slugs []string) ([]Form, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		formSelect+` WHERE slug = ANY($1)`, pq.Array(slugs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Form, 0, len(slugs))
	for rows.Next() {
		f, err := scanForm(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Create parses the markup, rejects on hard errors (unknown type, missing
// name, duplicate name), and inserts a new row.
func (s *Service) Create(ctx context.Context, req UpsertFormRequest) (*Form, []ParseError, error) {
	if err := validateCreateUpdate(req); err != nil {
		return nil, nil, err
	}
	fields, parseErrs := ParseForm(req.Markup)
	if len(parseErrs) > 0 {
		return nil, parseErrs, nil
	}

	fieldsJSON, _ := json.Marshal(fields)
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO forms (
			slug, title, markup, fields,
			mail_to, mail_from, mail_subject, mail_body, mail_reply_to,
			reply_enabled, reply_to_field, reply_from, reply_subject, reply_body,
			success_message, error_message, recaptcha_action
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING `+formColumns,
		req.Slug, req.Title, req.Markup, fieldsJSON,
		req.MailTo, req.MailFrom, req.MailSubject, req.MailBody, req.MailReplyTo,
		req.ReplyEnabled, req.ReplyToField, req.ReplyFrom, req.ReplySubject, req.ReplyBody,
		defaultStr(req.SuccessMessage, "Thank you for your message."),
		defaultStr(req.ErrorMessage, "There was an error. Please try again."),
		defaultStr(req.RecaptchaAction, "contact_form"),
	)
	f, err := scanForm(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrSlugExists
		}
		return nil, nil, err
	}
	return &f, nil, nil
}

func (s *Service) Update(ctx context.Context, id string, req UpsertFormRequest) (*Form, []ParseError, error) {
	if err := validateCreateUpdate(req); err != nil {
		return nil, nil, err
	}
	fields, parseErrs := ParseForm(req.Markup)
	if len(parseErrs) > 0 {
		return nil, parseErrs, nil
	}
	fieldsJSON, _ := json.Marshal(fields)
	row := s.db.QueryRowContext(ctx, `
		UPDATE forms SET
			slug=$2, title=$3, markup=$4, fields=$5,
			mail_to=$6, mail_from=$7, mail_subject=$8, mail_body=$9, mail_reply_to=$10,
			reply_enabled=$11, reply_to_field=$12, reply_from=$13, reply_subject=$14, reply_body=$15,
			success_message=$16, error_message=$17, recaptcha_action=$18,
			updated_at=NOW()
		WHERE id=$1
		RETURNING `+formColumns,
		id, req.Slug, req.Title, req.Markup, fieldsJSON,
		req.MailTo, req.MailFrom, req.MailSubject, req.MailBody, req.MailReplyTo,
		req.ReplyEnabled, req.ReplyToField, req.ReplyFrom, req.ReplySubject, req.ReplyBody,
		defaultStr(req.SuccessMessage, "Thank you for your message."),
		defaultStr(req.ErrorMessage, "There was an error. Please try again."),
		defaultStr(req.RecaptchaAction, "contact_form"),
	)
	f, err := scanForm(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrSlugExists
		}
		return nil, nil, err
	}
	return &f, nil, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM forms WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ──────────────────────── Submission flow ────────────────────────

// Submit validates the payload against the form spec, verifies reCAPTCHA,
// stores the submission, and dispatches admin notification + optional
// auto-reply emails. Email failures are recorded on the submission row but
// don't fail the whole operation — matches the pattern used elsewhere in
// the codebase (orders, low-stock, etc.).
func (s *Service) Submit(ctx context.Context, slug, ip, userAgent string, req SubmitRequest) (*Submission, *Form, error) {
	form, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, nil, err
	}

	if verrs := validatePayload(form.Fields, req.Data); verrs != nil {
		return nil, form, verrs
	}

	var scorePtr *float64
	if s.recaptcha != nil {
		score, vErr := s.recaptcha.Verify(ctx, req.RecaptchaToken, form.RecaptchaAction)
		if vErr != nil {
			return nil, form, ErrRecaptcha
		}
		if s.recaptcha.Enabled(ctx) {
			scorePtr = &score
		}
	}

	// Persist submission first so we have a record even if email fails.
	dataJSON, _ := json.Marshal(req.Data)
	sub := Submission{FormID: form.ID, Data: req.Data, IP: ip, UserAgent: userAgent, RecaptchaScore: scorePtr}
	var ipArg any
	if ip != "" {
		ipArg = ip
	}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO form_submissions (form_id, data, ip, user_agent, recaptcha_score)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		form.ID, dataJSON, ipArg, userAgent, scorePtr,
	).Scan(&sub.ID, &sub.CreatedAt)
	if err != nil {
		return nil, form, fmt.Errorf("insert submission: %w", err)
	}

	mailErr := s.dispatchEmails(ctx, form, req.Data)
	if mailErr != nil {
		log.Printf("forms: submit %s: mail dispatch failed: %v", form.Slug, mailErr)
		sub.MailError = mailErr.Error()
		_, _ = s.db.ExecContext(ctx,
			`UPDATE form_submissions SET mail_sent=FALSE, mail_error=$2 WHERE id=$1`,
			sub.ID, mailErr.Error())
	} else {
		sub.MailSent = true
		_, _ = s.db.ExecContext(ctx,
			`UPDATE form_submissions SET mail_sent=TRUE WHERE id=$1`, sub.ID)
	}

	return &sub, form, nil
}

func (s *Service) dispatchEmails(ctx context.Context, form *Form, data map[string]string) error {
	if s.emailSvc == nil {
		return errors.New("email service not configured")
	}

	// Admin notification — always sent.
	notif := email.ContactFormParams{
		FormTitle: form.Title,
		To:        substitutePlaceholders(form.MailTo, data),
		From:      substitutePlaceholders(form.MailFrom, data),
		ReplyTo:   substitutePlaceholders(form.MailReplyTo, data),
		Subject:   form.MailSubject,
		Body:      form.MailBody,
		Fields:    data,
	}
	if !looksLikeEmail(notif.To) {
		return fmt.Errorf("notification recipient is not a valid email: %q", notif.To)
	}
	if err := s.emailSvc.SendContactFormNotification(ctx, notif); err != nil {
		return err
	}

	if !form.ReplyEnabled {
		return nil
	}

	// Auto-reply — recipient comes from one of the form's own email fields.
	replyTo := strings.TrimSpace(data[form.ReplyToField])
	if !looksLikeEmail(replyTo) {
		// Not a fatal error: notification already went out. Just skip the reply.
		log.Printf("forms: submit %s: auto-reply skipped (field %q is not an email)", form.Slug, form.ReplyToField)
		return nil
	}
	reply := email.ContactFormParams{
		FormTitle: form.Title,
		To:        replyTo,
		From:      substitutePlaceholders(form.ReplyFrom, data),
		Subject:   form.ReplySubject,
		Body:      form.ReplyBody,
		Fields:    data,
	}
	return s.emailSvc.SendContactFormAutoReply(ctx, reply)
}

// ──────────────────────── Submissions admin ────────────────────────

type SubmissionsPage struct {
	Items []Submission `json:"items"`
	Total int          `json:"total"`
}

func (s *Service) ListSubmissions(ctx context.Context, formID string, limit, offset int) (*SubmissionsPage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM form_submissions WHERE form_id=$1`, formID).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, form_id, data, COALESCE(ip::text, ''), user_agent, recaptcha_score, mail_sent, mail_error, created_at
		FROM form_submissions
		WHERE form_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, formID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Submission, 0)
	for rows.Next() {
		var (
			sub      Submission
			dataJSON []byte
		)
		if err := rows.Scan(&sub.ID, &sub.FormID, &dataJSON, &sub.IP, &sub.UserAgent, &sub.RecaptchaScore, &sub.MailSent, &sub.MailError, &sub.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(dataJSON, &sub.Data)
		items = append(items, sub)
	}
	return &SubmissionsPage{Items: items, Total: total}, rows.Err()
}

func (s *Service) GetSubmission(ctx context.Context, id string) (*Submission, error) {
	var (
		sub      Submission
		dataJSON []byte
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, form_id, data, COALESCE(ip::text, ''), user_agent, recaptcha_score, mail_sent, mail_error, created_at
		FROM form_submissions WHERE id=$1`, id).
		Scan(&sub.ID, &sub.FormID, &dataJSON, &sub.IP, &sub.UserAgent, &sub.RecaptchaScore, &sub.MailSent, &sub.MailError, &sub.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(dataJSON, &sub.Data)
	return &sub, nil
}

func (s *Service) DeleteSubmission(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM form_submissions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ──────────────────────── Helpers ────────────────────────

const formColumns = `id, slug, title, markup, fields,
	mail_to, mail_from, mail_subject, mail_body, mail_reply_to,
	reply_enabled, reply_to_field, reply_from, reply_subject, reply_body,
	success_message, error_message, recaptcha_action,
	created_at, updated_at`

const formSelect = `SELECT ` + formColumns + ` FROM forms`

func scanForm(row interface{ Scan(...any) error }) (Form, error) {
	var (
		f          Form
		fieldsJSON []byte
	)
	err := row.Scan(
		&f.ID, &f.Slug, &f.Title, &f.Markup, &fieldsJSON,
		&f.MailTo, &f.MailFrom, &f.MailSubject, &f.MailBody, &f.MailReplyTo,
		&f.ReplyEnabled, &f.ReplyToField, &f.ReplyFrom, &f.ReplySubject, &f.ReplyBody,
		&f.SuccessMessage, &f.ErrorMessage, &f.RecaptchaAction,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return f, err
	}
	if len(fieldsJSON) > 0 {
		_ = json.Unmarshal(fieldsJSON, &f.Fields)
	}
	if f.Fields == nil {
		f.Fields = []FormField{}
	}
	return f, nil
}

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validateCreateUpdate(req UpsertFormRequest) error {
	if !slugRE.MatchString(req.Slug) {
		return ValidationErrors{"slug": "slug must be lowercase letters, digits, and hyphens"}
	}
	if strings.TrimSpace(req.Title) == "" {
		return ValidationErrors{"title": "title is required"}
	}
	if !looksLikeEmail(req.MailTo) {
		return ValidationErrors{"mail_to": "mail_to must be a valid email address"}
	}
	if strings.TrimSpace(req.MailSubject) == "" {
		return ValidationErrors{"mail_subject": "subject is required"}
	}
	if strings.TrimSpace(req.MailBody) == "" {
		return ValidationErrors{"mail_body": "body is required"}
	}
	if req.ReplyEnabled {
		if strings.TrimSpace(req.ReplyToField) == "" {
			return ValidationErrors{"reply_to_field": "select which form field carries the submitter's email"}
		}
		if strings.TrimSpace(req.ReplySubject) == "" {
			return ValidationErrors{"reply_subject": "reply subject is required when auto-reply is enabled"}
		}
		if strings.TrimSpace(req.ReplyBody) == "" {
			return ValidationErrors{"reply_body": "reply body is required when auto-reply is enabled"}
		}
	}
	return nil
}

// validatePayload checks the submitted values against the form spec.
// Submit-only and hidden fields are skipped (hidden values come from the
// form definition's `default`, not the user). Returns nil on success.
func validatePayload(fields []FormField, data map[string]string) ValidationErrors {
	errs := ValidationErrors{}
	allowed := make(map[string]FormField, len(fields))
	for _, f := range fields {
		if f.Type == FieldSubmit {
			continue
		}
		allowed[f.Name] = f
	}

	// Reject unexpected keys — guards against payload tampering.
	for k := range data {
		if _, ok := allowed[k]; !ok {
			errs[k] = "unknown field"
		}
	}

	for name, f := range allowed {
		raw := strings.TrimSpace(data[name])
		if f.Required && raw == "" {
			errs[name] = "this field is required"
			continue
		}
		if raw == "" {
			continue
		}
		if f.MaxLength > 0 && len(raw) > f.MaxLength {
			errs[name] = fmt.Sprintf("must be at most %d characters", f.MaxLength)
			continue
		}
		if f.MinLength > 0 && len(raw) < f.MinLength {
			errs[name] = fmt.Sprintf("must be at least %d characters", f.MinLength)
			continue
		}
		switch f.Type {
		case FieldEmail:
			if !looksLikeEmail(raw) {
				errs[name] = "must be a valid email address"
			}
		case FieldSelect, FieldRadio:
			if !optionsContain(f.Options, raw) {
				errs[name] = "selected value is not one of the allowed options"
			}
		case FieldCheckbox:
			// Comma-joined: each token must match.
			for _, tok := range strings.Split(raw, ",") {
				tok = strings.TrimSpace(tok)
				if tok == "" {
					continue
				}
				if !optionsContain(f.Options, tok) {
					errs[name] = "selected value is not one of the allowed options"
					break
				}
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func optionsContain(opts []FieldOption, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

func looksLikeEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

// substitutePlaceholders does CF7 [field-name] → value rewriting in the
// To/From/ReplyTo headers (which are short strings — full template
// machinery is reserved for the body/subject in email/contact_form.go).
func substitutePlaceholders(s string, data map[string]string) string {
	if !strings.Contains(s, "[") {
		return s
	}
	return cfPlaceholderRE.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1 : len(m)-1]
		if v, ok := data[key]; ok {
			return v
		}
		return m
	})
}

var cfPlaceholderRE = regexp.MustCompile(`\[[a-zA-Z][a-zA-Z0-9_-]*\]`)

func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// isUniqueViolation peeks at the underlying pq error code to detect a slug
// collision without depending on the message text.
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
