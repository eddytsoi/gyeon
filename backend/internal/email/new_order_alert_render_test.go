package email

import (
	"context"
	"strings"
	"testing"
)

// TestNewOrderAlertCompiledDefaultRenders verifies the compiled-in default for
// new_order_alert renders end-to-end against the sample params WITHOUT a DB
// template store (the production path until an admin customizes it). This guards
// the BaseURL-global gotcha: the default must reference only struct fields, so a
// nil-store render must produce non-empty subject/text/html.
func TestNewOrderAlertCompiledDefaultRenders(t *testing.T) {
	s := &Service{} // tmplStore nil -> applyTemplate returns the compiled default
	p, ok := SampleParamsFor("new_order_alert").(OrderEmailParams)
	if !ok {
		t.Fatalf("sample params for new_order_alert: got %T", SampleParamsFor("new_order_alert"))
	}

	subject, text, html, err := s.RenderTemplate(context.Background(), "new_order_alert", p)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(text) == "" || strings.TrimSpace(html) == "" {
		t.Fatalf("empty render: subject=%q textLen=%d htmlLen=%d", subject, len(text), len(html))
	}

	// Subject carries the order number.
	if !strings.Contains(subject, "ORD-0001") {
		t.Errorf("subject missing order number: %q", subject)
	}
	// The admin deep-link must render in both bodies (the field, not the global).
	if !strings.Contains(html, p.AdminOrderURL) {
		t.Errorf("html missing admin order URL %q", p.AdminOrderURL)
	}
	if !strings.Contains(text, p.AdminOrderURL) {
		t.Errorf("text missing admin order URL %q", p.AdminOrderURL)
	}
	// No unresolved global leaked through as a literal field error.
	if strings.Contains(html, "<no value>") || strings.Contains(html, "{{") {
		t.Errorf("html has unresolved template tokens")
	}
	// Customer contact + a line item must be present.
	if !strings.Contains(html, p.CustomerEmail) {
		t.Errorf("html missing customer email")
	}
	if !strings.Contains(text, "Sample Product") {
		t.Errorf("text missing line item")
	}
}
