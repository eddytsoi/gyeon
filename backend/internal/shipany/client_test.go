package shipany

import (
	"encoding/json"
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
