package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

// AnalyticsHandler exposes time-series and breakdown queries for the admin
// dashboard (P2 #16). All routes are admin-protected and accept optional
// `from` / `to` ISO date params; defaults to last 30 days.
type AnalyticsHandler struct {
	db *sql.DB
}

func NewAnalyticsHandler(db *sql.DB) *AnalyticsHandler {
	return &AnalyticsHandler{db: db}
}

func (h *AnalyticsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/revenue", h.revenue)
	r.Get("/top-products", h.topProducts)
	r.Get("/top-customers", h.topCustomers)
	r.Get("/order-status-breakdown", h.statusBreakdown)
	r.Get("/refund-total", h.refundTotal)
	return r
}

// parseRange returns from + to in ISO format. Defaults to past 30 days.
func parseRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now()
	to := now
	from := now.AddDate(0, 0, -30)

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid from")
		}
		from = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid to")
		}
		// inclusive: include the entire `to` day
		to = t.Add(24 * time.Hour)
	}
	return from, to, nil
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("bad date format")
}

// ── Revenue trend ────────────────────────────────────────────────────────────

type revenuePoint struct {
	Date       string  `json:"date"` // YYYY-MM-DD
	Revenue    float64 `json:"revenue"`
	OrderCount int     `json:"order_count"`
}

func (h *AnalyticsHandler) revenue(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD') AS d,
		        COALESCE(SUM(total), 0) AS rev,
		        COUNT(*) AS n
		   FROM orders
		  WHERE created_at >= $1 AND created_at < $2
		    AND status NOT IN ('cancelled')
		  GROUP BY 1 ORDER BY 1`, from, to)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()
	out := make([]revenuePoint, 0)
	for rows.Next() {
		var p revenuePoint
		if err := rows.Scan(&p.Date, &p.Revenue, &p.OrderCount); err != nil {
			respond.InternalError(w)
			return
		}
		out = append(out, p)
	}
	respond.JSON(w, http.StatusOK, out)
}

// ── Top products ─────────────────────────────────────────────────────────────

type topProduct struct {
	VariantID    *string `json:"variant_id,omitempty"`
	ProductName  string  `json:"product_name"`
	VariantSKU   string  `json:"variant_sku"`
	QtySold      int     `json:"qty_sold"`
	Revenue      float64 `json:"revenue"`
}

func (h *AnalyticsHandler) topProducts(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	by := r.URL.Query().Get("by")
	orderCol := "qty_sold DESC"
	if by == "revenue" {
		orderCol = "revenue DESC"
	}

	q := `
		SELECT oi.variant_id, oi.product_name, oi.variant_sku,
		       SUM(oi.quantity) AS qty_sold,
		       SUM(oi.line_total) AS revenue
		  FROM order_items oi
		  JOIN orders o ON o.id = oi.order_id
		 WHERE o.created_at >= $1 AND o.created_at < $2
		   AND o.status NOT IN ('cancelled')
		 GROUP BY oi.variant_id, oi.product_name, oi.variant_sku
		 ORDER BY ` + orderCol + `
		 LIMIT 10`

	rows, err := h.db.QueryContext(r.Context(), q, from, to)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()
	out := make([]topProduct, 0)
	for rows.Next() {
		var p topProduct
		if err := rows.Scan(&p.VariantID, &p.ProductName, &p.VariantSKU, &p.QtySold, &p.Revenue); err != nil {
			respond.InternalError(w)
			return
		}
		out = append(out, p)
	}
	respond.JSON(w, http.StatusOK, out)
}

// ── Top customers ────────────────────────────────────────────────────────────

type topCustomer struct {
	CustomerID *string `json:"customer_id,omitempty"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	OrderCount int     `json:"order_count"`
	TotalSpent float64 `json:"total_spent"`
}

func (h *AnalyticsHandler) topCustomers(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT o.customer_id,
		        COALESCE(c.email, o.customer_email, '') AS email,
		        COALESCE(c.name, o.customer_name, '') AS name,
		        COUNT(*) AS n,
		        SUM(o.total) AS spent
		   FROM orders o
		   LEFT JOIN customers c ON c.id = o.customer_id
		  WHERE o.created_at >= $1 AND o.created_at < $2
		    AND o.status NOT IN ('cancelled')
		  GROUP BY o.customer_id, c.email, o.customer_email, c.name, o.customer_name
		  ORDER BY spent DESC
		  LIMIT 10`, from, to)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()
	out := make([]topCustomer, 0)
	for rows.Next() {
		var p topCustomer
		if err := rows.Scan(&p.CustomerID, &p.Email, &p.Name, &p.OrderCount, &p.TotalSpent); err != nil {
			respond.InternalError(w)
			return
		}
		out = append(out, p)
	}
	respond.JSON(w, http.StatusOK, out)
}

// ── Order status breakdown ───────────────────────────────────────────────────

type statusBreakdownPoint struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func (h *AnalyticsHandler) statusBreakdown(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT status, COUNT(*) FROM orders
		  WHERE created_at >= $1 AND created_at < $2
		  GROUP BY status ORDER BY count DESC`, from, to)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()
	out := make([]statusBreakdownPoint, 0)
	for rows.Next() {
		var p statusBreakdownPoint
		if err := rows.Scan(&p.Status, &p.Count); err != nil {
			respond.InternalError(w)
			return
		}
		out = append(out, p)
	}
	respond.JSON(w, http.StatusOK, out)
}

// ── Refund total (for the new KPI card) ──────────────────────────────────────

type refundTotalResp struct {
	Refunds float64 `json:"refunds"`
}

func (h *AnalyticsHandler) refundTotal(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseRange(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	var total float64
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(SUM(refund_amount), 0) FROM orders
		  WHERE refunded_at IS NOT NULL
		    AND refunded_at >= $1 AND refunded_at < $2`, from, to).
		Scan(&total); err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, refundTotalResp{Refunds: total})
}
