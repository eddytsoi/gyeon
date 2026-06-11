package shipany

import (
	"context"
	"log"
	"strings"
	"unicode"

	"gyeon/backend/internal/orders"
)

// normalizeStatus canonicalises a ShipAny state string so matching is robust to
// formatting drift. ShipAny exposes the same vocabulary in two shapes:
//   - the `_pr_shipment_shipany_order_state` order meta (push, via the wcshim
//     webhook) is underscore_case — "Order_Delivered", "Collected_By_Courier";
//   - `cur_stat` from the orders API (pull, via FetchOrder) is spaced Title
//     Case — "Order Delivered", "Collected By Courier".
//
// We lowercase and collapse every run of underscores/whitespace to a single
// space so both forms (and odd-cased variants) map identically.
func normalizeStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	pendingSep := false
	for _, r := range s {
		if r == '_' || unicode.IsSpace(r) {
			pendingSep = b.Len() > 0
			continue
		}
		if pendingSep {
			b.WriteByte(' ')
			pendingSep = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// MapOrderState translates a ShipAny shipment state into Gyeon's internal order
// status enum. Returns "" (no advance) for pre-pickup and unrecognised states.
//
// This is the single source of truth shared by both the push path (the wcshim
// webhook, reading `_pr_shipment_shipany_order_state`) and the pull path
// (SyncOrderStatus, reading `cur_stat`) so the two can never drift. The
// vocabulary mirrors the plugin's ShipanyHelper::get_all_order_status().
//
// Gyeon's transition graph is paid→processing→shipped→delivered. paid→processing
// is handled at shipment-create time, so here we only map the in-transit
// milestones (→ shipped) and the terminal ones (→ delivered). "completed" is the
// WooCommerce status word ShipAny also sends in the callback's top-level `status`
// field; it's kept as a fallback for when the order-state meta is absent.
func MapOrderState(state string) orders.OrderStatus {
	switch normalizeStatus(state) {
	case "collected by courier", "collected by courier overdue",
		"in transit", "shipping",
		"arrived transit point", "departed transit point",
		"ready for shipment", "ready for delivery",
		"delivery in progress",
		"out for delivery", "delivering to convenience store":
		return orders.StatusShipped
	case "order delivered", "order completed", "completed",
		"collected by customer",
		"delivered to locker", "delivered to service point":
		return orders.StatusDelivered
	}
	return ""
}

// shipMilestoneLadder is the ordered set of shipping milestones MapOrderState can
// target. AdvanceOrderTo walks it so a callback that skips an earlier milestone
// (e.g. a "delivered" event arriving while the order is still "prepared" because
// the "shipped" event was missed) still lands instead of being dropped.
var shipMilestoneLadder = []orders.OrderStatus{orders.StatusShipped, orders.StatusDelivered}

// AdvanceOrderTo advances an order through the shipping milestones up to and
// including target. Each step is best-effort: an invalid forward transition
// (the order is already past that step, or not yet eligible for it) is logged
// and skipped — it never moves the order backwards and never errors out the
// caller. rawState is the original ShipAny state string, recorded on the
// order-timeline note for ops visibility.
func AdvanceOrderTo(ctx context.Context, svc *orders.OrderService, orderID, rawState string, target orders.OrderStatus) {
	if target == "" {
		return
	}
	for _, step := range shipMilestoneLadder {
		note := "ShipAny: " + rawState + " (→ " + string(step) + ")"
		if _, err := svc.UpdateStatus(ctx, orderID,
			orders.UpdateStatusRequest{Status: step, Note: &note}); err != nil {
			// Most often a benign "cannot transition" (already past this step, or
			// the order isn't eligible yet). The shipment row stays the source of
			// truth for ops either way.
			log.Printf("shipany: advance order %s to %s: %v", orderID, step, err)
		}
		if step == target {
			break
		}
	}
}
