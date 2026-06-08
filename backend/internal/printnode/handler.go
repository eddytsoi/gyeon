package printnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/queue"
	"gyeon/backend/internal/respond"
)

// Enqueuer is the slice of queue.Service the handler needs to schedule a
// print job. Mirrors receipt.Enqueuer.
type Enqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...queue.EnqueueOptions) (string, error)
}

// receiptableStatuses mirrors receipt.receiptableStatuses — the statuses for
// which a receipt (and therefore a printable PDF) exists.
var receiptableStatuses = map[orders.OrderStatus]bool{
	orders.StatusPaid:       true,
	orders.StatusProcessing: true,
	orders.StatusShipped:    true,
	orders.StatusDelivered:  true,
}

type Handler struct {
	client   *Client
	enqueuer Enqueuer
	orders   OrderLookup
}

func NewHandler(client *Client, enqueuer Enqueuer, orders OrderLookup) *Handler {
	return &Handler{client: client, enqueuer: enqueuer, orders: orders}
}

// AdminRoutes registers admin endpoints. Mount under the admin auth group.
//
//	GET  /printers              — list PrintNode printers (for the picker)
//	POST /test-print            — print a one-page test document
//	POST /orders/{id}/print     — manually (re)print an order receipt
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/printers", h.listPrinters)
	r.Post("/test-print", h.testPrint)
	r.Post("/orders/{id}/print", h.printOrder)
	return r
}

func (h *Handler) listPrinters(w http.ResponseWriter, r *http.Request) {
	printers, err := h.client.ListPrinters(r.Context())
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			respond.Error(w, http.StatusBadRequest, "PrintNode API key is not set")
			return
		}
		respond.Error(w, http.StatusBadGateway, "PrintNode: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"printers": printers})
}

func (h *Handler) testPrint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PrinterID int `json:"printer_id"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body)

	printerID := body.PrinterID
	if printerID == 0 {
		printerID = h.client.PrinterID(r.Context())
	}
	if printerID == 0 {
		respond.Error(w, http.StatusBadRequest, "no printer selected")
		return
	}

	jobID, err := h.client.SubmitPDF(r.Context(), printerID, "GYEON PrintNode test", testPagePDF(), 1)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			respond.Error(w, http.StatusBadRequest, "PrintNode API key is not set")
			return
		}
		respond.Error(w, http.StatusBadGateway, "PrintNode: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"job_id": jobID})
}

func (h *Handler) printOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.orders.GetByID(r.Context(), id)
	if errors.Is(err, orders.ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if !receiptableStatuses[order.Status] {
		respond.Error(w, http.StatusConflict, "order is not in a receiptable status")
		return
	}
	// Early, friendly error for the common misconfiguration instead of a
	// silently-stuck queue job.
	if !h.client.Configured(r.Context()) {
		respond.Error(w, http.StatusBadRequest, "PrintNode is not configured (set API key and printer)")
		return
	}
	if h.enqueuer == nil {
		respond.Error(w, http.StatusServiceUnavailable, "queue not configured")
		return
	}

	// Manual reprint: Force bypasses the auto-enabled toggle. zh-Hant matches
	// the storefront/receipt default.
	payload, _ := json.Marshal(PrintReceiptJob{OrderID: id, Locale: "zh-Hant", Force: true})
	if _, err := h.enqueuer.Enqueue(r.Context(), queue.JobTypePrintReceipt, payload); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to enqueue print job")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// testPagePDF builds a minimal, valid single-page PDF for the test-print
// button. Built programmatically so the cross-reference offsets are always
// correct regardless of the content length.
func testPagePDF() []byte {
	content := "BT /F1 16 Tf 36 150 Td (GYEON PrintNode test page) Tj " +
		"0 -28 Td /F1 11 Tf (If you can read this, remote printing works.) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xrefPos := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xrefPos)
	return buf.Bytes()
}
