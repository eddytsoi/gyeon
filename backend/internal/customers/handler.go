package customers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/ratelimit"
	"gyeon/backend/internal/respond"
)

// OrderFetcherFunc loads a single order if and only if it belongs to the
// given customer. Wired from main.go to orders.OrderService.GetByIDForCustomer
// via a closure, so the customers package doesn't take an orders import (which
// would create a cycle — orders already depends on customers).
type OrderFetcherFunc func(ctx context.Context, orderID, customerID string) (any, error)

// EmailSender is the slice of email.Service the customers handler needs.
type EmailSender interface {
	PublicBaseURL(ctx context.Context) string
	SendPasswordResetEmail(ctx context.Context, p email.PasswordResetParams) error
}

type Handler struct {
	svc          *Service
	emailSvc     EmailSender
	jwtSecret    string
	fetchOrder   OrderFetcherFunc
}

func NewHandler(svc *Service, emailSvc EmailSender, jwtSecret string, fetchOrder OrderFetcherFunc) *Handler {
	return &Handler{svc: svc, emailSvc: emailSvc, jwtSecret: jwtSecret, fetchOrder: fetchOrder}
}

// Routes combines public and authenticated customer routes under one router.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	// Per-IP throttles on credential-bearing endpoints. Single limiter shared
	// across register/login/setup so an attacker can't ping-pong between
	// them to skirt the budget.
	authRL := ratelimit.Middleware(10, 5*time.Minute)
	forgotRL := ratelimit.Middleware(5, 10*time.Minute)
	r.With(authRL).Post("/register", h.register)
	r.With(authRL).Post("/login", h.login)
	r.With(authRL).Post("/setup-password", h.setupPassword)
	r.With(forgotRL).Post("/forgot-password", h.forgotPassword)
	r.Group(func(r chi.Router) {
		r.Use(auth.CustomerMiddleware(h.jwtSecret))
		r.Get("/me", h.getProfile)
		r.Put("/me", h.updateProfile)
		r.Get("/me/addresses", h.listAddresses)
		r.Post("/me/addresses", h.createAddress)
		r.Put("/me/addresses/{addressID}", h.updateAddress)
		r.Delete("/me/addresses/{addressID}", h.deleteAddress)
		r.Get("/me/orders", h.listOrders)
		r.Get("/me/orders/lookup/{number}", h.lookupOrder)
		r.Get("/me/orders/{id}", h.getOrder)
		r.Post("/me/sign-out-everywhere", h.signOutEverywhere)
	})
	return r
}

// PublicRoutes — kept for compatibility; delegates to Routes.
func (h *Handler) PublicRoutes() chi.Router { return h.Routes() }

// AuthenticatedRoutes — kept for compatibility; routes are embedded in Routes.
func (h *Handler) AuthenticatedRoutes() chi.Router { return chi.NewRouter() }

// AdminRoutes — list all customers (admin JWT required, mounted separately)
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.getByID)
	r.Post("/{id}/send-reset-password-email", h.sendResetPasswordEmail)
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		respond.BadRequest(w, "email, password, first_name and last_name are required")
		return
	}
	customer, err := h.svc.Register(r.Context(), req)
	if errors.Is(err, ErrEmailTaken) {
		respond.Error(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	tv, _ := h.svc.TokenVersion(r.Context(), customer.ID)
	token, err := auth.GenerateCustomerToken(h.jwtSecret, customer.ID, tv)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]interface{}{
		"customer": customer,
		"token":    token,
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	customer, err := h.svc.Login(r.Context(), req)
	if errors.Is(err, ErrInvalidCredentials) {
		respond.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	tv, _ := h.svc.TokenVersion(r.Context(), customer.ID)
	token, err := auth.GenerateCustomerToken(h.jwtSecret, customer.ID, tv)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"customer": customer,
		"token":    token,
	})
}

func (h *Handler) setupPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		respond.BadRequest(w, "token and password are required")
		return
	}
	if err := h.svc.ConsumeSetupToken(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			respond.Error(w, http.StatusGone, "this link is invalid or has expired")
			return
		}
		respond.BadRequest(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	customer, err := h.svc.GetByID(r.Context(), customerID)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, customer)
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	customer, err := h.svc.UpdateProfile(r.Context(), customerID, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, customer)
}

func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	addrs, err := h.svc.ListAddresses(r.Context(), customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, addrs)
}

