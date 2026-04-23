package pricing

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// AdminRoutes returns admin-protected routes for managing campaigns and coupons.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Route("/campaigns", func(r chi.Router) {
		r.Get("/", h.listCampaigns)
		r.Post("/", h.createCampaign)
		r.Get("/{id}", h.getCampaign)
		r.Put("/{id}", h.updateCampaign)
		r.Delete("/{id}", h.deleteCampaign)
	})

	r.Route("/coupons", func(r chi.Router) {
		r.Get("/", h.listCoupons)
		r.Post("/", h.createCoupon)
		r.Get("/{id}", h.getCoupon)
		r.Put("/{id}", h.updateCoupon)
		r.Delete("/{id}", h.deleteCoupon)
	})

	return r
}

// PublicRoutes returns customer-facing routes (coupon validation preview).
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/validate-coupon", h.validateCoupon)
	return r
}

// --- Campaigns ---

func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.svc.ListCampaigns(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, campaigns)
}

func (h *Handler) getCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaign, err := h.svc.GetCampaign(r.Context(), id)
	if errors.Is(err, ErrCampaignNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, campaign)
}

func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request) {
	var req CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" {
		respond.BadRequest(w, "name is required")
		return
	}
	if req.DiscountValue <= 0 {
		respond.BadRequest(w, "discount_value must be positive")
		return
	}
	campaign, err := h.svc.CreateCampaign(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, campaign)
}

func (h *Handler) updateCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	campaign, err := h.svc.UpdateCampaign(r.Context(), id, req)
	if errors.Is(err, ErrCampaignNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, campaign)
}

func (h *Handler) deleteCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteCampaign(r.Context(), id); err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Coupons ---

func (h *Handler) listCoupons(w http.ResponseWriter, r *http.Request) {
	coupons, err := h.svc.ListCoupons(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, coupons)
}

func (h *Handler) getCoupon(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	coupon, err := h.svc.GetCoupon(r.Context(), id)
	if errors.Is(err, ErrCouponNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, coupon)
}

func (h *Handler) createCoupon(w http.ResponseWriter, r *http.Request) {
	var req CreateCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Code == "" {
		respond.BadRequest(w, "code is required")
		return
	}
	if req.DiscountValue <= 0 {
		respond.BadRequest(w, "discount_value must be positive")
		return
	}
	coupon, err := h.svc.CreateCoupon(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, coupon)
}

func (h *Handler) updateCoupon(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	coupon, err := h.svc.UpdateCoupon(r.Context(), id, req)
	if errors.Is(err, ErrCouponNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, coupon)
}

func (h *Handler) deleteCoupon(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteCoupon(r.Context(), id); err != nil {
		if errors.Is(err, ErrCouponNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Public ---

type validateCouponRequest struct {
	Code     string  `json:"code"`
	Subtotal float64 `json:"subtotal"`
}

type validateCouponResponse struct {
	Valid          bool    `json:"valid"`
	DiscountType   string  `json:"discount_type,omitempty"`
	DiscountValue  float64 `json:"discount_value,omitempty"`
	DiscountAmount float64 `json:"discount_amount,omitempty"`
	Message        string  `json:"message,omitempty"`
}

func (h *Handler) validateCoupon(w http.ResponseWriter, r *http.Request) {
	var req validateCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Code == "" {
		respond.BadRequest(w, "code is required")
		return
	}

	coupon, err := h.svc.ValidateCoupon(r.Context(), req.Code, req.Subtotal)
	if err != nil {
		msg := "invalid coupon"
		switch {
		case errors.Is(err, ErrCouponExpired):
			msg = "coupon has expired"
		case errors.Is(err, ErrCouponExhausted):
			msg = "coupon usage limit reached"
		case errors.Is(err, ErrCouponMinOrder):
			msg = "order amount below coupon minimum"
		}
		respond.JSON(w, http.StatusOK, validateCouponResponse{Valid: false, Message: msg})
		return
	}

	var discountAmount float64
	switch coupon.DiscountType {
	case DiscountPercentage:
		discountAmount = req.Subtotal * (coupon.DiscountValue / 100)
	case DiscountFixed:
		discountAmount = coupon.DiscountValue
		if discountAmount > req.Subtotal {
			discountAmount = req.Subtotal
		}
	}

	respond.JSON(w, http.StatusOK, validateCouponResponse{
		Valid:          true,
		DiscountType:   string(coupon.DiscountType),
		DiscountValue:  coupon.DiscountValue,
		DiscountAmount: discountAmount,
	})
}
