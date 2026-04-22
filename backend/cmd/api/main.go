package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gyeon/backend/internal/admin"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/cms"
	"gyeon/backend/internal/db"
	"gyeon/backend/internal/orders"
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
	adminPassword := getenv("ADMIN_PASSWORD", "admin123")

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

	// Handlers
	authHandler := auth.NewHandler(jwtSecret, adminPassword)
	statsHandler := admin.NewStatsHandler(conn)
	pageHandler := cms.NewPageHandler(pageSvc)
	postHandler := cms.NewPostHandler(postSvc)
	postCatHandler := cms.NewPostCategoryHandler(postCatSvc)
	navHandler := cms.NewNavHandler(navSvc)
	adminMW := auth.Middleware(jwtSecret)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// CORS for admin frontend on same origin (Vite proxy)
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

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public storefront
		r.Mount("/categories", shop.NewCategoryHandler(categorySvc).Routes())
		r.Mount("/products", shop.NewProductHandler(productSvc).Routes())
		r.Mount("/cart", orders.NewCartHandler(cartSvc).Routes())
		r.Mount("/orders", orders.NewOrderHandler(orderSvc).Routes())

		// Public CMS (published content only)
		r.Mount("/cms/pages", pageHandler.PublicRoutes())
		r.Mount("/cms/posts", postHandler.PublicRoutes())
		r.Mount("/cms/post-categories", postCatHandler.PublicRoutes())
		r.Mount("/cms/nav", navHandler.PublicRoutes())

		// Admin auth (public)
		r.Post("/admin/login", authHandler.Login)

		// Admin protected
		r.Group(func(r chi.Router) {
			r.Use(adminMW)
			r.Get("/admin/stats", statsHandler.Get)

			// CMS admin (full CRUD)
			r.Mount("/admin/cms/pages", pageHandler.AdminRoutes())
			r.Mount("/admin/cms/posts", postHandler.AdminRoutes())
			r.Mount("/admin/cms/post-categories", postCatHandler.AdminRoutes())
			r.Mount("/admin/cms/nav", navHandler.AdminRoutes())
		})
	})

	log.Println("API server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
