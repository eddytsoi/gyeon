package admin

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

// StockVelocityHandler powers the admin "Stock Velocity" (庫存速率) report: for a
// trailing window it lists, per product variant with ≥1 sale, the units/revenue
// sold, the daily sell-through rate and how many days of stock are left. All
// routes are admin-protected (mounted in the Tier-2 admin+super_admin group).
type StockVelocityHandler struct {
	db *sql.DB
}

func NewStockVelocityHandler(db *sql.DB) *StockVelocityHandler {
	return &StockVelocityHandler{db: db}
}

func (h *StockVelocityHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/export.csv", h.exportCSV)
	return r
}

// validVelocityDays bounds the trailing window to the presets the UI offers. The
// window length is also the denominator for daily_gross_sold.
var validVelocityDays = map[int]bool{7: true, 15: true, 30: true, 60: true, 90: true, 180: true, 365: true}

// velocitySort maps a public sort key to a vetted ORDER BY expression. The value
// is concatenated into SQL, so it MUST stay a fixed whitelist — never interpolate
// raw user input. All expressions reference SELECT-output column aliases.
var velocitySort = map[string]string{
	"gross_sales_desc":      "gross_sales DESC, sku ASC",
	"gross_sales_asc":       "gross_sales ASC, sku ASC",
	"gross_sold_desc":       "gross_sold DESC, sku ASC",
	"gross_sold_asc":        "gross_sold ASC, sku ASC",
	"daily_gross_sold_desc": "daily_gross_sold DESC, sku ASC",
	"daily_gross_sold_asc":  "daily_gross_sold ASC, sku ASC",
	"days_left_asc":         "days_left ASC NULLS LAST, sku ASC",
	"days_left_desc":        "days_left DESC NULLS LAST, sku ASC",
	"stock_desc":            "stock_qty DESC, sku ASC",
	"stock_asc":             "stock_qty ASC, sku ASC",
	"variation_asc":         "variation ASC",
	"variation_desc":        "variation DESC",
}

const (
	defaultVelocitySort = "gross_sold_desc"
	velocityRowCap      = 5000
)

type velocityRow struct {
	VariantID      string  `json:"variant_id"`
	ProductID      string  `json:"product_id"`
	ProductName    string  `json:"product_name"`
	SKU            string  `json:"sku"`
	Variation      string  `json:"variation"`
	StockQty       int     `json:"stock_qty"`
	InStock        bool    `json:"in_stock"`
	GrossSales     float64 `json:"gross_sales"`
	GrossSold      int     `json:"gross_sold"`
	DailyGrossSold float64 `json:"daily_gross_sold"`
	DaysLeft       *int    `json:"days_left,omitempty"`
}

type velocityResponse struct {
	Items  []velocityRow `json:"items"`
	Total  int           `json:"total"`
	Days   int           `json:"days"`
	Capped bool          `json:"capped"`
}

// parseVelocityParams validates the window + sort. days default 30 (invalid →
// error → 400). Unknown sort falls back to the default (tolerant, like the
// products list), never a 400.
func parseVelocityParams(r *http.Request) (days int, sortKey string, err error) {
	days = 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || !validVelocityDays[n] {
			return 0, "", errors.New("invalid days")
		}
		days = n
	}
	sortKey = defaultVelocitySort
	if v := r.URL.Query().Get("sort"); v != "" {
		if _, ok := velocitySort[v]; ok {
			sortKey = v
		}
	}
	return days, sortKey, nil
}

