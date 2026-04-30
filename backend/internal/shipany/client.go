// Package shipany integrates with the ShipAny logistics gateway
// (https://www.shipany.io). Hong Kong only, v1.
//
// The wire contract (URL pattern, header names, payload shapes, response
// envelope) was reverse-engineered from the official WordPress / WooCommerce
// plugin source code (see source/shipany/) — ShipAny does not publish a
// public API spec.
//
// Base URL convention: https://api[-region][-env].shipany.io/
//   Region suffixes: -sg / -tw / -th  (HK = no suffix, v1 default)
//   Env suffixes:    -sbx1 / -sbx2 / -dev / -demo  (prod = no suffix)
//   The env is encoded as a prefix on the API key itself (SHIPANYDEV,
//   SHIPANYSBX1, etc.) which is stripped before the key is sent.
//
// Auth: single header  api-tk: <stripped-api-key>
// All responses follow  { result: {descr, details}, data: { objects: [...] } }
package shipany

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gyeon/backend/internal/settings"
)

// ErrNotConfigured is returned when shipany_api_key is empty.
var ErrNotConfigured = errors.New("shipany is not configured")

// Env prefixes the merchant portal embeds in the API key. We strip them
// before sending and use them to pick the subdomain variant.
var envPrefixes = map[string]string{
	"SHIPANYSBX1": "-sbx1",
	"SHIPANYSBX2": "-sbx2",
	"SHIPANYDEV":  "-dev",
	"SHIPANYDEMO": "-demo",
}

const productPlatform = "Gyeon"

// HTTPClient talks to the ShipAny REST API. Credentials are read from
// site_settings on every call so toggling them in admin takes effect immediately.
type HTTPClient struct {
	settings *settings.Service
	hc       *http.Client
	// baseOverride lets local mock servers shortcut the host derivation.
	// When empty, the host is derived from the API key prefix per call.
	baseOverride string
	pluginVer    string
}

func NewHTTPClient(s *settings.Service, baseOverride string) *HTTPClient {
	return &HTTPClient{
		settings:     s,
		hc:           &http.Client{Timeout: 20 * time.Second},
		baseOverride: strings.TrimRight(baseOverride, "/"),
		pluginVer:    "0.4.0",
	}
}

// ── Public types (stable shape returned to handler/frontend) ──────────

