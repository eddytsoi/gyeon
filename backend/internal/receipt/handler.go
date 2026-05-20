package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/queue"
	"gyeon/backend/internal/respond"
)

// Enqueuer is the slice of queue.Service the handler needs to schedule a
// regenerate. Mirrors the existing pattern (see email/enqueuer.go).
type Enqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...queue.EnqueueOptions) (string, error)
}

type Handler struct {
	svc      *Service
	cache    *Cache
	enqueuer Enqueuer
}

func NewHandler(svc *Service, cache *Cache, enqueuer Enqueuer) *Handler {
	return &Handler{svc: svc, cache: cache, enqueuer: enqueuer}
}

// AdminRoutes registers admin endpoints. Mount under the admin auth group
// from main.go.
//   GET  /{id}/receipt.pdf             — download (cache-first)
//   GET  /{id}/receipt-cache-status    — JSON {available: bool}
//   POST /{id}/receipt/regenerate      — clear cache + enqueue job
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}/receipt.pdf", h.adminDownload)
	r.Get("/{id}/receipt-cache-status", h.adminCacheStatus)
	r.Post("/{id}/receipt/regenerate", h.adminRegenerate)
	return r
}

// CustomerRoutes registers customer endpoints. Mount under the customer
// auth group so callers must present a valid customer JWT.
//   GET /{id}/receipt.pdf            — download (cache-first)
//   GET /{id}/receipt-cache-status   — JSON {available: bool}
func (h *Handler) CustomerRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}/receipt.pdf", h.customerDownload)
	r.Get("/{id}/receipt-cache-status", h.customerCacheStatus)
	return r
}

func (h *Handler) adminDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.svc.orderSvc.GetByID(r.Context(), id)
	if errors.Is(err, orders.ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	h.serve(w, r, order)
}

func (h *Handler) customerDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customerID := auth.CustomerIDFromContext(r.Context())
	order, err := h.svc.orderSvc.GetByIDForCustomer(r.Context(), id, customerID)
	if errors.Is(err, orders.ErrOrderNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	h.serve(w, r, order)
}

func (h *Handler) adminCacheStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.svc.orderSvc.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	h.writeCacheStatus(w, r, id)
}

func (h *Handler) customerCacheStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customerID := auth.CustomerIDFromContext(r.Context())
	if _, err := h.svc.orderSvc.GetByIDForCustomer(r.Context(), id, customerID); err != nil {
		if errors.Is(err, orders.ErrOrderNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	h.writeCacheStatus(w, r, id)
}

func (h *Handler) writeCacheStatus(w http.ResponseWriter, r *http.Request, orderID string) {
	locale := pickLocale(r)
	if locale == "" {
		locale = "en"
	}
	if !isKnownLocale(locale) {
		respond.BadRequest(w, "invalid locale")
		return
	}
	available := h.cache.Exists(orderID, locale)
	// Status responses are dynamic — don't let intermediaries cache them or
	// the lightning icon will lag behind reality.
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
	respond.JSON(w, http.StatusOK, map[string]any{
		"available": available,
		"locale":    resolveLocale(locale),
	})
}

func (h *Handler) adminRegenerate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	order, err := h.svc.orderSvc.GetByID(r.Context(), id)
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
	locale := pickLocale(r)
	if locale == "" {
		locale = "zh-Hant"
	}
	if !isKnownLocale(locale) {
		respond.BadRequest(w, "invalid locale")
		return
	}
	// Clear the existing cache so the icon disappears immediately on the
	// admin UI; the worker will write a fresh PDF and broadcast ready.
	_ = h.cache.DeleteForOrder(id)

	if h.enqueuer == nil {
		respond.Error(w, http.StatusServiceUnavailable, "queue not configured")
		return
	}
	payload, _ := json.Marshal(GenerateReceiptCacheJob{OrderID: id, Locale: resolveLocale(locale)})
	if _, err := h.enqueuer.Enqueue(r.Context(), queue.JobTypeGenerateReceiptCache, payload); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to enqueue regenerate job")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, order *orders.Order) {
	rawLocale := pickLocale(r)
	if rawLocale != "" && !isKnownLocale(rawLocale) {
		respond.BadRequest(w, "invalid locale")
		return
	}
	locale := resolveLocale(rawLocale)

	if !receiptableStatuses[order.Status] {
		respond.Error(w, http.StatusConflict, "order is not in a receiptable status")
		return
	}

	pdf, err := h.cache.Get(order.ID, locale)
	if err != nil {
		pdf, err = h.svc.GenerateForOrder(r.Context(), order, locale)
		if errors.Is(err, ErrNotReceiptable) {
			respond.Error(w, http.StatusConflict, "order is not in a receiptable status")
			return
		}
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "failed to generate receipt: "+err.Error())
			return
		}
		// Best-effort cache write — a write failure shouldn't fail the
		// in-flight download.
		_ = h.cache.Put(order.ID, locale, pdf)
	}

	filename := "Receipt-" + receiptFilenamePart(order) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdf)))
	// Receipts contain PII — never let a shared cache hold them, but allow
	// the user's own browser to cache briefly so a fast re-download hits
	// the disk cache rather than the server cache.
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(pdf)
}

func receiptFilenamePart(o *orders.Order) string {
	if o.OrderNumber != "" {
		// strip anything that's not [A-Za-z0-9_-] so we don't ship a weird
		// filename that browsers re-encode in surprising ways.
		var b strings.Builder
		for _, r := range o.OrderNumber {
			switch {
			case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '-', r == '_':
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			return b.String()
		}
	}
	return fmt.Sprintf("%d", o.Number)
}

// pickLocale resolves the receipt locale from (in order):
//  1. ?locale=… query string
//  2. Accept-Language header, taking the first tag
//
// Anything unrecognised falls back to English.
func pickLocale(r *http.Request) string {
	if v := strings.TrimSpace(r.URL.Query().Get("locale")); v != "" {
		return v
	}
	if v := r.Header.Get("Accept-Language"); v != "" {
		// "zh-Hant,en;q=0.9" → "zh-Hant"
		first := v
		if i := strings.IndexAny(first, ",;"); i >= 0 {
			first = first[:i]
		}
		return strings.TrimSpace(first)
	}
	return ""
}

// isKnownLocale rejects locales we don't have a label bundle for. Used at
// the boundary so a typo'd / hostile `?locale=` value never reaches the
// filesystem cache layer. We also accept the same aliases resolveLocale
// understands (`zh`, `zh-TW`, etc.) so admins can pass any tag the
// template supports.
func isKnownLocale(raw string) bool {
	switch raw {
	case "en", "zh-Hant", "zh-hant", "zh-TW", "zh-tw", "zh-HK", "zh-hk", "zh":
		return true
	}
	// Accept-Language may carry the bare default; pickLocale returns
	// "" for that and we treat empty as the fallback elsewhere.
	return raw == ""
}
