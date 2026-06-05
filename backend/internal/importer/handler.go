package importer

import (
	"encoding/json"
	"net/http"

	"gyeon/backend/internal/auth"
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
// Enqueues the products import onto the shared single-worker FIFO queue
// (decoupled from this request, so closing/leaving the admin page does not
// abort it) and returns 202 immediately. If a products import is already
// running or queued, it's a no-op enqueue (queued:false) and the caller just
// reconnects via ImportStatus. Progress is read back via ImportStatus; stop
// via CancelImport.
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
	actorID, _ := auth.AdminIDFromContext(r.Context())
	pos, queued := h.svc.EnqueueProducts(req, actorID)
	respond.JSON(w, http.StatusAccepted, map[string]any{"queued": queued, "position": pos})
}

// StartOrders handles POST /api/v1/admin/import/woocommerce/orders/start.
// Enqueues the orders import onto the shared FIFO queue (see StartImport).
func (h *Handler) StartOrders(w http.ResponseWriter, r *http.Request) {
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
	actorID, _ := auth.AdminIDFromContext(r.Context())
	pos, queued := h.svc.EnqueueOrders(req, actorID)
	respond.JSON(w, http.StatusAccepted, map[string]any{"queued": queued, "position": pos})
}

// StartCustomers handles POST /api/v1/admin/import/woocommerce/customers/start.
// Enqueues the customers import onto the shared FIFO queue (see StartImport).
func (h *Handler) StartCustomers(w http.ResponseWriter, r *http.Request) {
	var req CustomersImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.WCURL == "" || req.WCKey == "" || req.WCSecret == "" {
		respond.BadRequest(w, "wc_url, wc_key, and wc_secret are required")
		return
	}
	actorID, _ := auth.AdminIDFromContext(r.Context())
	pos, queued := h.svc.EnqueueCustomers(req, actorID)
	respond.JSON(w, http.StatusAccepted, map[string]any{"queued": queued, "position": pos})
}

// ImportStatus handles GET /api/v1/admin/import/woocommerce/status.
// Returns one combined view of the whole import queue so all three tabs can
// reconnect from a single poll:
//
//	{
//	  "running_type": "products" | "orders" | "customers" | null,
//	  "queue":        ["orders", ...],           // types waiting, FIFO
//	  "jobs": {                                  // per-type view (omitted if never run/queued)
//	    "products": { running, queued, position, done, failed, fail_msg, progress },
//	    ...
//	  }
//	}
//
// For each type the view reflects priority running > queued > last-finished:
// a running job carries its live progress; a queued job carries its 1-based
// position and a nil progress (no stale snapshot); a finished job carries its
// final snapshot with done/failed set.
func (h *Handler) ImportStatus(w http.ResponseWriter, r *http.Request) {
	running, queue, last := h.svc.QueueStatus()

	posOf := func(typ string) int {
		for i, t := range queue {
			if t == typ {
				return i + 1
			}
		}
		return 0
	}
	view := func(typ string) map[string]any {
		if running != nil && running.Type == typ {
			return map[string]any{
				"type": typ, "running": true, "queued": false,
				"failed": false, "done": false, "progress": running.Progress,
			}
		}
		if pos := posOf(typ); pos > 0 {
			return map[string]any{
				"type": typ, "running": false, "queued": true, "position": pos,
				"failed": false, "done": false, "progress": nil,
			}
		}
		if lj, ok := last[typ]; ok {
			return map[string]any{
				"type": typ, "running": false, "queued": false,
				"failed": lj.Failed, "fail_msg": lj.FailMsg,
				"done": !lj.Failed, "progress": lj.Progress,
			}
		}
		return nil
	}

	jobs := map[string]any{}
	for _, t := range []string{jobTypeProducts, jobTypeOrders, jobTypeCustomers} {
		if v := view(t); v != nil {
			jobs[t] = v
		}
	}
	var runningType any
	if running != nil {
		runningType = running.Type
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"running_type": runningType,
		"queue":        queue,
		"jobs":         jobs,
	})
}

// CancelImport handles POST /api/v1/admin/import/woocommerce/cancel.
// Body: {"type": "products"|"orders"|"customers"} (optional — empty targets the
// running job). A running job is signalled to stop (takes effect within one
// item); a merely-queued job is removed from the queue. Returns 200 if
// something matched, 409 otherwise.
func (h *Handler) CancelImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // body is optional; empty ⇒ running job
	if !h.svc.Cancel(body.Type) {
		respond.Error(w, http.StatusConflict, "no import is running or queued for that type")
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

// Orders and customers imports run through the shared FIFO queue (StartOrders /
// StartCustomers + ImportStatus polling), so they no longer have dedicated SSE
// stream handlers — the live view is driven by ImportStatus like products.
