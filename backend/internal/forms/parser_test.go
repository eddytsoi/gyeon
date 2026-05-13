package forms

import (
	"reflect"
	"testing"
)

func TestParseForm_EveryFieldType(t *testing.T) {
	markup := `
[text* your-name placeholder "Your name"]
[email* your-email]
[tel your-phone]
[textarea your-message placeholder "How can we help?"]
[select* country first_as_label "Hong Kong|hk" "Japan|jp"]
[checkbox interests "Newsletter" "Promotions"]
[radio* contact-pref "Email" "Phone"]
[date your-birthday]
[hidden source "homepage"]
[submit "Send message"]
`
	fields, errs := ParseForm(markup)
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errors: %+v", errs)
	}
	if len(fields) != 10 {
		t.Fatalf("expected 10 fields, got %d: %+v", len(fields), fields)
	}

	want := []struct {
		Type     FieldType
		Name     string
		Required bool
	}{
		{FieldText, "your-name", true},
		{FieldEmail, "your-email", true},
		{FieldTel, "your-phone", false},
		{FieldTextarea, "your-message", false},
		{FieldSelect, "country", true},
		{FieldCheckbox, "interests", false},
		{FieldRadio, "contact-pref", true},
		{FieldDate, "your-birthday", false},
		{FieldHidden, "source", false},
		{FieldSubmit, "", false},
	}
	for i, w := range want {
		if fields[i].Type != w.Type {
			t.Errorf("field %d: type=%s want %s", i, fields[i].Type, w.Type)
		}
		if fields[i].Name != w.Name {
			t.Errorf("field %d: name=%q want %q", i, fields[i].Name, w.Name)
		}
		if fields[i].Required != w.Required {
			t.Errorf("field %d: required=%v want %v", i, fields[i].Required, w.Required)
		}
	}

	if fields[0].Placeholder != "Your name" {
		t.Errorf("text placeholder = %q", fields[0].Placeholder)
	}

	selectOpts := fields[4].Options
	if !reflect.DeepEqual(selectOpts, []FieldOption{
		{Label: "Hong Kong", Value: "hk"},
		{Label: "Japan", Value: "jp"},
	}) {
		t.Errorf("select options = %+v", selectOpts)
	}

	checkOpts := fields[5].Options
	if !reflect.DeepEqual(checkOpts, []FieldOption{
		{Label: "Newsletter", Value: "Newsletter"},
		{Label: "Promotions", Value: "Promotions"},
	}) {
		t.Errorf("checkbox options = %+v", checkOpts)
	}

	if fields[8].Default != "homepage" {
		t.Errorf("hidden default = %q", fields[8].Default)
	}
	if fields[9].Label != "Send message" {
		t.Errorf("submit label = %q", fields[9].Label)
	}
}

func TestParseForm_ImpliedLabelFromName(t *testing.T) {
	fields, _ := ParseForm(`[text* your-full-name]`)
	if len(fields) != 1 {
		t.Fatal("expected 1 field")
	}
	if fields[0].Label != "Your full name" {
		t.Errorf("label = %q want %q", fields[0].Label, "Your full name")
	}
}

func TestParseForm_KeyValueAttrs(t *testing.T) {
	fields, errs := ParseForm(`[text your-name id:fld-1 class:big maxlength:100 minlength:2 size:30]`)
	if len(errs) != 0 {
		t.Fatalf("errs: %+v", errs)
	}
	f := fields[0]
	if f.ID != "fld-1" || f.Class != "big" || f.MaxLength != 100 || f.MinLength != 2 || f.Size != 30 {
		t.Errorf("attrs not applied: %+v", f)
	}
}

func TestParseForm_DefaultValue(t *testing.T) {
	fields, _ := ParseForm(`[text greeting "Hello"]`)
	if fields[0].Default != "Hello" {
		t.Errorf("default = %q", fields[0].Default)
	}
}

func TestParseForm_Errors(t *testing.T) {
	cases := []struct {
		name   string
		markup string
		want   string
	}{
		{"unknown type", `[foo* bar]`, `unsupported field type "foo"`},
		{"missing name", `[text]`, "field name is required"},
		{"duplicate name", "[text a]\n[text a]", `duplicate field name "a"`},
		{"unterminated quote", `[text a placeholder "unterminated]`, "unterminated quoted string"},
		{"file unsupported (phase 1)", `[file* upload]`, `unsupported field type "file"`},
		{"invalid name", `[text 1bad]`, `invalid field name "1bad"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := ParseForm(tc.markup)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q", tc.want)
			}
			found := false
			for _, e := range errs {
				if contains(e.Message, tc.want) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no error matched %q; got: %+v", tc.want, errs)
			}
		})
	}
}

func TestParseForm_IgnoresTextBetweenTags(t *testing.T) {
	markup := `
Your name (required)
[text* your-name]

Your email
[email* your-email]
`
	fields, errs := ParseForm(markup)
	if len(errs) != 0 {
		t.Fatalf("errs: %+v", errs)
	}
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

func TestParseForm_EscapedBracketSkipped(t *testing.T) {
	// `\[text x]` should NOT be parsed as a tag.
	fields, _ := ParseForm(`\[text x]`)
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %+v", fields)
	}
}

func TestParseForm_RecordsPosition(t *testing.T) {
	_, errs := ParseForm("\n\n[unknownType x]")
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
	if errs[0].Position != 2 {
		t.Errorf("position = %d, want 2", errs[0].Position)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
