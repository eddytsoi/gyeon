package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gyeon/backend/internal/admin"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/cms"
	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/db"
	"gyeon/backend/internal/media"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/settings"
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

	// Services
	categorySvc := shop.NewCategoryService(conn)
	productSvc := shop.NewProductService(conn)
	cartSvc := orders.NewCartService(conn)
	orderSvc := orders.NewOrderService(conn, cartSvc)
	pageSvc := cms.NewPageService(conn)
	postSvc := cms.NewPostService(conn)
	postCatSvc := cms.NewPostCategoryService(conn)
	navSvc := cms.NewNavService(conn)
	customerSvc := customers.NewService(conn)
	settingsSvc := settings.NewService(conn)
	adminUserSvc := admin.NewUserService(conn)

	// Seed first super_admin from env if table is empty
	if err := adminUserSvc.SeedSuperAdmin(context.Background(), adminEmail, adminPassword); err != nil {
		log.Printf("warn: seed super admin: %v", err)
	}

	// Handlers
	statsHandler := admin.NewStatsHandler(conn)
	pageHandler := cms.NewPageHandler(pageSvc)
	postHandler := cms.NewPostHandler(postSvc)
	postCatHandler := cms.NewPostCategoryHandler(postCatSvc)
	navHandler := cms.NewNavHandler(navSvc)
	productHandler := shop.NewProductHandler(productSvc)
	customerHandler := customers.NewHandler(customerSvc, customerJWTSecret)
	settingsHandler := settings.NewHandler(settingsSvc)
	mediaHandler := media.NewHandler(conn, baseURL)
	adminUserHandler := admin.NewUserHandler(adminUserSvc, jwtSecret)
	adminMW := auth.Middleware(jwtSecret)

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

	// Serve uploaded files
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
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

		// Customer auth (public)
		r.Mount("/customers", customerHandler.PublicRoutes())

		// Customer authenticated routes
		r.Mount("/customers", customerHandler.AuthenticatedRoutes())

		// Admin auth (now uses admin_users table)
		r.Post("/admin/login", adminUserHandler.Login)

		// Admin protected
		r.Group(func(r chi.Router) {
			r.Use(adminMW)

			r.Get("/admin/stats", statsHandler.Get)

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

			// Admin user management
			r.Mount("/admin/users", adminUserHandler.AdminRoutes())
		})
	})

	log.Println("API server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
