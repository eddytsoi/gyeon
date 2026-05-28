package catalog

import (
	"reflect"
	"testing"
)

func TestValidateHeader(t *testing.T) {
	cases := []struct {
		name    string
		header  []string
		wantErr bool
	}{
		{"exact", []string{"name", "variant", "quantity"}, false},
		{"trailing spaces", []string{" name ", "variant", " quantity"}, false},
		{"mixed case", []string{"Name", "VARIANT", "Quantity"}, false},
		{"with bom", []string{"\ufeffname", "variant", "quantity"}, false},
		{"wrong column", []string{"name", "variant", "qty"}, true},
		{"too few columns", []string{"name", "variant"}, true},
		{"too many columns", []string{"name", "variant", "quantity", "extra"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHeader(tc.header)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateHeader(%v): got err=%v, wantErr=%v", tc.header, err, tc.wantErr)
			}
		})
	}
}

func TestIsBlankRow(t *testing.T) {
	cases := []struct {
		name string
		rec  []string
		want bool
	}{
		{"empty slice", []string{}, true},
		{"all empty", []string{"", "", ""}, true},
		{"whitespace only", []string{" ", "\t", "  "}, true},
		{"one populated", []string{"", "x", ""}, false},
		{"all populated", []string{"a", "b", "c"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBlankRow(tc.rec); got != tc.want {
				t.Fatalf("IsBlankRow(%v) = %v, want %v", tc.rec, got, tc.want)
			}
		})
	}
}

func TestParsePgArray(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "{}", nil},
		{"null array", "{NULL}", nil},
		{"single", "{abc}", []string{"abc"}},
		{"multiple", "{a,b,c}", []string{"a", "b", "c"}},
		{"quoted with comma", `{"a,b","c"}`, []string{"a,b", "c"}},
		{"escaped quote", `{"he said \"hi\""}`, []string{`he said "hi"`}},
		{"strip nulls", "{a,NULL,b}", []string{"a", "b"}},
		{"malformed no braces", "abc", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePgArray(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parsePgArray(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPgTextArrayScan(t *testing.T) {
	var a pgTextArray
	if err := a.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if a.values != nil {
		t.Fatalf("Scan(nil) should yield nil values, got %v", a.values)
	}
	if err := a.Scan([]byte(`{x,y}`)); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	if !reflect.DeepEqual(a.values, []string{"x", "y"}) {
		t.Fatalf("Scan([]byte): got %v", a.values)
	}
	if err := a.Scan(`{p}`); err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if !reflect.DeepEqual(a.values, []string{"p"}) {
		t.Fatalf("Scan(string): got %v", a.values)
	}
	if err := a.Scan(42); err == nil {
		t.Fatalf("Scan(int) should error")
	}
}
