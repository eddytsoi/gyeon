package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/cloudflare"
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

// PublicRoutes — read-only access to the allowlisted public settings
// (storefront config). Must NOT expose Stripe/SMTP/ShipAny secrets.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listPublic)
	return r
}

// AdminRoutes — full CRUD (mounted under /admin/settings)
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listAll)
	r.Put("/", h.bulkSet)
	r.Post("/test-email", h.testEmail)
	r.Post("/purge-cloudflare", h.purgeCloudflareAll)
	r.Get("/{key}", h.get)
	r.Put("/{key}", h.set)
	return r
}

func (h *Handler) listPublic(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.ListPublic(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, settings)
}

func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.ListAll(r.Context())
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

// purgeCloudflareAll triggers a full-zone Cloudflare cache purge. Surfaces
// missing-credentials and CF API errors back to the admin UI as 400s.
func (h *Handler) purgeCloudflareAll(w http.ResponseWriter, r *http.Request) {
	zone, _ := h.svc.Get(r.Context(), "cloudflare_zone_id")
	tok, _ := h.svc.Get(r.Context(), "cloudflare_api_token")
	var zoneVal, tokVal string
	if zone != nil {
		zoneVal = zone.Value
	}
	if tok != nil {
		tokVal = tok.Value
	}
	if err := cloudflare.PurgeAll(r.Context(), zoneVal, tokVal); err != nil {
		if errors.Is(err, cloudflare.ErrNotConfigured) {
			respond.BadRequest(w, "Cloudflare credentials are not configured")
			return
		}
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
