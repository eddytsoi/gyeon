package cms

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type PostHandler struct {
	svc *PostService
}

func NewPostHandler(svc *PostService) *PostHandler {
	return &PostHandler{svc: svc}
}

// AdminRoutes returns routes protected behind admin middleware (full CRUD + translation management).
func (h *PostHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/translations", h.listTranslations)
	r.Put("/{id}/translations/{locale}", h.upsertTranslation)
	r.Delete("/{id}/translations/{locale}", h.deleteTranslation)
	return r
}

// PublicRoutes returns routes accessible without auth (published only).
func (h *PostHandler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listPublished)
	r.Get("/by-slug/{slug}", h.getBySlug)
	return r
}

func (h *PostHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	posts, err := h.svc.List(r.Context(),
		r.URL.Query().Get("lang"),
		r.URL.Query().Get("q"),
		r.URL.Query().Get("category"),
		limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, posts)
}

func (h *PostHandler) listPublished(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	lang := r.URL.Query().Get("lang")
	category := r.URL.Query().Get("category")

	var posts []Post
	var err error
	if category != "" {
		posts, err = h.svc.ListPublishedByCategorySlug(r.Context(), lang, category, limit, offset)
	} else {
		posts, err = h.svc.ListPublished(r.Context(), lang, limit, offset)
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, posts)
}

func (h *PostHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	post, err := h.svc.GetByID(r.Context(), id, r.URL.Query().Get("lang"))
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, post)
}

func (h *PostHandler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	post, err := h.svc.GetBySlug(r.Context(), slug, r.URL.Query().Get("lang"))
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, post)
}

func (h *PostHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	post, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, post)
}

func (h *PostHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	post, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, post)
}

func (h *PostHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PostHandler) listTranslations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	translations, err := h.svc.ListTranslations(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, translations)
}

func (h *PostHandler) upsertTranslation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	locale := chi.URLParam(r, "locale")
	var req UpsertPostTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	t, err := h.svc.UpsertTranslation(r.Context(), id, locale, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *PostHandler) deleteTranslation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	locale := chi.URLParam(r, "locale")
	if err := h.svc.DeleteTranslation(r.Context(), id, locale); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pagination reads ?limit=&offset= query params with sensible defaults.
func pagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}
