package audit

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}

type listResponse struct {
	Items []Row `json:"items"`
	Total int   `json:"total"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	rows, total, err := h.svc.List(r.Context(), ListFilter{
		Action:     q.Get("action"),
		EntityType: q.Get("entity_type"),
		AdminID:    q.Get("admin_user_id"),
		From:       q.Get("from"),
		To:         q.Get("to"),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, listResponse{Items: rows, Total: total})
}
