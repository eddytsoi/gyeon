package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/lib/pq"
)

// shipanyMeta builds the two WC order meta_data entries that the ShipAny WC
// plugin writes for an already-created waybill, keyed off a shipment uid +
// courier tracking number so each test order can carry a distinct waybill.
// The order_detail key is emitted twice to mirror the real payload (#16681).
func shipanyMeta(shipmentID, trackingNo string) []wcMeta {
	labelTracking, _ := json.Marshal(map[string]any{
		"label_path":              "/uploads/woocommerce_shipany_label/" + shipmentID + ".pdf",
		"label_path_s3":           "https://labels.shipany.io/test/" + trackingNo + ".pdf",
		"shipment_id":             shipmentID,
		"tracking_number":         shipmentID, // plugin stores the uid here, not the courier no
		"courier_tracking_number": trackingNo,
		"courier_tracking_url":    "https://hk.sf-express.com/hk/tc/waybill/waybill-detail/" + trackingNo,
		"courier_service_plan":    "順豐速運 - HK$37",
	})
	orderDetail, _ := json.Marshal(map[string]any{
		"uid":             shipmentID,
		"cour_uid":        "6ae8b366-test-courier",
		"cour_name":       "SF Express",
		"cour_svc_pl":     "SF Express",
		"cour_svc_pl_act": "sf-speedy-express",
		"cour_ttl_cost":   map[string]any{"val": 37, "ccy": "HKD"},
		"trk_no":          trackingNo,
		"trk_url":         "https://hk.sf-express.com/hk/tc/waybill/waybill-detail/" + trackingNo,
		"lab_url":         "https://labels.shipany.io/test/" + trackingNo + ".pdf",
	})
	return []wcMeta{
		{Key: "_pr_shipment_shipany_label_tracking", Value: labelTracking},
		{Key: "_pr_shipment_shipany_order_detail", Value: orderDetail},
		{Key: "_pr_shipment_shipany_order_detail", Value: orderDetail},
	}
}