func (h *Handler) createAddress(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	var req CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	addr, err := h.svc.CreateAddress(r.Context(), customerID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, addr)
}

func (h *Handler) updateAddress(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	addressID := chi.URLParam(r, "addressID")
	var req CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	addr, err := h.svc.UpdateAddress(r.Context(), customerID, addressID, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, addr)
}

func (h *Handler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	addressID := chi.URLParam(r, "addressID")
	if err := h.svc.DeleteAddress(r.Context(), customerID, addressID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	orders, err := h.svc.ListOrders(r.Context(), customerID, limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, orders)
}

// signOutEverywhere increments this customer's token_version, instantly
// invalidating every previously issued JWT. The current token also stops
// working; the client is expected to drop its cookie and prompt re-login.
func (h *Handler) signOutEverywhere(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	if customerID == "" {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if _, err := h.svc.IncrementTokenVersion(r.Context(), customerID); err != nil {
		respond.InternalError(w)
		return
	}
	// Drop the cached tv so the very next request from any node sees the
	// new value (otherwise revocation has cache-TTL of lag).
	auth.InvalidateCustomerVersion(customerID)
	w.WriteHeader(http.StatusNoContent)
}

// getOrder returns an order's detail only if it belongs to the authenticated
// customer. Backed by an OrderFetcherFunc wired from main.go to avoid an
// orders→customers→orders import cycle.
func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	if h.fetchOrder == nil {
		respond.NotFound(w)
		return
	}
	customerID := auth.CustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	order, err := h.fetchOrder(r.Context(), id, customerID)
	if err != nil {
		// Treat any error as "not found" to avoid leaking ownership signals.
		respond.NotFound(w)
		return
	}
	respond.JSON(w, http.StatusOK, order)
}

// lookupOrder resolves a sequential order display number (e.g. ORD-8 → 8)
// to the underlying UUID. Customer-scoped: only resolves orders owned by the
// authenticated customer, so sequential IDs cannot be enumerated.
func (h *Handler) lookupOrder(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	n, err := strconv.ParseInt(chi.URLParam(r, "number"), 10, 64)
	if err != nil || n <= 0 {
		respond.NotFound(w)
		return
	}
	id, err := h.svc.GetOrderIDByNumber(r.Context(), customerID, n)
	if errors.Is(err, sql.ErrNoRows) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"id": id})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	customers, total, err := h.svc.List(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": customers,
		"total": total,
	})
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, customer)
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		respond.BadRequest(w, "email is required")
		return
	}

	customer, err := h.svc.GetByEmail(r.Context(), email)
	if errors.Is(err, ErrNotFound) {
		// Don't leak whether the email is registered.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}

	go h.deliverPasswordReset(customer)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deliverPasswordReset(customer *Customer) {
	ctx := context.Background()
	token, _, err := h.svc.IssuePasswordResetToken(ctx, customer.ID)
	if err != nil {
		return
	}
	resetURL := strings.TrimRight(h.emailSvc.PublicBaseURL(ctx), "/") + "/account/reset-password?token=" + token
	name := strings.TrimSpace(customer.FirstName + " " + customer.LastName)
	if name == "" {
		name = customer.Email
	}
	_ = h.emailSvc.SendPasswordResetEmail(ctx, email.PasswordResetParams{
		CustomerName:  name,
		CustomerEmail: customer.Email,
		ResetURL:      resetURL,
		ExpiryHours:   24,
	})
}

func (h *Handler) sendResetPasswordEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if customer.Email == "" {
		respond.BadRequest(w, "customer has no email on file")
		return
	}

	token, _, err := h.svc.IssuePasswordResetToken(r.Context(), customer.ID)
	if err != nil {
		respond.InternalError(w)
		return
	}

	resetURL := strings.TrimRight(h.emailSvc.PublicBaseURL(r.Context()), "/") + "/account/reset-password?token=" + token
	name := strings.TrimSpace(customer.FirstName + " " + customer.LastName)
	if name == "" {
		name = customer.Email
	}

	if err := h.emailSvc.SendPasswordResetEmail(r.Context(), email.PasswordResetParams{
		CustomerName:  name,
		CustomerEmail: customer.Email,
		ResetURL:      resetURL,
		ExpiryHours:   24,
	}); err != nil {
		if errors.Is(err, email.ErrNotConfigured) {
			respond.Error(w, http.StatusServiceUnavailable, "email is not configured")
			return
		}
		respond.Error(w, http.StatusBadGateway, "failed to send email")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
