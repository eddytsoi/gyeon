package admin

import (
	"encoding/json"
	"net/http"

	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

// DashboardPrefsHandler serves the calling admin's dashboard customisation.
// Mounted at GET/PUT /admin/me/dashboard inside the admin-auth group; every
// operation is scoped to the JWT subject so an admin only ever sees/edits their
// own presets.
type DashboardPrefsHandler struct {
	svc *DashboardPrefsService
}

func NewDashboardPrefsHandler(svc *DashboardPrefsService) *DashboardPrefsHandler {
	return &DashboardPrefsHandler{svc: svc}
}

func (h *DashboardPrefsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.AdminIDFromContext(r.Context())
	if !ok || id == "" {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	prefs, err := h.svc.Get(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, prefs)
}

func (h *DashboardPrefsHandler) Put(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.AdminIDFromContext(r.Context())
	if !ok || id == "" {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var prefs DashboardPrefs
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.Save(r.Context(), id, prefs); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
