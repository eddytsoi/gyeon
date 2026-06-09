package shipany

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestCourierEnvelopeDecode pins the wire shape ListCouriers expects from
// `GET couriers/`. The full HTTPClient.do() path goes through settings +
// http stack which the package has no test infra for yet; this verifies
// the JSON tags on Courier / CourierSvcPl directly so a typo in the
// struct tags is caught at unit-test time.
func TestCourierEnvelopeDecode(t *testing.T) {
	body := []byte(`{
		"data": {
			"objects": [
				{
					"uid": "abc-123",
					"name": "SF Express",
					"cour_svc_plans": [
						{"cour_svc_pl": "SF Standard Delivery (Domestic)", "is_intl": false},
						{"cour_svc_pl": "SF International Standard", "is_intl": true}
					]
				},
				{"uid": "def-456", "name": "Hongkong Post"}
			]
		}
	}`)

	var resp envelope[Courier]
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := resp.Data.Objects
	if len(got) != 2 {
		t.Fatalf("want 2 couriers, got %d", len(got))
	}
	if got[0].UID != "abc-123" || got[0].Name != "SF Express" {
		t.Fatalf("courier[0] = %+v", got[0])
	}
	if len(got[0].SvcPlans) != 2 {
		t.Fatalf("want 2 svc plans, got %d", len(got[0].SvcPlans))
	}
	if got[0].SvcPlans[0].CourSvcPl != "SF Standard Delivery (Domestic)" || got[0].SvcPlans[0].IsIntl {
		t.Fatalf("svc[0] = %+v", got[0].SvcPlans[0])
	}
	if !got[0].SvcPlans[1].IsIntl {
		t.Fatalf("svc[1] should be intl: %+v", got[0].SvcPlans[1])
	}
	if got[1].UID != "def-456" || len(got[1].SvcPlans) != 0 {
		t.Fatalf("courier[1] = %+v", got[1])
	}
}

// TestBuildItems asserts the items slice that POST orders/ receives has the
// per-line shape ShipAny expects, with parcel-level fallback when per-row
// weight/dim is missing.
func TestBuildItems(t *testing.T) {
	parcel := Parcel{WeightGrams: 1200, LengthCM: 30, WidthCM: 20, HeightCM: 10}
	items := []ShipanyItem{
		{SKU: "SKU-A", Name: "Widget", UnitPriceHKD: 99.5, Quantity: 2, WeightGrams: 300, LengthCM: 5, WidthCM: 4, HeightCM: 3},
		{SKU: "SKU-B", Name: "Gadget", UnitPriceHKD: 12, Quantity: 1}, // falls back to parcel
	}
	out := buildItems(items, parcel)
	if len(out) != 2 {
		t.Fatalf("want 2 items, got %d", len(out))
	}
	a := out[0].(map[string]any)
	if a["sku"] != "SKU-A" || a["name"] != "Widget" || a["qty"] != 2 {
		t.Fatalf("item[0] header wrong: %+v", a)
	}
	if a["unt_price"].(map[string]any)["val"].(float64) != 99.5 {
		t.Fatalf("item[0] price wrong: %+v", a["unt_price"])
	}
	if a["wt"].(map[string]any)["val"].(float64) != 0.3 {
		t.Fatalf("item[0] weight should be 0.3 kg, got %+v", a["wt"])
	}
	if a["stg"] != "Normal" {
		t.Fatalf("item[0] stg = %v", a["stg"])
	}

	b := out[1].(map[string]any)
	// Per-line weight missing → falls back to parcel total (1.2 kg).
	if b["wt"].(map[string]any)["val"].(float64) != 1.2 {
		t.Fatalf("item[1] weight should fall back to 1.2 kg, got %+v", b["wt"])
	}
	// Per-line dims missing → fall back to parcel dims.
	dim := b["dim"].(map[string]any)
	if dim["len"].(float64) != 30 || dim["wid"].(float64) != 20 || dim["hgt"].(float64) != 10 {
		t.Fatalf("item[1] dim fallback wrong: %+v", dim)
	}
}

