package orders

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type CartHandler struct {
	svc *CartService
}

func NewCartHandler(svc *CartService) *CartHandler {
	return &CartHandler{svc: svc}
}

func (h *CartHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.getOrCreate)
	r.Get("/{id}", h.get)
	r.Post("/{id}/items", h.addItem)
	r.Put("/{id}/items/{itemID}", h.updateItem)
	r.Delete("/{id}/items/{itemID}", h.removeItem)
	return r
}

func (h *CartHandler) getOrCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionToken string  `json:"session_token"`
		CustomerID   *string `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionToken == "" {
		respond.BadRequest(w, "session_token is required")
		return
	}
	cart, err := h.svc.GetOrCreate(r.Context(), body.SessionToken, body.CustomerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cart, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrCartNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cart)
}

func (h *CartHandler) addItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	item, err := h.svc.AddItem(r.Context(), id, req)
	if errors.Is(err, ErrInsufficientStock) {
		respond.Error(w, http.StatusConflict, "insufficient stock")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, item)
}

func (h *CartHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")
	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	item, err := h.svc.UpdateItem(r.Context(), id, itemID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if item == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respond.JSON(w, http.StatusOK, item)
}

func (h *CartHandler) removeItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")
	if err := h.svc.RemoveItem(r.Context(), id, itemID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
