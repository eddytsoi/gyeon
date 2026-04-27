package settings

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

// testEmailSender is satisfied by *email.Service without creating an import cycle.
type testEmailSender interface {
	SendTest(ctx context.Context, to string) error
}

type Handler struct {
	svc      *Service
	emailSvc testEmailSender
}

func NewHandler(svc *Service, emailSvc testEmailSender) *Handler {
	return &Handler{svc: svc, emailSvc: emailSvc}
}

// PublicRoutes — read-only access to settings (for storefront config)
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}

// AdminRoutes — full CRUD (mounted under /admin/settings)
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Put("/", h.bulkSet)
	r.Post("/test-email", h.testEmail)
	r.Get("/{key}", h.get)
	r.Put("/{key}", h.set)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.List(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, settings)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	setting, err := h.svc.Get(r.Context(), key)
	if err != nil {
		respond.NotFound(w)
		return
	}
	respond.JSON(w, http.StatusOK, setting)
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	setting, err := h.svc.Set(r.Context(), key, body.Value)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, setting)
}

func (h *Handler) testEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To == "" {
		respond.BadRequest(w, "missing or invalid 'to' address")
		return
	}
	if err := h.emailSvc.SendTest(r.Context(), body.To); err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{})
}

func (h *Handler) bulkSet(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	settings, err := h.svc.BulkSet(r.Context(), updates)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, settings)
}
