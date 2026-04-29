package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gyeon/backend/internal/admin"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/cache"
	"gyeon/backend/internal/cms"
	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/db"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/importer"
	"gyeon/backend/internal/lookup"
	mcpsrv "gyeon/backend/internal/mcp"
	"gyeon/backend/internal/media"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/pricing"
	"gyeon/backend/internal/settings"
	"gyeon/backend/internal/shipany"
	"gyeon/backend/internal/shop"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	dsn := getenv("DATABASE_URL", "postgres://gyeon:gyeon@localhost:5432/gyeon?sslmode=disable")
	jwtSecret := getenv("ADMIN_JWT_SECRET", "change-me-in-production")
	customerJWTSecret := getenv("CUSTOMER_JWT_SECRET", "change-me-customer-secret")
	adminEmail := getenv("ADMIN_EMAIL", "admin@gyeon.local")
	adminPassword := getenv("ADMIN_PASSWORD", "admin123")
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
	productSvc := shop.NewProductService(conn, cacheStore, shopTTL)
	cartSvc := orders.NewCartService(conn)
	pricingSvc := pricing.NewService(conn)
	customerSvc := customers.NewService(conn)
	paymentSvc := payment.NewService(settingsSvc, conn)
	emailSvc := email.NewService(settingsSvc)
	orderSvc := orders.NewOrderService(conn, cartSvc, pricingSvc, customerSvc, paymentSvc, emailSvc)
	shipanyClient := shipany.NewHTTPClient(settingsSvc, getenv("SHIPANY_BASE_URL", ""))
	shipanySvc := shipany.NewService(shipanyClient, settingsSvc, conn, orderSvc)
	pageSvc := cms.NewPageService(conn, cacheStore, cmsTTL)
	postSvc := cms.NewPostService(conn, cacheStore, cmsTTL)
	postCatSvc := cms.NewPostCategoryService(conn)
	navSvc := cms.NewNavService(conn, cacheStore, navTTL)
	adminUserSvc := admin.NewUserService(conn)

	// Seed first super_admin from env if table is empty
	if err := adminUserSvc.SeedSuperAdmin(context.Background(), adminEmail, adminPassword); err != nil {
		log.Printf("warn: seed super admin: %v", err)
	}

	// Handlers
	pricingHandler := pricing.NewHandler(pricingSvc)
	paymentHandler := payment.NewHandler(
		paymentSvc,
		func(r *http.Request, paymentIntentID string) {
			if err := orderSvc.MarkPaidByPaymentIntent(r.Context(), paymentIntentID); err != nil {
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
			brand, last4, expMonth, expYear, err := paymentSvc.FetchPaymentMethodDetails(ctx, stripePMID)
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
	pageHandler := cms.NewPageHandler(pageSvc)
	postHandler := cms.NewPostHandler(postSvc)
	postCatHandler := cms.NewPostCategoryHandler(postCatSvc)
	navHandler := cms.NewNavHandler(navSvc)
	productHandler := shop.NewProductHandler(productSvc)
	customerHandler := customers.NewHandler(customerSvc, emailSvc, customerJWTSecret)
	settingsHandler := settings.NewHandler(settingsSvc, emailSvc)
	mediaSvc := media.NewService(conn, baseURL)
	mediaHandler := media.NewHandler(conn, baseURL, settingsSvc)
	adminUserHandler := admin.NewUserHandler(adminUserSvc, jwtSecret)
	importHandler := importer.NewHandler(importer.NewService(categorySvc, productSvc, mediaSvc))
	shipanyHandler := shipany.NewHandler(shipanySvc, cartSvc)
	adminMW := auth.Middleware(jwtSecret)

	// Admin SSE hub: broadcasts new-order events to all connected admin clients.
	adminHub := admin.NewHub()
	adminEventsHandler := admin.NewEventsHandler(adminHub, jwtSecret)
	orderSvc.SetOnOrderCreated(func(_ context.Context, o *orders.Order) {
		short := o.ID
		if len(short) > 8 {
			short = short[:8]
		}
		name := ""
		if o.CustomerName != nil {
			name = *o.CustomerName
		}
		adminHub.Broadcast("new_order", map[string]any{
			"order_id":      o.ID,
			"short_id":      short,
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

	// CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Serve uploaded files — Cloudflare caches these; stale files are purged on delete
	uploadsFS := http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads")))
	r.Handle("/uploads/*", uploadsFS)

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
		r.Mount("/categories", shop.NewCategoryHandler(categorySvc).Routes())
		r.Mount("/products", productHandler.Routes())
		r.Mount("/cart", orders.NewCartHandler(cartSvc).Routes())
		r.Mount("/orders", orders.NewOrderHandler(orderSvc).Routes())

		// Public CMS (published content only)
		r.Mount("/cms/pages", pageHandler.PublicRoutes())
		r.Mount("/cms/posts", postHandler.PublicRoutes())
		r.Mount("/cms/post-categories", postCatHandler.PublicRoutes())
		r.Mount("/cms/nav", navHandler.PublicRoutes())

		// Public settings (storefront config)
		r.Mount("/settings", settingsHandler.PublicRoutes())

		// Public coupon validation
		r.Mount("/pricing", pricingHandler.PublicRoutes())

		// Payment config + Stripe webhook (public)
		r.Mount("/payments", paymentHandler.Routes())

		// ShipAny logistics: quote, pickup-points, webhook (public)
		r.Mount("/shipany", shipanyHandler.PublicRoutes())

		// Customer routes (public + authenticated)
		r.Mount("/customers", customerHandler.Routes())

		// Admin auth (now uses admin_users table)
		r.Post("/admin/login", adminUserHandler.Login)

		// Admin SSE event stream — auth via ?token= query (EventSource can't set headers)
		r.Get("/admin/events", adminEventsHandler.Stream)

		// Admin protected
		r.Group(func(r chi.Router) {
			r.Use(adminMW)

			r.Get("/admin/stats", statsHandler.Get)

			// Resolve prefix-id (PRD-8, ORD-1, ...) to UUID for admin URLs
			r.Mount("/admin/lookup", lookup.NewHandler(productSvc, orderSvc, pageSvc, postSvc).Routes())

			// Product admin routes (inventory)
			r.Mount("/admin/inventory", productHandler.AdminRoutes())

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

			// Order admin (delete; list/update use the public /orders mount with admin JWT)
			r.Mount("/admin/orders", orders.NewOrderHandler(orderSvc).AdminRoutes())

			// ShipAny admin: test connection, create shipment, request pickup
			r.Mount("/admin/shipany", shipanyHandler.AdminRoutes())

			// Admin user management
			r.Mount("/admin/users", adminUserHandler.AdminRoutes())

			// Pricing: campaigns and coupons
			r.Mount("/admin/pricing", pricingHandler.AdminRoutes())

			// WooCommerce import
			r.Post("/admin/import/woocommerce/test", importHandler.Test)
			r.Post("/admin/import/woocommerce/stream", importHandler.ImportStream)
		})
	})

	// MCP storefront server — safe public tools only (browse + cart + checkout)
	mcpServer := mcpsrv.NewServer(categorySvc, productSvc, cartSvc, orderSvc, pricingSvc)
	r.With(mcpGate).Mount("/mcp", mcpServer.Handler())

	log.Println("API server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
