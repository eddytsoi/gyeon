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
	"strconv"
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

// SettingsReader is the slice of settings.Service we need to look up the
// global upload cap and the public base URL (for admin deep-links in
// notification emails). Matches settings.Service.Get; defined as an
// interface so tests can stub without an in-memory DB.
type SettingsReader interface {
	Get(ctx context.Context, key string) (*SettingValue, error)
}

// SettingValue is the minimal shape callers consume — just the raw string
// value. Mirrors a subset of settings.Setting so we don't pull that type
// into the forms package.
type SettingValue struct {
	Value string
}

type Service struct {
	db        *sql.DB
	emailSvc  EmailSender
	recaptcha RecaptchaVerifier
	settings  SettingsReader
}

func NewService(db *sql.DB, emailSvc EmailSender, rc RecaptchaVerifier, st SettingsReader) *Service {
	return &Service{db: db, emailSvc: emailSvc, recaptcha: rc, settings: st}
}

// DefaultUploadHardCapMB is the fallback ceiling when the site setting is
// missing/unparseable. Mirrors the value seeded by migration 077.
const DefaultUploadHardCapMB = 25

// uploadHardCapBytes reads form_upload_hard_cap_mb and converts to bytes.
// Returns the default cap on any settings/parse failure so the path never
// silently lets through unbounded uploads.
func (s *Service) uploadHardCapBytes(ctx context.Context) int64 {
	mb := int64(DefaultUploadHardCapMB)
	if s.settings != nil {
		if st, err := s.settings.Get(ctx, "form_upload_hard_cap_mb"); err == nil && st != nil {
			if n, perr := strconv.ParseInt(strings.TrimSpace(st.Value), 10, 64); perr == nil && n > 0 {
				mb = n
			}
		}
	}
	return mb << 20
}

