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

// Test handles POST /api/v1/admin/import/woocommerce/test.
// Verifies WC credentials and read access without making any changes.
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
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ImportStream handles POST /api/v1/admin/import/woocommerce/stream.
// Streams Server-Sent Events with ProgressUpdate JSON as import progresses.
func (h *Handler) ImportStream(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
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

	send := func(p ProgressUpdate) {
		b, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	h.svc.RunStreaming(r.Context(), req, send)
}
