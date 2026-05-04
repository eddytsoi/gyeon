package shipany

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc     *Service
	cartSvc *orders.CartService
}

func NewHandler(svc *Service, cartSvc *orders.CartService) *Handler {
	return &Handler{svc: svc, cartSvc: cartSvc}
}

// PublicRoutes — quote + pickup-point lookup for the storefront,
// plus the webhook receiver. None of these require authentication.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/quote", h.quote)
	r.Get("/pickup-points", h.pickupPoints)
	r.Post("/webhook", h.webhook)
	return r
}

// AdminRoutes — fulfilment actions on a single order.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/test-connection", h.testConnection)
	r.Get("/couriers", h.listCouriers)
	r.Get("/orders/{id}/shipment", h.getShipment)
	r.Post("/orders/{id}/shipment", h.createShipment)
	r.Post("/orders/{id}/pickup", h.requestPickup)
	return r
}

// ── Public ─────────────────────────────────────────────────────────────

type quoteRequest struct {
	CartID          string  `json:"cart_id"`
	ShippingAddress Address `json:"shipping_address"`
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request) {
	var req quoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.CartID == "" {
		respond.BadRequest(w, "cart_id is required")
		return
	}
	if req.ShippingAddress.Line1 == "" {
		respond.BadRequest(w, "shipping_address.line1 is required")
		return
	}

	cart, err := h.cartSvc.GetByID(r.Context(), req.CartID)
	if err != nil {
		respond.BadRequest(w, "cart not found")
		return
	}

	// Per-variant weight is now persisted (product_variants.weight_grams).
	// Lines without it fall back to shipany_default_weight_grams in
	// QuoteForCart.
	lines := make([]CartLine, len(cart.Items))
	subtotal := 0.0
	for i, item := range cart.Items {
		w := 0
		if item.WeightGrams != nil {
			w = *item.WeightGrams
		}
		l, wid, h := 0, 0, 0
		if item.LengthMM != nil {
			l = *item.LengthMM
		}
		if item.WidthMM != nil {
			wid = *item.WidthMM
		}
		if item.HeightMM != nil {
			h = *item.HeightMM
		}
		lines[i] = CartLine{WeightGrams: w, Quantity: item.Quantity, LengthMM: l, WidthMM: wid, HeightMM: h}
		subtotal += item.Price * float64(item.Quantity)
	}

	rates, err := h.svc.QuoteForCart(r.Context(), req.ShippingAddress, lines, subtotal)
	if errors.Is(err, ErrNotConfigured) {
		respond.JSON(w, http.StatusOK, []RateOption{})
		return
	}
	if err != nil {
		log.Printf("shipany quote: %v", err)
		respond.JSON(w, http.StatusOK, []RateOption{})
		return
	}
	if rates == nil {
		rates = []RateOption{}
	}
	respond.JSON(w, http.StatusOK, rates)
}

func (h *Handler) pickupPoints(w http.ResponseWriter, r *http.Request) {
	carrier := r.URL.Query().Get("carrier")
	district := r.URL.Query().Get("district")
	if carrier == "" {
		respond.BadRequest(w, "carrier query param is required")
		return
	}
	points, err := h.svc.PickupPoints(r.Context(), carrier, district)
	if errors.Is(err, ErrNotConfigured) {
		respond.JSON(w, http.StatusOK, []PickupPoint{})
		return
	}
	if err != nil {
		log.Printf("shipany pickup-points: %v", err)
		respond.JSON(w, http.StatusOK, []PickupPoint{})
		return
	}
	if points == nil {
		points = []PickupPoint{}
	}
	respond.JSON(w, http.StatusOK, points)
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respond.BadRequest(w, "cannot read body")
		return
	}

	// Header name is a best guess until portal docs are confirmed —
	// log on first hit so the implementer can compare.
	sig := r.Header.Get("X-ShipAny-Signature")
	if err := h.svc.VerifyWebhook(r.Context(), body, sig); err != nil {
		log.Printf("shipany webhook signature failed (sig header=%q): %v", sig, err)
		respond.Error(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	evt, err := ParseEvent(body)
	if err != nil {
		log.Printf("shipany webhook decode: %v body=%s", err, truncate(string(body), 256))
		// Return 200 anyway so ShipAny stops retrying — body is logged.
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := h.svc.HandleTrackingEvent(r.Context(), *evt, body); err != nil {
		log.Printf("shipany webhook handle %s: %v", evt.Event, err)
		// Still 200: a 500 makes ShipAny retry, which can amplify a bug.
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ── Admin ──────────────────────────────────────────────────────────────

type listCouriersResponse struct {
	Couriers []Courier `json:"couriers"`
	Error    string    `json:"error,omitempty"`
}

func (h *Handler) listCouriers(w http.ResponseWriter, r *http.Request) {
	couriers, err := h.svc.ListCouriers(r.Context())
	if errors.Is(err, ErrNotConfigured) {
		respond.JSON(w, http.StatusOK, listCouriersResponse{Couriers: []Courier{}, Error: "ShipAny is not configured."})
		return
	}
	if err != nil {
		log.Printf("shipany list-couriers: %v", err)
		respond.JSON(w, http.StatusOK, listCouriersResponse{Couriers: []Courier{}, Error: err.Error()})
		return
	}
	if couriers == nil {
		couriers = []Courier{}
	}
	respond.JSON(w, http.StatusOK, listCouriersResponse{Couriers: couriers})
}

func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	if !h.svc.Configured(r.Context()) {
		respond.JSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": "ShipAny is not enabled or credentials are blank.",
		})
		return
	}
	if err := h.svc.client.Ping(r.Context()); err != nil {
		respond.JSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Connected.",
	})
}

func (h *Handler) getShipment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	sh, err := h.svc.GetByOrderID(r.Context(), orderID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if sh == nil {
		respond.JSON(w, http.StatusOK, nil)
		return
	}
	respond.JSON(w, http.StatusOK, sh)
}

type createShipmentRequest struct {
	Carrier string `json:"carrier,omitempty"`
	Service string `json:"service,omitempty"`
}

func (h *Handler) createShipment(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	var body createShipmentRequest
	_ = json.NewDecoder(r.Body).Decode(&body) // body is optional

	var override *RateOption
	if body.Carrier != "" && body.Service != "" {
		override = &RateOption{Carrier: body.Carrier, Service: body.Service}
	}

	sh, err := h.svc.CreateForOrder(r.Context(), orderID, override)
	if errors.Is(err, ErrShipmentExists) {
		respond.Error(w, http.StatusConflict, "shipment already exists for this order")
		return
	}
	if errors.Is(err, ErrCarrierNotSelected) {
		respond.BadRequest(w, "no carrier selected on this order — pass {carrier, service}")
		return
	}
	if errors.Is(err, ErrNotConfigured) {
		respond.Error(w, http.StatusServiceUnavailable, "ShipAny is not configured")
		return
	}
	if err != nil {
		log.Printf("shipany createShipment: %v", err)
		respond.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, sh)
}

func (h *Handler) requestPickup(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	sh, err := h.svc.RequestPickup(r.Context(), orderID)
	if errors.Is(err, ErrShipmentNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrNotConfigured) {
		respond.Error(w, http.StatusServiceUnavailable, "ShipAny is not configured")
		return
	}
	if err != nil {
		log.Printf("shipany requestPickup: %v", err)
		respond.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, sh)
}
