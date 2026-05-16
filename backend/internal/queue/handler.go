package queue

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/{id}/retry", h.retry)
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
		Status: q.Get("status"),
		Type:   q.Get("type"),
		From:   q.Get("from"),
		To:     q.Get("to"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, listResponse{Items: rows, Total: total})
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respond.BadRequest(w, "missing id")
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"id": id})
}
