package receipt

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// AdminRoutes registers GET /{id}/receipt.pdf for admin access. Mount under
// the admin auth group from main.go.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}/receipt.pdf", h.adminDownload)
	return r
}

// CustomerRoutes registers GET /{id}/receipt.pdf for the customer's own
// orders. Mount under the customer-auth group at a path like
// /api/v1/customers/me/orders so the final URL is
// /api/v1/customers/me/orders/{id}/receipt.pdf — keeps the URL alongside the
// existing customer order-detail endpoint without re-mounting /customers
// (which already has a handler).
func (h *Handler) CustomerRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}/receipt.pdf", h.customerDownload)
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

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, order *orders.Order) {
	locale := pickLocale(r)
	pdf, err := h.svc.GenerateForOrder(r.Context(), order, locale)
	if errors.Is(err, ErrNotReceiptable) {
		respond.Error(w, http.StatusConflict, "order is not in a receiptable status")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to generate receipt: "+err.Error())
		return
	}
	filename := "Receipt-" + receiptFilenamePart(order) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdf)))
	// Receipts can contain PII (name, address). Don't let intermediaries cache.
	w.Header().Set("Cache-Control", "private, no-store, max-age=0")
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
