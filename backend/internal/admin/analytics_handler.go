package admin

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"gyeon/backend/internal/respond"
)

// AnalyticsHandler exposes time-series and breakdown queries for the admin
// dashboard. All routes are admin-protected and accept the shared dashboard
// filters: `from` / `to` (ISO date, defaults to last 30 days), `role`
// (comma-separated customer roles) and `category` (a single category slug).
// Every aggregate respects these filters so the whole dashboard moves together.
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
	r.Get("/summary", h.summary)
	r.Get("/revenue-breakdown", h.revenueBreakdown)
	return r
}

// ── Shared dashboard filters ─────────────────────────────────────────────────

var validDashRoles = map[string]bool{"customer": true, "installer": true}

// dashFilters is the parsed, validated set of dashboard filters. The zero value
// of Roles / CategorySlug means "no filter on that dimension".
type dashFilters struct {
	From, To     time.Time
	Roles        []string // validated subset of {customer, installer}
	CategorySlug string   // "" = all categories
}

// parseFilters reads the shared dashboard filters off the request. Defaults to
// the past 30 days. Dates are parsed in the server's local timezone so the
// dashboard's day boundaries line up with the admin orders list.
func parseFilters(r *http.Request) (dashFilters, error) {
	now := time.Now()
	f := dashFilters{From: now.AddDate(0, 0, -30), To: now}

	if v := r.URL.Query().Get("from"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			return f, errors.New("invalid from")
		}
		f.From = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := parseDate(v)
		if err != nil {
			return f, errors.New("invalid to")
		}
		// inclusive: include the entire `to` day
		f.To = t.Add(24 * time.Hour)
	}
	for _, tok := range strings.Split(r.URL.Query().Get("role"), ",") {
		if tok = strings.TrimSpace(tok); validDashRoles[tok] {
			f.Roles = append(f.Roles, tok)
		}
	}
	f.CategorySlug = strings.TrimSpace(r.URL.Query().Get("category"))
	return f, nil
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("bad date format")
}

