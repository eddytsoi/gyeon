package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gyeon/backend/internal/abandoned"
	"gyeon/backend/internal/admin"
	"gyeon/backend/internal/audit"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/cms"
	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/db"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/forms"
	"gyeon/backend/internal/importer"
	"gyeon/backend/internal/lookup"
	"gyeon/backend/internal/loyalty"
	mcpsrv "gyeon/backend/internal/mcp"
	"gyeon/backend/internal/media"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/pricing"
	"gyeon/backend/internal/ratelimit"
	"gyeon/backend/internal/recaptcha"
	"gyeon/backend/internal/redirects"
	"gyeon/backend/internal/settings"
	"gyeon/backend/internal/shipany"
	"gyeon/backend/internal/shop"
	"gyeon/backend/internal/tax"
	"gyeon/backend/internal/wishlist"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitCSV returns the comma-separated values in s, trimmed, with empties
// dropped. Used to read FRONTEND_ORIGIN.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// originAllowed reports whether origin matches one of the allowed entries
// exactly (no wildcard expansion — keep the rule simple and obvious).
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

// redirectsAuditAdapter bridges redirects.AuditRecorder → audit.Service so the
// redirects package doesn't need to import the audit package.
type redirectsAuditAdapter struct{ svc *audit.Service }

func (a redirectsAuditAdapter) Record(ctx context.Context, e redirects.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// settingsAuditAdapter bridges settings.AuditRecorder → audit.Service.
type settingsAuditAdapter struct{ svc *audit.Service }

func (a settingsAuditAdapter) Record(ctx context.Context, e settings.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// productsAuditAdapter bridges shop.AuditRecorder → audit.Service.
type productsAuditAdapter struct{ svc *audit.Service }

func (a productsAuditAdapter) Record(ctx context.Context, e shop.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// ordersAuditAdapter bridges orders.AuditRecorder → audit.Service.
type ordersAuditAdapter struct{ svc *audit.Service }

func (a ordersAuditAdapter) Record(ctx context.Context, e orders.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// cmsAuditAdapter bridges cms.AuditRecorder → audit.Service. Shared by both
// PageService and PostService since they live in the same cms package and
// reuse the same recorder interface.
type cmsAuditAdapter struct{ svc *audit.Service }

func (a cmsAuditAdapter) Record(ctx context.Context, e cms.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// customersAuditAdapter bridges customers.AuditRecorder → audit.Service.
type customersAuditAdapter struct{ svc *audit.Service }

func (a customersAuditAdapter) Record(ctx context.Context, e customers.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// adminUsersAuditAdapter bridges admin.AuditRecorder → audit.Service.
type adminUsersAuditAdapter struct{ svc *audit.Service }

func (a adminUsersAuditAdapter) Record(ctx context.Context, e admin.AuditEntry) {
	a.svc.Record(ctx, audit.Entry{
		Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Before: e.Before, After: e.After,
	})
}

// tokenVersionAdapter implements auth.TokenVersionStore by reading the
// admin_users / customers token_version columns, with a short-TTL cache so
// the middleware doesn't hit Postgres on every request.
type tokenVersionAdapter struct {
	admin    *admin.UserService
	customer *customers.Service
	cache    cache.Store
	ttl      time.Duration
}

func (a *tokenVersionAdapter) AdminVersion(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, nil
	}
	key := "tv:admin:" + userID
	if v, ok := a.cache.Get(key); ok {
		return v.(int), nil
	}
	tv, err := a.admin.TokenVersion(ctx, userID)
	if err != nil {
		return 0, err
	}
	a.cache.Set(key, tv, a.ttl)
	return tv, nil
}

func (a *tokenVersionAdapter) CustomerVersion(ctx context.Context, customerID string) (int, error) {
	if customerID == "" {
		return 0, nil
	}
	key := "tv:customer:" + customerID
	if v, ok := a.cache.Get(key); ok {
		return v.(int), nil
	}
	tv, err := a.customer.TokenVersion(ctx, customerID)
	if err != nil {
		return 0, err
	}
	a.cache.Set(key, tv, a.ttl)
	return tv, nil
}

func (a *tokenVersionAdapter) InvalidateAdmin(userID string) {
	a.cache.Delete("tv:admin:" + userID)
}

func (a *tokenVersionAdapter) InvalidateCustomer(customerID string) {
	a.cache.Delete("tv:customer:" + customerID)
}

func main() {
	dsn := getenv("DATABASE_URL", "postgres://gyeon:gyeon@localhost:5432/gyeon?sslmode=disable")
	jwtSecret := os.Getenv("ADMIN_JWT_SECRET")
	customerJWTSecret := os.Getenv("CUSTOMER_JWT_SECRET")
	if jwtSecret == "" || customerJWTSecret == "" {
		log.Fatal("ADMIN_JWT_SECRET and CUSTOMER_JWT_SECRET must be set; refusing to start with predictable defaults")
	}
	// Warn (but don't fail) when the secrets look like the historical
	// fallback strings — leaving them in place is unsafe in any non-dev env.
	for _, p := range []struct{ name, val string }{
		{"ADMIN_JWT_SECRET", jwtSecret},
		{"CUSTOMER_JWT_SECRET", customerJWTSecret},
	} {
		if strings.Contains(strings.ToLower(p.val), "change-me") {
			log.Printf("warning: %s contains 'change-me' — rotate before exposing this build publicly", p.name)
		}
	}
	adminEmail := getenv("ADMIN_EMAIL", "admin@gyeon.local")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		// Empty triggers a fail-closed path: SeedSuperAdmin only runs when
		// admin_users is empty, so a missing password just means "don't
		// auto-seed the bootstrap user". Existing admins keep working.
		log.Printf("info: ADMIN_PASSWORD unset — initial super_admin seed skipped")
	} else if adminPassword == "admin123" {
		log.Printf("warning: ADMIN_PASSWORD is the historical default 'admin123' — change it before exposing this build")
	}
	baseURL := getenv("BASE_URL", "http://localhost:8080")

	conn, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close()

	// In-memory cache (cleanup every 10 min; per-item TTLs read from site_settings at runtime)
	cacheStore := cache.NewInMemory(10 * time.Minute)
	settingsSvc := settings.NewService(conn)

	shopTTL := func(ctx context.Context) time.Duration { return settingsSvc.TTL(ctx, "cache_ttl_shop", 300) }
	cmsTTL := func(ctx context.Context) time.Duration { return settingsSvc.TTL(ctx, "cache_ttl_cms", 300) }
	navTTL := func(ctx context.Context) time.Duration { return settingsSvc.TTL(ctx, "cache_ttl_nav", 900) }

	// Services
	categorySvc := shop.NewCategoryService(conn, cacheStore, shopTTL)
	productSvc := shop.NewProductService(conn, cacheStore, shopTTL, settingsSvc)
	cartSvc := orders.NewCartService(conn)
	pricingSvc := pricing.NewService(conn)
	customerSvc := customers.NewService(conn)
	paymentSvc := payment.NewService(settingsSvc, conn)
	emailSvc := email.NewService(settingsSvc)
	emailTemplateStore := email.NewStore(conn)
	emailSvc.SetTemplateStore(emailTemplateStore)
	emailTemplateHandler := email.NewTemplateHandler(emailTemplateStore, emailSvc)
	taxSvc := tax.NewService(settingsSvc)
	orderSvc := orders.NewOrderService(conn, cartSvc, pricingSvc, customerSvc, paymentSvc, emailSvc)
	orderSvc.SetTaxService(taxSvc)
	noticeSvc := orders.NewNoticeService(conn)
	noticeHandler := orders.NewNoticeHandler(noticeSvc, emailSvc, jwtSecret)
	shipanyClient := shipany.NewHTTPClient(settingsSvc, getenv("SHIPANY_BASE_URL", ""))
	shipanySvc := shipany.NewService(shipanyClient, settingsSvc, conn, orderSvc)
	pageSvc := cms.NewPageService(conn, cacheStore, cmsTTL)
	postSvc := cms.NewPostService(conn, cacheStore, cmsTTL)
	postCatSvc := cms.NewPostCategoryService(conn)
	navSvc := cms.NewNavService(conn, cacheStore, navTTL)
	adminUserSvc := admin.NewUserService(conn)

	// Seed first super_admin from env if the table is empty. Skip when the
	// password env is unset — bootstrapping is opt-in so misconfigured
	// deploys don't end up with a guessable account silently created.
	if adminPassword != "" {
		if err := adminUserSvc.SeedSuperAdmin(context.Background(), adminEmail, adminPassword); err != nil {
			log.Printf("warn: seed super admin: %v", err)
		}
	}

	// Handlers
	pricingHandler := pricing.NewHandler(pricingSvc)
	paymentHandler := payment.NewHandler(
		paymentSvc,
		func(r *http.Request, paymentIntentID, paymentMethodID string) {
			ctx := r.Context()
			var pmType, cardBrand, cardLast4 string
			if paymentMethodID != "" {
				t, b, l4, _, _, err := paymentSvc.FetchPaymentMethodDetails(ctx, paymentMethodID)
				if err != nil {
					// Non-fatal: still mark order paid; Method just stays "—".
					log.Printf("payment_intent.succeeded: fetch pm details %s: %v", paymentMethodID, err)
				} else {
					pmType, cardBrand, cardLast4 = t, b, l4
				}
			}
			if err := orderSvc.MarkPaidByPaymentIntent(ctx, paymentIntentID, pmType, cardBrand, cardLast4); err != nil {
				log.Printf("mark paid for payment_intent %s: %v", paymentIntentID, err)
			}
		},
		func(r *http.Request, stripeCustomerID, stripePMID string) {
			ctx := r.Context()
			gyeonCustomerID, err := paymentSvc.LookupCustomerByStripeID(ctx, stripeCustomerID)
			if err != nil {
				log.Printf("setup_intent.succeeded: lookup customer for stripe %s: %v", stripeCustomerID, err)
				return
			}
			_, brand, last4, expMonth, expYear, err := paymentSvc.FetchPaymentMethodDetails(ctx, stripePMID)
			if err != nil {
				log.Printf("setup_intent.succeeded: fetch pm details %s: %v", stripePMID, err)
				return
			}
			if err := paymentSvc.StoreSavedPaymentMethod(ctx, gyeonCustomerID, stripePMID, brand, last4, expMonth, expYear); err != nil {
				log.Printf("setup_intent.succeeded: store pm for customer %s: %v", gyeonCustomerID, err)
			}
		},
		customerJWTSecret,
	)
	statsHandler := admin.NewStatsHandler(conn)
	analyticsHandler := admin.NewAnalyticsHandler(conn)
	pageHandler := cms.NewPageHandler(pageSvc)
	postHandler := cms.NewPostHandler(postSvc)
	postCatHandler := cms.NewPostCategoryHandler(postCatSvc)
	navHandler := cms.NewNavHandler(navSvc)
	productHandler := shop.NewProductHandler(productSvc)
	customerHandler := customers.NewHandler(customerSvc, emailSvc, customerJWTSecret,
		func(ctx context.Context, orderID, customerID string) (any, error) {
			return orderSvc.GetByIDForCustomer(ctx, orderID, customerID)
		})
	settingsHandler := settings.NewHandler(settingsSvc, emailSvc)
	mediaSvc := media.NewService(conn, baseURL)
	mediaHandler := media.NewHandler(conn, baseURL, settingsSvc, mediaSvc)
	productSvc.SetThumbnailEnsurer(mediaHandler)
	adminUserHandler := admin.NewUserHandler(adminUserSvc, jwtSecret)
	importHandler := importer.NewHandler(importer.NewService(conn, categorySvc, productSvc, mediaSvc, settingsSvc, customerSvc, emailSvc))
	shipanyHandler := shipany.NewHandler(shipanySvc, cartSvc)
	redirectsSvc := redirects.NewService(conn)
	redirectsHandler := redirects.NewHandler(redirectsSvc)
	auditSvc := audit.NewService(conn)
	auditHandler := audit.NewHandler(auditSvc)
	loyaltySvc := loyalty.NewService(conn)
	loyaltyHandler := loyalty.NewHandler(loyaltySvc)

	// Contact forms (CF7-style) + reCAPTCHA v3 verifier
	recaptchaVerifier := recaptcha.New(settingsSvc)
	formsSvc := forms.NewService(conn, emailSvc, recaptchaVerifier)
	formsHandler := forms.NewHandler(formsSvc)
	orderSvc.SetOnOrderPaid(func(ctx context.Context, o *orders.Order) {
		// Earn rate operates on order subtotal (post-discount, pre-tax/shipping).
		base := o.Subtotal - o.DiscountAmount
		if base < 0 {
			base = 0
		}
		if o.CustomerID == nil || *o.CustomerID == "" {
			return // guest checkout — no customer to credit
		}
		if err := loyaltySvc.EarnFromOrder(ctx, *o.CustomerID, o.ID, base); err != nil {
			log.Printf("loyalty: earn order %s: %v", o.ID, err)
		}
	})
	redirectsSvc.SetAudit(redirectsAuditAdapter{svc: auditSvc})
	settingsSvc.SetAudit(settingsAuditAdapter{svc: auditSvc})
	productSvc.SetAudit(productsAuditAdapter{svc: auditSvc})
	orderSvc.SetAudit(ordersAuditAdapter{svc: auditSvc})
	pageSvc.SetAudit(cmsAuditAdapter{svc: auditSvc})
	postSvc.SetAudit(cmsAuditAdapter{svc: auditSvc})
	customerSvc.SetAudit(customersAuditAdapter{svc: auditSvc})
	adminUserSvc.SetAudit(adminUsersAuditAdapter{svc: auditSvc})
	// JWT revocation: the middleware consults a TokenVersionStore on every
	// request. Back it with a tiny in-memory cache to keep the per-request
	// DB cost negligible; sign-out-everywhere endpoints below explicitly
	// invalidate the cached entry, so revocation propagates immediately on
	// the calling node and within `tvCacheTTL` on peers.
	const tvCacheTTL = 15 * time.Second
	auth.SetVersionStore(&tokenVersionAdapter{
		admin:    adminUserSvc,
		customer: customerSvc,
		cache:    cacheStore,
		ttl:      tvCacheTTL,
	})
	adminMW := auth.AdminMiddleware(jwtSecret)
	auditInfoMW := audit.RequestInfoMiddleware()

	// Admin SSE hub: broadcasts new-order events to all connected admin clients.
	adminHub := admin.NewHub()
	adminEventsHandler := admin.NewEventsHandler(adminHub, jwtSecret)
	orderSvc.SetOnOrderCreated(func(_ context.Context, o *orders.Order) {
		name := ""
		if o.CustomerName != nil {
			name = *o.CustomerName
		}
		adminHub.Broadcast("new_order", map[string]any{
			"order_id":      o.ID,
			"order_number":  o.OrderNumber,
			"customer_name": name,
			"total":         o.Total,
		})
	})

	// mcpGate returns 404 when mcp_enabled != 'true', checked per-request so toggling takes effect immediately.
	mcpGate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			st, err := settingsSvc.Get(r.Context(), "mcp_enabled")
			if err != nil || st.Value != "true" {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Baseline security headers applied to every response. Kept here (and not
	// only on /api responses) so static /uploads/ files inherit them too.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	})

	// CORS — allowlist of origins from FRONTEND_ORIGIN (comma-separated).
	// Empty / unset → wildcard `*`, which excludes credentialed requests by
	// design (browsers reject "*" + Authorization+SameSite cookies). Setting
	// the env locks the API down to its known frontends.
	allowedOrigins := splitCSV(os.Getenv("FRONTEND_ORIGIN"))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if len(allowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if originAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Responsive image resize endpoint — handled before the catch-all FileServer
	// because chi's radix tree prefers specific routes over wildcards.
	r.Get("/uploads/r/{width:[0-9]+}/{filename}", mediaHandler.ServeResized)

	// Serve uploaded files — Cloudflare caches these; stale files are purged on
	// delete. Hidden paths (e.g. /uploads/.cache/ used by the resize endpoint)
	// are blocked so cache contents are only reachable via /uploads/r/...
	uploadsFS := http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads")))
	r.Handle("/uploads/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/.") {
			http.NotFound(w, req)
			return
		}
		// SVG can carry inline <script>; uploaded by an admin doesn't mean
		// safe to render top-level. Lock the response down with a strict
		// per-file CSP so direct navigation can't execute scripts, and force
		// download for paranoia in case the CSP isn't honored.
		if strings.HasSuffix(strings.ToLower(req.URL.Path), ".svg") {
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data: 'self'")
			w.Header().Set("Content-Disposition", "inline")
		}
		uploadsFS.ServeHTTP(w, req)
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// MCP discoverability — agents can probe this to find the MCP endpoint
	r.With(mcpGate).Get("/.well-known/mcp.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(`{"mcp_endpoint":"` + baseURL + `/mcp/sse","name":"Gyeon Storefront","description":"Browse products, manage cart, validate coupons, and place orders. Checkout accepts customer and shipping details and returns a Stripe PaymentIntent client_secret for the client to confirm payment."}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public storefront
		r.Mount("/categories", shop.NewCategoryHandler(categorySvc, productSvc.HiddenCategoryIDs).Routes())
		r.Mount("/products", productHandler.Routes())
		r.Mount("/cart", orders.NewCartHandler(cartSvc).Routes())
		r.Mount("/orders", orders.NewOrderHandler(orderSvc).PublicRoutes())

		// Public CMS (published content only)
		r.Mount("/cms/pages", pageHandler.PublicRoutes())
		r.Mount("/cms/posts", postHandler.PublicRoutes())
		r.Mount("/cms/post-categories", postCatHandler.PublicRoutes())
		r.Mount("/cms/nav", navHandler.PublicRoutes())

		// Public settings (storefront config)
		r.Mount("/settings", settingsHandler.PublicRoutes())

		// Public forms: read form spec, submit form
		r.Mount("/forms", formsHandler.PublicRoutes())

		// Public coupon validation
		r.Mount("/pricing", pricingHandler.PublicRoutes())

		// Public redirect lookup (called by SvelteKit hooks for storefront URL match)
		r.Mount("/redirects", redirectsHandler.PublicRoutes())

		// Payment config + Stripe webhook (public)
		r.Mount("/payments", paymentHandler.Routes())

		// ShipAny logistics: quote, pickup-points, webhook (public)
		r.Mount("/shipany", shipanyHandler.PublicRoutes())

		// Customer routes (public + authenticated)
		r.Mount("/customers", customerHandler.Routes())

		// Wishlist (authenticated; guest uses client-side localStorage)
		wishlistSvc := wishlist.NewService(conn)
		wishlistHandler := wishlist.NewHandler(wishlistSvc, customerJWTSecret)
		r.Mount("/wishlist", wishlistHandler.Routes())

		// Customer-side order notices (auth via customer JWT)
		r.Group(func(r chi.Router) {
			r.Use(auth.CustomerMiddleware(customerJWTSecret))
			r.Mount("/order-notices", noticeHandler.CustomerRoutes())
			r.Mount("/loyalty", loyaltyHandler.CustomerRoutes())
		})

		// Admin auth (now uses admin_users table) — per-IP throttle to slow
		// down credential stuffing. Tight bucket because the admin pool is
		// small and lockouts are recoverable via DB reset.
		adminLoginRL := ratelimit.Middleware(10, 5*time.Minute)
		r.With(adminLoginRL).Post("/admin/login", adminUserHandler.Login)

		// Admin SSE event stream — auth via ?token= query (EventSource can't set headers)
		r.Get("/admin/events", adminEventsHandler.Stream)

		// Admin protected
		r.Group(func(r chi.Router) {
			r.Use(adminMW)
			r.Use(auditInfoMW)

			r.Get("/admin/stats", statsHandler.Get)

			// Sign out everywhere — bumps the calling admin's token_version,
			// killing every existing JWT for that user.
			r.Post("/admin/me/sign-out-everywhere", adminUserHandler.SignOutEverywhere)

			// Analytics (P2 #16): time-series + top-N + breakdowns
			r.Mount("/admin/analytics", analyticsHandler.Routes())

			// Resolve prefix-id (PRD-8, ORD-1, ...) to UUID for admin URLs
			r.Mount("/admin/lookup", lookup.NewHandler(productSvc, orderSvc, pageSvc, postSvc).Routes())

			// Product admin routes (inventory)
			r.Mount("/admin/inventory", productHandler.AdminRoutes())

			// Product admin write routes — auth-gated, audited. Mutating
			// product/variant/image/translation/bundle endpoints. Storefront
			// GETs remain on /products (no auth, no audit).
			r.Mount("/admin/products", productHandler.AdminWriteRoutes())

			// Cross-product stock movement log (進出記錄).
			r.Mount("/admin/stock-history", productHandler.AdminStockHistoryRoutes())

			// CMS admin
			r.Mount("/admin/cms/pages", pageHandler.AdminRoutes())
			r.Mount("/admin/cms/posts", postHandler.AdminRoutes())
			r.Mount("/admin/cms/post-categories", postCatHandler.AdminRoutes())
			r.Mount("/admin/cms/nav", navHandler.AdminRoutes())

			// Settings admin
			r.Mount("/admin/settings", settingsHandler.AdminRoutes())

			// Media library
			r.Mount("/admin/media", mediaHandler.AdminRoutes())

			// Customer management
			r.Mount("/admin/customers", customerHandler.AdminRoutes())

			// Order admin — list / get / status / delete / refund. Public /orders
			// only exposes checkout + PI-authorized read-back.
			r.Mount("/admin/orders", orders.NewOrderHandler(orderSvc).AdminRoutes())

			// Admin-side order notices
			r.Mount("/admin/order-notices", noticeHandler.AdminRoutes())

			// ShipAny admin: test connection, create shipment, request pickup
			r.Mount("/admin/shipany", shipanyHandler.AdminRoutes())

			// Admin user management — super_admin only. Lower-privileged admins
			// (editor / viewer) must not be able to create new accounts, change
			// roles, or deactivate others, since that would let them escalate.
			r.With(auth.RequireRole(jwtSecret, "super_admin")).
				Mount("/admin/users", adminUserHandler.AdminRoutes())

			// Pricing: campaigns and coupons
			r.Mount("/admin/pricing", pricingHandler.AdminRoutes())

			// Redirects (P2 #22): admin CRUD; public match endpoint is mounted above
			r.Mount("/admin/redirects", redirectsHandler.AdminRoutes())

			// Audit log (P2 #17): list-only — entries are inserted by services
			r.Mount("/admin/audit-log", auditHandler.AdminRoutes())

			// Email templates (P2 #20): admin-editable overrides for the
			// hardcoded transactional emails
			r.Mount("/admin/email-templates", emailTemplateHandler.AdminRoutes())

			// Contact-form admin (CF7-style) — CRUD + submissions viewer
			r.Mount("/admin/forms", formsHandler.AdminRoutes())

			// Loyalty (P3 #24): per-customer balance + ledger + manual adjust
			r.Mount("/admin/customers/{id}/loyalty", loyaltyHandler.AdminRoutes())

			// Abandoned cart recovery (list + manual run; external cron may also POST run)
			abandonedSvc := abandoned.NewService(conn, emailSvc, settingsSvc)
			r.Mount("/admin/abandoned-cart", abandoned.NewHandler(abandonedSvc).AdminRoutes())

			// WooCommerce import
			r.Get("/admin/import/woocommerce/credentials", importHandler.GetCredentials)
			r.Put("/admin/import/woocommerce/credentials", importHandler.SaveCredentials)
			r.Post("/admin/import/woocommerce/test", importHandler.Test)
			r.Post("/admin/import/woocommerce/stream", importHandler.ImportStream)
			r.Post("/admin/import/woocommerce/customers/test", importHandler.CustomersTest)
			r.Post("/admin/import/woocommerce/customers/stream", importHandler.CustomersImportStream)
			r.Post("/admin/import/woocommerce/orders/test", importHandler.OrdersTest)
			r.Post("/admin/import/woocommerce/orders/stream", importHandler.OrdersImportStream)
		})
	})

	// MCP storefront server — safe public tools only (browse + cart + checkout)
	mcpServer := mcpsrv.NewServer(categorySvc, productSvc, cartSvc, orderSvc, pricingSvc)
	r.With(mcpGate).Mount("/mcp", mcpServer.Handler())

	addr := ":" + getenv("PORT", "8080")
	log.Println("API server listening on", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
