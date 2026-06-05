package orders

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/ratelimit"
	"gyeon/backend/internal/respond"
	"gyeon/backend/internal/shop"
)

type OrderHandler struct {
	svc        *OrderService
	productSvc *shop.ProductService
	// reconcilePayment, when set, is invoked by the public success-page lookup
	// for a still-pending order: it verifies the PaymentIntent with Stripe and
	// marks the order paid synchronously. The pending→paid flip otherwise only
	// happens via the async Stripe webhook, which can lag the customer's
	// redirect to the success page (or fail to be delivered at all). Wired in
	// main.go; left nil on the admin handler.
	reconcilePayment func(ctx context.Context, paymentIntentID string)
}

// NewOrderHandler builds an OrderHandler. productSvc is optional — it is
// only required by the admin CSV-import endpoint, which resolves
// product/variant names into the variant/bundle data the admin UI needs.
// Other entry points (public checkout, list/get/etc.) work without it.
func NewOrderHandler(svc *OrderService, productSvc *shop.ProductService) *OrderHandler {
	return &OrderHandler{svc: svc, productSvc: productSvc}
}

// SetPaymentReconciler wires the synchronous Stripe reconcile used by the
// public checkout-success lookup (see OrderHandler.reconcilePayment).
func (h *OrderHandler) SetPaymentReconciler(fn func(ctx context.Context, paymentIntentID string)) {
	h.reconcilePayment = fn
}

// PublicRoutes registers the customer-facing storefront endpoints. These are
// either fully public (checkout) or authorized via a Stripe payment_intent /
// client_secret carried in the URL — so anyone holding a fresh PI from a
// completed checkout can read back the redacted order summary, but ids alone
// are never enough to enumerate orders.
func (h *OrderHandler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	// Checkout is the most expensive public path (Stripe call, DB tx) and a
	// natural target for card-testing / abuse. Throttle per-IP.
	checkoutRL := ratelimit.Middleware(10, time.Minute)
	r.With(checkoutRL).Post("/checkout", h.checkout)
	// /quote is the storefront's per-cart-change pricing preview. Read-only
	// (no DB writes, no Stripe), but it scans discount_campaigns + reads
	// product_variants for every cart line so we still throttle — at a
	// higher rate than /checkout since the checkout page calls it on every
	// cart / coupon mutation.
	quoteRL := ratelimit.Middleware(60, time.Minute)
	r.With(quoteRL).Post("/quote", h.quote)
	// Storefront resume-payment lookup: does this cart have an outstanding
	// unpaid order? Static "by-cart" segment is matched ahead of the "{id}"
	// param route below by chi. Authorized by possession of the cart_id.
	r.Get("/by-cart/{cartID}/pending", h.pendingOrderForCart)
	r.Get("/{id}", h.getPublic)
	r.Get("/{id}/payment-info", h.paymentInfo)
	r.Post("/{id}/setup-token", h.createSetupToken)
	return r
}

// AdminRoutes registers admin-only order endpoints. Mount under the admin
// auth group so callers must present a valid admin JWT. List/get/update-status
// live here too — they used to be public, which let any unauthenticated
// caller enumerate or mutate every order in the system.
func (h *OrderHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/carriers", h.listCarriers)
	r.Post("/", h.adminCreate)
	r.Post("/items/csv-resolve", h.adminResolveCSVItems)
	r.Get("/{id}", h.get)
	r.Post("/{id}/status", h.updateStatus)
	r.Patch("/{id}/shipping-address", h.updateShippingAddress)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/refund", h.refund)
	return r
}

// GetForCustomer is exposed for the customer-protected route mounted from the
// customers package (see customers.Handler). It returns the order only if it
// belongs to the authenticated customer; otherwise 404.
func (h *OrderHandler) GetForCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customerID := auth.CustomerIDFromContext(r.Context())
	order, err := h.svc.GetByIDForCustomer(r.Context(), id, customerID)
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

// getPublic backs the public GET /orders/{id} route. Authorization is via the
// `payment_intent` query parameter — the visitor must hold a PI from a
// completed checkout. The response is a redacted order (no PII).
func (h *OrderHandler) getPublic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pi := r.URL.Query().Get("payment_intent")
	if pi == "" {
		respond.NotFound(w)
		return
	}
	order, err := h.svc.GetByIDForPaymentIntent(r.Context(), id, pi)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	// The order flips pending→paid via the async Stripe webhook, which can lag
	// the customer's redirect to this success page (or fail to be delivered at
	// all). If we still see a pending order, reconcile against Stripe and
	// re-read, so the confirmation page doesn't show a stale "待付款".
	// MarkPaidByPaymentIntent no-ops once the order is already paid, so a later
	// webhook for the same PI won't double-fire the side effects.
	if order.Status == StatusPending && h.reconcilePayment != nil {
		h.reconcilePayment(r.Context(), pi)
		if updated, rerr := h.svc.GetByIDForPaymentIntent(r.Context(), id, pi); rerr == nil {
			order = updated
		}
	}
	respond.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) refund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	order, err := h.svc.IssueRefund(r.Context(), id, req)
	if errors.Is(err, ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrRefundExceedsTotal) || errors.Is(err, ErrOrderNotRefundable) {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, order)
}

