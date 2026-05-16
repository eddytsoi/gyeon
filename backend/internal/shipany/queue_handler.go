package shipany

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	svc     *Service
	notices NoticeWriter
}

func NewQueueHandler(svc *Service, notices NoticeWriter) *QueueHandler {
	return &QueueHandler{svc: svc, notices: notices}
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

	var b strings.Builder
	b.WriteString("Shipany 已自動建單\n")
	if sh.TrackingNumber != nil && *sh.TrackingNumber != "" {
		fmt.Fprintf(&b, "運單號碼：%s\n", *sh.TrackingNumber)
	}
	if sh.Carrier != "" {
		if sh.Service != "" {
			fmt.Fprintf(&b, "貨運公司：%s（%s）\n", sh.Carrier, sh.Service)
		} else {
			fmt.Fprintf(&b, "貨運公司：%s\n", sh.Carrier)
		}
	}
	if sh.FeeHKD > 0 {
		fmt.Fprintf(&b, "運費：HKD %.2f\n", sh.FeeHKD)
	}
	if sh.LabelURL != nil && *sh.LabelURL != "" {
		fmt.Fprintf(&b, "標籤：%s\n", *sh.LabelURL)
	}
	h.note(ctx, job.OrderID, strings.TrimRight(b.String(), "\n"))
	return nil
}

func (h *QueueHandler) note(ctx context.Context, orderID, body string) {
	if h.notices == nil {
		return
	}
	_, _ = h.notices.CreateSystemNotice(ctx, orderID, body)
}