// publicBaseURL returns the absolute URL the storefront/admin is served at,
// used to build admin deep-links in notification emails. Falls back to a
// localhost value so missing config doesn't break submission flow.
func (s *Service) publicBaseURL(ctx context.Context) string {
	if s.settings != nil {
		if st, err := s.settings.Get(ctx, "public_base_url"); err == nil && st != nil {
			if v := strings.TrimRight(strings.TrimSpace(st.Value), "/"); v != "" {
				return v
			}
		}
	}
	return "http://localhost:5173"
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

// Submit is the JSON-only entry point (no file uploads). Delegates to
// SubmitWithFiles with an empty file slice so the heavy lifting lives in
// one place.
func (s *Service) Submit(ctx context.Context, slug, ip, userAgent string, req SubmitRequest) (*Submission, *Form, error) {
	return s.SubmitWithFiles(ctx, slug, ip, userAgent, req, nil)
}

// SubmitWithFiles validates the payload + uploaded files, verifies reCAPTCHA,
// stores the submission + file rows in a single transaction, copies the
// uploads to disk, and dispatches admin notification + optional auto-reply
// emails. Email failures don't fail the whole operation — matches the
// pattern used elsewhere (orders, low-stock).
//
// On any failure after the submission row is inserted, the transaction
// rolls back and the per-submission upload directory is unlinked so we
// don't leave orphan bytes on disk.
func (s *Service) SubmitWithFiles(ctx context.Context, slug, ip, userAgent string, req SubmitRequest, files []UploadedFile) (*Submission, *Form, error) {
	form, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, nil, err
	}

	if req.Data == nil {
		req.Data = map[string]string{}
	}

	// Validate text first (unknown-key check stays accurate even when file
	// fields are present), then merge file-specific errors.
	verrs := validatePayloadAllowingFiles(form.Fields, req.Data)
	if fileErrs := validateFiles(form.Fields, files, s.uploadHardCapBytes(ctx)); fileErrs != nil {
		if verrs == nil {
			verrs = ValidationErrors{}
		}
		for k, v := range fileErrs {
			verrs[k] = v
		}
	}
	if len(verrs) > 0 {
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

	// Reflect each uploaded file's original filename into req.Data so the
	// existing email body / CSV export machinery picks it up under the
	// field name without any plumbing changes.
	for _, f := range files {
		req.Data[f.FieldName] = f.Header.Filename
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, form, fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is a no-op after a successful commit, so this is safe to
	// always-defer.
	defer tx.Rollback()

	dataJSON, _ := json.Marshal(req.Data)
	sub := Submission{FormID: form.ID, Data: req.Data, IP: ip, UserAgent: userAgent, RecaptchaScore: scorePtr}
	var ipArg any
	if ip != "" {
		ipArg = ip
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO form_submissions (form_id, data, ip, user_agent, recaptcha_score)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		form.ID, dataJSON, ipArg, userAgent, scorePtr,
	).Scan(&sub.ID, &sub.CreatedAt); err != nil {
		return nil, form, fmt.Errorf("insert submission: %w", err)
	}

	// Write files to ./uploads/forms/<sid>/... and record rows. On any
	// error here we abort: rollback discards the submission row, and we
	// recursively unlink the on-disk folder so no orphan bytes remain.
	var stored []storedFile
	for _, u := range files {
		sf, saveErr := saveSubmissionFile(sub.ID, u)
		if saveErr != nil {
			_ = removeSubmissionDir(sub.ID)
			return nil, form, fmt.Errorf("save upload: %w", saveErr)
		}
		stored = append(stored, *sf)
		var fileID string
		var createdAt = sub.CreatedAt
		err := tx.QueryRowContext(ctx, `
			INSERT INTO form_submission_files
				(submission_id, field_name, original_name, stored_filename, mime_type, size_bytes)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			sub.ID, sf.FieldName, sf.OriginalName, sf.StoredName, sf.MimeType, sf.Size,
		).Scan(&fileID, &createdAt)
		if err != nil {
			_ = removeSubmissionDir(sub.ID)
			return nil, form, fmt.Errorf("insert submission file: %w", err)
		}
		sub.Files = append(sub.Files, SubmissionFile{
			ID:           fileID,
			FieldName:    sf.FieldName,
			OriginalName: sf.OriginalName,
			MimeType:     sf.MimeType,
			SizeBytes:    sf.Size,
			CreatedAt:    createdAt,
		})
	}

	if err := tx.Commit(); err != nil {
		_ = removeSubmissionDir(sub.ID)
		return nil, form, fmt.Errorf("commit submission: %w", err)
	}

	mailErr := s.dispatchEmails(ctx, form, req.Data, sub.ID, sub.Files)
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

func (s *Service) dispatchEmails(ctx context.Context, form *Form, data map[string]string, submissionID string, files []SubmissionFile) error {
	if s.emailSvc == nil {
		return errors.New("email service not configured")
	}

	// Build the attachment summary lines once — only the admin notification
	// gets the deep-link, since downloads require admin auth. Auto-reply
	// (to the submitter) doesn't need it.
	adminURL := ""
	if len(files) > 0 {
		adminURL = fmt.Sprintf("%s/admin/forms/%s/submissions", s.publicBaseURL(ctx), form.ID)
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
		Files:     attachmentSummary(files),
		AdminURL:  adminURL,
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
		Files:     attachmentSummary(files), // submitter sees filenames, no admin URL
	}
	return s.emailSvc.SendContactFormAutoReply(ctx, reply)
}

// attachmentSummary turns a SubmissionFile slice into the lines the email
// template renders as a bulleted list. Returns nil for empty input so the
// template knows to skip the section entirely.
func attachmentSummary(files []SubmissionFile) []string {
	if len(files) == 0 {
		return nil
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, fmt.Sprintf("%s (%s)", f.OriginalName, humanSize(f.SizeBytes)))
	}
	return out
}

// humanSize formats a byte count as "N B" / "N.N KB" / "N.N MB".
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
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

	files, err := s.listSubmissionFiles(ctx, id)
	if err != nil {
		return nil, err
	}
	sub.Files = files
	return &sub, nil
}

func (s *Service) listSubmissionFiles(ctx context.Context, submissionID string) ([]SubmissionFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, field_name, original_name, mime_type, size_bytes, created_at
		FROM form_submission_files
		WHERE submission_id = $1
		ORDER BY created_at ASC, id ASC`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubmissionFile
	for rows.Next() {
		var f SubmissionFile
		if err := rows.Scan(&f.ID, &f.FieldName, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// LoadSubmissionFile resolves a submission-file row and returns the on-disk
// path the admin handler streams to the client. Returns ErrNotFound if the
// file id doesn't belong to the given submission id (cheap auth: the URL
// carries both, the row must match both).
type SubmissionFileBlob struct {
	OriginalName string
	MimeType     string
	SizeBytes    int64
	DiskPath     string
}

func (s *Service) LoadSubmissionFile(ctx context.Context, submissionID, fileID string) (*SubmissionFileBlob, error) {
	var f SubmissionFileBlob
	var stored string
	err := s.db.QueryRowContext(ctx, `
		SELECT original_name, mime_type, size_bytes, stored_filename
		FROM form_submission_files
		WHERE submission_id = $1 AND id = $2`,
		submissionID, fileID).
		Scan(&f.OriginalName, &f.MimeType, &f.SizeBytes, &stored)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.DiskPath = filepathJoinUploads(submissionID, stored)
	return &f, nil
}

// filepathJoinUploads is split out so test code can swap uploadsRoot.
func filepathJoinUploads(submissionID, storedName string) string {
	return uploadsRoot + "/" + submissionID + "/" + storedName
}

func (s *Service) DeleteSubmission(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM form_submissions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	// DB rows cascade away; reclaim the on-disk folder. Errors here are
	// logged but don't fail the delete — the user's intent (remove the
	// record) succeeded.
	if rmErr := removeSubmissionDir(id); rmErr != nil {
		log.Printf("forms: delete submission %s: cleanup of uploads dir failed: %v", id, rmErr)
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
	return validatePayloadAllowingFiles(fields, data)
}

// validatePayloadAllowingFiles is the multipart-aware variant: file fields
// don't carry their value in `data` (the multipart bytes do), so we skip
// the required-non-empty check for them here — validateFiles handles it.
func validatePayloadAllowingFiles(fields []FormField, data map[string]string) ValidationErrors {
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
		// File fields skip the data-map checks; validateFiles owns them.
		if f.Type == FieldFile {
			continue
		}
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

// validateFiles enforces required-ness, allowed extensions, and the size
// cap (per-field MaxBytes, never higher than the server-wide hardCap) for
// every uploaded file. Unknown field names are rejected so a tampered
// payload can't drop a file under an arbitrary key. Returns nil when
// everything passes.
func validateFiles(fields []FormField, files []UploadedFile, hardCap int64) ValidationErrors {
	errs := ValidationErrors{}

	fileFields := make(map[string]FormField)
	for _, f := range fields {
		if f.Type == FieldFile {
			fileFields[f.Name] = f
		}
	}

	// Group uploads by field — defensively take only the first one per
	// field since [file] is single-file in CF7. Extras are silently ignored.
	seen := make(map[string]bool)
	for _, u := range files {
		f, ok := fileFields[u.FieldName]
		if !ok {
			errs[u.FieldName] = "unknown file field"
			continue
		}
		if seen[u.FieldName] {
			continue
		}
		seen[u.FieldName] = true

		if u.Header == nil || u.Header.Size == 0 || strings.TrimSpace(u.Header.Filename) == "" {
			if f.Required {
				errs[u.FieldName] = "file is required"
			}
			continue
		}

		ext := extensionOf(u.Header.Filename)
		if len(f.Filetypes) > 0 {
			allowed := false
			for _, want := range f.Filetypes {
				if ext == want {
					allowed = true
					break
				}
			}
			if !allowed {
				errs[u.FieldName] = "file type not allowed"
				continue
			}
		}

		cap := f.MaxBytes
		if cap <= 0 || cap > hardCap {
			cap = hardCap
		}
		if u.Header.Size > cap {
			errs[u.FieldName] = fmt.Sprintf("file too large (max %s)", humanSize(cap))
			continue
		}
	}

	// Required file fields that received no upload at all.
	for name, f := range fileFields {
		if !f.Required || seen[name] {
			continue
		}
		errs[name] = "file is required"
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
