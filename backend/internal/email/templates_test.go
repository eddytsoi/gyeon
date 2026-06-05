package email

import (
	"slices"
	"strings"
	"testing"
)

// TestDefaultTemplatesRender verifies that every compiled-in default template
// (subject, HTML, text) parses and executes cleanly against SampleParamsFor.
// Catches regressions like unbalanced {{...}} braces, unknown funcmap keys,
// or field references that don't exist on the params struct.
func TestDefaultTemplatesRender(t *testing.T) {
	for _, key := range AllKeys() {
		key := key
		t.Run(key, func(t *testing.T) {
			def := defaultsFor(key)
			params := SampleParamsFor(key)

			subj, ok := executeTemplate("test-subj:"+key, def.subject, params)
			if !ok {
				t.Fatalf("subject failed to render")
			}
			if strings.Contains(subj, "{{") {
				t.Errorf("subject still contains {{ — directive not resolved: %q", subj)
			}

			html, ok := executeTemplate("test-html:"+key, def.html, params)
			if !ok {
				t.Fatalf("html failed to render")
			}
			if strings.Contains(html, "{{") {
				t.Errorf("html still contains {{ — directive not resolved")
			}

			text, ok := executeTemplate("test-text:"+key, def.text, params)
			if !ok {
				t.Fatalf("text failed to render")
			}
			if strings.Contains(text, "{{") {
				t.Errorf("text still contains {{ — directive not resolved")
			}

			if subj == "" || html == "" || text == "" {
				t.Errorf("empty output: subj=%d html=%d text=%d bytes", len(subj), len(html), len(text))
			}
		})
	}
}

// TestOrderConfirmationFields spot-checks that fields from OrderEmailParams
// appear in the rendered output, including looped Items and conditional rows.
func TestOrderConfirmationFields(t *testing.T) {
	def := defaultsFor("order_confirmation")
	p := OrderEmailParams{
		OrderID:        "abcd1234-5678",
		OrderNumber:    "ORD-9999",
		CustomerName:   "Alice <Test>",
		CustomerEmail:  "alice@example.com",
		Currency:       "HKD",
		Subtotal:       200,
		ShippingFee:    30,
		DiscountAmount: 20,
		AppliedPromotions: []EmailPromotion{
			{Name: "Summer Sale", Description: "10% off"},
			{Name: "WELCOME10"},
		},
		TaxAmount: 18,
		TaxLabel:  "GST",
		Total:     228,
		Items: []OrderEmailItem{
			{Name: "Widget <A>", Quantity: 2, UnitPrice: 50, LineTotal: 100},
			{Name: "Gadget B", Quantity: 1, UnitPrice: 100, LineTotal: 100},
		},
		ShippingLine1:   "1 Test Street",
		ShippingCity:    "HK",
		ShippingPostal:  "0000",
		ShippingCountry: "Hong Kong",
	}

	html, _ := executeTemplate("html", def.html, p)
	text, _ := executeTemplate("text", def.text, p)

	for _, want := range []string{"ORD-9999", "Widget &lt;A&gt;", "× 2", "Gadget B", "HKD 228", "GST", "HKD 18", "-HKD 20", "1 Test Street", "Summer Sale", "10% off", "WELCOME10"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(html, "Alice <Test>") {
		t.Errorf("html should HTML-escape CustomerName, found unescaped")
	}
	if !strings.Contains(html, "Alice &lt;Test&gt;") {
		t.Errorf("html should contain escaped CustomerName")
	}
	for _, want := range []string{"ORD-9999", "Widget <A>", "× 2", "Gadget B", "HKD 228", "GST", "HKD 18", "Summer Sale", "10% off", "WELCOME10"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q", want)
		}
	}
}

