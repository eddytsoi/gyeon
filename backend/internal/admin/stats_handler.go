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
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}

	var stats Stats

	// Products count is a current snapshot — intentionally not scoped by the
	// dashboard date filter (asking "how many active products in the last
	// 7 days" is rarely what an admin actually wants here).
	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM products WHERE status = 'active'`).
		Scan(&stats.TotalProducts)

	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders
		 WHERE created_at >= $1 AND created_at < $2`, from, to).
		Scan(&stats.TotalOrders)

	h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(SUM(total), 0) FROM orders
		 WHERE created_at >= $1 AND created_at < $2
		   AND status NOT IN ('cancelled', 'refunded')`, from, to).
		Scan(&stats.TotalRevenue)

	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders
		 WHERE created_at >= $1 AND created_at < $2
		   AND status = 'pending'`, from, to).
		Scan(&stats.PendingOrders)

	respond.JSON(w, http.StatusOK, stats)
}
