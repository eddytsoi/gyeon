// Package lookup provides admin-only endpoints that resolve sequential
// display numbers (e.g. PRD-8, ORD-1) to their underlying UUIDs.
//
// The endpoints are mounted under the admin auth middleware so that
// guessable sequential IDs are never exposed to anonymous callers.
package lookup

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/cms"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/respond"
	"gyeon/backend/internal/shop"
)

type Handler struct {
	products *shop.ProductService
	orders   *orders.OrderService
	pages    *cms.PageService
	posts    *cms.PostService
}

func NewHandler(
	products *shop.ProductService,
	ordersSvc *orders.OrderService,
	pages *cms.PageService,
	posts *cms.PostService,
) *Handler {
	return &Handler{products: products, orders: ordersSvc, pages: pages, posts: posts}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{entity}/{number}", h.lookup)
	return r
}

func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	entity := chi.URLParam(r, "entity")
	numStr := chi.URLParam(r, "number")
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || n <= 0 {
		respond.NotFound(w)
		return
	}

	id, err := h.resolve(r.Context(), entity, n)
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

func (h *Handler) resolve(ctx context.Context, entity string, n int64) (string, error) {
	switch entity {
	case "products":
		return h.products.GetIDByNumber(ctx, n)
	case "orders":
		return h.orders.GetIDByNumber(ctx, n)
	case "pages":
		return h.pages.GetIDByNumber(ctx, n)
	case "posts":
		return h.posts.GetIDByNumber(ctx, n)
	default:
		return "", sql.ErrNoRows
	}
}