// TestNormalizePhoneNumber covers the formats operators paste into the
// origin-phone setting and the formats customers type at checkout.
func TestNormalizePhoneNumber(t *testing.T) {
	cases := []struct {
		raw, country, want string
	}{
		{"98765432", "HK", "98765432"},
		{"+852 9876 5432", "HK", "98765432"},
		{"(852) 9876-5432", "HK", "98765432"},
		{"85298765432", "HK", "98765432"},
		{"+886 912-345-678", "TW", "912345678"},
		{"+65 6123 4567", "SG", "61234567"},
		{"", "HK", ""},
		{"+852", "HK", ""}, // dial code only → empty (better surfaced upstream than masked)
	}
	for _, c := range cases {
		got := normalizePhoneNumber(c.raw, c.country)
		if got != c.want {
			t.Errorf("normalizePhoneNumber(%q, %q) = %q, want %q", c.raw, c.country, got, c.want)
		}
	}
}

// TestAPIErrorFormatting pins what callers see when ShipAny rejects a
// CreateShipment with the real-world "order already exists" envelope. The
// red banner in the admin UI is built from APIError.Error(), so this
// guards against regressions that would either re-truncate the body or
// drop the parsed result.details.
func TestAPIErrorFormatting(t *testing.T) {
	err := &APIError{
		Status:  403,
		Code:    403,
		Descr:   "Forbidden",
		Details: []string{"Order creation failed as order(uid: c68e4a46-8603-43d8-941c-9ec63e2b9689, ref: ORD-4916) already exists."},
		Method:  "POST",
		Route:   "orders/",
	}
	msg := err.Error()
	want := "shipany POST orders/: 403 Forbidden — Order creation failed as order(uid: c68e4a46-8603-43d8-941c-9ec63e2b9689, ref: ORD-4916) already exists."
	if msg != want {
		t.Fatalf("APIError formatting:\n  got:  %s\n  want: %s", msg, want)
	}
}

// TestAPIErrorFallsBackToRaw covers the non-JSON path: a gateway 502 returning
// an HTML page should still surface intact (no parsed envelope = use Raw).
func TestAPIErrorFallsBackToRaw(t *testing.T) {
	err := &APIError{
		Status: 502,
		Raw:    "<html>nginx: bad gateway</html>",
		Method: "GET",
		Route:  "merchants/self/",
	}
	msg := err.Error()
	if !strings.Contains(msg, "502") || !strings.Contains(msg, "nginx: bad gateway") {
		t.Fatalf("expected status + raw body in fallback, got: %s", msg)
	}
}

