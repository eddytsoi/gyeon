package importer

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gyeon/backend/internal/respond"
)

// Handler exposes the import endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetCredentials returns the saved WooCommerce credentials, if any.
// GET /api/v1/admin/import/woocommerce/credentials
func (h *Handler) GetCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := h.svc.GetCredentials(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respond.JSON(w, http.StatusOK, creds)
}

// SaveCredentials persists the WooCommerce credentials to site_settings
// without running an import. The Test endpoint is the way to verify the
// values; this one just stores whatever the admin sent.
// PUT /api/v1/admin/import/woocommerce/credentials
func (h *Handler) SaveCredentials(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if err := h.svc.SaveCredentials(r.Context(), creds); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Test handles POST /api/v1/admin/import/woocommerce/test.
// Verifies WC credentials and read access without making any changes.
// On success returns the total product count so the admin UI can display
// a meaningful "connection ok — N products" message.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	if err := h.svc.TestConnection(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"total_products": h.svc.ProductTotal(req),
	})
}

// StartImport handles POST /api/v1/admin/import/woocommerce/start.
// Launches the products import as a detached background job (decoupled from
// this request, so closing/leaving the admin page does not abort it) and
// returns 202 immediately. Returns 409 if an import is already running.
// Progress is read back via ImportStatus; stop via CancelImport.
func (h *Handler) StartImport(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	if !h.svc.StartProductsImport(req) {
		respond.Error(w, http.StatusConflict, "an import is already running")
		return
	}
	respond.JSON(w, http.StatusAccepted, map[string]bool{"running": true})
}

// ImportStatus handles GET /api/v1/admin/import/woocommerce/status.
// Returns the latest progress snapshot of the products import plus a running
// flag, so a returning page can reconnect. When no import has run this process
// lifetime, returns {exists:false, running:false}.
func (h *Handler) ImportStatus(w http.ResponseWriter, r *http.Request) {
	job, ok := h.svc.ProductsJobStatus()
	if !ok {
		respond.JSON(w, http.StatusOK, map[string]any{"exists": false, "running": false})
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"exists":   true,
		"running":  job.Running,
		"failed":   job.Failed,
		"fail_msg": job.FailMsg,
		"progress": job.Progress,
	})
}

// CancelImport handles POST /api/v1/admin/import/woocommerce/cancel.
// Signals the running products import to stop (takes effect within one
// product). Returns 200 if a job was signalled, 409 if nothing is running.
func (h *Handler) CancelImport(w http.ResponseWriter, r *http.Request) {
	if !h.svc.CancelProductsImport() {
		respond.Error(w, http.StatusConflict, "no import is running")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]bool{"cancelling": true})
}

// CustomersTest handles POST /api/v1/admin/import/woocommerce/customers/test.
// Same connectivity check as the products test path (the same WC creds
// authenticate /wc/v3/customers), but additionally returns the total
// customer count so the admin UI can preview the run size.
func (h *Handler) CustomersTest(w http.ResponseWriter, r *http.Request) {
	var req CustomersImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	if err := h.svc.TestConnection(ImportRequest{WCURL: req.WCURL, WCKey: req.WCKey, WCSecret: req.WCSecret}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"total_customers": h.svc.CustomerTotal(req),
	})
}

// OrdersTest handles POST /api/v1/admin/import/woocommerce/orders/test.
// Reuses the products test for connectivity (same WC creds authenticate
// /wc/v3/orders) and returns the total order count.
func (h *Handler) OrdersTest(w http.ResponseWriter, r *http.Request) {
	var req OrdersImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	req.Status = normalizeWCOrderStatus(req.Status)
	if err := h.svc.TestConnection(ImportRequest{WCURL: req.WCURL, WCKey: req.WCKey, WCSecret: req.WCSecret}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"total_orders": h.svc.OrderTotal(req),
	})
}

// OrdersImportStream handles POST /api/v1/admin/import/woocommerce/orders/stream.
// Streams Server-Sent Events with OrdersProgressUpdate JSON.
func (h *Handler) OrdersImportStream(w http.ResponseWriter, r *http.Request) {
	var req OrdersImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	req.Status = normalizeWCOrderStatus(req.Status)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(p OrdersProgressUpdate) {
		b, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	h.svc.RunOrdersStreaming(r.Context(), req, send)
}

// CustomersImportStream handles POST /api/v1/admin/import/woocommerce/customers/stream.
// Streams Server-Sent Events with CustomersProgressUpdate JSON.
func (h *Handler) CustomersImportStream(w http.ResponseWriter, r *http.Request) {
	var req CustomersImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(p CustomersProgressUpdate) {
		b, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	h.svc.RunCustomersStreaming(r.Context(), req, send)
}
