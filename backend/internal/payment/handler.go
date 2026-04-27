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
	onSuccess func(r *http.Request, paymentIntentID string)
}

// NewHandler wires the public payment routes. onSuccess is invoked from the
// webhook on `payment_intent.succeeded` events with the request's context.
func NewHandler(svc *Service, onSuccess func(r *http.Request, paymentIntentID string)) *Handler {
	return &Handler{svc: svc, onSuccess: onSuccess}
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
			ID string `json:"id"`
		}
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			log.Printf("stripe webhook decode pi: %v", err)
			respond.BadRequest(w, "bad event payload")
			return
		}
		if pi.ID != "" && h.onSuccess != nil {
			h.onSuccess(r, pi.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
}