// TestExtractExistingShipanyUID covers the recovery hook in CreateForOrder:
// the regex must pull the uid out of the real ShipAny details string and
// must NOT match on unrelated 403s (e.g. permission errors).
func TestExtractExistingShipanyUID(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "real-world already-exists details",
			err: &APIError{
				Status:  403,
				Code:    403,
				Details: []string{"Order creation failed as order(uid: c68e4a46-8603-43d8-941c-9ec63e2b9689, ref: ORD-4916) already exists."},
			},
			want: "c68e4a46-8603-43d8-941c-9ec63e2b9689",
		},
		{
			name: "uid present in raw body only (envelope shape changed)",
			err: &APIError{
				Status: 403,
				Raw:    `{"result":{"descr":"Forbidden","details":["order(uid: a1b2c3d4-e5f6-7890-1234-567890abcdef, ref: ORD-1) already exists."]}}`,
			},
			want: "a1b2c3d4-e5f6-7890-1234-567890abcdef",
		},
		{
			name: "unrelated 403 (no uid present)",
			err:  &APIError{Status: 403, Details: []string{"API key revoked"}},
			want: "",
		},
		{
			name: "non-APIError",
			err:  errors.New("boom"),
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractExistingShipanyUID(c.err)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestBuildOrderPayloadCreateMode verifies that buildOrderPayload populates
// the create-specific fields (paid_by_rcvr) and shared scalars in the
// expected shape. The additional doc-required scalars (self_drop_off, stg,
// incoterms, items, etc.) are set by CreateShipment itself after this call.
func TestBuildOrderPayloadCreateMode(t *testing.T) {
	req := QuoteRequest{
		Origin:      Address{Name: "Origin Co", Country: "HK", Line1: "1 Origin St", Phone: "23456789"},
		Destination: Address{Name: "Jane Doe", Country: "HK", Line1: "2 Dest St", Phone: "98765432"},
		Parcel:      Parcel{WeightGrams: 500, ValueHKD: 199},
	}
	p := buildOrderPayload("mch-uid", req, "create", "cour-uid", "SF Express", "", 25, true)
	if p["mode"] != "create" {
		t.Fatalf("mode = %v", p["mode"])
	}
	if p["cour_uid"] != "cour-uid" || p["cour_svc_pl"] != "SF Express" {
		t.Fatalf("courier fields wrong: %+v", p)
	}
	if p["paid_by_rcvr"] != true {
		t.Fatalf("paid_by_rcvr should be true: %v", p["paid_by_rcvr"])
	}
	cost := p["cour_ttl_cost"].(map[string]any)
	if cost["val"].(float64) != 25 || cost["ccy"] != "HKD" {
		t.Fatalf("cour_ttl_cost wrong: %+v", cost)
	}
	if _, ok := p["sndr_ctc"]; !ok {
		t.Fatalf("sndr_ctc missing")
	}
	if _, ok := p["rcvr_ctc"]; !ok {
		t.Fatalf("rcvr_ctc missing")
	}
}

// TestBuildAddressPatchOps pins the JSON-Patch ops emitted when an admin edits
// an existing shipment's address. Every op must be an "add" (upsert) targeting
// the right /rcvr_ctc/* path with the same values create-mode would have sent.
func TestBuildAddressPatchOps(t *testing.T) {
	dest := Address{
		Name:    "Jane Doe",
		Country: "HK",
		Phone:   "98765432",
		Line1:   "2 Dest St",
		Line2:   "Flat A",
		City:    "Central",
	}
	ops := buildAddressPatchOps(dest)

	byPath := map[string]any{}
	for _, op := range ops {
		if op["op"] != "add" {
			t.Fatalf("op should be add, got %v for %v", op["op"], op["path"])
		}
		byPath[op["path"].(string)] = op["value"]
	}

	want := map[string]string{
		"/rcvr_ctc/addr/ln":    "2 Dest St",
		"/rcvr_ctc/addr/ln2":   "Flat A",
		"/rcvr_ctc/addr/distr": "Central",            // district falls back to city
		"/rcvr_ctc/addr/cnty":  "HKG",                // alpha-2 → alpha-3
		"/rcvr_ctc/addr/city":  "Hong Kong S.A.R.",   // HK city override
		"/rcvr_ctc/ctc/f_name": "Jane",
		"/rcvr_ctc/ctc/l_name": "Doe",
	}
	for path, exp := range want {
		got, ok := byPath[path]
		if !ok {
			t.Fatalf("missing op for %s", path)
		}
		if got != exp {
			t.Fatalf("%s = %v, want %q", path, got, exp)
		}
	}

	// Phone is patched as the phs array; email is omitted when blank.
	if _, ok := byPath["/rcvr_ctc/ctc/phs"]; !ok {
		t.Fatalf("missing /rcvr_ctc/ctc/phs")
	}
	if _, ok := byPath["/rcvr_ctc/ctc/email"]; ok {
		t.Fatalf("email op should be absent when dest.Email is blank")
	}
}

// TestCanRegenLabel pins the regen gate mirrored from the WC plugin: a label is
// only regenerated before pickup, when one already exists, and the external
// order didn't fail to create.
func TestCanRegenLabel(t *testing.T) {
	cases := []struct {
		name string
		o    orderObject
		want bool
	}{
		{"pre-pickup with label", orderObject{CurStat: "Order Created", LabURL: "http://x/label.pdf"}, true},
		{"no label yet", orderObject{CurStat: "Order Created", LabURL: ""}, false},
		{"already picked up", orderObject{CurStat: "Collected_By_Courier", LabURL: "http://x/label.pdf"}, false},
		{"ext order failed", orderObject{CurStat: "Order Created", LabURL: "http://x/label.pdf", ExtOrderNotCreated: "x"}, false},
	}
	for _, c := range cases {
		if got := c.o.canRegenLabel(); got != c.want {
			t.Fatalf("%s: canRegenLabel = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestBuildContactMacau pins the receiver contact shape for a HK→Macau
// cross-border shipment: the country maps to ShipAny's alpha-3 "MAC" and the
// phone carries Macau's "853" dial code. Both were previously unmapped, which
// caused SF to reject the waybill.
func TestBuildContactMacau(t *testing.T) {
	contact := buildContact(Address{
		Name:    "Chan Tai Man",
		Country: "MO",
		Phone:   "+853 6612 3456",
		Line1:   "Avenida da Praia Grande 1",
		City:    "Macau",
	})

	addr, ok := contact["addr"].(map[string]any)
	if !ok {
		t.Fatalf("addr missing or wrong type: %#v", contact["addr"])
	}
	if addr["cnty"] != "MAC" {
		t.Fatalf("addr.cnty = %v, want MAC", addr["cnty"])
	}

	ctc, ok := contact["ctc"].(map[string]any)
	if !ok {
		t.Fatalf("ctc missing or wrong type: %#v", contact["ctc"])
	}
	phs, ok := ctc["phs"].([]map[string]any)
	if !ok || len(phs) == 0 {
		t.Fatalf("ctc.phs missing or empty: %#v", ctc["phs"])
	}
	if phs[0]["cnty_code"] != "853" {
		t.Fatalf("phone cnty_code = %v, want 853", phs[0]["cnty_code"])
	}
	// Dial code is stripped from the local number (carried by cnty_code).
	if phs[0]["num"] != "66123456" {
		t.Fatalf("phone num = %v, want 66123456", phs[0]["num"])
	}
}

// TestPickCrossBorderOption verifies the cross-border plan selection: prefer the
// configured courier's cheapest plan, fall back to the cheapest plan overall,
// and return nil on no options.
func TestPickCrossBorderOption(t *testing.T) {
	opts := []RateOption{
		{Carrier: "sf", Service: "SF Speedy Express - HKMOTW", FeeHKD: 80},
		{Carrier: "sf", Service: "SF Standard Express - HKMOTW", FeeHKD: 55},
		{Carrier: "hkpost", Service: "SpeedPost", FeeHKD: 40},
	}

	// Prefer SF → cheapest SF plan (Standard, 55), not the cheaper HK Post one.
	if got := pickCrossBorderOption(opts, "sf"); got == nil || got.Service != "SF Standard Express - HKMOTW" {
		t.Fatalf("preferred-carrier pick = %#v, want SF Standard Express - HKMOTW", got)
	}

	// Preferred carrier absent → cheapest overall (HK Post, 40).
	if got := pickCrossBorderOption(opts, "dhl"); got == nil || got.Carrier != "hkpost" {
		t.Fatalf("fallback pick = %#v, want hkpost (cheapest overall)", got)
	}

	// No preference → cheapest overall.
	if got := pickCrossBorderOption(opts, ""); got == nil || got.Carrier != "hkpost" {
		t.Fatalf("no-preference pick = %#v, want hkpost (cheapest overall)", got)
	}

	if got := pickCrossBorderOption(nil, "sf"); got != nil {
		t.Fatalf("empty input pick = %#v, want nil", got)
	}
}
