package shop

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"gyeon/backend/internal/auth"
)

// InventoryHistoryRow is the API/DB shape for one stock-change record.
type InventoryHistoryRow struct {
	ID         string  `json:"id"`
	VariantID  string  `json:"variant_id"`
	Delta      int     `json:"delta"`
	BeforeQty  int     `json:"before_qty"`
	AfterQty   int     `json:"after_qty"`
	Reason     string  `json:"reason"`
	ActorID    *string `json:"actor_user_id,omitempty"`
	ActorEmail *string `json:"actor_email,omitempty"`
	OrderID    *string `json:"order_id,omitempty"`
	Note       *string `json:"note,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// StockMovementRow is the wider row returned by the cross-product admin list;
// includes product/variant/order display fields so the UI can render a single
// flat table without per-row lookups.
type StockMovementRow struct {
	InventoryHistoryRow
	ProductID   *string `json:"product_id,omitempty"`
	ProductName *string `json:"product_name,omitempty"`
	VariantSKU  *string `json:"variant_sku,omitempty"`
	OrderNumber *string `json:"order_number,omitempty"`
}

// StockMovementFilters narrows the cross-product stock-history list.
type StockMovementFilters struct {
	From         string // RFC3339 / Postgres-parseable timestamp
	To           string
	Reason       string // exact match (e.g. "admin.adjust")
	SourcePrefix string // "admin" or "order" → reason LIKE prefix||'.%'
	ProductID    string
	VariantID    string
	Search       string // ILIKE on product name OR variant SKU
	ActorUserID  string
	Limit        int
	Offset       int
}

// StockMovementList wraps the paginated response.
type StockMovementList struct {
	Items []StockMovementRow `json:"items"`
	Total int                `json:"total"`
}

// recordStockChange writes one inventory_history row inside the caller's
// transaction (or directly via *sql.DB). Caller must compute beforeQty and
// afterQty so the row remains useful even after subsequent edits. delta == 0
// is a no-op (avoids spam from variant edits that don't touch stock).
//
// actorID is read from context (set by auth.AdminMiddleware). NULL when the
// change is customer-driven (checkout deduction).
func recordStockChange(ctx context.Context, exec sqlExecer, variantID string, beforeQty, afterQty int,
	reason string, orderID *string, note *string) {
	delta := afterQty - beforeQty
	if delta == 0 {
		return
	}
	var actorIDArg any
	if id, ok := auth.AdminIDFromContext(ctx); ok {
		actorIDArg = id
	}
	var orderIDArg any
	if orderID != nil && *orderID != "" {
		orderIDArg = *orderID
	}
	var noteArg any
	if note != nil && *note != "" {
		noteArg = *note
	}
	if _, err := exec.ExecContext(ctx,
		`INSERT INTO inventory_history (variant_id, delta, before_qty, after_qty, reason, actor_user_id, order_id, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		variantID, delta, beforeQty, afterQty, reason, actorIDArg, orderIDArg, noteArg,
	); err != nil {
		log.Printf("inventory_history: insert variant=%s reason=%s: %v", variantID, reason, err)
	}
}

// sqlExecer is the minimal interface satisfied by both *sql.DB and *sql.Tx.
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ListVariantHistory returns the most recent stock changes for a variant.
func (s *ProductService) ListVariantHistory(ctx context.Context, variantID string, limit int) ([]InventoryHistoryRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.variant_id, h.delta, h.before_qty, h.after_qty, h.reason,
		        h.actor_user_id, u.email, h.order_id, h.note, h.created_at
		   FROM inventory_history h
		   LEFT JOIN admin_users u ON u.id = h.actor_user_id
		  WHERE h.variant_id = $1
		  ORDER BY h.created_at DESC
		  LIMIT $2`, variantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]InventoryHistoryRow, 0)
	for rows.Next() {
		var r InventoryHistoryRow
		if err := rows.Scan(&r.ID, &r.VariantID, &r.Delta, &r.BeforeQty, &r.AfterQty, &r.Reason,
			&r.ActorID, &r.ActorEmail, &r.OrderID, &r.Note, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListInventoryHistory returns paginated stock movements across all products.
// All filter fields are optional; an empty filters struct returns the most
// recent movements site-wide. limit/offset default to 50/0 if non-positive.
func (s *ProductService) ListInventoryHistory(ctx context.Context, f StockMovementFilters) (StockMovementList, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var (
		where []string
		args  []any
	)
	add := func(clause string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.From != "" {
		add("h.created_at >= $%d", f.From)
	}
	if f.To != "" {
		add("h.created_at <= $%d", f.To)
	}
	if f.Reason != "" {
		add("h.reason = $%d", f.Reason)
	}
	if f.SourcePrefix != "" {
		add("h.reason LIKE $%d", f.SourcePrefix+".%")
	}
	if f.ProductID != "" {
		add("v.product_id = $%d", f.ProductID)
	}
	if f.VariantID != "" {
		add("h.variant_id = $%d", f.VariantID)
	}
	if f.ActorUserID != "" {
		add("h.actor_user_id = $%d", f.ActorUserID)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where = append(where, fmt.Sprintf("(p.name ILIKE $%d OR v.sku ILIKE $%d)", len(args), len(args)))
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	out := StockMovementList{Items: make([]StockMovementRow, 0)}

	countSQL := `
		SELECT COUNT(*)
		  FROM inventory_history h
		  LEFT JOIN product_variants v ON v.id = h.variant_id
		  LEFT JOIN products         p ON p.id = v.product_id
		` + whereSQL
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&out.Total); err != nil {
		return out, err
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, limit, offset)
	listSQL := `
		SELECT h.id, h.variant_id, h.delta, h.before_qty, h.after_qty, h.reason,
		       h.actor_user_id, u.email, h.order_id, h.note, h.created_at,
		       v.product_id, p.name, v.sku, o.order_number
		  FROM inventory_history h
		  LEFT JOIN admin_users     u ON u.id = h.actor_user_id
		  LEFT JOIN product_variants v ON v.id = h.variant_id
		  LEFT JOIN products         p ON p.id = v.product_id
		  LEFT JOIN orders           o ON o.id = h.order_id
		` + whereSQL + fmt.Sprintf(`
		 ORDER BY h.created_at DESC
		 LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var r StockMovementRow
		var orderNumber sql.NullString
		if err := rows.Scan(&r.ID, &r.VariantID, &r.Delta, &r.BeforeQty, &r.AfterQty, &r.Reason,
			&r.ActorID, &r.ActorEmail, &r.OrderID, &r.Note, &r.CreatedAt,
			&r.ProductID, &r.ProductName, &r.VariantSKU, &orderNumber); err != nil {
			return out, err
		}
		if orderNumber.Valid && orderNumber.String != "" {
			s := orderNumber.String
			r.OrderNumber = &s
		}
		out.Items = append(out.Items, r)
	}
	return out, rows.Err()
}
