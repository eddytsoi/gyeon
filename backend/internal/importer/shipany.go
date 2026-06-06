package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// importedWaybill is the normalised ShipAny shipment we extract from a WC
// order's meta_data and write into the local `shipments` table. It mirrors the
// subset of columns the native shipany.CreateForOrder path fills in
// (see shipany/service.go) so imported and natively-created shipments look the
// same to the admin order page.
type importedWaybill struct {
	ShipmentID     string // shipments.shipany_shipment_id (ShipAny order uid)
	TrackingNumber string // courier waybill number, e.g. SF0221318002736
	TrackingURL    string
	LabelURL       string
	Carrier        string // ShipAny courier UID (resolves to a name via the admin couriers list)
	Service        string // ShipAny service-plan code, e.g. sf-speedy-express
	FeeHKD         float64
}

// shipanyLabelTracking mirrors the object stored under the WC order meta key
// `_pr_shipment_shipany_label_tracking`. Note this is the *stored* shape read
// back via the WC REST API — it differs from the callback shape wcshim parses
// (which uses tracking_number / tracking_url / label_url). Here the courier
// waybill lives under courier_tracking_number / _url and the public label under
// label_path_s3.
type shipanyLabelTracking struct {
	ShipmentID            string `json:"shipment_id"`
	CourierTrackingNumber string `json:"courier_tracking_number"`
	CourierTrackingURL    string `json:"courier_tracking_url"`
	LabelPathS3           string `json:"label_path_s3"`
}

// shipanyOrderDetail mirrors the richer object stored under the WC order meta
// key `_pr_shipment_shipany_order_detail`. We read the courier identity, the
// quoted fee, and (as fallbacks) the tracking/label fields. The key sometimes
// appears more than once in meta_data with identical content.
type shipanyOrderDetail struct {
	UID          string `json:"uid"`
	TrkNo        string `json:"trk_no"`
	TrkURL       string `json:"trk_url"`
	LabURL       string `json:"lab_url"`
	CourUID      string `json:"cour_uid"`
	CourName     string `json:"cour_name"`
	CourSvcPl    string `json:"cour_svc_pl"`
	CourSvcPlAct string `json:"cour_svc_pl_act"`
	CourTtlCost  struct {
		Val float64 `json:"val"`
	} `json:"cour_ttl_cost"`
}

// decodeMetaValue unmarshals a WC meta value into target. WC usually stores the
// value as a JSON object, but sometimes as a serialized JSON string; we try the
// object first then unwrap one level of string, mirroring wcshim.extractTracking.
// Returns true only when a decode succeeded.
func decodeMetaValue(raw json.RawMessage, target any) bool {
	if len(raw) == 0 {
		return false
	}
	if err := json.Unmarshal(raw, target); err == nil {
		return true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if err2 := json.Unmarshal([]byte(s), target); err2 == nil {
			return true
		}
	}
	return false
}

// parseShipanyWaybill extracts an already-created ShipAny waybill from a WC
// order's meta_data. Returns nil when the order has no ShipAny shipment (the
// common case for orders that were never fulfilled before import). A waybill is
// considered present only when we can resolve a non-empty ShipAny shipment id —
// that's the NOT NULL minimum for the shipments row.
func parseShipanyWaybill(meta []wcMeta) *importedWaybill {
	var lt *shipanyLabelTracking
	var od *shipanyOrderDetail
	for _, m := range meta {
		switch m.Key {
		case "_pr_shipment_shipany_label_tracking":
			if lt == nil {
				var v shipanyLabelTracking
				if decodeMetaValue(m.Value, &v) {
					lt = &v
				}
			}
		case "_pr_shipment_shipany_order_detail":
			// May appear more than once (identical) — keep the first that
			// parses to a non-empty uid.
			if od == nil {
				var v shipanyOrderDetail
				if decodeMetaValue(m.Value, &v) && strings.TrimSpace(v.UID) != "" {
					od = &v
				}
			}
		}
	}
	if lt == nil && od == nil {
		return nil
	}

	wb := &importedWaybill{}
	if lt != nil {
		wb.ShipmentID = lt.ShipmentID
		wb.TrackingNumber = lt.CourierTrackingNumber
		wb.TrackingURL = lt.CourierTrackingURL
		wb.LabelURL = lt.LabelPathS3
	}
	if od != nil {
		wb.ShipmentID = firstNonEmpty(wb.ShipmentID, od.UID)
		wb.TrackingNumber = firstNonEmpty(wb.TrackingNumber, od.TrkNo)
		wb.TrackingURL = firstNonEmpty(wb.TrackingURL, od.TrkURL)
		wb.LabelURL = firstNonEmpty(wb.LabelURL, od.LabURL)
		wb.Carrier = firstNonEmpty(od.CourUID, od.CourName)
		wb.Service = firstNonEmpty(od.CourSvcPlAct, od.CourSvcPl)
		wb.FeeHKD = od.CourTtlCost.Val
	}

	if strings.TrimSpace(wb.ShipmentID) == "" {
		return nil
	}
	return wb
}

// upsertImportedShipment writes the extracted waybill into the shipments table
// inside the importer's transaction. Idempotent on the unique
// shipany_shipment_id so re-imports refresh tracking/label/courier/fee in place.
// status is left at 'created' on insert (matching the native auto-create) and is
// deliberately not overwritten on conflict, so a later ShipAny/wcshim advance
// survives a re-import. The order's selected_carrier/service are backfilled only
// when unset, so a customer's checkout selection is never clobbered.
func upsertImportedShipment(ctx context.Context, tx *sql.Tx, orderID string, wb *importedWaybill) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO shipments
		  (order_id, shipany_shipment_id, tracking_number, tracking_url, label_url,
		   carrier, service, fee_hkd, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'created')
		ON CONFLICT (shipany_shipment_id) DO UPDATE SET
		   tracking_number = EXCLUDED.tracking_number,
		   tracking_url    = EXCLUDED.tracking_url,
		   label_url       = EXCLUDED.label_url,
		   carrier         = EXCLUDED.carrier,
		   service         = EXCLUDED.service,
		   fee_hkd         = EXCLUDED.fee_hkd,
		   updated_at      = NOW()`,
		orderID, strings.TrimSpace(wb.ShipmentID),
		nullableString(wb.TrackingNumber), nullableString(wb.TrackingURL), nullableString(wb.LabelURL),
		wb.Carrier, wb.Service, wb.FeeHKD,
	); err != nil {
		return fmt.Errorf("upsert shipment: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET selected_carrier = $1, selected_service = $2
		  WHERE id = $3 AND selected_carrier IS NULL`,
		nullableString(wb.Carrier), nullableString(wb.Service), orderID); err != nil {
		return fmt.Errorf("backfill order courier: %w", err)
	}
	return nil
}
