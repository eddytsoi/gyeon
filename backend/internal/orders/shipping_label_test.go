package orders

import "testing"

func TestShippingLabel(t *testing.T) {
	cases := []struct {
		name     string
		free     bool
		locale   string
		expected string
	}{
		{"zh free", true, "zh-Hant", "順豐速運（免運費）"},
		{"zh cod", false, "zh-Hant", "順豐速運（到付）"},
		{"en free", true, "en", "SF Express (free)"},
		{"en cod", false, "en", "SF Express (pay on delivery)"},
		{"unknown locale falls back to en", true, "fr", "SF Express (free)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ShippingLabel(&Order{ShippingFree: c.free}, c.locale)
			if got != c.expected {
				t.Errorf("ShippingLabel(free=%v, %q) = %q; want %q", c.free, c.locale, got, c.expected)
			}
		})
	}
}
