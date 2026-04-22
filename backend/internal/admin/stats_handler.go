package admin

import (
	"database/sql"
	"net/http"

	"gyeon/backend/internal/respond"
)

type Stats struct {
	TotalProducts int     `json:"total_products"`
	TotalOrders   int     `json:"total_orders"`
	TotalRevenue  float64 `json:"total_revenue"`
	PendingOrders int     `json:"pending_orders"`
}

type StatsHandler struct {
	db *sql.DB
}

func NewStatsHandler(db *sql.DB) *StatsHandler {
	return &StatsHandler{db: db}
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	var stats Stats

	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM products WHERE is_active = TRUE`).
		Scan(&stats.TotalProducts)

	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders`).
		Scan(&stats.TotalOrders)

	h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(SUM(total), 0) FROM orders
		 WHERE status NOT IN ('cancelled', 'refunded')`).
		Scan(&stats.TotalRevenue)

	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders WHERE status = 'pending'`).
		Scan(&stats.PendingOrders)

	respond.JSON(w, http.StatusOK, stats)
}
