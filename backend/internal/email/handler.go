package email

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type TemplateHandler struct {
	store *Store
	svc   *Service
}

func NewTemplateHandler(store *Store, svc *Service) *TemplateHandler {
	return &TemplateHandler{store: store, svc: svc}
}

func (h *TemplateHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{key}", h.get)
	r.Put("/{key}", h.upsert)
	r.Post("/{key}/reset", h.reset)
	r.Post("/{key}/test", h.test)
	r.Get("/{key}/preview", h.preview)
	return r
}

type listItem struct {
	Key         string  `json:"key"`
	DisplayName string  `json:"display_name"`
	IsCustom    bool    `json:"is_custom"`
	IsEnabled   bool    `json:"is_enabled"`
	UpdatedAt   *string `json:"updated_at,omitempty"`
}

func (h *TemplateHandler) list(w http.ResponseWriter, r *http.Request) {
	overrides, err := h.store.List(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	byKey := make(map[string]Template, len(overrides))
	for _, o := range overrides {
		byKey[o.Key] = o
	}
	out := make([]listItem, 0, len(AllKeys()))
	for _, k := range AllKeys() {
		item := listItem{Key: k, DisplayName: DisplayName(k), IsCustom: false, IsEnabled: true}
		if o, ok := byKey[k]; ok {
			item.IsCustom = true
			item.IsEnabled = o.IsEnabled
			ua := o.UpdatedAt
			item.UpdatedAt = &ua
		}
		out = append(out, item)
	}
	respond.JSON(w, http.StatusOK, out)
}

type getResponse struct {
	Key         string    `json:"key"`
	DisplayName string    `json:"display_name"`
	Override    *Template `json:"override,omitempty"`
	Defaults    struct {
		Subject string `json:"subject"`
		HTML    string `json:"html"`
		Text    string `json:"text"`
	} `json:"defaults"`
	Variables []string `json:"variables"`
}

func (h *TemplateHandler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !validKey(key) {
		respond.NotFound(w)
		return
	}
	override, err := h.store.Get(r.Context(), key)
	if err != nil {
		respond.InternalError(w)
		return
	}
	def := defaultsFor(key)

	resp := getResponse{Key: key, DisplayName: DisplayName(key), Override: override, Variables: VariablesFor(key)}
	resp.Defaults.Subject = def.subject
	resp.Defaults.HTML = def.html
	resp.Defaults.Text = def.text
	respond.JSON(w, http.StatusOK, resp)
}

func (h *TemplateHandler) upsert(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !validKey(key) {
		respond.NotFound(w)
		return
	}
	var in UpsertInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if in.Subject == "" || in.HTML == "" {
		respond.BadRequest(w, "subject and html are required")
		return
	}
	if id, ok := auth.AdminIDFromContext(r.Context()); ok {
		in.UpdatedBy = &id
	}
	t, err := h.store.Upsert(r.Context(), key, in)
	var pe *ParseError
	if errors.As(err, &pe) {
		respond.Error(w, http.StatusUnprocessableEntity, pe.Error())
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *TemplateHandler) reset(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !validKey(key) {
		respond.NotFound(w)
		return
	}
	if err := h.store.Reset(r.Context(), key); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type testRequest struct {
	To string `json:"to"`
}

func (h *TemplateHandler) test(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !validKey(key) {
		respond.NotFound(w)
		return
	}
	var req testRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.To == "" {
		respond.BadRequest(w, "to is required")
		return
	}
	cfg, err := h.svc.loadConfig(r.Context())
	if err != nil {
		respond.Error(w, http.StatusServiceUnavailable, "SMTP not configured")
		return
	}
	params := SampleParamsFor(key)
	subject, htmlBody, textBody := h.svc.applyTemplate(r.Context(), key, params, func() (string, string, string) {
		def := defaultsFor(key)
		return renderDefault("test-subject:"+key, def.subject, params),
			renderDefault("test-html:"+key, def.html, params),
			renderDefault("test-text:"+key, def.text, params)
	})
	if err := h.svc.send(cfg, req.To, "[TEST] "+subject, textBody, htmlBody); err != nil {
		respond.Error(w, http.StatusBadGateway, "send failed: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type previewResponse struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

func (h *TemplateHandler) preview(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !validKey(key) {
		respond.NotFound(w)
		return
	}
	params := SampleParamsFor(key)
	subject, htmlBody, textBody := h.svc.applyTemplate(r.Context(), key, params, func() (string, string, string) {
		def := defaultsFor(key)
		return renderDefault("preview-subject:"+key, def.subject, params),
			renderDefault("preview-html:"+key, def.html, params),
			renderDefault("preview-text:"+key, def.text, params)
	})
	respond.JSON(w, http.StatusOK, previewResponse{Subject: subject, HTML: htmlBody, Text: textBody})
}

func validKey(k string) bool {
	for _, x := range AllKeys() {
		if x == k {
			return true
		}
	}
	return false
}

type defaults struct {
	subject string
	html    string
	text    string
}

func defaultsFor(key string) defaults {
	// Returns the raw template *source* (Go text/template syntax) so admins see
	// `{{.X}}` and `{{range .Items}}` in the editor when they "Reset to defaults".
	// Preview rendering happens elsewhere via SampleParamsFor + executeTemplate.
	switch key {
	case "order_confirmation":
		return defaults{subject: orderConfirmationSubject, html: orderConfirmationHTML, text: orderConfirmationText}
	case "order_shipped":
		return defaults{subject: orderShippedSubject, html: orderShippedHTML, text: orderShippedText}
	case "order_refunded":
		return defaults{subject: orderRefundedSubject, html: orderRefundedHTML, text: orderRefundedText}
	case "payment_link":
		return defaults{subject: paymentLinkSubject, html: paymentLinkHTML, text: paymentLinkText}
	case "password_reset":
		return defaults{subject: passwordResetSubject, html: passwordResetHTML, text: passwordResetText}
	case "admin_message":
		return defaults{subject: adminMessageSubject, html: adminMessageHTML, text: adminMessageText}
	case "abandoned_cart":
		return defaults{subject: abandonedCartSubject, html: abandonedCartHTML, text: abandonedCartText}
	case "low_stock_alert":
		return defaults{subject: lowStockAlertSubject, html: lowStockAlertHTML, text: lowStockAlertText}
	}
	return defaults{}
}
