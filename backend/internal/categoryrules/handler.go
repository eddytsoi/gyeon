package categoryrules

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// AdminRoutes mounts the role↔category rules matrix endpoints. Tier-2
// (admin / super_admin) only — these settings change storefront pricing
// behavior and are not safe for editor-tier accounts.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Put("/", h.save)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.List(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// save accepts the full ruleset from the admin matrix UI. Any role on the
// payload that isn't recognised gets rejected before the transaction starts —
// SaveBulk would normalise it to "customer", which would silently corrupt
// the customer rules with rows the admin meant for some other role.
func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rules []Rule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	for _, rule := range req.Rules {
		if err := ValidateRole(rule.Role); err != nil {
			respond.BadRequest(w, "invalid role: "+rule.Role)
			return
		}
	}
	if err := h.svc.SaveBulk(r.Context(), req.Rules); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
