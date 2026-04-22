package cms

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type NavHandler struct {
	svc *NavService
}

func NewNavHandler(svc *NavService) *NavHandler { return &NavHandler{svc: svc} }

// AdminRoutes — full CRUD behind admin middleware.
func (h *NavHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listMenus)
	r.Get("/{id}", h.getMenuByID)

	// Items within a menu
	r.Post("/{id}/items", h.addItem)
	r.Put("/{id}/items", h.replaceItems) // bulk reorder/replace
	r.Put("/{id}/items/{itemID}", h.updateItem)
	r.Delete("/{id}/items/{itemID}", h.deleteItem)
	return r
}

// PublicRoutes — read-only by handle (for the storefront).
func (h *NavHandler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/by-handle/{handle}", h.getByHandle)
	return r
}

func (h *NavHandler) listMenus(w http.ResponseWriter, r *http.Request) {
	menus, err := h.svc.ListMenus(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, menus)
}

func (h *NavHandler) getMenuByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	menu, err := h.svc.GetMenuByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, menu)
}

func (h *NavHandler) getByHandle(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	menu, err := h.svc.GetMenuByHandle(r.Context(), handle)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, menu)
}

func (h *NavHandler) addItem(w http.ResponseWriter, r *http.Request) {
	menuID := chi.URLParam(r, "id")
	var req UpsertNavItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	item, err := h.svc.AddItem(r.Context(), menuID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, item)
}

func (h *NavHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	var req UpsertNavItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	item, err := h.svc.UpdateItem(r.Context(), itemID, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, item)
}

func (h *NavHandler) deleteItem(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	if err := h.svc.DeleteItem(r.Context(), itemID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NavHandler) replaceItems(w http.ResponseWriter, r *http.Request) {
	menuID := chi.URLParam(r, "id")
	var items []UpsertNavItemRequest
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	result, err := h.svc.ReplaceItems(r.Context(), menuID, items)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}
