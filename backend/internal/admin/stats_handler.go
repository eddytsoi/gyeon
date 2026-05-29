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
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}

	var stats Stats

	// Products count is a current snapshot — intentionally not scoped by the
	// dashboard filters (asking "how many active products in the last 7 days"
	// is rarely what an admin actually wants here).
	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM products WHERE status = 'active'`).
		Scan(&stats.TotalProducts)

	// Total orders — count of orders in range, respecting role + category.
	ordJoin, ordWhere, ordArgs := f.scopeOrders()
	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders o`+ordJoin+ordWhere, ordArgs...).
		Scan(&stats.TotalOrders)

	// Total revenue — excludes cancelled/refunded; under a category filter this
	// becomes the sum of matching line items.
	custJoin, catJoin, rev, _, revWhere, revArgs := f.scopeRevenue("o.status NOT IN ('cancelled', 'refunded')")
	h.db.QueryRowContext(r.Context(),
		`SELECT `+rev+` FROM orders o`+custJoin+catJoin+revWhere, revArgs...).
		Scan(&stats.TotalRevenue)

	// Pending orders in range.
	penJoin, penWhere, penArgs := f.scopeOrders("o.status = 'pending'")
	h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders o`+penJoin+penWhere, penArgs...).
		Scan(&stats.PendingOrders)

	respond.JSON(w, http.StatusOK, stats)
}
