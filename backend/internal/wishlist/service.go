// Package wishlist stores customer-saved products. One row per (customer,
// product). Variant selection happens on the PDP, so the wishlist itself only
// tracks products.
package wishlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type Item struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	CreatedAt string `json:"created_at"`
	// Joined product fields for storefront display.
	ProductSlug     string  `json:"product_slug,omitempty"`
	ProductName     string  `json:"product_name,omitempty"`
	ProductImageURL *string `json:"product_image_url,omitempty"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) List(ctx context.Context, customerID string) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT w.id, w.product_id, w.created_at,
		        p.slug, p.name,
		        (SELECT COALESCE(mf.url, pi.url)
		         FROM product_images pi
		         LEFT JOIN media_files mf ON mf.id = pi.media_file_id
		         WHERE pi.product_id = p.id
		         ORDER BY pi.is_primary DESC, pi.sort_order ASC LIMIT 1)
		 FROM wishlist_items w
		 JOIN products p ON p.id = w.product_id
		 WHERE w.customer_id = $1 AND p.status = 'active'
		 ORDER BY w.created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProductID, &it.CreatedAt,
			&it.ProductSlug, &it.ProductName, &it.ProductImageURL); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Service) Add(ctx context.Context, customerID, productID string) (*Item, error) {
	var it Item
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO wishlist_items (customer_id, product_id) VALUES ($1, $2)
		 ON CONFLICT (customer_id, product_id) DO UPDATE SET created_at = wishlist_items.created_at
		 RETURNING id, product_id, created_at`,
		customerID, productID).Scan(&it.ID, &it.ProductID, &it.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (s *Service) Remove(ctx context.Context, customerID, productID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM wishlist_items WHERE customer_id = $1 AND product_id = $2`,
		customerID, productID)
	return err
}

// Merge inserts each productID for the customer (best-effort). Used to combine
// a guest localStorage list with the server list right after login.
func (s *Service) Merge(ctx context.Context, customerID string, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, pid := range productIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO wishlist_items (customer_id, product_id) VALUES ($1, $2)
			 ON CONFLICT (customer_id, product_id) DO NOTHING`,
			customerID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── HTTP handler ─────────────────────────────────────────────────────

type Handler struct {
	svc       *Service
	jwtSecret string
}

func NewHandler(svc *Service, jwtSecret string) *Handler {
	return &Handler{svc: svc, jwtSecret: jwtSecret}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.CustomerMiddleware(h.jwtSecret))
	r.Get("/", h.list)
	r.Post("/", h.add)
	r.Delete("/{productID}", h.remove)
	r.Post("/merge", h.merge)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	items, err := h.svc.List(r.Context(), cid)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, items)
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	var body struct {
		ProductID string `json:"product_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.ProductID == "" {
		respond.BadRequest(w, "product_id is required")
		return
	}
	it, err := h.svc.Add(r.Context(), cid, body.ProductID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, it)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	productID := chi.URLParam(r, "productID")
	if productID == "" {
		respond.BadRequest(w, "product id is required")
		return
	}
	if err := h.svc.Remove(r.Context(), cid, productID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) merge(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	var body struct {
		ProductIDs []string `json:"product_ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.Merge(r.Context(), cid, body.ProductIDs); err != nil {
		respond.InternalError(w)
		return
	}
	items, err := h.svc.List(r.Context(), cid)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, items)
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
