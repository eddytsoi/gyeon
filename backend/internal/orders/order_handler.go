package orders

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type OrderHandler struct {
	svc *OrderService
}

func NewOrderHandler(svc *OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/checkout", h.checkout)
	r.Get("/{id}/payment-info", h.paymentInfo)
	r.Post("/{id}/setup-token", h.createSetupToken)
	r.Get("/{id}", h.get)
	r.Post("/{id}/status", h.updateStatus)
	return r
}

func (h *OrderHandler) createSetupToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		PaymentIntent string `json:"payment_intent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	res, err := h.svc.CreateSetupTokenForOrder(r.Context(), id, body.PaymentIntent)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrPaymentLinkInvalid) {
		respond.Error(w, http.StatusUnauthorized, "invalid payment_intent for this order")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, res)
}

func (h *OrderHandler) paymentInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cs := r.URL.Query().Get("cs")
	info, err := h.svc.PaymentInfo(r.Context(), id, cs)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrPaymentLinkInvalid) {
		respond.Error(w, http.StatusUnauthorized, "invalid payment link")
		return
	}
	if errors.Is(err, ErrPaymentLinkExpired) {
		respond.Error(w, http.StatusGone, "payment already completed or order is no longer payable")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, info)
}

func (h *OrderHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	orders, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) checkout(w http.ResponseWriter, r *http.Request) {
	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.CartID == "" {
		respond.BadRequest(w, "cart_id is required")
		return
	}
	result, err := h.svc.Checkout(r.Context(), req)
	if errors.Is(err, ErrEmptyCart) {
		respond.BadRequest(w, "cart is empty")
		return
	}
	if errors.Is(err, ErrCartNotFound) {
		respond.BadRequest(w, "cart not found")
		return
	}
	if errors.Is(err, ErrCustomerInfoRequired) || errors.Is(err, ErrShippingRequired) {
		respond.BadRequest(w, err.Error())
		return
	}
	if err != nil {
		respond.Error(w, http.StatusConflict, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, result)
}

func (h *OrderHandler) updateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	order, err := h.svc.UpdateStatus(r.Context(), id, req)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, order)
}
