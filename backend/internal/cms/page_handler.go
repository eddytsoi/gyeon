package cms

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type PageHandler struct {
	svc *PageService
}

func NewPageHandler(svc *PageService) *PageHandler {
	return &PageHandler{svc: svc}
}

// AdminRoutes returns routes protected behind admin middleware (full CRUD + translation management).
func (h *PageHandler) AdminRoutes() chi.Router {
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

// PublicRoutes returns routes accessible without auth.
func (h *PageHandler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/by-slug/{slug}", h.getBySlug)
	return r
}

func (h *PageHandler) list(w http.ResponseWriter, r *http.Request) {
	pages, err := h.svc.List(r.Context(),
		r.URL.Query().Get("lang"), r.URL.Query().Get("q"))
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, pages)
}

func (h *PageHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	page, err := h.svc.GetByID(r.Context(), id, r.URL.Query().Get("lang"))
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *PageHandler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page, err := h.svc.GetBySlug(r.Context(), slug, r.URL.Query().Get("lang"))
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *PageHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreatePageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	page, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, page)
}

func (h *PageHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdatePageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	page, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *PageHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PageHandler) listTranslations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	translations, err := h.svc.ListTranslations(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, translations)
}

func (h *PageHandler) upsertTranslation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	locale := chi.URLParam(r, "locale")
	var req UpsertPageTranslationRequest
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

func (h *PageHandler) deleteTranslation(w http.ResponseWriter, r *http.Request) {
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
