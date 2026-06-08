package printnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/queue"
)

// PrintReceiptJob is the queue payload for remote-printing an order receipt.
// Force=true (manual reprint from the admin order page) bypasses the
// printnode_enabled auto-toggle; the auto-on-paid path leaves it false.
type PrintReceiptJob struct {
	OrderID string `json:"order_id"`
	Locale  string `json:"locale"`
	Force   bool   `json:"force,omitempty"`
}

// PDFProvider yields the receipt PDF bytes for an order+locale. Wired in
// main.go to "cache.Get, else GenerateForOrder + cache.Put" so this package
// stays decoupled from the receipt package (no import cycle).
type PDFProvider interface {
	ReceiptPDF(ctx context.Context, orderID, locale string) ([]byte, error)
}

// PDFProviderFunc adapts a plain func to PDFProvider.
type PDFProviderFunc func(ctx context.Context, orderID, locale string) ([]byte, error)

func (f PDFProviderFunc) ReceiptPDF(ctx context.Context, orderID, locale string) ([]byte, error) {
	return f(ctx, orderID, locale)
}

// OrderLookup is the slice of orders.OrderService used to build a print job
// title from the order number.
type OrderLookup interface {
	GetByID(ctx context.Context, id string) (*orders.Order, error)
}

type QueueHandler struct {
	client *Client
	pdfs   PDFProvider
	orders OrderLookup
}

func NewQueueHandler(client *Client, pdfs PDFProvider, orders OrderLookup) *QueueHandler {
	return &QueueHandler{client: client, pdfs: pdfs, orders: orders}
}

// Handle runs one print_receipt job: resolves the cached receipt PDF and
// submits it to PrintNode. Config errors are permanent (retry is pointless);
// rate-limit / 5xx / network errors are retried via the worker's backoff.
func (h *QueueHandler) Handle(ctx context.Context, payload []byte) error {
	var job PrintReceiptJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode print receipt job: %w", err))
	}
	if job.OrderID == "" {
		return queue.Permanent(errors.New("print receipt job: empty order_id"))
	}

	// Auto-print respects the enabled toggle; manual reprints (Force) don't.
	if !job.Force && !h.client.Enabled(ctx) {
		return nil
	}

	printerID := h.client.PrinterID(ctx)
	if printerID == 0 {
		return queue.Permanent(errors.New("print receipt job: printnode_printer_id not set"))
	}

	pdf, err := h.pdfs.ReceiptPDF(ctx, job.OrderID, job.Locale)
	if err != nil {
		// PDF not ready yet / transient render failure — let it retry. A
		// vanished order surfaces here too, but the few wasted retries are
		// cheaper than special-casing it.
		return fmt.Errorf("load receipt pdf for order %s: %w", job.OrderID, err)
	}

	if _, err := h.client.SubmitPDF(ctx, printerID, h.title(ctx, job.OrderID), pdf, h.client.Copies(ctx)); err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return queue.Permanent(err)
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 429 {
			// 4xx (bad printer id, malformed body, auth) — retry won't fix it.
			return queue.Permanent(err)
		}
		// 429 / 5xx / network — transient, retry with backoff.
		return err
	}
	return nil
}

func (h *QueueHandler) title(ctx context.Context, orderID string) string {
	if h.orders != nil {
		if o, err := h.orders.GetByID(ctx, orderID); err == nil && o != nil {
			if o.OrderNumber != "" {
				return "Receipt " + o.OrderNumber
			}
			return fmt.Sprintf("Receipt #%d", o.Number)
		}
	}
	return "Receipt"
}
