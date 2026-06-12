package orders

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// CombineMutationsRequest rolls several already-executed stock-out mutations
// (出貨單) into one accounting-only sales order. The goods already left
// inventory when those mutations executed, so the resulting order never touches
// stock (skipStockDeduction + stock_managed=false) and the source mutations are
// atomically locked to it so the same shipment can't be billed twice.
//
// Pricing/customer/shipping mirror AdminCreate: line prices are the variants'
// current prices and role-eligible promotions apply automatically.
type CombineMutationsRequest struct {
	// MutationIDs are the executed out-mutations to combine. Duplicates are
	// ignored.
	MutationIDs []string `json:"mutation_ids"`

	// Customer — either an existing CustomerID or guest CustomerInfo, same as
	// AdminCreate. The customer's role drives promotion + free-shipping rules.
	CustomerID   *string       `json:"customer_id,omitempty"`
	CustomerInfo *CustomerInfo `json:"customer_info,omitempty"`

	ShippingAddressID *string               `json:"shipping_address_id,omitempty"`
	ShippingAddress   *ShippingAddressInput `json:"shipping_address,omitempty"`
	SaveAddress       bool                  `json:"save_address,omitempty"`

	CouponCode    *string     `json:"coupon_code,omitempty"`
	Notes         *string     `json:"notes,omitempty"`
	InitialStatus OrderStatus `json:"initial_status,omitempty"`
}

var (
	ErrNoMutationsSelected = errors.New("at least one mutation must be selected")
	// ErrMutationNotCombinable is returned when the in-transaction lock claims
	// fewer mutations than requested — one was consumed or made ineligible by a
	// concurrent operation after validation. Surfaced as 409.
	ErrMutationNotCombinable = errors.New("one or more selected mutations are no longer available to combine")
)

// MutationCombineProblem names a single mutation that failed the eligibility
// check, with a machine-readable reason for the UI.
type MutationCombineProblem struct {
	MutationID string `json:"mutation_id"`
	Reason     string `json:"reason"` // not_found | not_out | not_executed | already_consumed
}

// MutationsNotCombinableError lists the selected mutations that can't be
// combined. The handler renders it as 422 with the problems array so the UI can
// point at the offending rows.
type MutationsNotCombinableError struct {
	Problems []MutationCombineProblem `json:"problems"`
}

func (e *MutationsNotCombinableError) Error() string {
	return fmt.Sprintf("%d selected mutation(s) cannot be combined", len(e.Problems))
}

// CreateOrderFromMutations validates the selected out-mutations, flattens their
// leaf items (skipping bundle parent rows), aggregates per variant, and builds
// an accounting-only order via the shared admin-create builder. Stock is never
// touched; the source mutations are locked to the new order inside the create
// transaction.
func (s *OrderService) CreateOrderFromMutations(ctx context.Context, r CombineMutationsRequest) (*Order, error) {
	ids := dedupeStrings(r.MutationIDs)
	if len(ids) == 0 {
		return nil, ErrNoMutationsSelected
	}

	// --- Validate eligibility (pre-tx, fast-fail with per-id reasons) ---------
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, status, consumed_by_order_id
		   FROM stock_mutations WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	type mrow struct {
		typ        string
		status     string
		consumedBy *string
	}
	found := make(map[string]mrow, len(ids))
	for rows.Next() {
		var id, typ, status string
		var consumedBy *string
		if err := rows.Scan(&id, &typ, &status, &consumedBy); err != nil {
			rows.Close()
			return nil, err
		}
		found[id] = mrow{typ: typ, status: status, consumedBy: consumedBy}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var problems []MutationCombineProblem
	for _, id := range ids {
		m, ok := found[id]
		switch {
		case !ok:
			problems = append(problems, MutationCombineProblem{id, "not_found"})
		case m.typ != "out":
			problems = append(problems, MutationCombineProblem{id, "not_out"})
		case m.status != "executed":
			problems = append(problems, MutationCombineProblem{id, "not_executed"})
		case m.consumedBy != nil:
			problems = append(problems, MutationCombineProblem{id, "already_consumed"})
		}
	}
	if len(problems) > 0 {
		return nil, &MutationsNotCombinableError{Problems: problems}
	}

	// --- Flatten + aggregate leaf items ---------------------------------------
	// Bundle parent rows (parent_item_id IS NULL AND kind='bundle') are display-
	// only and carry no stock; the real movement lives on the leaves (standalone
	// simple rows + component rows) — same rule as Execute() and the list aggregate.
	itemRows, err := s.db.QueryContext(ctx,
		`SELECT si.variant_id, SUM(si.quantity)::int AS qty
		   FROM stock_mutation_items si
		   LEFT JOIN product_variants pv ON pv.id = si.variant_id
		   LEFT JOIN products         p  ON p.id  = pv.product_id
		  WHERE si.mutation_id = ANY($1)
		    AND NOT (si.parent_item_id IS NULL AND p.kind = 'bundle')
		  GROUP BY si.variant_id
		  ORDER BY MIN(si.position)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	var items []AdminCreateItem
	for itemRows.Next() {
		var variantID string
		var qty int
		if err := itemRows.Scan(&variantID, &qty); err != nil {
			itemRows.Close()
			return nil, err
		}
		if qty <= 0 {
			continue
		}
		items = append(items, AdminCreateItem{VariantID: variantID, Quantity: qty})
	}
	if err := itemRows.Close(); err != nil {
		return nil, err
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrAdminCreateNoItems
	}

	// --- Build via shared admin-create builder (no stock, locked sources) -----
	req := AdminCreateRequest{
		CustomerID:        r.CustomerID,
		CustomerInfo:      r.CustomerInfo,
		Items:             items,
		ShippingAddressID: r.ShippingAddressID,
		ShippingAddress:   r.ShippingAddress,
		SaveAddress:       r.SaveAddress,
		CouponCode:        r.CouponCode,
		Notes:             r.Notes,
		InitialStatus:     r.InitialStatus,
	}
	return s.adminCreateOrder(ctx, req, adminCreateOptions{
		skipStockDeduction: true,
		stockManaged:       false,
		sourceMutationIDs:  ids,
	})
}

// dedupeStrings returns the input with empty + duplicate values removed,
// preserving first-seen order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
