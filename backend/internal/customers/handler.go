package customers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/oauth"
	"gyeon/backend/internal/ratelimit"
	"gyeon/backend/internal/respond"
)

// OrderFetcherFunc loads a single order if and only if it belongs to the
// given customer. Wired from main.go to orders.OrderService.GetByIDForCustomer
// via a closure, so the customers package doesn't take an orders import (which
// would create a cycle — orders already depends on customers).
type OrderFetcherFunc func(ctx context.Context, orderID, customerID string) (any, error)

// OrderPaymentFetcherFunc returns the /pay-page payload (client secret etc.) for
// a logged-in customer's own still-payable order. Wired from main.go to
// orders.OrderService.PaymentInfoForCustomer via a closure, same as
// OrderFetcherFunc, to avoid an orders→customers import cycle.
type OrderPaymentFetcherFunc func(ctx context.Context, orderID, customerID string) (any, error)

// EmailSender is the slice of email.Service the customers handler needs.
type EmailSender interface {
	PublicBaseURL(ctx context.Context) string
	SendPasswordResetEmail(ctx context.Context, p email.PasswordResetParams) error
}

type Handler struct {
	svc               *Service
	emailSvc          EmailSender
	jwtSecret         string
	fetchOrder        OrderFetcherFunc
	fetchOrderPayment OrderPaymentFetcherFunc
	oauth             *oauth.Service
	tokenTTL          func(context.Context) time.Duration
}

func NewHandler(svc *Service, emailSvc EmailSender, jwtSecret string, fetchOrder OrderFetcherFunc) *Handler {
	return &Handler{svc: svc, emailSvc: emailSvc, jwtSecret: jwtSecret, fetchOrder: fetchOrder}
}

// SetOrderPaymentFetcher wires the owner-authenticated payment-info lookup used
// by GET /me/orders/{id}/payment-info. Optional — when unset, the route 404s.
func (h *Handler) SetOrderPaymentFetcher(fn OrderPaymentFetcherFunc) { h.fetchOrderPayment = fn }

// SetOAuth wires the social-login service. Optional — when unset, the
// /oauth/* routes redirect back to login with an error.
func (h *Handler) SetOAuth(o *oauth.Service) { h.oauth = o }

// SetTokenTTL wires the customer session length provider (reads the
// customer_token_ttl_hours setting). Optional — falls back to 30 days when unset.
func (h *Handler) SetTokenTTL(fn func(context.Context) time.Duration) { h.tokenTTL = fn }

// customerTTL resolves the configured customer session length, defaulting to 30 days.
func (h *Handler) customerTTL(ctx context.Context) time.Duration {
	if h.tokenTTL == nil {
		return 30 * 24 * time.Hour
	}
	return h.tokenTTL(ctx)
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
	// Social login (Google / Apple). start = top-level GET redirect to provider;
	// callback = GET (Google) or POST form_post (Apple). The handler sets the
	// customer_token cookie itself, so no SvelteKit callback route is needed.
	oauthRL := ratelimit.Middleware(20, 5*time.Minute)
	r.With(oauthRL).Get("/oauth/{provider}/start", h.oauthStart)
	r.With(oauthRL).HandleFunc("/oauth/{provider}/callback", h.oauthCallback)
	r.Group(func(r chi.Router) {
		r.Use(auth.CustomerMiddleware(h.jwtSecret))
		r.Get("/me", h.getProfile)
		r.Put("/me", h.updateProfile)
		r.Get("/me/addresses", h.listAddresses)
		r.Post("/me/addresses", h.createAddress)
		r.Put("/me/addresses/{addressID}", h.updateAddress)
		r.Delete("/me/addresses/{addressID}", h.deleteAddress)
		r.Get("/me/orders", h.listOrders)
		r.Get("/me/products/purchased", h.listPurchasedProducts)
		r.Get("/me/orders/lookup/{number}", h.lookupOrder)
		r.Get("/me/orders/{id}", h.getOrder)
		r.Get("/me/orders/{id}/payment-info", h.getOrderPaymentInfo)
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
	r.Put("/{id}/role", h.adminUpdateRole)
	r.Get("/{id}/addresses", h.adminListAddresses)
	r.Post("/{id}/send-reset-password-email", h.sendResetPasswordEmail)
	return r
}

// adminUpdateRole sets a customer's storefront role (customer | installer).
// Body: {"role": "installer"}. Unknown roles fall back to "customer" via
// NormalizeRole rather than 400-ing — the admin UI ships a closed dropdown,
// so a bad value here means a buggy caller, not a user typo.
func (h *Handler) adminUpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	customer, err := h.svc.UpdateRole(r.Context(), id, req.Role)
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

// adminListAddresses returns a customer's saved addresses for the admin
// create-order picker. Mirrors listAddresses but resolves the customer id
// from the URL path instead of the customer JWT.
func (h *Handler) adminListAddresses(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	addrs, err := h.svc.ListAddresses(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, addrs)
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
	ttl := h.customerTTL(r.Context())
	token, err := auth.GenerateCustomerToken(h.jwtSecret, customer.ID, tv, ttl)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]interface{}{
		"customer":   customer,
		"token":      token,
		"expires_in": int(ttl.Seconds()),
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
	ttl := h.customerTTL(r.Context())
	token, err := auth.GenerateCustomerToken(h.jwtSecret, customer.ID, tv, ttl)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"customer":   customer,
		"token":      token,
		"expires_in": int(ttl.Seconds()),
	})
}

