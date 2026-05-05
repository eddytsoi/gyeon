// Package abandoned recovers abandoned shopping carts by emailing the
// associated customer when the cart has been idle past the configured
// threshold. Designed to be triggered by an external cron / scheduler hitting
// POST /api/v1/admin/abandoned-cart/run; the same endpoint also surfaces the
// pending list for an at-a-glance admin review.
package abandoned

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/respond"
	"gyeon/backend/internal/settings"
)

type Candidate struct {
	CartID        string                    `json:"cart_id"`
	CustomerID    string                    `json:"customer_id"`
	CustomerEmail string                    `json:"customer_email"`
	CustomerName  string                    `json:"customer_name"`
	Subtotal      float64                   `json:"subtotal"`
	UpdatedAt     string                    `json:"updated_at"`
	Items         []email.AbandonedCartItem `json:"items"`
}

type Service struct {
	db       *sql.DB
	emailSvc *email.Service
	settings *settings.Service
}

func NewService(db *sql.DB, emailSvc *email.Service, s *settings.Service) *Service {
	return &Service{db: db, emailSvc: emailSvc, settings: s}
}

func (s *Service) thresholdHours(ctx context.Context) int {
	v, _ := s.settings.Get(ctx, "abandoned_cart_threshold_hours")
	if v != nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v.Value)); err == nil && n > 0 {
			return n
		}
	}
	return 24
}

func (s *Service) enabled(ctx context.Context) bool {
	v, _ := s.settings.Get(ctx, "abandoned_cart_enabled")
	return v != nil && strings.EqualFold(strings.TrimSpace(v.Value), "true")
}

// ListCandidates returns logged-in carts with items that have been idle past
// the threshold and have not yet received a reminder. Excludes carts that
// already produced a paid/processing/shipped/delivered order (to avoid sending
// reminders after checkout).
func (s *Service) ListCandidates(ctx context.Context) ([]Candidate, error) {
	thr := s.thresholdHours(ctx)
	cutoff := time.Now().Add(-time.Duration(thr) * time.Hour)

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.customer_id, cu.email,
		       COALESCE(NULLIF(TRIM(cu.first_name || ' ' || cu.last_name), ''), '') AS customer_name,
		       c.updated_at
		FROM carts c
		JOIN customers cu ON cu.id = c.customer_id
		WHERE c.customer_id IS NOT NULL
		  AND c.abandoned_email_sent_at IS NULL
		  AND c.updated_at <= $1
		  AND EXISTS (SELECT 1 FROM cart_items ci WHERE ci.cart_id = c.id)
		  AND NOT EXISTS (
		      SELECT 1 FROM orders o
		      WHERE o.cart_id = c.id
		        AND o.status IN ('paid','processing','shipped','delivered','refunded')
		  )
		ORDER BY c.updated_at ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Candidate, 0)
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.CartID, &c.CustomerID, &c.CustomerEmail, &c.CustomerName, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items, subtotal, err := s.cartContents(ctx, c.CartID)
		if err != nil {
			continue
		}
		c.Items = items
		c.Subtotal = subtotal
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) cartContents(ctx context.Context, cartID string) ([]email.AbandonedCartItem, float64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(p.name, ''), pv.price, ci.quantity
		FROM cart_items ci
		JOIN product_variants pv ON pv.id = ci.variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE ci.cart_id = $1
		ORDER BY ci.added_at ASC`, cartID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []email.AbandonedCartItem{}
	var subtotal float64
	for rows.Next() {
		var it email.AbandonedCartItem
		if err := rows.Scan(&it.Name, &it.UnitPrice, &it.Quantity); err != nil {
			return nil, 0, err
		}
		subtotal += it.UnitPrice * float64(it.Quantity)
		items = append(items, it)
	}
	return items, subtotal, rows.Err()
}

// Run scans for candidates and sends one reminder per cart. Returns how many
// emails were sent. Caller should check enabled() first or pass force=true.
func (s *Service) Run(ctx context.Context, force bool) (int, error) {
	if !force && !s.enabled(ctx) {
		return 0, nil
	}
	cands, err := s.ListCandidates(ctx)
	if err != nil {
		return 0, err
	}
	base := s.emailSvc.PublicBaseURL(ctx)
	sent := 0
	for _, c := range cands {
		if c.CustomerEmail == "" {
			continue
		}
		err := s.emailSvc.SendAbandonedCart(ctx, email.AbandonedCartParams{
			CustomerName:  c.CustomerName,
			CustomerEmail: c.CustomerEmail,
			Items:         c.Items,
			Subtotal:      c.Subtotal,
			Currency:      "HKD",
			ResumeURL:     fmt.Sprintf("%s/cart", base),
		})
		if err != nil {
			continue
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE carts SET abandoned_email_sent_at = NOW() WHERE id = $1`, c.CartID); err == nil {
			sent++
		}
	}
	return sent, nil
}

// ── HTTP handler ─────────────────────────────────────────────────────

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/run", h.run)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	cands, err := h.svc.ListCandidates(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, cands)
}

func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	sent, err := h.svc.Run(r.Context(), body.Force)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int{"sent": sent})
}
