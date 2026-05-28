package receipt

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/queue"
)

// batchZipDir holds built batch ZIPs. Sits under the same /uploads volume as
// the per-order receipt cache so it rides the bind-mounted volume in prod.
const batchZipDir = "./uploads/receipts/batches"

// GenerateReceiptBatchJob is the queue payload for building one receipt ZIP.
// Only the batch id travels on the queue; everything else lives in the
// receipt_batch_jobs row so the handler reads a consistent snapshot.
type GenerateReceiptBatchJob struct {
	BatchID string `json:"batch_id"`
}

type BatchQueueHandler struct {
	svc   *Service
	cache *Cache
	store *BatchStore
	hub   Broadcaster
}

func NewBatchQueueHandler(svc *Service, cache *Cache, store *BatchStore, hub Broadcaster) *BatchQueueHandler {
	return &BatchQueueHandler{svc: svc, cache: cache, store: store, hub: hub}
}

// Handle runs one generate_receipt_batch job: for each requested order it
// uses the cached PDF or renders a fresh one, skipping (and recording) orders
// that are unpaid/non-receiptable or fail to render. Successful PDFs are
// packed into a ZIP on disk. A skipped order is never fatal — the job only
// fails outright if the batch row can't be loaded or the ZIP can't be written.
func (h *BatchQueueHandler) Handle(ctx context.Context, payload []byte) error {
	var job GenerateReceiptBatchJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode receipt batch job: %w", err))
	}
	if job.BatchID == "" {
		return queue.Permanent(errors.New("receipt batch job: empty batch_id"))
	}

	batch, err := h.store.GetBatch(ctx, job.BatchID)
	if err != nil {
		return queue.Permanent(fmt.Errorf("load batch %s: %w", job.BatchID, err))
	}
	if err := h.store.MarkProcessing(ctx, job.BatchID); err != nil {
		return err
	}

	locale := resolveLocale(batch.Locale)
	type pdfEntry struct {
		name string
		pdf  []byte
	}
	var entries []pdfEntry
	errs := []BatchError{}

	for _, orderID := range batch.OrderIDs {
		order, err := h.svc.orderSvc.GetByID(ctx, orderID)
		if err != nil {
			errs = append(errs, BatchError{OrderID: orderID, Reason: "not_found"})
			continue
		}
		ref := orderRef(order)

		// Status-gate before the cache, mirroring serve(): a receipt cached while
		// the order was paid must not leak into the ZIP after it's cancelled/refunded.
		if !receiptableStatuses[order.Status] {
			errs = append(errs, BatchError{OrderID: orderID, OrderNumber: ref, Reason: "not_receiptable"})
			continue
		}

		pdf, err := h.cache.Get(orderID, locale)
		if err != nil {
			pdf, err = h.svc.GenerateForOrder(ctx, order, locale)
			if errors.Is(err, ErrNotReceiptable) {
				errs = append(errs, BatchError{OrderID: orderID, OrderNumber: ref, Reason: "not_receiptable"})
				continue
			}
			if err != nil {
				errs = append(errs, BatchError{OrderID: orderID, OrderNumber: ref, Reason: "generation_failed"})
				continue
			}
			// Warm the cache as a side effect so a re-run is fast.
			_ = h.cache.Put(orderID, locale, pdf)
		}
		entries = append(entries, pdfEntry{name: "Receipt-" + receiptFilenamePart(order) + ".pdf", pdf: pdf})
	}

	zipPath := ""
	if len(entries) > 0 {
		used := map[string]int{}
		var build = func(zw *zip.Writer) error {
			for _, e := range entries {
				name := e.name
				if n := used[name]; n > 0 {
					base := name[:len(name)-len(".pdf")]
					name = fmt.Sprintf("%s-%d.pdf", base, n+1)
				}
				used[e.name]++
				f, err := zw.Create(name)
				if err != nil {
					return err
				}
				if _, err := f.Write(e.pdf); err != nil {
					return err
				}
			}
			return nil
		}
		p, err := writeBatchZip(job.BatchID, build)
		if err != nil {
			return fmt.Errorf("write batch zip: %w", err)
		}
		zipPath = p
	}

	if err := h.store.CompleteBatch(ctx, job.BatchID, zipPath, len(entries), errs); err != nil {
		return err
	}

	if h.hub != nil {
		h.hub.Broadcast("receipt_batch_ready", map[string]any{"batch_id": job.BatchID})
	}
	return nil
}

// orderRef is the short human label for an order used in skip/error reports.
func orderRef(o *orders.Order) string {
	if o.OrderNumber != "" {
		return o.OrderNumber
	}
	return fmt.Sprintf("ORD-%d", o.Number)
}

// batchZipPath returns the on-disk path for a batch's ZIP, rejecting any id
// that could escape batchZipDir.
func batchZipPath(batchID string) (string, error) {
	if !safeOrderID(batchID) {
		return "", errInvalidOrderID
	}
	return filepath.Join(batchZipDir, batchID+".zip"), nil
}

// writeBatchZip builds a ZIP via fill and writes it atomically (temp + rename)
// so a crashed mid-build never leaves a half-written archive to serve.
func writeBatchZip(batchID string, fill func(*zip.Writer) error) (string, error) {
	dest, err := batchZipPath(batchID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(batchZipDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(batchZipDir, "batch-*.zip.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	zw := zip.NewWriter(tmp)
	if err := fill(zw); err != nil {
		zw.Close()
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	return dest, nil
}
