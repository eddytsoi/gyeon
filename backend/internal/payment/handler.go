package payment

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc       *Service
	onSuccess func(r *http.Request, paymentIntentID, paymentMethodID string)
	onFailed  func(r *http.Request, paymentIntentID, reason string)
}

// NewHandler wires the public payment routes. onSuccess is invoked from the
// webhook on `payment_intent.succeeded` events; onFailed on
// `payment_intent.payment_failed`.
func NewHandler(
	svc *Service,
	onSuccess func(r *http.Request, paymentIntentID, paymentMethodID string),
	onFailed func(r *http.Request, paymentIntentID, reason string),
) *Handler {
	return &Handler{
		svc:       svc,
		onSuccess: onSuccess,
		onFailed:  onFailed,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.config)
	r.Post("/webhook", h.webhook)
	return r
}

func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	cfg := h.svc.PublicConfig(r.Context())
	respond.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respond.BadRequest(w, "cannot read body")
		return
	}

	event, err := h.svc.VerifyWebhook(r.Context(), body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		log.Printf("stripe webhook verify failed: %v", err)
		respond.BadRequest(w, "invalid signature")
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi struct {
			ID            string          `json:"id"`
			PaymentMethod json.RawMessage `json:"payment_method"`
		}
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			log.Printf("stripe webhook decode pi: %v", err)
			respond.BadRequest(w, "bad event payload")
			return
		}
		pmID := decodePMRef(pi.PaymentMethod)
		if pi.ID != "" && h.onSuccess != nil {
			h.onSuccess(r, pi.ID, pmID)
		}

	case "payment_intent.payment_failed":
		var pi struct {
			ID               string            `json:"id"`
			LastPaymentError *lastPaymentError `json:"last_payment_error"`
		}
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			log.Printf("stripe webhook decode pi.failed: %v", err)
			respond.BadRequest(w, "bad event payload")
			return
		}
		if pi.ID != "" && h.onFailed != nil {
			h.onFailed(r, pi.ID, failureReason(pi.LastPaymentError))
		}
	}

	w.WriteHeader(http.StatusOK)
}

// lastPaymentError mirrors the subset of Stripe's last_payment_error we record
// on a failed PaymentIntent.
type lastPaymentError struct {
	Message     string `json:"message"`
	Code        string `json:"code"`
	DeclineCode string `json:"decline_code"`
}

// failureReason builds a concise human-readable reason from a failed
// PaymentIntent's last_payment_error, falling back to a generic string.
func failureReason(e *lastPaymentError) string {
	if e == nil {
		return "Payment failed"
	}
	reason := e.Message
	if reason == "" {
		reason = "Payment failed"
	}
	code := e.DeclineCode
	if code == "" {
		code = e.Code
	}
	if code != "" {
		reason += " (" + code + ")"
	}
	return reason
}

// decodePMRef tolerates both forms Stripe may send for a PaymentMethod field:
// a bare ID string ("pm_xxx") or an expanded object {"id":"pm_xxx", ...}.
// Webhooks today always send the string form, but expansion may change in
// future API versions. Returns "" for null/absent/unparseable input.
func decodePMRef(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID
	}
	return ""
}