type Address struct {
	Name       string `json:"name,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	District   string `json:"district,omitempty"`
	City       string `json:"city,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country"` // ISO 3166-1 alpha-2, "HK" for v1
}

type Parcel struct {
	WeightGrams int     `json:"weight_g"`
	LengthCM    float64 `json:"length_cm,omitempty"`
	WidthCM     float64 `json:"width_cm,omitempty"`
	HeightCM    float64 `json:"height_cm,omitempty"`
	ValueHKD    float64 `json:"value_hkd,omitempty"`
}

type QuoteRequest struct {
	Origin      Address
	Destination Address
	Parcel      Parcel
}

// RateOption is one row in the carrier-selection list.
type RateOption struct {
	// QuotUID is the opaque ShipAny quote identifier; pass it back when
	// creating the shipment to lock in the quoted price.
	QuotUID             string  `json:"quot_uid,omitempty"`
	Carrier             string  `json:"carrier"`             // courier UID
	CarrierName         string  `json:"carrier_name"`        // human label
	Service             string  `json:"service"`             // cour_svc_pl
	ServiceName         string  `json:"service_name"`
	FeeHKD              float64 `json:"fee_hkd"`
	ETADays             string  `json:"eta_days,omitempty"`
	RequiresPickupPoint bool    `json:"requires_pickup_point"`
}

type PickupPoint struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	District string `json:"district,omitempty"`
	Carrier  string `json:"carrier,omitempty"`
}

// Courier is one row from ShipAny's `GET couriers/` endpoint. The admin
// settings UI uses this to populate the "default courier" dropdown so the
// operator picks by name instead of pasting opaque UIDs.
type Courier struct {
	UID      string         `json:"uid"`
	Name     string         `json:"name"`
	SvcPlans []CourierSvcPl `json:"cour_svc_plans,omitempty"`
}

type CourierSvcPl struct {
	CourSvcPl string `json:"cour_svc_pl"`
	IsIntl    bool   `json:"is_intl,omitempty"`
}

type CreateShipmentRequest struct {
	Carrier       string  // cour_uid
	Service       string  // cour_svc_pl (optional)
	QuotUID       string  // optional: lock the quoted price
	OrderRef      string  // ext_order_ref
	Origin        Address // sender (also resolvable via merchants/self)
	Destination   Address // receiver
	Parcel        Parcel
	PickupPointID string
	CustomerNote  string
	FeeHKD        float64 // cour_ttl_cost.val
}

// Shipment is the post-create response. Status mirrors ShipAny's `cur_stat`
// strings as-is; the service layer normalises them into the local taxonomy.
type Shipment struct {
	ID             string  `json:"id"`              // ShipAny uid
	TrackingNumber string  `json:"tracking_number"` // courier trk_no
	TrackingURL    string  `json:"tracking_url"`
	LabelURL       string  `json:"label_url"`
	FeeHKD         float64 `json:"fee_hkd"`
	Status         string  `json:"status"` // raw cur_stat
}

// ── Calls ──────────────────────────────────────────────────────────────

// Ping confirms credentials work. Cheapest authenticated call we can use.
func (c *HTTPClient) Ping(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "merchants/self/", nil, nil, nil)
	return err
}

// merchantUID returns the merchant's own uid. Required when building
// quote / create-shipment payloads.
func (c *HTTPClient) merchantUID(ctx context.Context) (string, error) {
	var resp envelope[merchantSelf]
	if _, err := c.do(ctx, http.MethodGet, "merchants/self/", nil, nil, &resp); err != nil {
		return "", err
	}
	if len(resp.Data.Objects) == 0 {
		return "", errors.New("merchants/self returned no objects")
	}
	return resp.Data.Objects[0].UID, nil
}

func (c *HTTPClient) Quote(ctx context.Context, req QuoteRequest) ([]RateOption, error) {
	mchUID, err := c.merchantUID(ctx)
	if err != nil {
		return nil, err
	}
	payload := buildOrderPayload(mchUID, req, "query", "", "", "", 0)

	var resp envelope[orderObject]
	if _, err := c.do(ctx, http.MethodPost, "couriers-connector/query-rate/", payload, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data.Objects) == 0 || len(resp.Data.Objects[0].Quots) == 0 {
		return []RateOption{}, nil
	}

	out := make([]RateOption, 0, len(resp.Data.Objects[0].Quots))
	for _, q := range resp.Data.Objects[0].Quots {
		out = append(out, RateOption{
			QuotUID:             q.QuotUID,
			Carrier:             q.CourUID,
			CarrierName:         coalesce(q.CourName, q.CourUID),
			Service:             q.CourSvcPl,
			ServiceName:         q.CourSvcPl,
			FeeHKD:              q.CourTtlCost.Val,
			RequiresPickupPoint: q.RequiresPickupPoint,
		})
	}
	return out, nil
}

// ListCouriers returns every courier the merchant has enabled in their
// ShipAny portal. The list is unfiltered — caller can intersect with
// merchants/self.desig_cours if a subset is ever needed.
func (c *HTTPClient) ListCouriers(ctx context.Context) ([]Courier, error) {
	var resp envelope[Courier]
	if _, err := c.do(ctx, http.MethodGet, "couriers/", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data.Objects, nil
}

// ListPickupPoints returns ShipAny's full set of service points; carrier and
// district are applied client-side as the API does not narrow server-side
// (matches the WC plugin behaviour).
func (c *HTTPClient) ListPickupPoints(ctx context.Context, carrier, district string) ([]PickupPoint, error) {
	var resp envelope[servicePoint]
	if _, err := c.do(ctx, http.MethodGet, "courier-service-point-locations", nil, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]PickupPoint, 0)
	for _, sp := range resp.Data.Objects {
		if carrier != "" && sp.CourUID != carrier {
			continue
		}
		if district != "" && !strings.EqualFold(sp.District, district) {
			continue
		}
		out = append(out, PickupPoint{
			ID:       sp.UID,
			Name:     sp.Name,
			Address:  sp.Addr,
			District: sp.District,
			Carrier:  sp.CourUID,
		})
	}
	return out, nil
}

// CreateShipment posts to /orders/ in create mode. The returned label URL
// (lab_url) is a CDN link; callers can fetch the PDF at that URL directly.
func (c *HTTPClient) CreateShipment(ctx context.Context, req CreateShipmentRequest) (*Shipment, error) {
	mchUID, err := c.merchantUID(ctx)
	if err != nil {
		return nil, err
	}
	qreq := QuoteRequest{Origin: req.Origin, Destination: req.Destination, Parcel: req.Parcel}
	payload := buildOrderPayload(mchUID, qreq, "create", req.Carrier, req.Service, req.QuotUID, req.FeeHKD)
	if req.OrderRef != "" {
		payload["ext_order_ref"] = req.OrderRef
	}
	if req.PickupPointID != "" {
		payload["pickup_point_uid"] = req.PickupPointID
	}
	if req.CustomerNote != "" {
		payload["mch_notes"] = []string{req.CustomerNote}
	}

	var resp envelope[orderObject]
	status, err := c.do(ctx, http.MethodPost, "orders/", payload, nil, &resp)
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d creating shipment", status)
	}
	if len(resp.Data.Objects) == 0 {
		return nil, errors.New("orders/ returned no objects")
	}
	o := resp.Data.Objects[0]
	return &Shipment{
		ID:             o.UID,
		TrackingNumber: o.TrkNo,
		TrackingURL:    "https://portal.shipany.io/tracking?id=" + url.QueryEscape(o.UID),
		LabelURL:       o.LabURL,
		FeeHKD:         o.CourTtlCost.Val,
		Status:         o.CurStat,
	}, nil
}

// RequestPickup PATCHes the shipment to add a "Pickup Request Sent" state.
func (c *HTTPClient) RequestPickup(ctx context.Context, shipmentID string) error {
	body := map[string]any{
		"ops": []map[string]any{{
			"op":    "add",
			"path":  "/states/0",
			"value": map[string]any{"stat": "Pickup Request Sent"},
		}},
	}
	_, err := c.do(ctx, http.MethodPatch,
		"orders/"+url.PathEscape(shipmentID)+"/", body, nil, nil)
	return err
}

// FetchOrder polls a shipment for its latest tracking state. Used by the
// admin "refresh status" button — ShipAny does not push updates.
func (c *HTTPClient) FetchOrder(ctx context.Context, shipmentID string) (*Shipment, error) {
	var resp envelope[orderObject]
	if _, err := c.do(ctx, http.MethodGet,
		"orders/"+url.PathEscape(shipmentID)+"/", nil, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data.Objects) == 0 {
		return nil, errors.New("orders/{id}/ returned no objects")
	}
	o := resp.Data.Objects[0]
	return &Shipment{
		ID:             o.UID,
		TrackingNumber: o.TrkNo,
		TrackingURL:    "https://portal.shipany.io/tracking?id=" + url.QueryEscape(o.UID),
		LabelURL:       o.LabURL,
		FeeHKD:         o.CourTtlCost.Val,
		Status:         o.CurStat,
	}, nil
}

// ── Wire envelope + structs (private) ──────────────────────────────────

type envelope[T any] struct {
	Result struct {
		Descr   string   `json:"descr,omitempty"`
		Details []string `json:"details,omitempty"`
	} `json:"result,omitempty"`
	Data struct {
		Objects []T `json:"objects"`
	} `json:"data,omitempty"`
}

type merchantSelf struct {
	UID    string `json:"uid"`
	CoInfo struct {
		CtcPers []struct {
			Ctc struct {
				FName string `json:"f_name"`
				LName string `json:"l_name"`
				Phs   []struct {
					Typ      string `json:"typ"`
					CntyCode string `json:"cnty_code"`
					Num      string `json:"num"`
				} `json:"phs"`
			} `json:"ctc"`
		} `json:"ctc_pers"`
		OrgCtcs []struct {
			Ctc struct {
				FName string `json:"f_name"`
				Email string `json:"email"`
			} `json:"ctc"`
			Addr struct {
				Typ   string `json:"typ"`
				Ln    string `json:"ln"`
				Ln2   string `json:"ln2,omitempty"`
				City  string `json:"city,omitempty"`
				Cnty  string `json:"cnty,omitempty"`
				Distr string `json:"distr,omitempty"`
				State string `json:"state,omitempty"`
			} `json:"addr"`
		} `json:"org_ctcs"`
	} `json:"co_info"`
}

type orderObject struct {
	UID         string  `json:"uid"`
	CurStat     string  `json:"cur_stat,omitempty"`
	TrkNo       string  `json:"trk_no,omitempty"`
	LabURL      string  `json:"lab_url,omitempty"`
	CourUID     string  `json:"cour_uid,omitempty"`
	CourSvcPl   string  `json:"cour_svc_pl,omitempty"`
	PayStat     string  `json:"pay_stat,omitempty"`
	CourTtlCost cost    `json:"cour_ttl_cost"`
	Quots       []quot  `json:"quots,omitempty"`
}

type quot struct {
	QuotUID             string `json:"quot_uid"`
	CourUID             string `json:"cour_uid"`
	CourName            string `json:"cour_name,omitempty"`
	CourSvcPl           string `json:"cour_svc_pl"`
	CourType            string `json:"cour_type,omitempty"`
	CourTtlCost         cost   `json:"cour_ttl_cost"`
	RequiresPickupPoint bool   `json:"requires_pickup_point,omitempty"`
}

type cost struct {
	Val float64 `json:"val"`
	Ccy string  `json:"ccy"`
}

type servicePoint struct {
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Addr     string `json:"addr,omitempty"`
	District string `json:"distr,omitempty"`
	CourUID  string `json:"cour_uid,omitempty"`
}

// buildOrderPayload mirrors the WC plugin's
// item_info_to_request_data_shipany() layout for both query-rate and
// create-order requests. mode = "query" or "create".
func buildOrderPayload(mchUID string, req QuoteRequest, mode, courUID, courSvcPl, quotUID string, feeHKD float64) map[string]any {
	weightKG := float64(req.Parcel.WeightGrams) / 1000.0
	if weightKG <= 0 {
		weightKG = 0.5
	}
	dim := map[string]any{
		"len": defaultFloat(req.Parcel.LengthCM, 1),
		"wid": defaultFloat(req.Parcel.WidthCM, 1),
		"hgt": defaultFloat(req.Parcel.HeightCM, 1),
		"unt": "cm",
	}
	cost := map[string]any{"val": feeHKD, "ccy": "HKD"}
	if mode == "query" {
		cost["val"] = 1 // sentinel; the API replaces it with quoted prices
	}

	payload := map[string]any{
		"mode":         mode,
		"mch_uid":      mchUID,
		"order_from":   strings.ToLower(productPlatform),
		"cour_uid":     courUID,
		"cour_svc_pl":  courSvcPl,
		"wt":           map[string]any{"val": weightKG, "unt": "kg"},
		"dim":          dim,
		"cour_ttl_cost": cost,
		"mch_ttl_val":  map[string]any{"val": req.Parcel.ValueHKD, "ccy": "HKD"},
		"sndr_ctc":     buildContact(req.Origin),
		"rcvr_ctc":     buildContact(req.Destination),
		"items":        []any{},
	}
	if quotUID != "" {
		payload["quot_uid"] = quotUID
	}
	return payload
}

func buildContact(a Address) map[string]any {
	first, last := splitName(a.Name)
	city := a.City
	district := a.District
	if district == "" {
		district = a.City
	}
	if strings.EqualFold(a.Country, "HK") {
		city = "Hong Kong S.A.R."
	}
	return map[string]any{
		"ctc": map[string]any{
			"f_name": first,
			"l_name": last,
			"phs": []map[string]any{{
				"typ":       "Mobile",
				"cnty_code": phoneCountryCode(a.Country),
				"num":       a.Phone,
			}},
		},
		"addr": map[string]any{
			"typ":   "Residential",
			"ln":    a.Line1,
			"ln2":   a.Line2,
			"distr": district,
			"cnty":  countryAlpha3(a.Country),
			"state": city,
			"city":  city,
			"zc":    a.PostalCode,
		},
	}
}

func splitName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.SplitN(full, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func countryAlpha3(alpha2 string) string {
	switch strings.ToUpper(alpha2) {
	case "HK":
		return "HKG"
	case "TW":
		return "TWN"
	case "TH":
		return "THA"
	case "SG":
		return "SGP"
	}
	return strings.ToUpper(alpha2)
}

func phoneCountryCode(alpha2 string) string {
	switch strings.ToUpper(alpha2) {
	case "HK":
		return "852"
	case "TW":
		return "886"
	case "TH":
		return "66"
	case "SG":
		return "65"
	}
	return ""
}

func defaultFloat(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return v
}

func coalesce(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// ── HTTP plumbing ──────────────────────────────────────────────────────

func (c *HTTPClient) do(ctx context.Context, method, route string, body, query any, out any) (int, error) {
	rawKey := c.read(ctx, "shipany_api_key")
	if rawKey == "" {
		return 0, ErrNotConfigured
	}
	apiTk, env := stripEnvPrefix(rawKey)
	region := strings.TrimSpace(c.read(ctx, "shipany_region")) // e.g. "-tw" — empty = HK

	baseURL := c.baseOverride
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://api%s%s.shipany.io", region, env)
	}
	full := baseURL + "/" + strings.TrimLeft(route, "/")

	var reader io.Reader
	if body != nil {
		// Match the WC plugin: when the body is an object, automatically
		// inject plugin_version + order_from for ShipAny's analytics.
		if m, ok := body.(map[string]any); ok {
			if _, exists := m["plugin_version"]; !exists {
				m["plugin_version"] = c.pluginVer
			}
			if _, exists := m["order_from"]; !exists {
				m["order_from"] = strings.ToLower(productPlatform)
			}
			body = m
		}
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("api-tk", apiTk)
	req.Header.Set("order-from", productPlatform)
	req.Header.Set("order-from-ver", c.pluginVer)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("shipany %s %s: %w", method, route, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("shipany %s %s: %d %s",
			method, route, resp.StatusCode, truncate(string(respBody), 256))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode shipany response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (c *HTTPClient) read(ctx context.Context, key string) string {
	st, err := c.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}

// stripEnvPrefix removes a SHIPANYSBX1 / SHIPANYDEV / etc. prefix from the
// API key and returns the matching subdomain suffix (or "").
func stripEnvPrefix(key string) (clean, envSuffix string) {
	for prefix, suffix := range envPrefixes {
		if strings.HasPrefix(key, prefix) {
			return strings.TrimPrefix(key, prefix), suffix
		}
	}
	return key, ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
