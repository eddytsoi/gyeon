package forms

import (
	"errors"
	"time"
)

// FieldType enumerates the inline-shortcode tag names CF7 supports that we
// implement.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldEmail    FieldType = "email"
	FieldTel      FieldType = "tel"
	FieldTextarea FieldType = "textarea"
	FieldSelect   FieldType = "select"
	FieldCheckbox FieldType = "checkbox"
	FieldRadio    FieldType = "radio"
	FieldDate     FieldType = "date"
	FieldFile     FieldType = "file"
	FieldSubmit   FieldType = "submit"
	FieldHidden   FieldType = "hidden"
)

// SupportedTypes is the canonical list used by the parser to reject unknown
// type names. Keep in sync with the FieldType constants above.
var SupportedTypes = map[string]FieldType{
	"text":     FieldText,
	"email":    FieldEmail,
	"tel":      FieldTel,
	"textarea": FieldTextarea,
	"select":   FieldSelect,
	"checkbox": FieldCheckbox,
	"radio":    FieldRadio,
	"date":     FieldDate,
	"file":     FieldFile,
	"submit":   FieldSubmit,
	"hidden":   FieldHidden,
}

// FieldOption is one choice in a select/checkbox/radio field. `Value` falls
// back to `Label` when the author writes `"Yes"` instead of `"Yes|yes"`.
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FormField is the canonical parsed representation of one CF7 inline tag.
// Stored in `forms.fields` JSONB; consumed by both the validator and the
// frontend renderer.
type FormField struct {
	Type        FieldType     `json:"type"`
	Name        string        `json:"name"`
	Required    bool          `json:"required,omitempty"`
	Label       string        `json:"label,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Default     string        `json:"default,omitempty"`
	ID          string        `json:"id,omitempty"`
	Class       string        `json:"class,omitempty"`
	Size        int           `json:"size,omitempty"`
	MaxLength   int           `json:"maxlength,omitempty"`
	MinLength   int           `json:"minlength,omitempty"`
	Min         string        `json:"min,omitempty"`
	Max         string        `json:"max,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
	// File-field constraints. `MaxBytes` is the per-file size cap parsed from
	// `limit:5mb` / `limit:500kb` / `limit:1048576`; 0 means use the server
	// default (`form_upload_hard_cap_mb`). `Filetypes` is the allow-list of
	// extensions from `filetypes:pdf|jpg|png` — lowercase, no leading dot.
	MaxBytes  int64    `json:"max_bytes,omitempty"`
	Filetypes []string `json:"filetypes,omitempty"`
	Raw       string   `json:"raw,omitempty"`
}

// ParseError describes a single parser-level problem in the form markup.
// Position is the byte offset within the markup where the problem starts; the
// admin UI surfaces this so authors can jump to the offending tag.
type ParseError struct {
	Position int    `json:"position"`
	Tag      string `json:"tag,omitempty"`
	Message  string `json:"message"`
}

func (e ParseError) Error() string { return e.Message }

// Form is the back-office model. `Markup` is the editable CF7 source and
// `Fields` is the parsed result (re-derived on every save). Mail/reply
// settings carry the CF7 [field-name] placeholder syntax verbatim — they're
// translated into Go text/template at send time.
type Form struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`

	Markup string      `json:"markup"`
	Fields []FormField `json:"fields"`

	MailTo      string `json:"mail_to"`
	MailFrom    string `json:"mail_from"`
	MailSubject string `json:"mail_subject"`
	MailBody    string `json:"mail_body"`
	MailReplyTo string `json:"mail_reply_to"`

	ReplyEnabled bool   `json:"reply_enabled"`
	ReplyToField string `json:"reply_to_field"`
	ReplyFrom    string `json:"reply_from"`
	ReplySubject string `json:"reply_subject"`
	ReplyBody    string `json:"reply_body"`

	SuccessMessage string `json:"success_message"`
	ErrorMessage   string `json:"error_message"`

	RecaptchaAction string `json:"recaptcha_action"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicForm is the slim shape returned to the storefront. Mail settings are
// admin-only and never reach the client. `Markup` is the layout template the
// storefront renders verbatim, substituting each `[type* name ...]` tag with
// its input element (CF7-style); `Fields` drives state, validation, and the
// per-field error map.
type PublicForm struct {
	ID              string      `json:"id"`
	Slug            string      `json:"slug"`
	Title           string      `json:"title"`
	Markup          string      `json:"markup"`
	Fields          []FormField `json:"fields"`
	SuccessMessage  string      `json:"success_message"`
	ErrorMessage    string      `json:"error_message"`
	RecaptchaAction string      `json:"recaptcha_action"`
}

func (f Form) Public() PublicForm {
	return PublicForm{
		ID:              f.ID,
		Slug:            f.Slug,
		Title:           f.Title,
		Markup:          f.Markup,
		Fields:          f.Fields,
		SuccessMessage:  f.SuccessMessage,
		ErrorMessage:    f.ErrorMessage,
		RecaptchaAction: f.RecaptchaAction,
	}
}

// Submission is one row in form_submissions. `Data` is a flat map of field
// name → submitted value (string for scalar fields, comma-joined for
// checkbox groups, original filename for file fields). `Files` is populated
// by the detail endpoint only — the list endpoint keeps it nil to keep the
// table view payload small.
type Submission struct {
	ID             string            `json:"id"`
	FormID         string            `json:"form_id"`
	Data           map[string]string `json:"data"`
	IP             string            `json:"ip,omitempty"`
	UserAgent      string            `json:"user_agent,omitempty"`
	RecaptchaScore *float64          `json:"recaptcha_score,omitempty"`
	MailSent       bool              `json:"mail_sent"`
	MailError      string            `json:"mail_error,omitempty"`
	Files          []SubmissionFile  `json:"files,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// SubmissionFile is one uploaded attachment. The download URL is constructed
// client-side from `/api/v1/admin/forms/submissions/{sid}/files/{id}` — the
// stored on-disk filename is deliberately not exposed.
type SubmissionFile struct {
	ID           string    `json:"id"`
	FieldName    string    `json:"field_name"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

// UpsertFormRequest is the JSON body for create + update admin endpoints.
type UpsertFormRequest struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Markup string `json:"markup"`

	MailTo      string `json:"mail_to"`
	MailFrom    string `json:"mail_from"`
	MailSubject string `json:"mail_subject"`
	MailBody    string `json:"mail_body"`
	MailReplyTo string `json:"mail_reply_to"`

	ReplyEnabled bool   `json:"reply_enabled"`
	ReplyToField string `json:"reply_to_field"`
	ReplyFrom    string `json:"reply_from"`
	ReplySubject string `json:"reply_subject"`
	ReplyBody    string `json:"reply_body"`

	SuccessMessage  string `json:"success_message"`
	ErrorMessage    string `json:"error_message"`
	RecaptchaAction string `json:"recaptcha_action"`
}

// SubmitRequest is the JSON body the storefront posts to /forms/{slug}/submit.
type SubmitRequest struct {
	Data           map[string]string `json:"data"`
	RecaptchaToken string            `json:"recaptcha_token,omitempty"`
}

// SubmitResponse is the JSON returned on a successful submit.
type SubmitResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ValidationErrors maps field name → human-readable error message. Returned
// in the 422 response so the frontend can highlight invalid inputs.
type ValidationErrors map[string]string

func (v ValidationErrors) Error() string { return "validation failed" }

var (
	ErrNotFound   = errors.New("not found")
	ErrSlugExists = errors.New("slug already exists")
	ErrRecaptcha  = errors.New("recaptcha verification failed")
)
