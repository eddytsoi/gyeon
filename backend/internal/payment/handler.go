package payment

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc               *Service
	onSuccess         func(r *http.Request, paymentIntentID string)
	onSetupSucceeded  func(r *http.Request, stripeCustomerID, stripePMID string)
	customerJWTSecret string
}

// NewHandler wires the public payment routes. onSuccess is invoked from the
// webhook on `payment_intent.succeeded` events.
func NewHandler(
	svc *Service,
	onSuccess func(r *http.Request, paymentIntentID string),
	onSetupSucceeded func(r *http.Request, stripeCustomerID, stripePMID string),
	customerJWTSecret string,
) *Handler {
	return &Handler{
		svc:               svc,
		onSuccess:         onSuccess,
		onSetupSucceeded:  onSetupSucceeded,
		customerJWTSecret: customerJWTSecret,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.config)
	r.Post("/webhook", h.webhook)
	r.Group(func(r chi.Router) {
		r.Use(auth.CustomerMiddleware(h.customerJWTSecret))
		r.Get("/saved-cards", h.listSavedCards)
		r.Delete("/saved-cards/{id}", h.deleteSavedCard)
		r.Put("/saved-cards/{id}/default", h.setDefaultCard)
	})
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

	case "setup_intent.succeeded":
		var si struct {
			Customer      string `json:"customer"`
			PaymentMethod string `json:"payment_method"`
		}
		if err := json.Unmarshal(event.Data.Raw, &si); err != nil {
			log.Printf("stripe webhook decode si: %v", err)
			respond.BadRequest(w, "bad event payload")
			return
		}
		if si.Customer != "" && si.PaymentMethod != "" && h.onSetupSucceeded != nil {
			h.onSetupSucceeded(r, si.Customer, si.PaymentMethod)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) listSavedCards(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	cards, err := h.svc.ListSavedPaymentMethods(r.Context(), customerID)
	if err != nil {
		log.Printf("list saved cards for customer %s: %v", customerID, err)
		respond.Error(w, http.StatusInternalServerError, "could not list saved cards")
		return
	}
	if cards == nil {
		cards = []SavedPaymentMethod{}
	}
	respond.JSON(w, http.StatusOK, cards)
}

func (h *Handler) deleteSavedCard(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.DetachPaymentMethod(r.Context(), id, customerID); err != nil {
		log.Printf("delete saved card %s for customer %s: %v", id, customerID, err)
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setDefaultCard(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.SetDefaultPaymentMethod(r.Context(), id, customerID); err != nil {
		log.Printf("set default card %s for customer %s: %v", id, customerID, err)
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