// TestUpsertOrderImportsShipanyWaybill proves the importer attaches an
// already-created ShipAny waybill: a WC `processing` order (mapped to 已付款)
// with shipment meta lands a shipments row and is bumped to 處理中; re-import is
// idempotent; and a WC `completed` order (已送達) keeps its status while still
// getting the waybill attached (no downgrade).
func TestUpsertOrderImportsShipanyWaybill(t *testing.T) {
	db := dialImporterTestDB(t)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	svc := &Service{db: db}

	const procWC, doneWC = 990000101, 990000102
	t.Cleanup(func() {
		db.ExecContext(ctx, `DELETE FROM shipments WHERE order_id IN (SELECT id FROM orders WHERE wc_order_id IN ($1,$2))`, procWC, doneWC)
		db.ExecContext(ctx, `DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE wc_order_id IN ($1,$2))`, procWC, doneWC)
		db.ExecContext(ctx, `DELETE FROM orders WHERE wc_order_id IN ($1,$2)`, procWC, doneWC)
	})

	// --- WC processing + waybill → 處理中 with a shipments row. ---
	proc := wcOrder{
		ID:                 procWC,
		Number:             "TST-SHIP",
		PaymentMethodTitle: "信用卡",
		DatePaidGMT:        "2026-06-06T02:08:49",
		Total:              "627.00",
		MetaData:           shipanyMeta("b518bf26-test-0001", "SFTEST00000001"),
	}
	var p OrdersProgressUpdate
	// mapWCOrderStatus("processing") == "paid"; the importer should bump to processing.
	if err := svc.upsertOrder(ctx, proc, "paid", "ORD", &p); err != nil {
		t.Fatalf("upsert processing order: %v", err)
	}
	if p.ImportedShipments != 1 {
		t.Errorf("ImportedShipments = %d, want 1", p.ImportedShipments)
	}

	var status, shipanyID, carrier, service string
	var trackNo, trackURL, labelURL, selCarrier, selService sql.NullString
	var fee float64
	if err := db.QueryRowContext(ctx, `
		SELECT o.status, o.selected_carrier, o.selected_service,
		       s.shipany_shipment_id, s.tracking_number, s.tracking_url, s.label_url,
		       s.carrier, s.service, s.fee_hkd
		  FROM orders o JOIN shipments s ON s.order_id = o.id
		 WHERE o.wc_order_id = $1`, procWC).
		Scan(&status, &selCarrier, &selService, &shipanyID, &trackNo, &trackURL, &labelURL, &carrier, &service, &fee); err != nil {
		t.Fatalf("read back processing order + shipment: %v", err)
	}
	if status != "processing" {
		t.Errorf("order status = %q, want processing (waybill bumps 已付款→處理中)", status)
	}
	if shipanyID != "b518bf26-test-0001" {
		t.Errorf("shipany_shipment_id = %q, want b518bf26-test-0001", shipanyID)
	}
	if trackNo.String != "SFTEST00000001" {
		t.Errorf("tracking_number = %q, want SFTEST00000001 (courier no, not the uid)", trackNo.String)
	}
	if labelURL.String != "https://labels.shipany.io/test/SFTEST00000001.pdf" {
		t.Errorf("label_url = %q, want the s3 label url", labelURL.String)
	}
	if carrier != "6ae8b366-test-courier" {
		t.Errorf("carrier = %q, want the cour_uid", carrier)
	}
	if service != "sf-speedy-express" {
		t.Errorf("service = %q, want sf-speedy-express", service)
	}
	if fee != 37 {
		t.Errorf("fee_hkd = %v, want 37", fee)
	}
	if selCarrier.String != "6ae8b366-test-courier" || selService.String != "sf-speedy-express" {
		t.Errorf("order courier not backfilled: selected_carrier=%q selected_service=%q", selCarrier.String, selService.String)
	}

	// --- Idempotency: re-import → still exactly one shipment, status unchanged. ---
	if err := svc.upsertOrder(ctx, proc, "paid", "ORD", &p); err != nil {
		t.Fatalf("re-upsert processing order: %v", err)
	}
	var shipCnt int
	var statusAfter string
	if err := db.QueryRowContext(ctx, `
		SELECT count(*), max(o.status::text)
		  FROM orders o JOIN shipments s ON s.order_id = o.id
		 WHERE o.wc_order_id = $1`, procWC).Scan(&shipCnt, &statusAfter); err != nil {
		t.Fatalf("count shipments after re-import: %v", err)
	}
	if shipCnt != 1 {
		t.Errorf("re-import produced %d shipment rows, want 1", shipCnt)
	}
	if statusAfter != "processing" {
		t.Errorf("status after re-import = %q, want processing", statusAfter)
	}

	// --- WC completed + waybill → waybill attached but status stays 已送達. ---
	done := wcOrder{
		ID:       doneWC,
		Number:   "TST-DONE",
		Total:    "100.00",
		MetaData: shipanyMeta("b518bf26-test-0002", "SFTEST00000002"),
	}
	// mapWCOrderStatus("completed") == "delivered".
	if err := svc.upsertOrder(ctx, done, "delivered", "ORD", &p); err != nil {
		t.Fatalf("upsert completed order: %v", err)
	}
	var doneStatus string
	var doneTrack sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT o.status, s.tracking_number
		  FROM orders o JOIN shipments s ON s.order_id = o.id
		 WHERE o.wc_order_id = $1`, doneWC).Scan(&doneStatus, &doneTrack); err != nil {
		t.Fatalf("read back completed order + shipment: %v", err)
	}
	if doneStatus != "delivered" {
		t.Errorf("completed order status = %q, want delivered (no downgrade)", doneStatus)
	}
	if doneTrack.String != "SFTEST00000002" {
		t.Errorf("completed order tracking_number = %q, want SFTEST00000002", doneTrack.String)
	}
}
