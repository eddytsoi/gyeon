package shipany

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"gyeon/backend/internal/orders"
)

// Per-order skip reasons reported back to the admin UI.
const (
	skipNotProcessing  = "not_processing"
	skipNoWaybill      = "no_waybill"
	skipNotFound       = "not_found"
	skipDownloadFailed = "download_failed"
)

// waybillHTTPClient downloads label PDFs from ShipAny's CDN. It is separate
// from the ShipAny API client — these are plain public-CDN GETs, not signed
// API calls.
var waybillHTTPClient = &http.Client{Timeout: 20 * time.Second}

type WaybillBatchSkip struct {
	OrderID     string `json:"order_id"`
	OrderNumber string `json:"order_number"`
	Reason      string `json:"reason"`
}

type WaybillBatchReport struct {
	Total          int                `json:"total"`
	SucceededCount int                `json:"succeeded_count"`
	Errors         []WaybillBatchSkip `json:"errors"`
}

// BuildWaybillBatch fetches the SF waybill PDF for each processing order and
// merges them into one PDF, preserving the caller's order. Per-order problems
// (not found, not processing, no waybill, download failure) are skipped and
// recorded in the report rather than failing the batch. The returned bytes are
// nil only when no order produced a waybill. A merge failure is fatal and
// returned as err.
func (s *Service) BuildWaybillBatch(ctx context.Context, orderIDs []string) ([]byte, WaybillBatchReport, error) {
	ids := dedupeNonEmpty(orderIDs)
	report := WaybillBatchReport{Total: len(ids), Errors: []WaybillBatchSkip{}}

	var pdfs [][]byte
	for _, id := range ids {
		order, err := s.orderSvc.GetByID(ctx, id)
		if err != nil {
			report.Errors = append(report.Errors, WaybillBatchSkip{OrderID: id, Reason: skipNotFound})
			continue
		}
		ref := order.OrderNumber
		if ref == "" {
			ref = fmt.Sprintf("%d", order.Number)
		}
		if order.Status != orders.StatusProcessing {
			report.Errors = append(report.Errors, WaybillBatchSkip{OrderID: id, OrderNumber: ref, Reason: skipNotProcessing})
			continue
		}
		sh, err := s.GetByOrderID(ctx, id)
		if err != nil || sh == nil || sh.LabelURL == nil || strings.TrimSpace(*sh.LabelURL) == "" {
			report.Errors = append(report.Errors, WaybillBatchSkip{OrderID: id, OrderNumber: ref, Reason: skipNoWaybill})
			continue
		}
		pdf, err := downloadWaybill(ctx, strings.TrimSpace(*sh.LabelURL))
		if err != nil {
			report.Errors = append(report.Errors, WaybillBatchSkip{OrderID: id, OrderNumber: ref, Reason: skipDownloadFailed})
			continue
		}
		pdfs = append(pdfs, pdf)
	}

	report.SucceededCount = len(pdfs)
	if len(pdfs) == 0 {
		return nil, report, nil
	}

	readers := make([]io.ReadSeeker, len(pdfs))
	for i, b := range pdfs {
		readers[i] = bytes.NewReader(b)
	}
	// Relaxed validation so third-party SF labels that don't perfectly match
	// the spec still merge instead of erroring the whole batch.
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	conf.ValidateLinks = false

	var buf bytes.Buffer
	if err := api.MergeRaw(readers, &buf, false, conf); err != nil {
		return nil, report, fmt.Errorf("merge waybill pdfs: %w", err)
	}
	return buf.Bytes(), report, nil
}

// downloadWaybill GETs a label PDF from ShipAny's CDN with a bounded body size.
func downloadWaybill(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := waybillHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("waybill download status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 20 MiB per label
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty waybill body")
	}
	return body, nil
}

// dedupeNonEmpty trims blanks and removes duplicate ids, preserving order.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
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
