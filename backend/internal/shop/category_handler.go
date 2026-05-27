package shop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type CategoryHandler struct {
	svc    *CategoryService
	roleFn RoleBlockedCategoryIDsFunc
}

// RoleBlockedCategoryIDsFunc returns the category UUIDs the storefront role
// shouldn't see in the public category nav. Implemented by
// categoryrules.Service.BlockedListCategoryIDs — the per-role replacement for
// the pre-migration-103 global hidden_category_ids site setting. nil disables
// role-based filtering (used by tests / bootstrap before categoryrules wires
// in).
type RoleBlockedCategoryIDsFunc func(ctx context.Context, role string) []string

func NewCategoryHandler(svc *CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// SetRoleBlockedFn wires the per-role category filter. Call from main after
// categoryrules.Service exists.
func (h *CategoryHandler) SetRoleBlockedFn(fn RoleBlockedCategoryIDsFunc) {
	h.roleFn = fn
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
	// Public callers (storefront) get categories minus the per-role blocked
	// set (BlockedListCategoryIDs — covers both "private link" unlisted
	// categories and fully view-blocked ones for this role). Admin opts out
	// via ?include_hidden=true so it can still assign products to / pick from
	// unlisted categories. The query param name is kept for backwards
	// compatibility with the existing admin client.
	if r.URL.Query().Get("include_hidden") != "true" && h.roleFn != nil {
		role := auth.CustomerRoleFromContext(r.Context())
		blocked := h.roleFn(r.Context(), role)
		if len(blocked) > 0 {
			hide := make(map[string]struct{}, len(blocked))
			for _, id := range blocked {
				hide[id] = struct{}{}
			}
			// Allocate a fresh slice — `cats` aliases the cached list's
			// backing array, so writing through `cats[:0]` would mutate
			// the cache and a later `include_hidden=true` read would
			// surface duplicated trailing entries.
			filtered := make([]Category, 0, len(cats))
			for _, c := range cats {
				if _, blockedID := hide[c.ID]; !blockedID {
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
