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
