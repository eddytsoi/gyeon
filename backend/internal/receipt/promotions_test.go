package receipt

import (
	"bytes"
	"strings"
	"testing"

	"gyeon/backend/internal/orders"
)

// TestReceiptRendersPromotions verifies the receipt template names the
// campaigns / coupons behind the discount (with optional description), not just
// the discount total — and renders nothing extra when there are none.
func TestReceiptRendersPromotions(t *testing.T) {
	render := func(vm viewModel) string {
		var buf bytes.Buffer
		if err := receiptTemplate.Execute(&buf, vm); err != nil {
			t.Fatalf("execute receipt template: %v", err)
		}
		return buf.String()
	}

	withPromos := render(viewModel{
		Locale:      "zh-Hant",
		L:           labels["zh-Hant"],
		Order:       &orders.Order{OrderNumber: "ORD-0001"},
		HasDiscount: true,
		DiscountFmt: "HK$50.00",
		Promotions: []viewPromotion{
			{Name: "夏季優惠", Description: "全單滿 $300 即減 $50"},
			{Name: "WELCOME10"},
		},
	})
	for _, want := range []string{"夏季優惠", "全單滿 $300 即減 $50", "WELCOME10"} {
		if !strings.Contains(withPromos, want) {
			t.Errorf("receipt missing %q", want)
		}
	}

	none := render(viewModel{Locale: "zh-Hant", L: labels["zh-Hant"], Order: &orders.Order{OrderNumber: "ORD-0001"}})
	if strings.Contains(none, "夏季優惠") {
		t.Errorf("receipt should render no promotion lines when Promotions is empty")
	}
}
