package receipt

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/queue"
	"gyeon/backend/internal/respond"
)

// maxBatchOrders caps how many orders one batch can request, bounding both the
// worker's runtime and the resulting ZIP size.
const maxBatchOrders = 200

type batchCreateRequest struct {
	OrderIDs []string `json:"order_ids"`
	Locale   string   `json:"locale"`
}

// adminBatchCreate accepts a set of order ids, records a pending batch and
// enqueues the worker job. Responds 202 with the new batch id; the client
// polls adminBatchStatus until the ZIP is ready.
func (h *Handler) adminBatchCreate(w http.ResponseWriter, r *http.Request) {
	if h.batchStore == nil || h.enqueuer == nil {
		respond.Error(w, http.StatusServiceUnavailable, "batch receipts not configured")
		return
	}
	var req batchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	ids := dedupeNonEmpty(req.OrderIDs)
	if len(ids) == 0 {
		respond.BadRequest(w, "order_ids must not be empty")
		return
	}
	if len(ids) > maxBatchOrders {
		respond.BadRequest(w, fmt.Sprintf("too many orders: max %d per batch", maxBatchOrders))
		return
	}
	if req.Locale != "" && !isKnownLocale(req.Locale) {
		respond.BadRequest(w, "invalid locale")
		return
	}
	locale := resolveLocale(req.Locale)

	id, err := h.batchStore.CreateBatch(r.Context(), locale, ids)
	if err != nil {
		respond.InternalError(w)
		return
	}
	payload, _ := json.Marshal(GenerateReceiptBatchJob{BatchID: id})
	if _, err := h.enqueuer.Enqueue(r.Context(), queue.JobTypeGenerateReceiptBatch, payload); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to enqueue batch job")
		return
	}
	respond.JSON(w, http.StatusAccepted, map[string]any{"batch_id": id})
}

// adminBatchStatus reports a batch's progress so the UI can poll for the
// download. zip_ready is true only once the ZIP exists on disk.
func (h *Handler) adminBatchStatus(w http.ResponseWriter, r *http.Request) {
	if h.batchStore == nil {
		respond.Error(w, http.StatusServiceUnavailable, "batch receipts not configured")
		return
	}
	id := chi.URLParam(r, "id")
	batch, err := h.batchStore.GetBatch(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
	respond.JSON(w, http.StatusOK, map[string]any{
		"status":          batch.Status,
		"total":           batch.Total,
		"succeeded_count": batch.SucceededCount,
		"errors":          batch.Errors,
		"zip_ready":       batch.Status == "succeeded" && batch.ZipPath != "",
	})
}

// adminBatchDownload streams the built ZIP. 404 until the batch has succeeded
// with at least one receipt.
func (h *Handler) adminBatchDownload(w http.ResponseWriter, r *http.Request) {
	if h.batchStore == nil {
		respond.Error(w, http.StatusServiceUnavailable, "batch receipts not configured")
		return
	}
	id := chi.URLParam(r, "id")
	batch, err := h.batchStore.GetBatch(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if batch.Status != "succeeded" || batch.ZipPath == "" {
		respond.NotFound(w)
		return
	}
	data, err := os.ReadFile(batch.ZipPath)
	if err != nil {
		respond.NotFound(w)
		return
	}
	filename := "receipts-" + time.Now().Format("20060102") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
	w.Write(data)
}

// dedupeNonEmpty trims blanks and removes duplicate ids while preserving the
// caller's order.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
