package wcshim

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

func TestMapStatus(t *testing.T) {
	cases := []struct {
		in   string
		want orders.OrderStatus
	}{
		// The real bug: ShipAny sends space-separated Title Case. These must
		// map even though the old underscore-only switch never matched them.
		{"Collected By Courier", orders.StatusShipped},
		{"Collected_By_Courier", orders.StatusShipped}, // tolerate underscore drift
		{"collected by courier", orders.StatusShipped},
		{"In Transit", orders.StatusShipped},
		{"Out For Delivery", orders.StatusShipped},
		{"Order Delivered", orders.StatusDelivered},
		{"Order Completed", orders.StatusDelivered},
		{"Collected By Customer", orders.StatusDelivered},
		{"Delivered To Locker", orders.StatusDelivered},

		// Pre-pickup / noise / empty → no advance.
		{"Order Created", ""},
		{"Pickup Request Sent", ""},
		{"Abnormal", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := mapStatus(c.in); got != c.want {
			t.Errorf("mapStatus(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
