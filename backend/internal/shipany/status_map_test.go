package shipany

import (
	"testing"

	"gyeon/backend/internal/orders"
)

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"Collected By Courier":      "collected by courier",
		"Collected_By_Courier":      "collected by courier",
		"  collected   by_courier ": "collected by courier",
		"Order Delivered":           "order delivered",
		"Order_Delivered":           "order delivered",
		"ORDER_COMPLETED":           "order completed",
		"":                          "",
		"   ":                       "",
	}
	for in, want := range cases {
		if got := normalizeStatus(in); got != want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapOrderState(t *testing.T) {
	cases := []struct {
		in   string
		want orders.OrderStatus
	}{
		// The real ShipAny push vocabulary lives in the order-state meta as
		// underscore_case — these must map.
		{"Order_Delivered", orders.StatusDelivered},
		{"Collected_By_Courier", orders.StatusShipped},
		// FetchOrder (pull) returns the spaced Title Case form — same mapping.
		{"Collected By Courier", orders.StatusShipped},
		{"Order Delivered", orders.StatusDelivered},
		{"In Transit", orders.StatusShipped},
		{"Out For Delivery", orders.StatusShipped},
		{"Ready For Shipment", orders.StatusShipped},
		{"Order Completed", orders.StatusDelivered},
		{"Collected By Customer", orders.StatusDelivered},
		{"Delivered To Locker", orders.StatusDelivered},
		// WooCommerce status word fallback (top-level `status` when no meta).
		{"completed", orders.StatusDelivered},

		// Pre-pickup / noise / empty → no advance.
		{"Order Created", ""},
		{"Pickup Request Sent", ""},
		{"Abnormal", ""},
		{"processing", ""}, // ambiguous WC word — must NOT advance
		{"", ""},
	}
	for _, c := range cases {
		if got := MapOrderState(c.in); got != c.want {
			t.Errorf("MapOrderState(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
