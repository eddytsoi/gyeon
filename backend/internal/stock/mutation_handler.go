package stock

import (
	"encoding/json"
	"errors"
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

// AdminRoutes registers admin-only stock-mutation endpoints. Mount under the
// admin auth group in main.go.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/duplicate", h.duplicate)
	r.Post("/{id}/execute", h.execute)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	// Allow page-based pagination as a UX convenience (?page=2 + ?per_page=50).
	if page, _ := strconv.Atoi(q.Get("page")); page > 0 {
		if limit <= 0 {
			limit = 50
			if per, _ := strconv.Atoi(q.Get("per_page")); per > 0 {
				limit = per
			}
		}
		offset = (page - 1) * limit
	}
	f := ListFilters{
		Status: q.Get("status"),
		Type:   q.Get("type"),
		From:   q.Get("from"),
		To:     q.Get("to"),
		Search: q.Get("q"),
		Limit:  limit,
		Offset: offset,
	}
	out, err := h.svc.List(r.Context(), f)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrMutationNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid JSON body")
		return
	}
	m, err := h.svc.Create(r.Context(), req)
	if writeValidationError(w, err) {
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, m)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid JSON body")
		return
	}
	m, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrMutationNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrMutationExecuted) {
		respond.Error(w, http.StatusConflict, err.Error())
		return
	}
	if writeValidationError(w, err) {
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.svc.Delete(r.Context(), id)
	if errors.Is(err, ErrMutationNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrMutationExecuted) {
		respond.Error(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.Duplicate(r.Context(), id)
	if errors.Is(err, ErrMutationNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, m)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.Execute(r.Context(), id)
	if errors.Is(err, ErrMutationNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrMutationExecuted) {
		respond.Error(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrNoItems) {
		respond.BadRequest(w, err.Error())
		return
	}
	var conflictErr *InsufficientStockError
	if errors.As(err, &conflictErr) {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":     "insufficient_stock",
			"message":   "one or more variants do not have enough on-hand stock",
			"conflicts": conflictErr.Conflicts,
		})
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, m)
}

// writeValidationError returns true (and writes the response) if err matches
// any of the known input-validation errors. Returns false otherwise so the
// caller can fall through to its own handling.
func writeValidationError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, ErrInvalidType),
		errors.Is(err, ErrNoItems),
		errors.Is(err, ErrInvalidQuantity),
		errors.Is(err, ErrDuplicateVariant),
		errors.Is(err, ErrVariantNotFound):
		respond.BadRequest(w, err.Error())
		return true
	}
	return false
}
