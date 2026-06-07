package orderexpiry

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/run", h.run)
	return r
}

// run triggers the expiry sweep on demand from the admin settings page. The
// per-category thresholds (and the 0 = disabled rule) still apply, so this only
// cancels orders the periodic ticker would also have cancelled.
func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Run(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"expired": n})
}
