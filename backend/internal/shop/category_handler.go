package shop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type CategoryHandler struct {
	svc      *CategoryService
	hiddenFn HiddenCategoryIDsFunc
}

// HiddenCategoryIDsFunc returns the category UUIDs that should be excluded
// from public list responses. Provided at wiring so the handler doesn't
// pull in a settings dep directly. Returning nil disables filtering.
type HiddenCategoryIDsFunc func(ctx context.Context) []string

func NewCategoryHandler(svc *CategoryService, hiddenFn HiddenCategoryIDsFunc) *CategoryHandler {
	return &CategoryHandler{svc: svc, hiddenFn: hiddenFn}
}

func (h *CategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	// Literal segments (by-slug, reorder) defined before /{id} so chi prefers them.
	r.Get("/by-slug/{slug}", h.getBySlug)
	r.Patch("/reorder", h.reorder)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *CategoryHandler) reorder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.Reorder(r.Context(), req.IDs); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CategoryHandler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.svc.GetBySlug(r.Context(), slug)
	if errors.Is(err, ErrCategoryNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.List(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	// Public callers (storefront) get categories minus the hidden set so
	// the nav + category pages don't expose private SKUs. Admin opts out
	// via ?include_hidden=true so it can still assign products to / pick
	// from hidden categories.
	if r.URL.Query().Get("include_hidden") != "true" && h.hiddenFn != nil {
		hidden := h.hiddenFn(r.Context())
		if len(hidden) > 0 {
			hide := make(map[string]struct{}, len(hidden))
			for _, id := range hidden {
				hide[id] = struct{}{}
			}
			filtered := cats[:0]
			for _, c := range cats {
				if _, blocked := hide[c.ID]; !blocked {
					filtered = append(filtered, c)
				}
			}
			cats = filtered
		}
	}
	respond.JSON(w, http.StatusOK, cats)
}

func (h *CategoryHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cat, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respond.NotFound(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	cat, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, cat)
}

func (h *CategoryHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	cat, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