// oauthLoginRedirect is where the storefront login page lives; OAuth flows
// bounce back here with an ?error= on any failure.
const oauthLoginRedirect = "/account/login"

func (h *Handler) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if h.oauth == nil || !oauth.ValidProvider(provider) {
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	authURL, err := h.oauth.AuthURL(r.Context(), provider)
	if err != nil {
		log.Printf("oauth start %s: %v", provider, err)
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (h *Handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if h.oauth == nil || !oauth.ValidProvider(provider) {
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	// ParseForm covers both Google (query params on a GET) and Apple
	// (form_post body on a POST).
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	if e := r.Form.Get("error"); e != "" {
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	info, err := h.oauth.Exchange(r.Context(), provider, r.Form.Get("code"), r.Form.Get("state"))
	if err != nil {
		log.Printf("oauth callback %s: %v", provider, err)
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	// Apple only sends the user's name on the first authorization, in the
	// `user` form field rather than the id_token.
	first, last := info.FirstName, info.LastName
	if provider == oauth.ProviderApple {
		if af, al := oauth.ParseAppleUserName(r.Form.Get("user")); af != "" || al != "" {
			first, last = af, al
		}
	}
	customer, err := h.svc.FindOrCreateByOAuth(r.Context(), provider, info.Subject, info.Email, first, last)
	if err != nil {
		log.Printf("oauth find-or-create %s: %v", provider, err)
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	if !customer.IsActive {
		http.Redirect(w, r, oauthLoginRedirect+"?error=inactive", http.StatusSeeOther)
		return
	}
	tv, _ := h.svc.TokenVersion(r.Context(), customer.ID)
	ttl := h.customerTTL(r.Context())
	token, err := auth.GenerateCustomerToken(h.jwtSecret, customer.ID, tv, ttl)
	if err != nil {
		http.Redirect(w, r, oauthLoginRedirect+"?error=oauth", http.StatusSeeOther)
		return
	}
	// Same cookie shape as the SvelteKit login action sets, so downstream
	// auth (layout guard, Bearer forwarding) is identical.
	http.SetCookie(w, &http.Cookie{
		Name:     "customer_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

// requestIsHTTPS reports whether the original client request was over TLS,
// accounting for the reverse proxy in front of the backend. Drives the cookie
// Secure flag — must stay false on plain-http localhost or the browser drops
// the cookie.
func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
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
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(q.Get("status"))
	search := strings.TrimSpace(q.Get("q"))
	orders, total, err := h.svc.ListOrders(r.Context(), customerID, limit, offset, status, search)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Orders []OrderSummary `json:"orders"`
		Total  int            `json:"total"`
	}{orders, total})
}

// listPurchasedProducts powers the "曾經購買" account page: every product the
// authenticated customer has ever bought (across all paid-or-later orders),
// aggregated to one row per product with order/bundle provenance attached.
func (h *Handler) listPurchasedProducts(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	products, err := h.svc.ListPurchasedProducts(r.Context(), customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, products)
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

// getOrderPaymentInfo returns the /pay-page payload (client secret, publishable
// key) for the authenticated customer's own pending order, so the account page
// can offer a "立即付款" button without the shopper holding a magic-link cs.
// Any error (not owned / not payable / Stripe lookup) is flattened to 404 so
// the caller cannot distinguish ownership or payability signals.
func (h *Handler) getOrderPaymentInfo(w http.ResponseWriter, r *http.Request) {
	if h.fetchOrderPayment == nil {
		respond.NotFound(w)
		return
	}
	customerID := auth.CustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	info, err := h.fetchOrderPayment(r.Context(), id, customerID)
	if err != nil {
		respond.NotFound(w)
		return
	}
	respond.JSON(w, http.StatusOK, info)
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
	filters := ListFilters{
		Active: r.URL.Query().Get("active"),
		Role:   r.URL.Query().Get("role"),
	}
	customers, total, err := h.svc.List(r.Context(), r.URL.Query().Get("q"), filters, limit, offset)
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
