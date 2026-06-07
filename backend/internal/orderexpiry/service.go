// Package orderexpiry auto-cancels unpaid pending orders that have sat past
// their configured age, releasing the stock that was decremented at checkout.
//
// Stock is taken immediately when an order is placed (orders.Checkout), so an
// abandoned pending order locks inventory until someone cancels it. This sweep
// is the automatic cancel: it reuses orders.OrderService.UpdateStatus
// (pending → cancelled), which already restocks the items and records the
// transition in order_status_history + a customer-visible system notice.
//
// Card/Stripe and bank-transfer orders carry separate thresholds because a wire
// transfer legitimately takes days; a threshold of 0 disables that category.
// Driven both by a periodic ticker in main.go and by an admin "run now"
// endpoint (POST /api/v1/admin/pending-order-expiry/run).
package orderexpiry

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"strings"

	"gyeon/backend/internal/email"
	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/settings"
)

const (
	keyCardHours = "pending_order_expiry_hours"
	keyBankHours = "pending_order_expiry_bank_transfer_hours"

	// Fallback when the setting row is absent. 0 = disabled, so the feature is
	// inert until an admin sets a threshold — including the window between
	// deploying this code and applying migration 130 on a server. Recommended
	// values once enabled are 24h (card) / 168h (bank transfer).
	defaultCardHours = 0
	defaultBankHours = 0

	expiryNote = "逾時未付款，系統自動取消"
)

// EmailSender is the slice of email.QueueEnqueuer this service uses.
type EmailSender interface {
	PublicBaseURL(ctx context.Context) string
	SendOrderCancelledUnpaid(ctx context.Context, p email.OrderCancelledUnpaidParams) error
}

type Service struct {
	db       *sql.DB
	orders   *orders.OrderService
	payment  *payment.Service
	settings *settings.Service
	email    EmailSender
}

func NewService(db *sql.DB, ord *orders.OrderService, pay *payment.Service, s *settings.Service, em EmailSender) *Service {
	return &Service{db: db, orders: ord, payment: pay, settings: s, email: em}
}

// thresholdHours reads an hours setting. 0 (or negative) means the category is
// disabled; a missing/unparseable value falls back to def. NOTE: deliberately
// not settings.TTLHours — that treats 0 as "use the fallback", which would make
// it impossible to switch the feature off.
func (s *Service) thresholdHours(ctx context.Context, key string, def int) int {
	v, err := s.settings.Get(ctx, key)
	if err != nil || v == nil {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v.Value))
	if err != nil {
		return def
	}
	if n < 0 {
		return 0
	}
	return n
}

type staleOrder struct {
	id              string
	orderNumber     string
	paymentIntentID string
	paymentMethod   string
	customerEmail   string
	customerName    string
}

// listStale returns pending orders older than the per-category threshold.
// Bank-transfer orders (payment_method = 'bank_transfer') use bankH; everything
// else uses cardH. A category with hours <= 0 is excluded entirely.
func (s *Service) listStale(ctx context.Context, cardH, bankH int) ([]staleOrder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(order_number, ''), COALESCE(payment_intent_id, ''),
		       COALESCE(payment_method, ''), COALESCE(customer_email, ''), COALESCE(customer_name, '')
		FROM orders
		WHERE status = 'pending'
		  AND payment_status IS DISTINCT FROM 'succeeded'
		  AND ( (COALESCE(payment_method, '') <> 'bank_transfer'
		           AND $1 > 0 AND created_at < NOW() - make_interval(hours => $1::int))
		     OR (payment_method = 'bank_transfer'
		           AND $2 > 0 AND created_at < NOW() - make_interval(hours => $2::int)) )
		ORDER BY created_at ASC
		LIMIT 200`, cardH, bankH)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]staleOrder, 0)
	for rows.Next() {
		var o staleOrder
		if err := rows.Scan(&o.id, &o.orderNumber, &o.paymentIntentID, &o.paymentMethod, &o.customerEmail, &o.customerName); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Run cancels every stale unpaid pending order and returns how many were
// expired. Safe to call concurrently with the Stripe webhook: a race where the
// order is paid between our SELECT and our cancel is handled two ways — Stripe
// refuses to cancel a succeeded PaymentIntent (we skip), and UpdateStatus
// rejects a non-pending → cancelled transition (we skip).
func (s *Service) Run(ctx context.Context) (int, error) {
	cardH := s.thresholdHours(ctx, keyCardHours, defaultCardHours)
	bankH := s.thresholdHours(ctx, keyBankHours, defaultBankHours)
	if cardH <= 0 && bankH <= 0 {
		return 0, nil
	}

	cands, err := s.listStale(ctx, cardH, bankH)
	if err != nil {
		return 0, err
	}
	base := s.email.PublicBaseURL(ctx)

	expired := 0
	for _, o := range cands {
		// Cancel the Stripe PaymentIntent first so an unpaid card can't be
		// charged for an order we're about to drop. If Stripe refuses (the PI
		// already succeeded / is non-cancelable) leave the order pending — a
		// late webhook will mark it paid, and we must not restock a paid order.
		if o.paymentIntentID != "" {
			if err := s.payment.CancelPaymentIntent(ctx, o.paymentIntentID); err != nil {
				log.Printf("orderexpiry: skip order %s — cancel payment intent: %v", o.id, err)
				continue
			}
		}

		note := expiryNote
		if _, err := s.orders.UpdateStatus(ctx, o.id, orders.UpdateStatusRequest{
			Status: orders.StatusCancelled,
			Note:   &note,
		}); err != nil {
			// Most likely a concurrent webhook moved it out of pending.
			log.Printf("orderexpiry: skip order %s — cancel: %v", o.id, err)
			continue
		}
		expired++

		// Best-effort customer notification; a send failure must not abort the
		// sweep (the order is already cancelled + restocked).
		if o.customerEmail != "" {
			if err := s.email.SendOrderCancelledUnpaid(ctx, email.OrderCancelledUnpaidParams{
				OrderID:       o.id,
				OrderNumber:   o.orderNumber,
				CustomerName:  o.customerName,
				CustomerEmail: o.customerEmail,
				ResumeURL:     base + "/",
			}); err != nil {
				log.Printf("orderexpiry: order %s cancelled but email failed: %v", o.id, err)
			}
		}
	}
	return expired, nil
}
