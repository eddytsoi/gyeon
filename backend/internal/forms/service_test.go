package forms

import (
	"testing"
)

func TestValidatePayload(t *testing.T) {
	fields := []FormField{
		{Type: FieldText, Name: "your-name", Required: true, MaxLength: 50},
		{Type: FieldEmail, Name: "your-email", Required: true},
		{Type: FieldSelect, Name: "country", Required: true, Options: []FieldOption{{Label: "HK", Value: "hk"}, {Label: "JP", Value: "jp"}}},
		{Type: FieldCheckbox, Name: "interests", Options: []FieldOption{{Label: "News", Value: "News"}, {Label: "Promo", Value: "Promo"}}},
		{Type: FieldSubmit, Label: "Send"},
	}

	cases := []struct {
		name     string
		data     map[string]string
		wantErrs []string
	}{
		{
			name: "happy path",
			data: map[string]string{
				"your-name": "Alice", "your-email": "a@b.com", "country": "hk", "interests": "News,Promo",
			},
		},
		{
			name:     "missing required",
			data:     map[string]string{"your-email": "a@b.com", "country": "hk"},
			wantErrs: []string{"your-name"},
		},
		{
			name:     "bad email",
			data:     map[string]string{"your-name": "A", "your-email": "not-an-email", "country": "hk"},
			wantErrs: []string{"your-email"},
		},
		{
			name:     "country not in options",
			data:     map[string]string{"your-name": "A", "your-email": "a@b.com", "country": "kr"},
			wantErrs: []string{"country"},
		},
		{
			name:     "checkbox value not in options",
			data:     map[string]string{"your-name": "A", "your-email": "a@b.com", "country": "hk", "interests": "Spam"},
			wantErrs: []string{"interests"},
		},
		{
			name: "unknown field rejected",
			data: map[string]string{
				"your-name": "A", "your-email": "a@b.com", "country": "hk", "evil": "1",
			},
			wantErrs: []string{"evil"},
		},
		{
			name: "maxlength enforced",
			data: map[string]string{
				"your-name":  "x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x x",
				"your-email": "a@b.com", "country": "hk",
			},
			wantErrs: []string{"your-name"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePayload(fields, tc.data)
			if len(tc.wantErrs) == 0 {
				if got != nil {
					t.Fatalf("unexpected errors: %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected errors on %v, got nil", tc.wantErrs)
			}
			for _, k := range tc.wantErrs {
				if _, ok := got[k]; !ok {
					t.Errorf("missing error for field %q; got %v", k, got)
				}
			}
		})
	}
}

func TestSubstitutePlaceholders(t *testing.T) {
	out := substitutePlaceholders("[your-email]", map[string]string{"your-email": "a@b.com"})
	if out != "a@b.com" {
		t.Errorf("got %q want a@b.com", out)
	}
	// Unknown placeholder is left as-is.
	out = substitutePlaceholders("[missing]", map[string]string{})
	if out != "[missing]" {
		t.Errorf("got %q want [missing]", out)
	}
}
