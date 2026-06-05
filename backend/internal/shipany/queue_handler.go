package shipany

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/queue"
)

// CreateShipanyShipmentJob is the queue payload for the auto-create-shipment
// flow triggered on Order paid. Lives here so callers in main.go don't need
// to invent the shape locally.
type CreateShipanyShipmentJob struct {
	OrderID string `json:"order_id"`
}

// NoticeWriter is the slice of *orders.NoticeService the queue handler needs
// to record the API response as a system message on the order.
type NoticeWriter interface {
	CreateSystemNotice(ctx context.Context, orderID, body string) (*orders.Notice, error)
}

// QueueHandler runs the create_shipany_shipment job. It calls into the
// existing CreateForOrder path and writes a system notice on the order with
// the result so admins can see the outcome from the order timeline.
type QueueHandler struct {
	svc      *Service
	notices  NoticeWriter
	orderSvc *orders.OrderService
}

func NewQueueHandler(svc *Service, notices NoticeWriter, orderSvc *orders.OrderService) *QueueHandler {
	return &QueueHandler{svc: svc, notices: notices, orderSvc: orderSvc}
}

// Handle dispatches one shipment-creation job. Returns nil on success or for
// terminal errors (the system notice is the audit). Returns a non-permanent
// error only for transient ShipAny failures so the queue retries.
func (h *QueueHandler) Handle(ctx context.Context, payload []byte) error {
	var job CreateShipanyShipmentJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return queue.Permanent(fmt.Errorf("decode shipany job: %w", err))
	}
	if job.OrderID == "" {
		return queue.Permanent(errors.New("shipany job: empty order_id"))
	}

	if !h.svc.Configured(ctx) {
		h.note(ctx, job.OrderID, "Shipany 自動建單已停用（shipany_enabled 或憑證未設定）。")
		return nil
	}

	sh, err := h.svc.CreateForOrder(ctx, job.OrderID, nil)
	if err != nil {
		switch {
		case errors.Is(err, ErrShipmentExists):
			h.note(ctx, job.OrderID, "Shipany 已有現存單號，跳過自動建單。")
			return nil
		case errors.Is(err, ErrCarrierNotSelected):
			h.note(ctx, job.OrderID, "未選擇貨運公司，未能自動建單。")
			return nil
		case errors.Is(err, ErrNotConfigured):
			h.note(ctx, job.OrderID, "Shipany 未完成設定，未能自動建單。")
			return nil
		}
		// Transient failure: write a note for the admin and let the queue
		// retry. The note is overwritten on a later success because each
		// attempt is appended; that's fine for forensics.
		h.note(ctx, job.OrderID, fmt.Sprintf("Shipany 建單失敗：%v", err))
		return err
	}

	h.note(ctx, job.OrderID, shipmentNoticeBody("Shipany 已自動建單", sh))

	// Advance paid → processing once the shipment exists. Only fire when the
	// order is still in `paid`: ShipAny status webhooks may already have
	// pushed it further (shipped/delivered), and the state machine rejects
	// re-entering processing from there. Best-effort: a transition failure
	// shouldn't fail the queue job — the shipment has already been created.
	if h.orderSvc != nil {
		if current, err := h.orderSvc.GetByID(ctx, job.OrderID); err == nil &&
			current.Status == orders.StatusPaid {
			if _, err := h.orderSvc.UpdateStatus(ctx, job.OrderID, orders.UpdateStatusRequest{
				Status: orders.StatusProcessing,
			}); err != nil {
				log.Printf("shipany: advance order %s to processing: %v", job.OrderID, err)
			}
		}
	}
	return nil
}

func (h *QueueHandler) note(ctx context.Context, orderID, body string) {
	if h.notices == nil {
		return
	}
	_, _ = h.notices.CreateSystemNotice(ctx, orderID, body)
}
