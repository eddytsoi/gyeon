package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gyeon/backend/internal/queue"
)

// GenerateReceiptCacheJob is the queue payload for pre-warming a receipt
// PDF cache. Lives here so callers in main.go reference the canonical shape.
type GenerateReceiptCacheJob struct {
	OrderID string `json:"order_id"`
	Locale  string `json:"locale"`
}

// Broadcaster is the slice of admin.Hub the queue handler needs to notify
// admin SSE subscribers when a receipt cache becomes available. Decoupled
// to keep the import direction one-way.
type Broadcaster interface {
	Broadcast(eventType string, data any)
}

type QueueHandler struct {
	svc      *Service
	cache    *Cache
	hub      Broadcaster
	onCached func(ctx context.Context, orderID, locale string)
}

func NewQueueHandler(svc *Service, cache *Cache, hub Broadcaster) *QueueHandler {
	return &QueueHandler{svc: svc, cache: cache, hub: hub}
}

// SetOnCached registers a callback fired right after a receipt PDF is
// successfully written to the cache. Wired in main.go to enqueue a PrintNode
// print job — kept as a generic hook so this package stays unaware of
// printing (one-way dependency).
func (h *QueueHandler) SetOnCached(fn func(ctx context.Context, orderID, locale string)) {
	h.onCached = fn
}

// Handle runs one generate_receipt_cache job: loads the order, renders the
// PDF, writes it to the cache, then broadcasts an SSE event so any admin
// looking at the order page sees the lightning icon appear without reload.
func (h *QueueHandler) Handle(ctx context.Context, payload []byte) error {
	var job GenerateReceiptCacheJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode receipt cache job: %w", err))
	}
	if job.OrderID == "" {
		return queue.Permanent(errors.New("receipt cache job: empty order_id"))
	}
	locale := resolveLocale(job.Locale)

	if h.cache.Exists(job.OrderID, locale) {
		return nil
	}

	order, err := h.svc.orderSvc.GetByID(ctx, job.OrderID)
	if err != nil {
		// Order vanished between enqueue and processing (e.g. admin deleted
		// it). Nothing to cache; not a transient condition.
		return queue.Permanent(fmt.Errorf("load order %s: %w", job.OrderID, err))
	}

	pdf, err := h.svc.GenerateForOrder(ctx, order, locale)
	if err != nil {
		// Order moved to a non-receiptable status (cancelled/refunded) before
		// the worker picked the job up. Not retryable.
		if errors.Is(err, ErrNotReceiptable) {
			return nil
		}
		return err
	}

	if err := h.cache.Put(job.OrderID, locale, pdf); err != nil {
		return err
	}

	if h.hub != nil {
		h.hub.Broadcast("receipt_cache_ready", map[string]any{
			"order_id": job.OrderID,
			"locale":   locale,
		})
	}

	// PDF is now on disk — fire the post-cache hook (e.g. enqueue a PrintNode
	// remote-print job). Runs after Put so the print path is guaranteed a
	// ready file.
	if h.onCached != nil {
		h.onCached(ctx, job.OrderID, locale)
	}
	return nil
}