// scopeOrders builds the WHERE clause for count / whole-order aggregates over
// `orders o`. Category membership is expressed as an EXISTS subquery (an order
// qualifies if it contains ≥1 item in the category) so it never multiplies rows.
// custJoin is the customers join needed for the role filter; callers that
// already join `customers c` themselves can ignore it (it is only non-empty
// when a role filter is active, and the role condition references `c`).
func (f dashFilters) scopeOrders(extra ...string) (custJoin, where string, args []any) {
	args = []any{f.From, f.To}
	conds := append([]string{"o.created_at >= $1", "o.created_at < $2"}, extra...)

	if len(f.Roles) > 0 {
		custJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
		conds = append(conds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(args)+1))
		args = append(args, pq.Array(f.Roles))
	}
	if f.CategorySlug != "" {
		conds = append(conds, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM order_items oi
			           JOIN product_variants pv ON pv.id = oi.variant_id
			           JOIN product_category_links pcl ON pcl.product_id = pv.product_id
			           JOIN categories cat ON cat.id = pcl.category_id
			          WHERE oi.order_id = o.id AND cat.slug = $%d)`, len(args)+1))
		args = append(args, f.CategorySlug)
	}
	where = " WHERE " + strings.Join(conds, " AND ")
	return
}

// scopeRevenue builds the pieces for sum-of-revenue aggregates over `orders o`.
// With a category filter active, revenue is the sum of the matching line items
// (so a multi-category order only contributes its in-category value) and orders
// are counted DISTINCT; otherwise revenue is the whole-order total. catJoin /
// custJoin are returned separately so callers that already join `customers c`
// can drop custJoin while keeping catJoin.
func (f dashFilters) scopeRevenue(extra ...string) (custJoin, catJoin, revExpr, countExpr, where string, args []any) {
	args = []any{f.From, f.To}
	conds := append([]string{"o.created_at >= $1", "o.created_at < $2"}, extra...)

	if len(f.Roles) > 0 {
		custJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
		conds = append(conds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(args)+1))
		args = append(args, pq.Array(f.Roles))
	}

	revExpr = "COALESCE(SUM(o.total), 0)"
	countExpr = "COUNT(*)"
	if f.CategorySlug != "" {
		catJoin = ` JOIN order_items oi ON oi.order_id = o.id
		            JOIN product_variants pv ON pv.id = oi.variant_id
		            JOIN product_category_links pcl ON pcl.product_id = pv.product_id
		            JOIN categories cat ON cat.id = pcl.category_id`
		conds = append(conds, fmt.Sprintf("cat.slug = $%d", len(args)+1))
		args = append(args, f.CategorySlug)
		revExpr = "COALESCE(SUM(oi.line_total), 0)"
		countExpr = "COUNT(DISTINCT o.id)"
	}
	where = " WHERE " + strings.Join(conds, " AND ")
	return
}

// ── Revenue trend ────────────────────────────────────────────────────────────

type revenuePoint struct {
	Date       string  `json:"date"` // YYYY-MM-DD
	Revenue    float64 `json:"revenue"`
	OrderCount int     `json:"order_count"`
}

func (h *AnalyticsHandler) revenue(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	custJoin, catJoin, rev, cnt, where, args := f.scopeRevenue("o.status NOT IN ('cancelled')")
	q := `SELECT TO_CHAR(DATE_TRUNC('day', o.created_at), 'YYYY-MM-DD') AS d,
	             ` + rev + ` AS rev,
	             ` + cnt + ` AS n
	        FROM orders o` + custJoin + catJoin + where + `
	       GROUP BY 1 ORDER BY 1`
	rows, err := h.db.QueryContext(r.Context(), q, args...)
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
	VariantID   *string `json:"variant_id,omitempty"`
	ProductName string  `json:"product_name"`
	VariantSKU  string  `json:"variant_sku"`
	QtySold     int     `json:"qty_sold"`
	Revenue     float64 `json:"revenue"`
}

func (h *AnalyticsHandler) topProducts(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	orderCol := "qty_sold DESC"
	if r.URL.Query().Get("by") == "revenue" {
		orderCol = "revenue DESC"
	}

	// Top products is inherently line-item based, so it joins order_items
	// directly rather than going through scopeRevenue.
	args := []any{f.From, f.To}
	conds := []string{"o.created_at >= $1", "o.created_at < $2", "o.status NOT IN ('cancelled')"}
	join := ""
	if len(f.Roles) > 0 {
		join = " LEFT JOIN customers c ON c.id = o.customer_id"
		conds = append(conds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(args)+1))
		args = append(args, pq.Array(f.Roles))
	}
	catJoin := ""
	if f.CategorySlug != "" {
		catJoin = ` JOIN product_variants pv ON pv.id = oi.variant_id
		            JOIN product_category_links pcl ON pcl.product_id = pv.product_id
		            JOIN categories cat ON cat.id = pcl.category_id`
		conds = append(conds, fmt.Sprintf("cat.slug = $%d", len(args)+1))
		args = append(args, f.CategorySlug)
	}

	q := `SELECT oi.variant_id, oi.product_name, oi.variant_sku,
	             SUM(oi.quantity) AS qty_sold,
	             SUM(oi.line_total) AS revenue
	        FROM order_items oi
	        JOIN orders o ON o.id = oi.order_id` + join + catJoin + `
	       WHERE ` + strings.Join(conds, " AND ") + `
	       GROUP BY oi.variant_id, oi.product_name, oi.variant_sku
	       ORDER BY ` + orderCol + `
	       LIMIT 10`

	rows, err := h.db.QueryContext(r.Context(), q, args...)
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
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	// We self-join customers (for name/email), so the role filter's condition
	// resolves against the same `c` — custJoin from scopeOrders is dropped.
	_, where, args := f.scopeOrders("o.status NOT IN ('cancelled')")
	q := `SELECT o.customer_id,
	             COALESCE(c.email, o.customer_email, '') AS email,
	             COALESCE(NULLIF(TRIM(COALESCE(c.first_name, '') || ' ' || COALESCE(c.last_name, '')), ''), o.customer_name, '') AS name,
	             COUNT(*) AS n,
	             SUM(o.total) AS spent
	        FROM orders o
	        LEFT JOIN customers c ON c.id = o.customer_id` + where + `
	       GROUP BY o.customer_id, c.email, o.customer_email, c.first_name, c.last_name, o.customer_name
	       ORDER BY spent DESC
	       LIMIT 10`
	rows, err := h.db.QueryContext(r.Context(), q, args...)
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
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	custJoin, where, args := f.scopeOrders()
	q := `SELECT o.status, COUNT(*) FROM orders o` + custJoin + where + `
	      GROUP BY o.status ORDER BY COUNT(*) DESC`
	rows, err := h.db.QueryContext(r.Context(), q, args...)
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

// ── Refund total + refund rate ───────────────────────────────────────────────

type refundResp struct {
	Refunds          float64 `json:"refunds"` // kept for back-compat with the KPI card
	RefundedOrders   int     `json:"refunded_orders"`
	TotalOrders      int     `json:"total_orders"`
	Revenue          float64 `json:"revenue"`
	RefundOrderRate  float64 `json:"refund_order_rate"`  // refunded orders / total orders
	RefundAmountRate float64 `json:"refund_amount_rate"` // refund amount / revenue
}

func (h *AnalyticsHandler) refundTotal(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	var resp refundResp

	// Refunded amount + count — scoped on refunded_at (the period the refund was
	// issued), with the same role / category membership filters.
	rArgs := []any{f.From, f.To}
	rConds := []string{"o.refunded_at IS NOT NULL", "o.refunded_at >= $1", "o.refunded_at < $2"}
	rJoin := ""
	if len(f.Roles) > 0 {
		rJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
		rConds = append(rConds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(rArgs)+1))
		rArgs = append(rArgs, pq.Array(f.Roles))
	}
	if f.CategorySlug != "" {
		rConds = append(rConds, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM order_items oi
			           JOIN product_variants pv ON pv.id = oi.variant_id
			           JOIN product_category_links pcl ON pcl.product_id = pv.product_id
			           JOIN categories cat ON cat.id = pcl.category_id
			          WHERE oi.order_id = o.id AND cat.slug = $%d)`, len(rArgs)+1))
		rArgs = append(rArgs, f.CategorySlug)
	}
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(SUM(o.refund_amount), 0), COUNT(*) FROM orders o`+rJoin+` WHERE `+strings.Join(rConds, " AND "),
		rArgs...).Scan(&resp.Refunds, &resp.RefundedOrders); err != nil {
		respond.InternalError(w)
		return
	}

	// Denominator: all orders created in the window (any status).
	cj, where, args := f.scopeOrders()
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM orders o`+cj+where, args...).Scan(&resp.TotalOrders); err != nil {
		respond.InternalError(w)
		return
	}

	// Gross revenue in the window (matches the revenue KPI: excl cancelled/refunded).
	custJoin, catJoin, rev, _, rwhere, rargs := f.scopeRevenue("o.status NOT IN ('cancelled', 'refunded')")
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT `+rev+` FROM orders o`+custJoin+catJoin+rwhere, rargs...).Scan(&resp.Revenue); err != nil {
		respond.InternalError(w)
		return
	}

	if resp.TotalOrders > 0 {
		resp.RefundOrderRate = float64(resp.RefundedOrders) / float64(resp.TotalOrders)
	}
	if resp.Revenue > 0 {
		resp.RefundAmountRate = resp.Refunds / resp.Revenue
	}
	respond.JSON(w, http.StatusOK, resp)
}

// ── Summary (AOV, new customers, repeat ratio) ───────────────────────────────

type summaryResp struct {
	Revenue         float64 `json:"revenue"`
	OrderCount      int     `json:"order_count"`
	AOV             float64 `json:"aov"`
	NewCustomers    int     `json:"new_customers"`
	RepeatCustomers int     `json:"repeat_customers"`
	RepeatRatio     float64 `json:"repeat_ratio"`
}

func (h *AnalyticsHandler) summary(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	var resp summaryResp

	// Revenue + order count (drives AOV).
	custJoin, catJoin, rev, cnt, where, args := f.scopeRevenue("o.status NOT IN ('cancelled')")
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT `+rev+`, `+cnt+` FROM orders o`+custJoin+catJoin+where, args...).
		Scan(&resp.Revenue, &resp.OrderCount); err != nil {
		respond.InternalError(w)
		return
	}
	if resp.OrderCount > 0 {
		resp.AOV = resp.Revenue / float64(resp.OrderCount)
	}

	// New customers registered in the window (category does not apply to a
	// customer; the role filter does).
	ncArgs := []any{f.From, f.To}
	ncConds := []string{"created_at >= $1", "created_at < $2"}
	if len(f.Roles) > 0 {
		ncConds = append(ncConds, fmt.Sprintf("role::text = ANY($%d::text[])", len(ncArgs)+1))
		ncArgs = append(ncArgs, pq.Array(f.Roles))
	}
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM customers WHERE `+strings.Join(ncConds, " AND "), ncArgs...).
		Scan(&resp.NewCustomers); err != nil {
		respond.InternalError(w)
		return
	}

	// Repeat ratio: customers with ≥2 orders vs customers with ≥1 order in the
	// window (respecting the role / category scope).
	cj, owhere, oargs := f.scopeOrders("o.status NOT IN ('cancelled')", "o.customer_id IS NOT NULL")
	var distinct int
	if err := h.db.QueryRowContext(r.Context(),
		`WITH oc AS (SELECT o.customer_id, COUNT(*) AS n FROM orders o`+cj+owhere+` GROUP BY o.customer_id)
		 SELECT COUNT(*) FILTER (WHERE n >= 2), COUNT(*) FROM oc`, oargs...).
		Scan(&resp.RepeatCustomers, &distinct); err != nil {
		respond.InternalError(w)
		return
	}
	if distinct > 0 {
		resp.RepeatRatio = float64(resp.RepeatCustomers) / float64(distinct)
	}

	respond.JSON(w, http.StatusOK, resp)
}

// ── Revenue breakdown (by category / role / carrier) ─────────────────────────

type breakdownRow struct {
	Label      string  `json:"label"`
	Value      float64 `json:"value"`
	OrderCount int     `json:"order_count"`
}

func (h *AnalyticsHandler) revenueBreakdown(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}

	var q string
	var args []any

	switch r.URL.Query().Get("by") {
	case "role":
		// Group by customer role; whole-order revenue (or matching line items
		// when a category filter is active).
		_, catJoin, rev, cnt, where, a := f.scopeRevenue("o.status NOT IN ('cancelled')")
		args = a
		q = `SELECT COALESCE(c.role::text, 'guest') AS label, ` + rev + ` AS value, ` + cnt + ` AS n
		       FROM orders o
		       LEFT JOIN customers c ON c.id = o.customer_id` + catJoin + where + `
		      GROUP BY COALESCE(c.role::text, 'guest')
		      ORDER BY value DESC`
	case "carrier":
		custJoin, catJoin, rev, cnt, where, a := f.scopeRevenue("o.status NOT IN ('cancelled')")
		args = a
		q = `SELECT COALESCE(NULLIF(o.selected_carrier, ''), '—') AS label, ` + rev + ` AS value, ` + cnt + ` AS n
		       FROM orders o` + custJoin + catJoin + where + `
		      GROUP BY COALESCE(NULLIF(o.selected_carrier, ''), '—')
		      ORDER BY value DESC`
	default: // "category"
		// Line-item revenue attributed to each product's primary category.
		args = []any{f.From, f.To}
		conds := []string{"o.created_at >= $1", "o.created_at < $2", "o.status NOT IN ('cancelled')"}
		custJoin := ""
		if len(f.Roles) > 0 {
			custJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
			conds = append(conds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(args)+1))
			args = append(args, pq.Array(f.Roles))
		}
		if f.CategorySlug != "" {
			conds = append(conds, fmt.Sprintf("cat.slug = $%d", len(args)+1))
			args = append(args, f.CategorySlug)
		}
		q = `SELECT COALESCE(cat.name, '—') AS label,
		            COALESCE(SUM(oi.line_total), 0) AS value,
		            COUNT(DISTINCT o.id) AS n
		       FROM order_items oi
		       JOIN orders o ON o.id = oi.order_id
		       JOIN product_variants pv ON pv.id = oi.variant_id
		       JOIN products p ON p.id = pv.product_id
		       LEFT JOIN categories cat ON cat.id = p.category_id` + custJoin + `
		      WHERE ` + strings.Join(conds, " AND ") + `
		      GROUP BY cat.name
		      ORDER BY value DESC`
	}

	rows, err := h.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()
	out := make([]breakdownRow, 0)
	for rows.Next() {
		var b breakdownRow
		if err := rows.Scan(&b.Label, &b.Value, &b.OrderCount); err != nil {
			respond.InternalError(w)
			return
		}
		out = append(out, b)
	}
	respond.JSON(w, http.StatusOK, out)
}
