package wcshim

import (
	"encoding/json"
	"testing"
)

// metaOf builds a meta_data slice with a single key/value, marshalling the value
// the way ShipAny sends it (a JSON scalar string).
func metaOf(t *testing.T, key, val string) []wcMeta {
	t.Helper()
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatalf("marshal %q: %v", val, err)
	}
	return []wcMeta{{Key: key, Value: raw}}
}

func TestExtractShipanyState(t *testing.T) {
	// The real callback puts the ShipAny state in the order-state meta, NOT the
	// top-level status — this is the field the fix reads.
	if got := extractShipanyState(metaOf(t, "_pr_shipment_shipany_order_state", "Order_Delivered")); got != "Order_Delivered" {
		t.Errorf("extractShipanyState = %q, want %q", got, "Order_Delivered")
	}

	// Tracking blob present but no order-state meta → "".
	if got := extractShipanyState(metaOf(t, "_pr_shipment_shipany_label_tracking", `{"shipment_id":"x"}`)); got != "" {
		t.Errorf("extractShipanyState (other key) = %q, want \"\"", got)
	}

	// Empty meta → "".
	if got := extractShipanyState(nil); got != "" {
		t.Errorf("extractShipanyState(nil) = %q, want \"\"", got)
	}
}