// TestOrderConfirmationBundleChildren verifies that bundle component rows
// are rendered indented under their bundle parent in both HTML and text.
func TestOrderConfirmationBundleChildren(t *testing.T) {
	def := defaultsFor("order_confirmation")
	p := OrderEmailParams{
		OrderNumber:  "ORD-0001",
		CustomerName: "Bundle Buyer",
		Currency:     "HKD",
		Subtotal:     289,
		Total:        289,
		Items: []OrderEmailItem{
			{Name: "內籠基本清潔套裝", Quantity: 1, UnitPrice: 289, LineTotal: 289, Children: []OrderEmailItem{
				{Name: "Q²M INTERIORDETAILER", Quantity: 1},
				{Name: "Q²M SCRUBPAD EVO", Quantity: 2},
			}},
		},
	}

	html, _ := executeTemplate("html", def.html, p)
	text, _ := executeTemplate("text", def.text, p)

	for _, want := range []string{"內籠基本清潔套裝", "↳ Q²M INTERIORDETAILER", "↳ Q²M SCRUBPAD EVO", "× 2"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q", want)
		}
	}
}

// TestOrderConfirmationConditionals verifies that optional rows (discount,
// tax, shipping address, setup URL) appear only when their fields are set.
func TestOrderConfirmationConditionals(t *testing.T) {
	def := defaultsFor("order_confirmation")
	minimal := OrderEmailParams{
		OrderNumber:  "ORD-0001",
		CustomerName: "Bob",
		Currency:     "HKD",
		Subtotal:     100,
		ShippingFee:  0,
		Total:        100,
		Items:        []OrderEmailItem{{Name: "X", Quantity: 1, UnitPrice: 100, LineTotal: 100}},
	}
	html, _ := executeTemplate("html", def.html, minimal)

	for _, gone := range []string{"折扣", "送貨地址", "完成註冊以追蹤訂單", "稅金", "GST"} {
		if strings.Contains(html, gone) {
			t.Errorf("html should not contain %q when corresponding field is zero/empty", gone)
		}
	}
}

// TestOrderRefundedIsFullRefundBranch verifies the IsFullRefund if/else.
func TestOrderRefundedIsFullRefundBranch(t *testing.T) {
	def := defaultsFor("order_refunded")
	full := RefundEmailParams{
		OrderNumber: "ORD-0001", CustomerName: "X", Currency: "HKD",
		RefundAmount: 100, OrderTotal: 100, IsFullRefund: true,
	}
	partial := full
	partial.IsFullRefund = false
	partial.RefundAmount = 30

	fullHTML, _ := executeTemplate("h", def.html, full)
	partialHTML, _ := executeTemplate("h", def.html, partial)

	if !strings.Contains(fullHTML, "已全額退款") {
		t.Errorf("full refund html should contain 已全額退款")
	}
	if strings.Contains(fullHTML, "訂單總額") {
		t.Errorf("full refund html should NOT contain 訂單總額 row")
	}
	if !strings.Contains(partialHTML, "已部分退款") {
		t.Errorf("partial refund html should contain 已部分退款")
	}
	if !strings.Contains(partialHTML, "訂單總額") {
		t.Errorf("partial refund html should contain 訂單總額 row")
	}
}

// TestEscFuncMap verifies the esc helper is registered and accessible from
// admin-edited templates (DB override path).
func TestEscFuncMap(t *testing.T) {
	body := `Hi {{.Name | esc}}!`
	out, ok := executeTemplate("t", body, struct{ Name string }{Name: "<script>"})
	if !ok {
		t.Fatalf("template failed to execute")
	}
	want := "Hi &lt;script&gt;!"
	if out != want {
		t.Errorf("esc output mismatch: got %q want %q", out, want)
	}
}

