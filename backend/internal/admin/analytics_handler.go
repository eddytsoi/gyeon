package admin

import (
	"context"
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
	r.Get("/dashboard", h.dashboard)
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

var validDashRoles = map[string]bool{"customer": true, "installer": true, "installer_v2": true}

// dashFilters is the parsed, validated set of dashboard filters. The zero value
// of Roles / CategorySlug means "no filter on that dimension".
type dashFilters struct {
	From, To     time.Time
	Roles        []string // validated subset of {customer, installer, installer_v2}
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
		// when a category filter is active). A role='customer' buyer counts as
		// a registered 顧客 only when they have a real account — a migrated WC
		// account (wc_customer_id), a password, or a linked Google/Apple login.
		// Everything else with role='customer', plus any order with no linked
		// customer at all, is a guest checkout (訪客). Installers keep their
		// role label regardless of account state.
		roleLabel := `CASE
		          WHEN c.id IS NULL THEN 'guest'
		          WHEN c.role::text = 'customer'
		               AND c.wc_customer_id IS NULL
		               AND (c.password_hash IS NULL OR c.password_hash = '')
		               AND NOT EXISTS (SELECT 1 FROM customer_oauth_identities oi WHERE oi.customer_id = c.id)
		            THEN 'guest'
		          ELSE c.role::text
		        END`
		_, catJoin, rev, cnt, where, a := f.scopeRevenue("o.status NOT IN ('cancelled')")
		args = a
		q = `SELECT ` + roleLabel + ` AS label, ` + rev + ` AS value, ` + cnt + ` AS n
		       FROM orders o
		       LEFT JOIN customers c ON c.id = o.customer_id` + catJoin + where + `
		      GROUP BY 1
		      ORDER BY value DESC`
	case "carrier":
		// Group by the WC shipping method title (順豐速運 (免運費)/(到付), Free
		// Shipping, …), set on import and on native orders from shipping_free.
		// Orders with no recorded method collapse into one bucket ('') that the
		// frontend renders as the localized 沒有記錄 / No record label.
		custJoin, catJoin, rev, cnt, where, a := f.scopeRevenue("o.status NOT IN ('cancelled')")
		args = a
		q = `SELECT COALESCE(NULLIF(o.shipping_method, ''), '') AS label, ` + rev + ` AS value, ` + cnt + ` AS n
		       FROM orders o` + custJoin + catJoin + where + `
		      GROUP BY 1
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

// ── Consolidated dashboard (KPIs + period comparison + sparklines + hero) ─────
//
// GET /admin/analytics/dashboard collapses the dashboard's scalar KPIs into a
// single response so the frontend doesn't fan out one HTTP call per metric, and
// so the period-over-period comparison is computed in one place. Each in-period
// metric is computed twice — once for the primary [from,to) window and once for
// the comparison window derived from the `compare` param — and the delta is the
// fractional change. All-time segments (customer counts, LTV) carry no
// comparison. Net profit / margin are emitted as disabled placeholders until a
// product-cost source exists.

type metricValue struct {
	Current  *float64 `json:"current"`
	Previous *float64 `json:"previous"`
	DeltaPct *float64 `json:"delta_pct"`
	Disabled bool     `json:"disabled,omitempty"`
}

type seriesPoint struct {
	Date  string  `json:"date"` // YYYY-MM-DD
	Value float64 `json:"value"`
}

type heroDayPoint struct {
	Day   int     `json:"day"` // day-of-month (1..31), used to overlay two months
	Value float64 `json:"value"`
}

type heroPeriod struct {
	Label string         `json:"label"` // YYYY-MM
	Total float64        `json:"total"`
	Daily []heroDayPoint `json:"daily"`
}

type dashboardResp struct {
	Range struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Compare     string `json:"compare"`
		CompareFrom string `json:"compare_from"`
		CompareTo   string `json:"compare_to"`
	} `json:"range"`
	Metrics map[string]metricValue   `json:"metrics"`
	Series  map[string][]seriesPoint `json:"series"`
	Hero    struct {
		Current  heroPeriod `json:"current"`
		Previous heroPeriod `json:"previous"`
	} `json:"hero"`
}

// comparisonWindow derives the comparison [from,to) window for a primary window
// and compare mode. ok=false means "no comparison" (mode == "none"), in which
// case every previous / delta is null.
func comparisonWindow(from, to time.Time, mode string) (cFrom, cTo time.Time, ok bool) {
	switch mode {
	case "prev_period":
		d := to.Sub(from)
		return from.Add(-d), from, true
	case "prev_year":
		return from.AddDate(-1, 0, 0), to.AddDate(-1, 0, 0), true
	case "prev_month":
		return from.AddDate(0, -1, 0), to.AddDate(0, -1, 0), true
	default:
		return time.Time{}, time.Time{}, false
	}
}

// dashScalars holds the raw in-period aggregates for one window.
type dashScalars struct {
	netRevenue     float64
	orders         int
	itemsSold      int
	refundedAmount float64
	refundsCount   int
	failedOrders   int
	newCustomers   int
	cartsStarted   int
	cartsAbandoned int
}

type allTimeMetrics struct {
	total     int
	single    int
	repeat    int
	ltv       float64
	avgOrders float64
}

func (h *AnalyticsHandler) dashboard(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilters(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	mode := r.URL.Query().Get("compare")
	switch mode {
	case "prev_month", "prev_period", "prev_year", "none":
	default:
		mode = "prev_month"
	}
	ctx := r.Context()

	cur, err := h.computeScalars(ctx, f)
	if err != nil {
		respond.InternalError(w)
		return
	}

	var prev dashScalars
	cFrom, cTo, hasCompare := comparisonWindow(f.From, f.To, mode)
	if hasCompare {
		cf := dashFilters{From: cFrom, To: cTo, Roles: f.Roles, CategorySlug: f.CategorySlug}
		if prev, err = h.computeScalars(ctx, cf); err != nil {
			respond.InternalError(w)
			return
		}
	}

	at, err := h.computeAllTime(ctx)
	if err != nil {
		respond.InternalError(w)
		return
	}
	series, err := h.computeSeries(ctx, f)
	if err != nil {
		respond.InternalError(w)
		return
	}
	heroCur, heroPrev, err := h.computeHero(ctx, f)
	if err != nil {
		respond.InternalError(w)
		return
	}

	m := map[string]metricValue{
		"net_revenue":     mvCompare(cur.netRevenue, prev.netRevenue, hasCompare),
		"orders":          mvCompare(float64(cur.orders), float64(prev.orders), hasCompare),
		"items_sold":      mvCompare(float64(cur.itemsSold), float64(prev.itemsSold), hasCompare),
		"refunded_amount": mvCompare(cur.refundedAmount, prev.refundedAmount, hasCompare),
		"refunds_count":   mvCompare(float64(cur.refundsCount), float64(prev.refundsCount), hasCompare),
		"failed_orders":   mvCompare(float64(cur.failedOrders), float64(prev.failedOrders), hasCompare),
		"new_customers":   mvCompare(float64(cur.newCustomers), float64(prev.newCustomers), hasCompare),
		"carts_started":   mvCompare(float64(cur.cartsStarted), float64(prev.cartsStarted), hasCompare),
		"carts_abandoned": mvCompare(float64(cur.cartsAbandoned), float64(prev.cartsAbandoned), hasCompare),

		"avg_order_net":     mvCompare(safeDiv(cur.netRevenue, cur.orders), safeDiv(prev.netRevenue, prev.orders), hasCompare),
		"avg_order_items":   mvCompare(safeDiv(float64(cur.itemsSold), cur.orders), safeDiv(float64(prev.itemsSold), prev.orders), hasCompare),
		"cart_placed_rate":  mvCompare(placedRate(cur), placedRate(prev), hasCompare),
		"cart_abandon_rate": mvCompare(abandonRate(cur), abandonRate(prev), hasCompare),

		"customers_total":     mvSingle(float64(at.total)),
		"customers_single":    mvSingle(float64(at.single)),
		"customers_repeat":    mvSingle(float64(at.repeat)),
		"avg_customer_ltv":    mvSingle(at.ltv),
		"avg_customer_orders": mvSingle(at.avgOrders),

		// No product-cost source yet — surfaced as disabled placeholders.
		"net_profit":    {Disabled: true},
		"profit_margin": {Disabled: true},
	}

	var resp dashboardResp
	resp.Range.From = f.From.Format("2006-01-02")
	resp.Range.To = f.To.Add(-time.Second).Format("2006-01-02")
	resp.Range.Compare = mode
	if hasCompare {
		resp.Range.CompareFrom = cFrom.Format("2006-01-02")
		resp.Range.CompareTo = cTo.Add(-time.Second).Format("2006-01-02")
	}
	resp.Metrics = m
	resp.Series = series
	resp.Hero.Current = heroCur
	resp.Hero.Previous = heroPrev
	respond.JSON(w, http.StatusOK, resp)
}

// computeScalars runs the in-period aggregates for one window, reusing the
// shared scope helpers so role / category filters apply identically to the
// primary and comparison runs. Carts ignore role/category (a cart has no clean
// role/category dimension); customer registrations honour role but not category.
func (h *AnalyticsHandler) computeScalars(ctx context.Context, f dashFilters) (dashScalars, error) {
	var s dashScalars

	// Net revenue + order count.
	custJoin, catJoin, rev, cnt, where, args := f.scopeRevenue("o.status NOT IN ('cancelled', 'refunded')")
	if err := h.db.QueryRowContext(ctx,
		`SELECT `+rev+`, `+cnt+` FROM orders o`+custJoin+catJoin+where, args...).
		Scan(&s.netRevenue, &s.orders); err != nil {
		return s, err
	}

	// Items sold (line-item quantity).
	{
		iArgs := []any{f.From, f.To}
		iConds := []string{"o.created_at >= $1", "o.created_at < $2", "o.status NOT IN ('cancelled', 'refunded')"}
		iJoin := ""
		if len(f.Roles) > 0 {
			iJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
			iConds = append(iConds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(iArgs)+1))
			iArgs = append(iArgs, pq.Array(f.Roles))
		}
		catJoin := ""
		if f.CategorySlug != "" {
			catJoin = ` JOIN product_variants pv ON pv.id = oi.variant_id
			            JOIN product_category_links pcl ON pcl.product_id = pv.product_id
			            JOIN categories cat ON cat.id = pcl.category_id`
			iConds = append(iConds, fmt.Sprintf("cat.slug = $%d", len(iArgs)+1))
			iArgs = append(iArgs, f.CategorySlug)
		}
		if err := h.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(oi.quantity), 0) FROM order_items oi JOIN orders o ON o.id = oi.order_id`+
				iJoin+catJoin+` WHERE `+strings.Join(iConds, " AND "), iArgs...).
			Scan(&s.itemsSold); err != nil {
			return s, err
		}
	}

	// Refunded amount + count, scoped on refunded_at (the period the refund issued).
	{
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
		if err := h.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(o.refund_amount), 0), COUNT(*) FROM orders o`+rJoin+` WHERE `+strings.Join(rConds, " AND "),
			rArgs...).Scan(&s.refundedAmount, &s.refundsCount); err != nil {
			return s, err
		}
	}

	// Failed / cancelled orders (no 'failed' status exists; 'cancelled' is the closest).
	{
		cj, where, args := f.scopeOrders("o.status = 'cancelled'")
		if err := h.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM orders o`+cj+where, args...).Scan(&s.failedOrders); err != nil {
			return s, err
		}
	}

	// New customers registered in the window (role applies, category does not).
	{
		ncArgs := []any{f.From, f.To}
		ncConds := []string{"created_at >= $1", "created_at < $2"}
		if len(f.Roles) > 0 {
			ncConds = append(ncConds, fmt.Sprintf("role::text = ANY($%d::text[])", len(ncArgs)+1))
			ncArgs = append(ncArgs, pq.Array(f.Roles))
		}
		if err := h.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM customers WHERE `+strings.Join(ncConds, " AND "), ncArgs...).
			Scan(&s.newCustomers); err != nil {
			return s, err
		}
	}

	// Carts started in the window (carts carry no role/category dimension).
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM carts WHERE created_at >= $1 AND created_at < $2`,
		f.From, f.To).Scan(&s.cartsStarted); err != nil {
		return s, err
	}

	// Carts abandoned: created in the window, has items, never converted to a
	// paid+ order. (Range-scoped analytics view of abandoned/service.go's
	// operational definition — the email-sent / idle-cutoff conditions there are
	// for the recovery emailer, not for counting.)
	if err := h.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM carts c
		WHERE c.created_at >= $1 AND c.created_at < $2
		  AND EXISTS (SELECT 1 FROM cart_items ci WHERE ci.cart_id = c.id)
		  AND NOT EXISTS (
		      SELECT 1 FROM orders o
		      WHERE o.cart_id = c.id
		        AND o.status IN ('paid','processing','prepared','shipped','delivered','refunded'))`,
		f.From, f.To).Scan(&s.cartsAbandoned); err != nil {
		return s, err
	}

	return s, nil
}

// computeAllTime returns the lifetime customer segments — independent of the
// date range and filters (these are the reference's "ALL-TIME" cards).
func (h *AnalyticsHandler) computeAllTime(ctx context.Context) (allTimeMetrics, error) {
	var a allTimeMetrics
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM customers`).Scan(&a.total); err != nil {
		return a, err
	}
	if err := h.db.QueryRowContext(ctx, `
		WITH oc AS (
		    SELECT o.customer_id, COUNT(*) AS n FROM orders o
		    WHERE o.status NOT IN ('cancelled') AND o.customer_id IS NOT NULL
		    GROUP BY o.customer_id)
		SELECT COUNT(*) FILTER (WHERE n = 1), COUNT(*) FILTER (WHERE n >= 2) FROM oc`).
		Scan(&a.single, &a.repeat); err != nil {
		return a, err
	}
	if err := h.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(o.total) / NULLIF(COUNT(DISTINCT o.customer_id), 0), 0)
		FROM orders o WHERE o.status NOT IN ('cancelled', 'refunded') AND o.customer_id IS NOT NULL`).
		Scan(&a.ltv); err != nil {
		return a, err
	}
	if err := h.db.QueryRowContext(ctx, `
		SELECT COALESCE(COUNT(*)::float / NULLIF(COUNT(DISTINCT o.customer_id), 0), 0)
		FROM orders o WHERE o.status NOT IN ('cancelled') AND o.customer_id IS NOT NULL`).
		Scan(&a.avgOrders); err != nil {
		return a, err
	}
	return a, nil
}

// computeSeries returns daily sparkline series for the primary window.
func (h *AnalyticsHandler) computeSeries(ctx context.Context, f dashFilters) (map[string][]seriesPoint, error) {
	out := map[string][]seriesPoint{}

	// Net revenue + orders share the revenue scope.
	{
		custJoin, catJoin, rev, cnt, where, args := f.scopeRevenue("o.status NOT IN ('cancelled', 'refunded')")
		q := `SELECT TO_CHAR(DATE_TRUNC('day', o.created_at), 'YYYY-MM-DD') AS d, ` + rev + ` AS rev, ` + cnt + `::float AS n
		        FROM orders o` + custJoin + catJoin + where + ` GROUP BY 1 ORDER BY 1`
		rows, err := h.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		revSeries := []seriesPoint{}
		ordSeries := []seriesPoint{}
		for rows.Next() {
			var d string
			var rv, n float64
			if err := rows.Scan(&d, &rv, &n); err != nil {
				rows.Close()
				return nil, err
			}
			revSeries = append(revSeries, seriesPoint{Date: d, Value: rv})
			ordSeries = append(ordSeries, seriesPoint{Date: d, Value: n})
		}
		rows.Close()
		out["net_revenue"] = revSeries
		out["orders"] = ordSeries
	}

	// Items sold per day.
	{
		iArgs := []any{f.From, f.To}
		iConds := []string{"o.created_at >= $1", "o.created_at < $2", "o.status NOT IN ('cancelled', 'refunded')"}
		iJoin := ""
		if len(f.Roles) > 0 {
			iJoin = " LEFT JOIN customers c ON c.id = o.customer_id"
			iConds = append(iConds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(iArgs)+1))
			iArgs = append(iArgs, pq.Array(f.Roles))
		}
		catJoin := ""
		if f.CategorySlug != "" {
			catJoin = ` JOIN product_variants pv ON pv.id = oi.variant_id
			            JOIN product_category_links pcl ON pcl.product_id = pv.product_id
			            JOIN categories cat ON cat.id = pcl.category_id`
			iConds = append(iConds, fmt.Sprintf("cat.slug = $%d", len(iArgs)+1))
			iArgs = append(iArgs, f.CategorySlug)
		}
		s, err := h.daySeries(ctx,
			`SELECT TO_CHAR(DATE_TRUNC('day', o.created_at), 'YYYY-MM-DD') AS d, COALESCE(SUM(oi.quantity), 0)::float AS v
			   FROM order_items oi JOIN orders o ON o.id = oi.order_id`+iJoin+catJoin+
				` WHERE `+strings.Join(iConds, " AND ")+` GROUP BY 1 ORDER BY 1`, iArgs...)
		if err != nil {
			return nil, err
		}
		out["items_sold"] = s
	}

	// New customers per day (role applies, category does not).
	{
		ncArgs := []any{f.From, f.To}
		ncConds := []string{"created_at >= $1", "created_at < $2"}
		if len(f.Roles) > 0 {
			ncConds = append(ncConds, fmt.Sprintf("role::text = ANY($%d::text[])", len(ncArgs)+1))
			ncArgs = append(ncArgs, pq.Array(f.Roles))
		}
		s, err := h.daySeries(ctx,
			`SELECT TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD') AS d, COUNT(*)::float AS v
			   FROM customers WHERE `+strings.Join(ncConds, " AND ")+` GROUP BY 1 ORDER BY 1`, ncArgs...)
		if err != nil {
			return nil, err
		}
		out["new_customers"] = s
	}

	// Carts started per day.
	{
		s, err := h.daySeries(ctx,
			`SELECT TO_CHAR(DATE_TRUNC('day', created_at), 'YYYY-MM-DD') AS d, COUNT(*)::float AS v
			   FROM carts WHERE created_at >= $1 AND created_at < $2 GROUP BY 1 ORDER BY 1`, f.From, f.To)
		if err != nil {
			return nil, err
		}
		out["carts_started"] = s
	}

	// Carts abandoned per day.
	{
		s, err := h.daySeries(ctx, `
			SELECT TO_CHAR(DATE_TRUNC('day', c.created_at), 'YYYY-MM-DD') AS d, COUNT(*)::float AS v
			  FROM carts c
			 WHERE c.created_at >= $1 AND c.created_at < $2
			   AND EXISTS (SELECT 1 FROM cart_items ci WHERE ci.cart_id = c.id)
			   AND NOT EXISTS (
			       SELECT 1 FROM orders o
			       WHERE o.cart_id = c.id
			         AND o.status IN ('paid','processing','prepared','shipped','delivered','refunded'))
			 GROUP BY 1 ORDER BY 1`, f.From, f.To)
		if err != nil {
			return nil, err
		}
		out["carts_abandoned"] = s
	}

	return out, nil
}

// daySeries runs a two-column (date, value) grouped query into seriesPoints.
func (h *AnalyticsHandler) daySeries(ctx context.Context, q string, args ...any) ([]seriesPoint, error) {
	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []seriesPoint{}
	for rows.Next() {
		var p seriesPoint
		if err := rows.Scan(&p.Date, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// computeHero builds the current-month-to-date vs same-period-last-month net
// revenue series, keyed by day-of-month so the two months overlay even when the
// previous month is shorter. Always month-over-month regardless of the global
// date filter; honours role/category for consistency with the rest of the page.
func (h *AnalyticsHandler) computeHero(ctx context.Context, f dashFilters) (cur, prev heroPeriod, err error) {
	now := time.Now()
	day := now.Day()
	firstThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	firstPrev := firstThis.AddDate(0, -1, 0)

	prevTo := firstPrev.AddDate(0, 0, day)
	if prevTo.After(firstThis) {
		prevTo = firstThis // previous month shorter than today's day-of-month
	}

	if cur, err = h.heroPeriod(ctx, f, firstThis, firstThis.AddDate(0, 0, day), firstThis.Format("2006-01")); err != nil {
		return
	}
	prev, err = h.heroPeriod(ctx, f, firstPrev, prevTo, firstPrev.Format("2006-01"))
	return
}

func (h *AnalyticsHandler) heroPeriod(ctx context.Context, f dashFilters, from, to time.Time, label string) (heroPeriod, error) {
	hf := dashFilters{From: from, To: to, Roles: f.Roles, CategorySlug: f.CategorySlug}
	custJoin, catJoin, rev, _, where, args := hf.scopeRevenue("o.status NOT IN ('cancelled', 'refunded')")
	q := `SELECT EXTRACT(DAY FROM o.created_at)::int AS d, ` + rev + ` AS v
	        FROM orders o` + custJoin + catJoin + where + ` GROUP BY 1 ORDER BY 1`
	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return heroPeriod{}, err
	}
	defer rows.Close()
	hp := heroPeriod{Label: label, Daily: []heroDayPoint{}}
	for rows.Next() {
		var p heroDayPoint
		if err := rows.Scan(&p.Day, &p.Value); err != nil {
			return heroPeriod{}, err
		}
		hp.Daily = append(hp.Daily, p)
		hp.Total += p.Value
	}
	return hp, rows.Err()
}

// ── metric helpers ────────────────────────────────────────────────────────────

// mvCompare builds a metric with an optional period-over-period delta. The delta
// is null when there is no comparison or the previous value is zero (no sane
// percentage from a zero base).
func mvCompare(current, previous float64, hasCompare bool) metricValue {
	c := current
	out := metricValue{Current: &c}
	if hasCompare {
		p := previous
		out.Previous = &p
		if previous != 0 {
			d := (current - previous) / previous
			out.DeltaPct = &d
		}
	}
	return out
}

// mvSingle builds an all-time metric (value only, no comparison).
func mvSingle(current float64) metricValue {
	c := current
	return metricValue{Current: &c}
}

func safeDiv(a float64, b int) float64 {
	if b == 0 {
		return 0
	}
	return a / float64(b)
}

func placedRate(s dashScalars) float64 {
	if s.cartsStarted == 0 {
		return 0
	}
	return float64(s.cartsStarted-s.cartsAbandoned) / float64(s.cartsStarted)
}

func abandonRate(s dashScalars) float64 {
	if s.cartsStarted == 0 {
		return 0
	}
	return float64(s.cartsAbandoned) / float64(s.cartsStarted)
}