func (h *OrderHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respond.NotFound(w)
			return
		}
		if errors.Is(err, ErrOrderNotDeletable) {
			respond.Error(w, http.StatusConflict,
				"Only cancelled or refunded orders can be deleted")
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *OrderHandler) pendingOrderForCart(w http.ResponseWriter, r *http.Request) {
	cartID := chi.URLParam(r, "cartID")
	res, err := h.svc.PendingOrderForCart(r.Context(), cartID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if res == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respond.JSON(w, http.StatusOK, res)
}

func (h *OrderHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	f := ListFilters{Search: q.Get("q")}

	// `status` is a comma-separated list of order_status enum values. Unknown
	// values are silently dropped so callers never get a 500 from typos in the
	// URL; an all-unknown list behaves like "no status filter".
	if raw := q.Get("status"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if isKnownStatus(s) {
				f.Statuses = append(f.Statuses, OrderStatus(s))
			}
		}
	}

	// `from` and `to` are calendar dates (YYYY-MM-DD) interpreted in the
	// server's local zone. `to` is inclusive of the named day — we add 24h
	// and use a half-open interval so created_at='2026-05-18 23:59' matches
	// to=2026-05-18.
	if raw := q.Get("from"); raw != "" {
		if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
			f.From = &t
		}
	}
	if raw := q.Get("to"); raw != "" {
		if t, err := time.ParseInLocation("2006-01-02", raw, time.Local); err == nil {
			end := t.Add(24 * time.Hour)
			f.To = &end
		}
	}

	switch q.Get("unread") {
	case "1", "true":
		f.HasUnread = true
	}

	// `role` is a comma-separated list (customer, installer, installer_v2).
	// Unknown values are silently dropped — mirrors the status param contract.
	if raw := q.Get("role"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "customer" || s == "installer" || s == "installer_v2" {
				f.Roles = append(f.Roles, s)
			}
		}
	}

	if raw := q.Get("carrier"); raw != "" {
		f.Carrier = raw
	}

	switch q.Get("pickup") {
	case "1", "true":
		t := true
		f.HasPickup = &t
	case "0", "false":
		fl := false
		f.HasPickup = &fl
	}

	switch q.Get("has_notes") {
	case "1", "true":
		f.HasNotes = true
	}

	orders, total, err := h.svc.List(r.Context(), f, limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": orders,
		"total": total,
	})
}

// listCarriers returns distinct selected_carrier values across all orders,
// sorted by frequency. Used to populate the carrier filter dropdown in the
// admin orders list without hardcoding the carrier set on the frontend.
func (h *OrderHandler) listCarriers(w http.ResponseWriter, r *http.Request) {
	carriers, err := h.svc.ListCarriers(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, carriers)
}

func isKnownStatus(s string) bool {
	switch OrderStatus(s) {
	case StatusPending, StatusPaid, StatusProcessing, StatusShipped,
		StatusDelivered, StatusCancelled, StatusRefunded:
		return true
	}
	return false
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

// quote backs POST /orders/quote — returns a pre-payment pricing breakdown
// for a cart so the storefront can show the discount line + promotion
// descriptions before the customer pays. Read-only.
func (h *OrderHandler) quote(w http.ResponseWriter, r *http.Request) {
	var req QuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.CartID == "" {
		respond.BadRequest(w, "cart_id is required")
		return
	}
	res, err := h.svc.Quote(r.Context(), req)
	if errors.Is(err, ErrCartNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, res)
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
	// Trust only the customer proven by the auth token, never a body-supplied
	// customer_id. When a customer is logged in, their verified id overrides the
	// body so the order links to the right account; their verified role decides
	// the payment method. Guests have no token: VerifiedRole stays empty ⇒
	// Stripe-only, so no one can place a no-pay bank-transfer order by spoofing
	// an installer's customer_id. (MCP/admin callers bypass this handler.)
	if vid := auth.CustomerIDFromContext(r.Context()); vid != "" {
		req.CustomerID = &vid
	}
	req.VerifiedRole = auth.CustomerRoleFromContext(r.Context())
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
	if errors.Is(err, ErrDefaultCarrierNotConfigured) {
		respond.BadRequest(w, "shipping defaults not configured")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusConflict, err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, result)
}

// adminCreate backs POST /admin/orders — manually build a new order from
// the admin panel (phone-in, walk-in, manual data entry from a customer
// service ticket). See orders.AdminCreate for the payload contract.
func (h *OrderHandler) adminCreate(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	order, err := h.svc.AdminCreate(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrAdminCreateNoItems),
			errors.Is(err, ErrAdminCreateInvalidStatus),
			errors.Is(err, ErrCustomerInfoRequired),
			errors.Is(err, ErrShippingRequired):
			respond.BadRequest(w, err.Error())
		case errors.Is(err, ErrAdminCreateVariantNotFound):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrAdminCreateInsufficientStock):
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrDefaultCarrierNotConfigured):
			respond.BadRequest(w, "shipping defaults not configured")
		default:
			respond.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	respond.JSON(w, http.StatusCreated, order)
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

// updateShippingAddress overwrites an order's frozen shipping snapshot and
// re-syncs the ShipAny waybill (if any) via the orders→shipany callback.
func (h *OrderHandler) updateShippingAddress(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req AdminShippingAddressInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	order, err := h.svc.UpdateShippingAddress(r.Context(), id, &req)
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