// query runs the velocity aggregate for the given window + sort. `days` is bound
// once and used both for the trailing interval (make_interval keeps it a clean
// integer — no text coercion, sidestepping the lib/pq mixed-type param gotcha)
// and as the daily-rate denominator.
func (h *StockVelocityHandler) query(ctx context.Context, days int, sortKey string) ([]velocityRow, error) {
	orderBy, ok := velocitySort[sortKey]
	if !ok {
		orderBy = velocitySort[defaultVelocitySort]
	}

	const sqlHead = `
WITH sales AS (
    SELECT oi.variant_id,
           SUM(oi.quantity)::int       AS gross_sold,
           SUM(oi.line_total)::numeric AS gross_sales
      FROM order_items oi
      JOIN orders o ON o.id = oi.order_id
     WHERE oi.parent_item_id IS NULL                                  -- top-level lines only (no bundle children)
       AND oi.variant_id IS NOT NULL                                  -- deleted variants (SET NULL) excluded
       AND o.status IN ('paid','processing','shipped','delivered')    -- real, stock-depleting sales
       AND o.created_at >= NOW() - make_interval(days => $1)
     GROUP BY oi.variant_id
)
SELECT pv.id, pv.product_id, p.name AS product_name, pv.sku,
       COALESCE(NULLIF(pv.name, ''), pv.sku) AS variation,
       pv.stock_qty,
       (pv.stock_qty > 0) AS in_stock,
       s.gross_sales,
       s.gross_sold,
       (s.gross_sold::numeric / $1) AS daily_gross_sold,
       FLOOR(pv.stock_qty / NULLIF(s.gross_sold::numeric / $1, 0))::int AS days_left
  FROM sales s
  JOIN product_variants pv ON pv.id = s.variant_id
  JOIN products p          ON p.id = pv.product_id
 ORDER BY `

	q := sqlHead + orderBy + " LIMIT $2"

	rows, err := h.db.QueryContext(ctx, q, days, velocityRowCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]velocityRow, 0)
	for rows.Next() {
		var row velocityRow
		var daysLeft sql.NullInt64
		if err := rows.Scan(
			&row.VariantID, &row.ProductID, &row.ProductName, &row.SKU,
			&row.Variation, &row.StockQty, &row.InStock,
			&row.GrossSales, &row.GrossSold, &row.DailyGrossSold, &daysLeft,
		); err != nil {
			return nil, err
		}
		if daysLeft.Valid {
			n := int(daysLeft.Int64)
			row.DaysLeft = &n
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (h *StockVelocityHandler) list(w http.ResponseWriter, r *http.Request) {
	days, sortKey, err := parseVelocityParams(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	rows, err := h.query(r.Context(), days, sortKey)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, velocityResponse{
		Items:  rows,
		Total:  len(rows),
		Days:   days,
		Capped: len(rows) == velocityRowCap,
	})
}

func (h *StockVelocityHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	days, sortKey, err := parseVelocityParams(r)
	if err != nil {
		respond.BadRequest(w, err.Error())
		return
	}
	rows, err := h.query(r.Context(), days, sortKey)
	if err != nil {
		respond.InternalError(w)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="stock-velocity-%dd-%s.csv"`, days, time.Now().Format("20060102-150405")))

	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{
		"product_name", "variation", "sku", "stock_qty", "in_stock",
		"gross_sales", "gross_sold", "daily_gross_sold", "days_left", "projected_stockout",
	})
	today := time.Now()
	for _, row := range rows {
		daysLeft, stockout := "", ""
		if row.DaysLeft != nil {
			daysLeft = strconv.Itoa(*row.DaysLeft)
			stockout = today.AddDate(0, 0, *row.DaysLeft).Format("2006-01-02")
		}
		_ = cw.Write([]string{
			velocityCSVSafe(row.ProductName),
			velocityCSVSafe(row.Variation),
			velocityCSVSafe(row.SKU),
			strconv.Itoa(row.StockQty),
			strconv.FormatBool(row.InStock),
			strconv.FormatFloat(row.GrossSales, 'f', 2, 64),
			strconv.Itoa(row.GrossSold),
			strconv.FormatFloat(row.DailyGrossSold, 'f', 3, 64),
			daysLeft,
			stockout,
		})
	}
}

// velocityCSVSafe neutralises CSV-injection by prefixing a single quote in front
// of cells starting with a formula trigger (mirrors safeCSVCell in package stock;
// kept local to avoid a cross-package refactor).
func velocityCSVSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
