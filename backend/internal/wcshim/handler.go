package wcshim

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"gyeon/backend/internal/orders"
)

// Handler exposes the small slice of WC REST endpoints that ShipAny
// needs in order to push shipment status updates back to the merchant.
//
// Routes are mounted at /wp-json/wc/v3 in main.go so the URL shape
// matches what ShipAny stored against the merchant when the original
// WC plugin authorised it via /wc-auth/v1/authorize.
type Handler struct {
	db       *sql.DB
	orderSvc *orders.OrderService
}

func NewHandler(db *sql.DB, orderSvc *orders.OrderService) *Handler {
	return &Handler{db: db, orderSvc: orderSvc}
}

// Routes returns a chi router that should be mounted at /orders under
// the /wp-json/wc/v3 prefix. Every route requires Basic Auth that
// resolves against legacy_wc_api_keys.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(BasicAuthMiddleware(h.db))
	r.Put("/{id}", h.updateOrderHandler)
	return r
}
