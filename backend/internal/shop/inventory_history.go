package shop

import (
	"context"
	"database/sql"
	"log"

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