// TestMergeTemplateGlobals verifies the params struct is flattened into a map
// (preserving concrete field types like []OrderEmailItem) and the site-wide
// globals are overlaid.
func TestMergeTemplateGlobals(t *testing.T) {
	p := OrderEmailParams{
		OrderNumber: "ORD-1",
		Currency:    "HKD",
		Items:       []OrderEmailItem{{Name: "X", Quantity: 1}},
	}
	merged := mergeTemplateGlobals(p, map[string]string{
		"BaseURL": "https://shop.test", "SiteName": "GYEON", "ContactEmail": "hi@shop.test",
	})
	m, ok := merged.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", merged)
	}
	if m["OrderNumber"] != "ORD-1" {
		t.Errorf("OrderNumber not carried: %v", m["OrderNumber"])
	}
	if m["BaseURL"] != "https://shop.test" || m["SiteName"] != "GYEON" || m["ContactEmail"] != "hi@shop.test" {
		t.Errorf("globals not overlaid: %v / %v / %v", m["BaseURL"], m["SiteName"], m["ContactEmail"])
	}
	items, ok := m["Items"].([]OrderEmailItem)
	if !ok || len(items) != 1 || items[0].Name != "X" {
		t.Errorf("Items slice lost its concrete type: %T", m["Items"])
	}
}

// TestTemplateGlobalsRender verifies an admin override can reference the three
// site-wide globals AND that existing params fields, looped {{range .Items}},
// and {{$.Currency}} root-access still resolve through the merged map.
func TestTemplateGlobalsRender(t *testing.T) {
	p := OrderEmailParams{
		OrderNumber: "ORD-1",
		Currency:    "HKD",
		Items:       []OrderEmailItem{{Name: "Widget", Quantity: 2}},
	}
	data := mergeTemplateGlobals(p, map[string]string{
		"BaseURL": "https://shop.test", "SiteName": "GYEON", "ContactEmail": "hi@shop.test",
	})
	body := `{{.OrderNumber}} | {{.BaseURL}} | {{.SiteName}} | {{.ContactEmail}} | {{range .Items}}{{$.Currency}} {{.Name}}{{end}}`
	out, ok := executeTemplate("globals", body, data)
	if !ok {
		t.Fatalf("render failed")
	}
	want := "ORD-1 | https://shop.test | GYEON | hi@shop.test | HKD Widget"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

// TestExecuteTemplateMissingKeyFails verifies that a typo'd reference against
// the merged map still fails the render (ok=false) so applyTemplate falls back
// to the compiled-in default instead of emitting "<no value>".
func TestExecuteTemplateMissingKeyFails(t *testing.T) {
	data := mergeTemplateGlobals(OrderEmailParams{OrderNumber: "ORD-1"},
		map[string]string{"BaseURL": "x", "SiteName": "y", "ContactEmail": "z"})
	if _, ok := executeTemplate("typo", `{{.Ordernumbr}}`, data); ok {
		t.Errorf("expected missing map key to fail render so caller falls back to default")
	}
}

// TestVariablesForIncludesGlobals verifies every template key surfaces the
// three site-wide globals as variable chips, and unknown keys stay nil.
func TestVariablesForIncludesGlobals(t *testing.T) {
	for _, key := range AllKeys() {
		vars := VariablesFor(key)
		for _, g := range []string{".BaseURL", ".SiteName", ".ContactEmail"} {
			if !slices.Contains(vars, g) {
				t.Errorf("VariablesFor(%q) missing global %q", key, g)
			}
		}
	}
	if VariablesFor("does_not_exist") != nil {
		t.Errorf("unknown key should return nil, not globals")
	}
}

// TestOrderrefFuncMap verifies the orderref helper.
func TestOrderrefFuncMap(t *testing.T) {
	cases := []struct {
		num, id, want string
	}{
		{"ORD-0001", "abcd1234efgh", "ORD-0001"},
		{"", "abcd1234efgh", "abcd1234"},
		{"", "short", "short"},
	}
	for _, c := range cases {
		body := `{{orderref .Num .ID}}`
		out, _ := executeTemplate("t", body, struct{ Num, ID string }{c.num, c.id})
		if out != c.want {
			t.Errorf("orderref(%q,%q): got %q want %q", c.num, c.id, out, c.want)
		}
	}
}
