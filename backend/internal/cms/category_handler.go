package cms

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type PostCategoryHandler struct {
	svc *PostCategoryService
}

func NewPostCategoryHandler(svc *PostCategoryService) *PostCategoryHandler {
	return &PostCategoryHandler{svc: svc}
}

// Routes returns both public (list) and admin (full CRUD) routes.
// Mount under /admin/cms/post-categories for admin, /cms/post-categories for public.
func (h *PostCategoryHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *PostCategoryHandler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/by-slug/{slug}", h.getBySlug)
	return r
}

func (h *PostCategoryHandler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.svc.GetBySlug(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *PostCategoryHandler) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.List(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cats)
}

func (h *PostCategoryHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cat, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *PostCategoryHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreatePostCategoryRequest
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

func (h *PostCategoryHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdatePostCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	cat, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cat)
}

func (h *PostCategoryHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
