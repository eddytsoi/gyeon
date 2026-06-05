package orders

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/customers"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/payment"
	"gyeon/backend/internal/pricing"
	"gyeon/backend/internal/shop"
	"gyeon/backend/internal/tax"
	"gyeon/backend/internal/util"
)

type OrderStatus string

const (
	StatusPending    OrderStatus = "pending"
	StatusPaid       OrderStatus = "paid"
	StatusProcessing OrderStatus = "processing"
	StatusShipped    OrderStatus = "shipped"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
	StatusRefunded   OrderStatus = "refunded"
)

// Bank-transfer payment constants. Bank transfer is the offline method offered
// only to installer / installer_v2 customers: no Stripe PaymentIntent is
// created, the order sits on hold until an admin confirms the wire, and the
// cart is cleared at checkout (there is no webhook to do it later).
const (
	PaymentMethodBankTransfer = "bank_transfer"
	// PaymentStatusAwaitingBankTransfer is distinct from the Stripe
	// 'requires_payment_method' state so a bank-transfer order is never confused
	// with an awaiting-card order and is never matched by a Stripe webhook
	// lookup (it has no payment_intent_id). It is IS DISTINCT FROM 'succeeded',
	// so the pending-order resume logic still treats it as unpaid.
	PaymentStatusAwaitingBankTransfer = "awaiting_bank_transfer"
)

// OrderAppliedPromotion is the per-order snapshot of one campaign or coupon
// that contributed to discount_amount. Kept on the order so the success
// page / account order detail can show "why" the discount was applied even
// after the underlying campaign or coupon is edited or removed.
type OrderAppliedPromotion struct {
	Kind        string  `json:"kind"` // "campaign" | "coupon"
	ID          string  `json:"id"`
	Name        string  `json:"name"` // campaign name OR coupon code
	Description *string `json:"description,omitempty"`
	Amount      float64 `json:"amount"`
}

// buildAppliedPromotions converts a pricing.DiscountResult into the order's
// snapshot shape. Order: campaigns in the order they applied, then the
// coupon (if any).
func buildAppliedPromotions(d pricing.DiscountResult) []OrderAppliedPromotion {
	out := make([]OrderAppliedPromotion, 0, len(d.AppliedCampaigns)+1)
	for _, c := range d.AppliedCampaigns {
		out = append(out, OrderAppliedPromotion{
			Kind:        "campaign",
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Amount:      c.Amount,
		})
	}
	if d.AppliedCoupon != nil {
		out = append(out, OrderAppliedPromotion{
			Kind:        "coupon",
			ID:          d.AppliedCoupon.ID,
			Name:        d.AppliedCoupon.Code,
			Description: d.AppliedCoupon.Description,
			Amount:      d.AppliedCoupon.Amount,
		})
	}
	return out
}

// marshalAppliedPromotions serialises the snapshot for the JSONB column.
// Always returns a valid JSON array ("[]" when empty) so the orders table's
// CHECK constraint never trips.
func marshalAppliedPromotions(promos []OrderAppliedPromotion) []byte {
	if len(promos) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(promos)
	if err != nil {
		// json.Marshal on a slice of structs cannot fail; fall back to [] so
		// we never violate the CHECK constraint.
		return []byte("[]")
	}
	return b
}

// scanAppliedPromotions decodes a JSONB column read into []byte. Tolerates
// NULL / empty input by returning an empty slice — keeps imported / pre-
// migration orders renderable instead of erroring out.
func scanAppliedPromotions(raw []byte) []OrderAppliedPromotion {
	if len(raw) == 0 {
		return nil
	}
	var out []OrderAppliedPromotion
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

type Order struct {
	ID                string                  `json:"id"`
	Number            int64                   `json:"number"`
	OrderNumber       string                  `json:"order_number"`
	CustomerID        *string                 `json:"customer_id,omitempty"`
	Status            OrderStatus             `json:"status"`
	ShippingAddressID *string                 `json:"shipping_address_id,omitempty"`
	ShippingAddress   *ShippingAddress        `json:"shipping_address,omitempty"`
	Subtotal          float64                 `json:"subtotal"`
	ShippingFee       float64                 `json:"shipping_fee"`
	ShippingFree      bool                    `json:"shipping_free"`
	DiscountAmount    float64                 `json:"discount_amount"`
	AppliedPromotions []OrderAppliedPromotion `json:"applied_promotions"`
	TaxAmount         float64                 `json:"tax_amount"`
	Total             float64                 `json:"total"`
	Notes             *string                 `json:"notes,omitempty"`
	CustomerEmail     *string                 `json:"customer_email,omitempty"`
	CustomerPhone     *string                 `json:"customer_phone,omitempty"`
	CustomerName      *string                 `json:"customer_name,omitempty"`
	PaymentIntentID   *string                 `json:"payment_intent_id,omitempty"`
	PaymentStatus     *string                 `json:"payment_status,omitempty"`
	PaymentMethod     *string                 `json:"payment_method,omitempty"`
	CardBrand         *string                 `json:"card_brand,omitempty"`
	CardLast4         *string                 `json:"card_last4,omitempty"`
	PaidAt            *string                 `json:"paid_at,omitempty"`
	RefundAmount      float64                 `json:"refund_amount"`
	RefundReason      *string                 `json:"refund_reason,omitempty"`
	RefundedAt        *string                 `json:"refunded_at,omitempty"`
	StripeRefundID    *string                 `json:"stripe_refund_id,omitempty"`
	SelectedCarrier   *string                 `json:"selected_carrier,omitempty"`
	SelectedService   *string                 `json:"selected_service,omitempty"`
	PickupPointID     *string                 `json:"pickup_point_id,omitempty"`
	PickupPointLabel  *string                 `json:"pickup_point_label,omitempty"`
	Items             []OrderItem             `json:"items,omitempty"`
	ItemsCount        *int                    `json:"items_count,omitempty"`
	CustomerRole      *string                 `json:"customer_role,omitempty"`
	CreatedAt         string                  `json:"created_at"`
	UpdatedAt         string                  `json:"updated_at"`
}

// ShippingAddress is the snapshot of the shipping address attached to an order.
// Populated by GetByID for the admin order detail view.
type ShippingAddress struct {
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	Phone      *string `json:"phone,omitempty"`
	Line1      string  `json:"line1"`
	Line2      *string `json:"line2,omitempty"`
	City       string  `json:"city"`
	State      *string `json:"state,omitempty"`
	PostalCode string  `json:"postal_code"`
	Country    string  `json:"country"`
}

type OrderItem struct {
	ID              string                 `json:"id"`
	OrderID         string                 `json:"order_id"`
	VariantID       *string                `json:"variant_id,omitempty"`
	ParentItemID    *string                `json:"parent_item_id,omitempty"` // set for bundle component rows
	ProductName     string                 `json:"product_name"`
	ProductSubtitle *string                `json:"product_subtitle,omitempty"`
	VariantSKU      string                 `json:"variant_sku"`
	VariantAttrs    map[string]interface{} `json:"variant_attrs,omitempty"`
	UnitPrice       float64                `json:"unit_price"`
	Quantity        int                    `json:"quantity"`
	LineTotal       float64                `json:"line_total"`
}

type CustomerInfo struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type ShippingAddressInput struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type CheckoutRequest struct {
	CartID            string                `json:"cart_id"`
	CustomerID        *string               `json:"customer_id"`
	CustomerInfo      *CustomerInfo         `json:"customer_info,omitempty"`
	ShippingAddressID *string               `json:"shipping_address_id,omitempty"`
	ShippingAddress   *ShippingAddressInput `json:"shipping_address,omitempty"`
	SaveAddress       bool                  `json:"save_address,omitempty"`
	ShippingFee       float64               `json:"shipping_fee"`
	CouponCode        *string               `json:"coupon_code"`
	Notes             *string               `json:"notes"`
	// ShipAny delivery selection (optional). Populated when the storefront
	// surfaces live rate quotes; null when ShipAny is disabled.
	SelectedCarrier  *string `json:"selected_carrier,omitempty"`
	SelectedService  *string `json:"selected_service,omitempty"`
	PickupPointID    *string `json:"pickup_point_id,omitempty"`
	PickupPointLabel *string `json:"pickup_point_label,omitempty"`
	// SendPaymentLink, when true, triggers a "complete payment" email containing
	// a magic link for the customer to finish Stripe payment in their browser.
	// Set internally by callers that have no Stripe.js (e.g. MCP); never read
	// from JSON so REST clients cannot trigger spam.
	SendPaymentLink bool `json:"-"`
	// VerifiedRole is the storefront role proven by the customer's auth token
	// (cookie/Bearer), set by the HTTP handler from request context — never read
	// from JSON. Empty for guests / unverified callers. It is the *sole*
	// authority for choosing the payment method, so a spoofed body CustomerID
	// can't unlock bank transfer: installer / installer_v2 ⇒ bank transfer,
	// everyone else ⇒ Stripe.
	VerifiedRole string `json:"-"`
}

type CheckoutResult struct {
	Order          *Order `json:"order"`
	ClientSecret   string `json:"client_secret"`
	PublishableKey string `json:"publishable_key"`
	Mode           string `json:"mode"`
}

type UpdateStatusRequest struct {
	Status OrderStatus `json:"status"`
	Note   *string     `json:"note"`
}

var ErrEmptyCart = errors.New("cart is empty")
var ErrCustomerInfoRequired = errors.New("customer_info is required for guest checkout")
var ErrShippingRequired = errors.New("shipping_address or shipping_address_id is required")
var ErrOrderNotFound = errors.New("order not found")
var ErrDefaultCarrierNotConfigured = errors.New("default carrier or service is not configured")

// valid forward transitions
var allowedTransitions = map[OrderStatus][]OrderStatus{
	StatusPending:    {StatusPaid, StatusCancelled},
	StatusPaid:       {StatusProcessing, StatusRefunded},
	StatusProcessing: {StatusShipped, StatusCancelled},
	StatusShipped:    {StatusDelivered},
	StatusDelivered:  {StatusRefunded},
	StatusCancelled:  {},
	StatusRefunded:   {},
}

// AuditRecorder is the minimal interface this service needs from the audit
// package. Decoupled to avoid an import cycle.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry)
}

type AuditEntry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

// EmailSender is the slice of email.Service the orders package needs.
// Both *email.Service and *email.QueueEnqueuer satisfy this so callers can
// pick the sync or queued implementation without changing this file.
type EmailSender interface {
	PublicBaseURL(ctx context.Context) string
	SendPaymentLink(ctx context.Context, p email.PaymentLinkParams) error
	SendBankTransferOnHold(ctx context.Context, p email.BankTransferOnHoldParams) error
	SendOrderConfirmation(ctx context.Context, p email.OrderEmailParams) error
	SendOrderShipped(ctx context.Context, p email.ShippedEmailParams) error
	SendOrderRefunded(ctx context.Context, p email.RefundEmailParams) error
	SendLowStockAlert(ctx context.Context, p email.LowStockParams) error
}

// ReceiptCacheInvalidator is the slice of *receipt.Cache the order service
// calls when an order mutates in a way that makes any cached PDF receipt
// stale (refund / delete). Decoupled to a local interface so this package
// doesn't take an import on receipt (which itself depends on orders).
type ReceiptCacheInvalidator interface {
	DeleteForOrder(orderID string) error
}

type OrderService struct {
	db           *sql.DB
	cartSvc      *CartService
	pricingSvc   *pricing.Service
	customerSvc  *customers.Service
	paymentSvc   *payment.Service
	emailSvc     EmailSender
	taxSvc       *tax.Service
	audit        AuditRecorder
	onCreated    func(ctx context.Context, order *Order)
	onPaid       func(ctx context.Context, order *Order)
	onShipped    func(ctx context.Context, order *Order)
	receiptCache ReceiptCacheInvalidator
}

// SetAudit wires an optional audit recorder. Call from main during setup.
func (s *OrderService) SetAudit(rec AuditRecorder) { s.audit = rec }

func (s *OrderService) record(ctx context.Context, action, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: "order", EntityID: entityID,
		Before: before, After: after,
	})
}

// SetOnOrderPaid registers a callback fired after an order's payment_intent
// has been confirmed and the order has flipped to status=paid. Used by
// loyalty (P3 #24) to credit points without an import-cycle dependency.
func (s *OrderService) SetOnOrderPaid(fn func(context.Context, *Order)) {
	s.onPaid = fn
}

// SetOnOrderCreated registers a callback fired after a new order is committed
// (best-effort, non-blocking). Used for SSE broadcasts to admin clients.
func (s *OrderService) SetOnOrderCreated(fn func(context.Context, *Order)) {
	s.onCreated = fn
}

// SetOnOrderShipped registers a callback fired after an order transitions
// 處理中 → 已發貨 (processing → shipped). Used to post the SF Express tracking
// notice without an import-cycle dependency on the shipany package.
func (s *OrderService) SetOnOrderShipped(fn func(context.Context, *Order)) {
	s.onShipped = fn
}

// SetReceiptCache wires the receipt cache invalidator. When set, the
// service deletes any cached receipt PDFs for an order at the points
// where its content would diverge from a previously generated receipt
// (admin delete, status transition to refunded, full refund).
func (s *OrderService) SetReceiptCache(c ReceiptCacheInvalidator) {
	s.receiptCache = c
}

// invalidateReceiptCache best-effort removes any cached receipt for orderID.
// Failures are logged and swallowed — invalidation is hygiene, not safety:
// a stale cache only means the next download serves a slightly outdated
// PDF, which is still better than failing the order operation.
func (s *OrderService) invalidateReceiptCache(orderID string) {
	if s.receiptCache == nil {
		return
	}
	if err := s.receiptCache.DeleteForOrder(orderID); err != nil {
		log.Printf("invalidate receipt cache for order %s: %v", orderID, err)
	}
}

// SetTaxService wires an optional tax calculator. When unset, orders skip the
// tax line entirely.
// recordInventoryHistory writes one inventory_history row. Failures are
// logged and swallowed so an audit-write blip doesn't break an order. delta
// of 0 is skipped.
func (s *OrderService) recordInventoryHistory(ctx context.Context, variantID string, before, after int, reason string, orderID *string) {
	if before == after {
		return
	}
	delta := after - before
	var orderIDArg any
	if orderID != nil && *orderID != "" {
		orderIDArg = *orderID
	}
	// Customer-driven checkouts have no admin actor; AdminIDFromContext returns
	// false and we leave actor_user_id NULL.
	var actorIDArg any
	if id, ok := auth.AdminIDFromContext(ctx); ok {
		actorIDArg = id
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO inventory_history (variant_id, delta, before_qty, after_qty, reason, actor_user_id, order_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		variantID, delta, before, after, reason, actorIDArg, orderIDArg,
	); err != nil {
		log.Printf("inventory_history: variant=%s reason=%s: %v", variantID, reason, err)
	}
}

// restockOrderItemsTx restocks every order_items row for the order inside the
// caller's transaction and writes one inventory_history row per variant. Used
// when an order is cancelled or fully refunded so the stock deducted at
// checkout is returned. Skips rows where variant_id has been NULLed (variant
// deleted after the order was placed). Caller must ensure restock is only
// invoked once per order to avoid double-counting.
func (s *OrderService) restockOrderItemsTx(ctx context.Context, tx *sql.Tx, orderID, reason string, note *string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT variant_id, quantity FROM order_items WHERE order_id = $1 AND variant_id IS NOT NULL`, orderID)
	if err != nil {
		return err
	}
	type item struct {
		variantID string
		quantity  int
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.variantID, &it.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var actorIDArg any
	if id, ok := auth.AdminIDFromContext(ctx); ok {
		actorIDArg = id
	}
	var noteArg any
	if note != nil && *note != "" {
		noteArg = *note
	}

	for _, it := range items {
		var before int
		if err := tx.QueryRowContext(ctx,
			`SELECT stock_qty FROM product_variants WHERE id = $1 FOR UPDATE`, it.variantID).Scan(&before); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // variant deleted — skip
			}
			return err
		}
		after := before + it.quantity
		if _, err := tx.ExecContext(ctx,
			`UPDATE product_variants SET stock_qty = $1, updated_at = NOW() WHERE id = $2`,
			after, it.variantID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO inventory_history (variant_id, delta, before_qty, after_qty, reason, actor_user_id, order_id, note)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			it.variantID, it.quantity, before, after, reason, actorIDArg, orderID, noteArg); err != nil {
			return err
		}
	}
	return nil
}

// freeShippingThresholdKeys picks the (enabled, amount) site_settings keys
// for the given customer role. Installer (施工店) and installer_v2 (施工店_v2)
// each have their own pair so admins can offer a different free-shipping bar
// without touching the default that applies to guests + role=customer.
func freeShippingThresholdKeys(role string) (enabledKey, amountKey string) {
	switch role {
	case customers.RoleInstaller:
		return "free_shipping_threshold_installer_enabled", "free_shipping_threshold_installer_hkd"
	case customers.RoleInstallerV2:
		return "free_shipping_threshold_installer_v2_enabled", "free_shipping_threshold_installer_v2_hkd"
	default:
		return "free_shipping_threshold_enabled", "free_shipping_threshold_hkd"
	}
}

// freeShippingThresholdHKD reads the admin-configured threshold (P3 #29)
// for the given customer role. Returns 0 when disabled or unparseable,
// which the caller treats as "always charge shipping_fee as quoted".
func (s *OrderService) freeShippingThresholdHKD(ctx context.Context, role string) float64 {
	_, amountKey := freeShippingThresholdKeys(role)
	var raw string
	if err := s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = $1`, amountKey,
	).Scan(&raw); err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// shippingFreeFor mirrors the shipany free/COD decision: the merchant
// absorbs SF Express shipping only when the role-specific threshold
// feature is enabled, the configured threshold is > 0, and the
// post-discount subtotal meets it. Used at checkout to freeze the
// outcome onto orders.shipping_free.
//
// Each role's threshold is independent — a disabled or zero installer
// threshold does NOT fall back to the default. Guests (no logged-in
// customer) use the default ("customer") threshold.
func (s *OrderService) shippingFreeFor(ctx context.Context, role string, subtotalAfterDiscount float64) bool {
	enabledKey, _ := freeShippingThresholdKeys(role)
	var enabledRaw string
	_ = s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = $1`, enabledKey,
	).Scan(&enabledRaw)
	if strings.TrimSpace(enabledRaw) != "true" {
		return false
	}
	threshold := s.freeShippingThresholdHKD(ctx, role)
	return threshold > 0 && subtotalAfterDiscount >= threshold
}

// ShippingLabel renders the SF carrier label for an order based on the
// frozen-at-checkout shipping_free flag. Locale "zh-Hant" returns the
// Traditional Chinese label; everything else returns English.
func ShippingLabel(o *Order, locale string) string {
	if locale == "zh-Hant" {
		if o.ShippingFree {
			return "順豐速運（免運費）"
		}
		return "順豐速運（到付）"
	}
	if o.ShippingFree {
		return "SF Express (free)"
	}
	return "SF Express (pay on delivery)"
}

func (s *OrderService) SetTaxService(t *tax.Service) {
	s.taxSvc = t
}

// orderNumberPrefix reads the configurable prefix from site_settings,
// falling back to "ORD" so old data and admins who haven't touched
// settings still get a sensible default.
func (s *OrderService) orderNumberPrefix(ctx context.Context) string {
	var v string
	_ = s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = 'order_number_prefix'`).Scan(&v)
	if v == "" {
		return "ORD"
	}
	return v
}

// resolveDefaultShipping reads the admin-configured default courier + service
// from site_settings. Every storefront checkout is stamped with these values
// so the auto-create-shipment job has what it needs without asking the
// customer to choose a carrier.
//
// Returns enabled=false when shipany_enabled is not "true" — caller should
// then place the order with NULL carrier/service (legacy behavior). Returns
// ErrDefaultCarrierNotConfigured when shipany is enabled but either default
// key is blank.
func (s *OrderService) resolveDefaultShipping(ctx context.Context) (enabled bool, carrier, service string, err error) {
	rows, qerr := s.db.QueryContext(ctx,
		`SELECT key, value FROM site_settings
		 WHERE key IN ('shipany_enabled', 'shipany_default_courier', 'shipany_default_service')`)
	if qerr != nil {
		return false, "", "", qerr
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return false, "", "", err
		}
		switch k {
		case "shipany_enabled":
			enabled = strings.EqualFold(strings.TrimSpace(v), "true")
		case "shipany_default_courier":
			carrier = strings.TrimSpace(v)
		case "shipany_default_service":
			service = strings.TrimSpace(v)
		}
	}
	if err := rows.Err(); err != nil {
		return false, "", "", err
	}
	if !enabled {
		return false, "", "", nil
	}
	if carrier == "" || service == "" {
		return true, "", "", ErrDefaultCarrierNotConfigured
	}
	return true, carrier, service, nil
}

func NewOrderService(
	db *sql.DB,
	cartSvc *CartService,
	pricingSvc *pricing.Service,
	customerSvc *customers.Service,
	paymentSvc *payment.Service,
	emailSvc EmailSender,
) *OrderService {
	return &OrderService{
		db:          db,
		cartSvc:     cartSvc,
		pricingSvc:  pricingSvc,
		customerSvc: customerSvc,
		paymentSvc:  paymentSvc,
		emailSvc:    emailSvc,
	}
}

func (s *OrderService) Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	cart, err := s.cartSvc.GetByID(ctx, req.CartID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, ErrEmptyCart
	}

	// Resolve customer: existing logged-in id, or upsert guest by email.
	customerID := req.CustomerID
	customerEmail := ""
	customerPhone := ""
	customerName := ""
	// Guests + role=customer share the default free-shipping threshold;
	// installers have their own. Default to RoleCustomer for guests.
	customerRole := customers.RoleCustomer
	// isGuest gates promotion eligibility (allow_guests vs allowed_roles).
	// Stays true unless we successfully resolve an existing customer.
	isGuest := true

	if customerID != nil && *customerID != "" {
		c, err := s.customerSvc.GetByID(ctx, *customerID)
		if err == nil {
			customerEmail = c.Email
			customerName = c.FirstName + " " + c.LastName
			if c.Phone != nil {
				customerPhone = *c.Phone
			}
			customerRole = customers.NormalizeRole(c.Role)
			isGuest = false
		}
		// Form-supplied customer_info overrides for this order's snapshot
		if req.CustomerInfo != nil {
			if req.CustomerInfo.Email != "" {
				customerEmail = req.CustomerInfo.Email
			}
			if req.CustomerInfo.Phone != "" {
				customerPhone = req.CustomerInfo.Phone
			}
			if req.CustomerInfo.FirstName != "" || req.CustomerInfo.LastName != "" {
				customerName = strings.TrimSpace(req.CustomerInfo.FirstName + " " + req.CustomerInfo.LastName)
			}
		}
	} else {
		if req.CustomerInfo == nil || req.CustomerInfo.Email == "" {
			return nil, ErrCustomerInfoRequired
		}
		var phonePtr *string
		if req.CustomerInfo.Phone != "" {
			p := req.CustomerInfo.Phone
			phonePtr = &p
		}
		c, _, err := s.customerSvc.UpsertGuest(ctx,
			req.CustomerInfo.Email,
			req.CustomerInfo.FirstName,
			req.CustomerInfo.LastName,
			phonePtr,
		)
		if err != nil {
			return nil, fmt.Errorf("upsert guest: %w", err)
		}
		customerID = &c.ID
		customerEmail = c.Email
		customerPhone = req.CustomerInfo.Phone
		customerName = strings.TrimSpace(req.CustomerInfo.FirstName + " " + req.CustomerInfo.LastName)
	}

	// Choose the payment method from the *verified* role only (set by the HTTP
	// handler from the auth token), never from the body-derived customerRole —
	// otherwise a guest spoofing an installer's customer_id could place a no-pay
	// bank-transfer order. installer / installer_v2 ⇒ bank transfer (offline,
	// on hold); everyone else (customer / guest / unverified) ⇒ Stripe.
	useBankTransfer := req.VerifiedRole == customers.RoleInstaller || req.VerifiedRole == customers.RoleInstallerV2

	// Resolve shipping address: existing id, or insert new
	shippingAddressID := req.ShippingAddressID
	if (shippingAddressID == nil || *shippingAddressID == "") && req.ShippingAddress != nil {
		country := req.ShippingAddress.Country
		if country == "" {
			country = "HK"
		}
		var line2, state *string
		if req.ShippingAddress.Line2 != "" {
			v := req.ShippingAddress.Line2
			line2 = &v
		}
		if req.ShippingAddress.State != "" {
			v := req.ShippingAddress.State
			state = &v
		}
		var phonePtr *string
		if customerPhone != "" {
			p := customerPhone
			phonePtr = &p
		}

		// Split name back into first/last for the address row
		first, last := splitName(customerName)

		// Dedup-on-write: reuse an existing matching saved address instead of
		// adding a near-identical row to the customer's address book on every
		// checkout.
		addrID, err := customers.FindOrCreateAddress(ctx, s.db, customerID, customers.AddressFields{
			FirstName:  first,
			LastName:   last,
			Phone:      phonePtr,
			Line1:      req.ShippingAddress.Line1,
			Line2:      line2,
			City:       req.ShippingAddress.City,
			State:      state,
			PostalCode: req.ShippingAddress.PostalCode,
			Country:    country,
		}, false)
		if err != nil {
			return nil, fmt.Errorf("find or create address: %w", err)
		}
		shippingAddressID = &addrID
	}

	if shippingAddressID == nil || *shippingAddressID == "" {
		return nil, ErrShippingRequired
	}

	// Fetch line item info (outside the transaction so we can compute discounts first)
	type bundleComponent struct {
		variantID   string
		productName string
		sku         string
		price       float64
		quantity    int // component quantity × cart item quantity
	}
	type lineItem struct {
		variantID   string
		productID   string
		categoryID  *string
		productName string
		subtitle    sql.NullString
		sku         string
		price       float64
		quantity    int
		kind        string
		components  []bundleComponent // populated for bundle items
	}

	var lines []lineItem
	var subtotal float64
	for _, item := range cart.Items {
		var li lineItem
		li.variantID = item.VariantID
		li.quantity = item.Quantity

		var variantName sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT pv.sku, pv.price, pv.product_id, p.category_id, p.name, p.subtitle, pv.name, p.kind
			 FROM product_variants pv
			 JOIN products p ON p.id = pv.product_id
			 WHERE pv.id = $1`, item.VariantID).
			Scan(&li.sku, &li.price, &li.productID, &li.categoryID, &li.productName, &li.subtitle, &variantName, &li.kind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("variant %s not found", item.VariantID)
		}
		if err != nil {
			return nil, err
		}
		// Bake the variant suffix ("500ml", "L / 紅") into the persisted
		// product_name so cart/checkout/email/order detail all show the same
		// combined label without needing a runtime lookup. Bundles have no
		// meaningful variant of their own — keep them bare.
		if li.kind != "bundle" {
			li.productName = shop.ProductDisplayName(li.productName, variantName.String)
		}

		// For bundle products, load components for later stock decrement and child order items.
		if li.kind == "bundle" {
			compRows, err := s.db.QueryContext(ctx,
				`SELECT bi.component_variant_id, p.name, pv.name, pv.sku, pv.price, bi.quantity
				 FROM bundle_items bi
				 JOIN product_variants pv ON pv.id = bi.component_variant_id
				 JOIN products p ON p.id = pv.product_id
				 WHERE bi.bundle_product_id = $1
				 ORDER BY bi.sort_order ASC`, li.productID)
			if err != nil {
				return nil, err
			}
			for compRows.Next() {
				var bc bundleComponent
				var compVariantName sql.NullString
				var compQty int
				if err := compRows.Scan(&bc.variantID, &bc.productName, &compVariantName, &bc.sku, &bc.price, &compQty); err != nil {
					compRows.Close()
					return nil, err
				}
				bc.productName = shop.ProductDisplayName(bc.productName, compVariantName.String)
				bc.quantity = compQty * item.Quantity // scale by cart qty
				li.components = append(li.components, bc)
			}
			compRows.Close()
			if err := compRows.Err(); err != nil {
				return nil, err
			}
		}

		subtotal += li.price * float64(li.quantity)
		lines = append(lines, li)
	}

	// Compute discounts before opening the transaction
	var discountResult pricing.DiscountResult
	if s.pricingSvc != nil {
		pricingItems := make([]pricing.LineItem, len(lines))
		for i, li := range lines {
			pricingItems[i] = pricing.LineItem{
				VariantID:  li.variantID,
				ProductID:  li.productID,
				CategoryID: li.categoryID,
				Price:      li.price,
				Quantity:   li.quantity,
			}
		}
		discountResult, err = s.pricingSvc.ComputeDiscount(ctx, pricingItems, subtotal, req.CouponCode, customerRole, isGuest)
		if err != nil {
			return nil, err
		}
	}

	discountAmount := discountResult.TotalDiscount
	taxableAmount := subtotal - discountAmount
	if taxableAmount < 0 {
		taxableAmount = 0
	}

	var taxAmount float64
	if s.taxSvc != nil {
		taxRes := s.taxSvc.Calculate(ctx, taxableAmount)
		taxAmount = taxRes.TaxAmount
		// Inclusive pricing: tax is back-calculated from displayed price; total stays put.
		// Exclusive pricing: tax adds on top of subtotal-discount before shipping.
		if !taxRes.Inclusive {
			taxableAmount += taxAmount
		}
	}

	// P3 #29 — free shipping threshold. Server-side enforcement so a tampered
	// client can't bypass it (and so the value stays correct even if the
	// storefront fee preview is slightly stale). We also freeze the outcome
	// onto the order so the receipt / account page / email can render the
	// correct SF carrier label months later even if the threshold settings
	// change in the meantime.
	shippingFree := s.shippingFreeFor(ctx, customerRole, subtotal-discountAmount)
	shippingFee := req.ShippingFee
	if shippingFree {
		shippingFee = 0
	}

	total := taxableAmount + shippingFee
	if total < 0 {
		total = 0
	}

	// Idempotency by cart: a cart holds at most one in-flight order. If this
	// cart already has a pending, not-yet-paid order, don't spawn a duplicate
	// (covers: shopper closes the checkout tab after a failed payment and
	// reopens cart/checkout in a new tab — same localStorage session → same
	// cart_id). Reuse the existing order when its contents still match; if the
	// cart changed since, cancel the stale order (which restocks) and create a
	// fresh one below.
	if existing, derr := s.findPendingOrderByCart(ctx, req.CartID); derr != nil {
		return nil, derr
	} else if existing != nil {
		want := map[string]int{}
		for _, li := range lines {
			want[li.variantID] += li.quantity
		}
		// Resume only when the existing order's payment method still matches the
		// current shopper's required one. If the role (and therefore the method)
		// changed since — e.g. a customer was promoted to installer between
		// attempts — the stale Stripe/bank order is superseded and a fresh one in
		// the correct method is created below.
		existingIsBankTransfer := existing.PaymentMethod != nil && *existing.PaymentMethod == PaymentMethodBankTransfer
		if existingIsBankTransfer == useBankTransfer && pendingOrderMatchesCart(existing, want, total) {
			return s.resumePendingOrder(ctx, existing, shippingAddressID, customerEmail, customerPhone, customerName, req.Notes)
		}
		note := "Superseded — cart or payment method changed before payment"
		if _, cerr := s.UpdateStatus(ctx, existing.ID, UpdateStatusRequest{Status: StatusCancelled, Note: &note}); cerr != nil {
			return nil, fmt.Errorf("cancel superseded order %s: %w", existing.ID, cerr)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Decrement stock atomically.
	// For bundle products: decrement each component's stock (not the bundle variant's own stock_qty).
	// For simple products: decrement the variant stock directly.
	// Track (variantID, qty, before, after) for post-commit low-stock alerts +
	// inventory_history rows.
	type stockDec struct {
		variantID string
		quantity  int
		before    int
		after     int
	}
	var stockDecs []stockDec
	deductOne := func(variantID string, qty int) error {
		var before, after int
		// RETURNING captures the post-update qty atomically; we derive before
		// from after+qty since the WHERE clause guaranteed enough stock.
		err := tx.QueryRowContext(ctx,
			`UPDATE product_variants SET stock_qty = stock_qty - $2
			 WHERE id = $1 AND stock_qty >= $2
			 RETURNING stock_qty`, variantID, qty).Scan(&after)
		if err == sql.ErrNoRows {
			return fmt.Errorf("insufficient stock for variant %s", variantID)
		}
		if err != nil {
			return err
		}
		before = after + qty
		stockDecs = append(stockDecs, stockDec{variantID, qty, before, after})
		return nil
	}
	for _, li := range lines {
		if li.kind == "bundle" {
			for _, bc := range li.components {
				if err := deductOne(bc.variantID, bc.quantity); err != nil {
					return nil, err
				}
			}
		} else {
			if err := deductOne(li.variantID, li.quantity); err != nil {
				return nil, err
			}
		}
	}

	var emailPtr, phonePtr, namePtr *string
	if customerEmail != "" {
		emailPtr = &customerEmail
	}
	if customerPhone != "" {
		phonePtr = &customerPhone
	}
	if customerName != "" {
		namePtr = &customerName
	}

	// Carrier + service are sourced from admin defaults — the storefront no
	// longer asks the customer to choose. Pickup-point columns stay NULL by
	// default; admin can still override carrier + service per order from the
	// order detail page. When shipany is disabled, the order is placed with
	// NULL carrier/service (legacy behavior).
	shipanyOn, defaultCarrier, defaultService, err := s.resolveDefaultShipping(ctx)
	if err != nil {
		return nil, err
	}
	var carrierPtr, servicePtr *string
	if shipanyOn {
		carrierPtr = &defaultCarrier
		servicePtr = &defaultService
	}

	appliedPromos := buildAppliedPromotions(discountResult)
	appliedJSON := marshalAppliedPromotions(appliedPromos)

	// Stripe orders start awaiting a card with the method set later by the
	// webhook; bank-transfer orders record the method up front and sit in the
	// distinct 'awaiting_bank_transfer' state with no PaymentIntent.
	paymentStatusInit := "requires_payment_method"
	var paymentMethodPtr *string
	if useBankTransfer {
		paymentStatusInit = PaymentStatusAwaitingBankTransfer
		m := PaymentMethodBankTransfer
		paymentMethodPtr = &m
	}

	// Freeze the shipping address onto the order (ship_* snapshot) so later
	// edits/deletes to the customer's address book never alter this order.
	shipSnap, err := s.loadShipSnapshot(ctx, shippingAddressID)
	if err != nil {
		return nil, fmt.Errorf("load shipping snapshot: %w", err)
	}

	var order Order
	err = tx.QueryRowContext(ctx,
		`INSERT INTO orders (customer_id, shipping_address_id, subtotal, shipping_fee, shipping_free, discount_amount, applied_promotions, tax_amount, total, notes,
		                     customer_email, customer_phone, customer_name, payment_status, payment_method,
		                     selected_carrier, selected_service, pickup_point_id, pickup_point_label, cart_id,
		                     ship_first_name, ship_last_name, ship_phone, ship_line1, ship_line2, ship_city, ship_state, ship_postal_code, ship_country)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
		         $21, $22, $23, $24, $25, $26, $27, $28, $29)
		 RETURNING id, number, customer_id, status, shipping_address_id, subtotal, shipping_fee, shipping_free, discount_amount, tax_amount, total, notes,
		           customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		           selected_carrier, selected_service, pickup_point_id, pickup_point_label,
		           created_at, updated_at`,
		append([]any{
			customerID, shippingAddressID, subtotal, shippingFee, shippingFree, discountAmount, appliedJSON, taxAmount, total, req.Notes,
			emailPtr, phonePtr, namePtr, paymentStatusInit, paymentMethodPtr,
			carrierPtr, servicePtr, nil, nil, req.CartID,
		}, shipSnap.args()...)...).
		Scan(&order.ID, &order.Number, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.ShippingFree, &order.DiscountAmount, &order.TaxAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.SelectedCarrier, &order.SelectedService, &order.PickupPointID, &order.PickupPointLabel,
			&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		// Concurrency backstop: the partial unique index
		// orders_one_pending_per_cart rejects a second pending order for the
		// same cart. If a concurrent checkout won the race, roll back (undoes
		// this tx's stock decrement) and reuse the winning order instead of
		// erroring out.
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "orders_one_pending_per_cart" {
			tx.Rollback()
			existing, ferr := s.findPendingOrderByCart(ctx, req.CartID)
			if ferr != nil {
				return nil, ferr
			}
			if existing != nil {
				return s.resumePendingOrder(ctx, existing, shippingAddressID, customerEmail, customerPhone, customerName, req.Notes)
			}
		}
		return nil, err
	}
	order.AppliedPromotions = appliedPromos

	// Format the customer-facing order number using the configurable
	// prefix and the auto-assigned sequential `number`. Persist it back
	// to the row so subsequent reads are stable.
	order.OrderNumber = fmt.Sprintf("%s-%04d", s.orderNumberPrefix(ctx), order.Number)
	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET order_number = $2 WHERE id = $1`,
		order.ID, order.OrderNumber); err != nil {
		return nil, err
	}

	// Insert order items. For bundles, insert a parent row then child rows per component.
	for _, li := range lines {
		lineTotal := li.price * float64(li.quantity)
		var item OrderItem
		err := tx.QueryRowContext(ctx,
			`INSERT INTO order_items (order_id, variant_id, product_name, product_subtitle, variant_sku, unit_price, quantity, line_total)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, order_id, variant_id, product_name, product_subtitle, variant_sku, unit_price, quantity, line_total`,
			order.ID, li.variantID, li.productName, li.subtitle, li.sku, li.price, li.quantity, lineTotal).
			Scan(&item.ID, &item.OrderID, &item.VariantID, &item.ProductName, &item.ProductSubtitle,
				&item.VariantSKU, &item.UnitPrice, &item.Quantity, &item.LineTotal)
		if err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)

		// For bundle products, insert child order items linked via parent_item_id.
		for _, bc := range li.components {
			var child OrderItem
			childTotal := bc.price * float64(bc.quantity)
			cerr := tx.QueryRowContext(ctx,
				`INSERT INTO order_items (order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total, parent_item_id)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				 RETURNING id, order_id, variant_id, product_name, variant_sku, unit_price, quantity, line_total`,
				order.ID, bc.variantID, bc.productName, bc.sku, bc.price, bc.quantity, childTotal, item.ID).
				Scan(&child.ID, &child.OrderID, &child.VariantID, &child.ProductName,
					&child.VariantSKU, &child.UnitPrice, &child.Quantity, &child.LineTotal)
			if cerr != nil {
				return nil, cerr
			}
			parentID := item.ID
			child.ParentItemID = &parentID
			order.Items = append(order.Items, child)
		}
	}

	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`, order.ID, StatusPending)

	// Mirror the audit row as a system notice so the user-visible timeline
	// starts cleanly on day one. Best-effort — failure here shouldn't break
	// checkout (the audit row is the source of truth).
	pending := StatusPending
	_ = CreateSystemNoticeTx(ctx, tx, order.ID, &pending, "Order placed")

	if discountResult.CouponID != nil {
		if err := pricing.IncrementCouponUsage(ctx, tx, *discountResult.CouponID); err != nil {
			return nil, err
		}
	}

	// NOTE: cart is NOT cleared here. Orders start in 'pending' until the
	// Stripe webhook fires payment_intent.succeeded; the cart is only
	// emptied at that point so an abandoned payment leaves the cart intact
	// for the customer to retry. See MarkPaidByPaymentIntent below.

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if len(stockDecs) > 0 {
		decs := make([]lowStockDec, len(stockDecs))
		for i, d := range stockDecs {
			decs[i] = lowStockDec{VariantID: d.variantID, Quantity: d.quantity}
		}
		go s.checkLowStockCrossings(context.Background(), decs)

		// Inventory history (P2 #23): one row per deducted variant, linked to
		// the order. actor is NULL because checkout is customer-driven.
		orderIDStr := order.ID
		for _, d := range stockDecs {
			s.recordInventoryHistory(ctx, d.variantID, d.before, d.after, "order.checkout", &orderIDStr)
		}
	}

	if s.onCreated != nil {
		go s.onCreated(context.Background(), &order)
	}

	result := &CheckoutResult{Order: &order}
	if useBankTransfer {
		// Bank transfer is offline: no PaymentIntent, no client_secret. The order
		// stays on hold (status=pending, payment_status=awaiting_bank_transfer)
		// until an admin confirms the wire. There is no Stripe webhook to clear
		// the cart later, so do it now (mirrors MarkPaidByPaymentIntent), and
		// email the customer the transfer instructions.
		if _, err := s.db.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, req.CartID); err != nil {
			log.Printf("clear cart for bank-transfer order %s: %v", order.ID, err)
		}
		s.sendBankTransferOnHoldEmail(ctx, &order)
		return result, nil
	}

	// Create Stripe PaymentIntent (outside the order tx — Stripe is the source of truth for the PI;
	// if this fails after the order is committed, the order can be retried/cancelled separately).
	if s.paymentSvc != nil {
		intent, err := s.paymentSvc.CreatePaymentIntent(ctx, payment.CreateIntentParams{
			AmountCents: int64(total*100 + 0.5), // round to nearest cent
			Currency:    "hkd",
			OrderID:     order.ID,
			Email:       customerEmail,
		})
		if err != nil {
			log.Printf("create payment intent for order %s: %v", order.ID, err)
			return nil, fmt.Errorf("payment setup failed: %w", err)
		}

		_, err = s.db.ExecContext(ctx,
			`UPDATE orders SET payment_intent_id=$2 WHERE id=$1`, order.ID, intent.ID)
		if err != nil {
			log.Printf("persist payment_intent_id on order %s: %v", order.ID, err)
		}
		order.PaymentIntentID = &intent.ID
		result.ClientSecret = intent.ClientSecret
		result.PublishableKey = s.paymentSvc.PublishableKey(ctx)
		result.Mode = s.paymentSvc.Mode(ctx)

		if req.SendPaymentLink && customerEmail != "" && s.emailSvc != nil {
			s.sendPaymentLinkEmail(ctx, &order, intent.ClientSecret)
		}
	}

	return result, nil
}

// findPendingOrderByCart returns the most recent pending, not-yet-paid order
// for a cart (with items loaded), or (nil, nil) when none exists. Used to dedup
// checkout so reopening cart/checkout in a new tab continues the same order
// instead of creating a clone.
func (s *OrderService) findPendingOrderByCart(ctx context.Context, cartID string) (*Order, error) {
	if cartID == "" {
		return nil, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM orders
		 WHERE cart_id = $1 AND status = 'pending'
		   AND payment_status IS DISTINCT FROM 'succeeded'
		 ORDER BY created_at DESC
		 LIMIT 1`, cartID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// pendingOrderMatchesCart reports whether an existing pending order still
// represents the same purchase as the current cart: identical top-level
// (variant → quantity) lines and the same computed total (within a cent).
// Bundle component rows (parent_item_id set) are skipped — they're derived
// from their parent line.
func pendingOrderMatchesCart(order *Order, want map[string]int, total float64) bool {
	have := map[string]int{}
	for _, it := range order.Items {
		if it.ParentItemID != nil {
			continue
		}
		if it.VariantID == nil {
			return false
		}
		have[*it.VariantID] += it.Quantity
	}
	if len(have) != len(want) {
		return false
	}
	for v, q := range want {
		if have[v] != q {
			return false
		}
	}
	diff := order.Total - total
	if diff < 0 {
		diff = -diff
	}
	return diff <= 0.01
}

// resumePendingOrder refreshes an existing pending order for a repeat checkout
// of the same cart: it updates the contact/address snapshot from the new
// payload, resets a failed payment back to payable, ensures a live Stripe
// client_secret, and returns the same CheckoutResult shape — without creating a
// new order or decrementing stock again.
func (s *OrderService) resumePendingOrder(ctx context.Context, order *Order, shippingAddressID *string, email, phone, name string, notes *string) (*CheckoutResult, error) {
	var emailPtr, phonePtr, namePtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}
	if name != "" {
		namePtr = &name
	}

	// Bank-transfer orders have no Stripe PaymentIntent and must never be flipped
	// to a Stripe payment_status. Caller only reaches here when the existing
	// order's method matches the current shopper, so refresh the snapshot, keep
	// it on hold, and return without touching Stripe.
	if order.PaymentMethod != nil && *order.PaymentMethod == PaymentMethodBankTransfer {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE orders SET
			    shipping_address_id = COALESCE($2, shipping_address_id),
			    customer_email = COALESCE($3, customer_email),
			    customer_phone = COALESCE($4, customer_phone),
			    customer_name  = COALESCE($5, customer_name),
			    notes = COALESCE($6, notes),
			    updated_at = now()
			 WHERE id = $1`,
			order.ID, shippingAddressID, emailPtr, phonePtr, namePtr, notes); err != nil {
			return nil, fmt.Errorf("refresh pending bank-transfer order %s: %w", order.ID, err)
		}
		if err := s.refreshOrderShipSnapshot(ctx, order.ID, shippingAddressID); err != nil {
			return nil, fmt.Errorf("refresh bank-transfer order ship snapshot %s: %w", order.ID, err)
		}
		return &CheckoutResult{Order: order}, nil
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE orders SET
		    shipping_address_id = COALESCE($2, shipping_address_id),
		    customer_email = COALESCE($3, customer_email),
		    customer_phone = COALESCE($4, customer_phone),
		    customer_name  = COALESCE($5, customer_name),
		    notes = COALESCE($6, notes),
		    payment_status = CASE WHEN payment_status = 'succeeded'
		                          THEN payment_status ELSE 'requires_payment_method' END,
		    updated_at = now()
		 WHERE id = $1`,
		order.ID, shippingAddressID, emailPtr, phonePtr, namePtr, notes); err != nil {
		return nil, fmt.Errorf("refresh pending order %s: %w", order.ID, err)
	}
	if err := s.refreshOrderShipSnapshot(ctx, order.ID, shippingAddressID); err != nil {
		return nil, fmt.Errorf("refresh order ship snapshot %s: %w", order.ID, err)
	}
	rps := "requires_payment_method"
	order.PaymentStatus = &rps

	result := &CheckoutResult{Order: order}
	if s.paymentSvc == nil {
		return result, nil
	}

	clientSecret := ""
	if order.PaymentIntentID != nil && *order.PaymentIntentID != "" {
		if pi, err := s.paymentSvc.FetchPaymentIntent(ctx, *order.PaymentIntentID); err == nil && pi.ClientSecret != "" {
			clientSecret = pi.ClientSecret
		}
	}
	if clientSecret == "" {
		intent, err := s.paymentSvc.CreatePaymentIntent(ctx, payment.CreateIntentParams{
			AmountCents: int64(order.Total*100 + 0.5),
			Currency:    "hkd",
			OrderID:     order.ID,
			Email:       email,
		})
		if err != nil {
			return nil, fmt.Errorf("payment setup failed: %w", err)
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE orders SET payment_intent_id=$2 WHERE id=$1`, order.ID, intent.ID); err != nil {
			log.Printf("persist payment_intent_id on resumed order %s: %v", order.ID, err)
		}
		order.PaymentIntentID = &intent.ID
		clientSecret = intent.ClientSecret
	}

	result.ClientSecret = clientSecret
	result.PublishableKey = s.paymentSvc.PublishableKey(ctx)
	result.Mode = s.paymentSvc.Mode(ctx)
	return result, nil
}

// PendingOrderForCartResult is the storefront payload that lets cart/checkout
// surface a "you have an unpaid order" banner linking to /pay/{id}.
type PendingOrderForCartResult struct {
	OrderID      string  `json:"order_id"`
	OrderNumber  string  `json:"order_number"`
	Total        float64 `json:"total"`
	ClientSecret string  `json:"client_secret"`
}

// PendingOrderForCart returns the cart's outstanding pending order (with a
// fresh Stripe client_secret) so the storefront can offer a resume-payment
// link, or (nil, nil) when the cart has no payable unpaid order. Authorized by
// possession of the cart_id, consistent with the rest of the cart API.
func (s *OrderService) PendingOrderForCart(ctx context.Context, cartID string) (*PendingOrderForCartResult, error) {
	order, err := s.findPendingOrderByCart(ctx, cartID)
	if err != nil || order == nil {
		return nil, err
	}
	if s.paymentSvc == nil || order.PaymentIntentID == nil || *order.PaymentIntentID == "" {
		return nil, nil
	}
	pi, err := s.paymentSvc.FetchPaymentIntent(ctx, *order.PaymentIntentID)
	if err != nil || pi.ClientSecret == "" {
		return nil, nil
	}
	return &PendingOrderForCartResult{
		OrderID:      order.ID,
		OrderNumber:  order.OrderNumber,
		Total:        order.Total,
		ClientSecret: pi.ClientSecret,
	}, nil
}

// buildOrderEmailItems converts an order's flat OrderItem slice into the
// nested OrderEmailItem shape, attaching bundle child rows under their
// parent so email templates can render the bundle's contents indented.
func buildOrderEmailItems(items []OrderItem) []email.OrderEmailItem {
	toEmailItem := func(it OrderItem) email.OrderEmailItem {
		subtitle := ""
		if it.ProductSubtitle != nil {
			subtitle = *it.ProductSubtitle
		}
		return email.OrderEmailItem{
			Name:      it.ProductName,
			Subtitle:  subtitle,
			SKU:       it.VariantSKU,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			LineTotal: it.LineTotal,
		}
	}
	// Index parents by ID so we can attach children by parent_item_id.
	parentIdx := map[string]int{}
	out := make([]email.OrderEmailItem, 0, len(items))
	for _, it := range items {
		if it.ParentItemID == nil {
			parentIdx[it.ID] = len(out)
			out = append(out, toEmailItem(it))
		}
	}
	for _, it := range items {
		if it.ParentItemID == nil {
			continue
		}
		if idx, ok := parentIdx[*it.ParentItemID]; ok {
			out[idx].Children = append(out[idx].Children, toEmailItem(it))
		}
	}
	return out
}

// sendPaymentLinkEmail emails the customer a magic link to complete the
// Stripe payment for an MCP-initiated pending order. Best-effort.
func (s *OrderService) sendPaymentLinkEmail(ctx context.Context, order *Order, clientSecret string) {
	if order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}
	base := s.emailSvc.PublicBaseURL(ctx)
	paymentURL := fmt.Sprintf("%s/pay/%s?cs=%s", base, order.ID, url.QueryEscape(clientSecret))

	items := buildOrderEmailItems(order.Items)
	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}
	err := s.emailSvc.SendPaymentLink(ctx, email.PaymentLinkParams{
		OrderID:       order.ID,
		OrderNumber:   order.OrderNumber,
		CustomerName:  name,
		CustomerEmail: *order.CustomerEmail,
		Items:         items,
		Total:         order.Total,
		Currency:      "HKD",
		PaymentURL:    paymentURL,
	})
	if err != nil {
		log.Printf("send payment link email for order %s: %v", order.ID, err)
	}
}

// PaymentInfoResult is the public payload returned to the customer-facing
// /pay/{id} page so they can mount a Stripe Element and finish payment.
type PaymentInfoResult struct {
	Order          *Order `json:"order"`
	ClientSecret   string `json:"client_secret"`
	PublishableKey string `json:"publishable_key"`
	Mode           string `json:"mode"`
	Currency       string `json:"currency"`
}

var ErrPaymentLinkInvalid = errors.New("invalid payment link")
var ErrPaymentLinkExpired = errors.New("payment already completed or order is no longer payable")

// PaymentInfo validates a magic-link `cs` query against the order's stored
// PaymentIntent and returns the data the /pay page needs. The cs is the
// Stripe client_secret in the form `pi_XXX_secret_YYY`; we authorize the
// caller by checking that pi_XXX matches the order's payment_intent_id.
func (s *OrderService) PaymentInfo(ctx context.Context, orderID, clientSecret string) (*PaymentInfoResult, error) {
	if clientSecret == "" {
		return nil, ErrPaymentLinkInvalid
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentIntentID == nil || *order.PaymentIntentID == "" {
		return nil, ErrPaymentLinkInvalid
	}
	idx := strings.Index(clientSecret, "_secret_")
	if idx <= 0 {
		return nil, ErrPaymentLinkInvalid
	}
	if clientSecret[:idx] != *order.PaymentIntentID {
		return nil, ErrPaymentLinkInvalid
	}
	if order.Status != StatusPending {
		return nil, ErrPaymentLinkExpired
	}
	if order.PaymentStatus != nil && *order.PaymentStatus == "succeeded" {
		return nil, ErrPaymentLinkExpired
	}

	// Strip PII from the order before returning to the (already-authorized but
	// link-shareable) caller. The link holder knows their own order, but we
	// avoid echoing fields they didn't already submit.
	safe := *order
	safe.CustomerEmail = nil
	safe.CustomerPhone = nil
	safe.CustomerID = nil
	safe.ShippingAddressID = nil
	safe.Notes = nil

	pubKey := ""
	mode := ""
	if s.paymentSvc != nil {
		pubKey = s.paymentSvc.PublishableKey(ctx)
		mode = s.paymentSvc.Mode(ctx)
	}
	return &PaymentInfoResult{
		Order:          &safe,
		ClientSecret:   clientSecret,
		PublishableKey: pubKey,
		Mode:           mode,
		Currency:       "HKD",
	}, nil
}

// PaymentInfoForCustomer is the owner-authenticated counterpart of PaymentInfo:
// it returns the data the /pay page needs for a logged-in customer's own
// pending order WITHOUT requiring them to already hold the `cs` magic-link.
// Ownership is enforced via GetByIDForCustomer; the client_secret is fetched
// fresh from Stripe (the DB only stores payment_intent_id). Used by the account
// order page's "立即付款" button.
func (s *OrderService) PaymentInfoForCustomer(ctx context.Context, orderID, customerID string) (*PaymentInfoResult, error) {
	order, err := s.GetByIDForCustomer(ctx, orderID, customerID)
	if err != nil {
		return nil, err
	}
	if order.PaymentIntentID == nil || *order.PaymentIntentID == "" {
		return nil, ErrPaymentLinkInvalid
	}
	if order.Status != StatusPending {
		return nil, ErrPaymentLinkExpired
	}
	if order.PaymentStatus != nil && *order.PaymentStatus == "succeeded" {
		return nil, ErrPaymentLinkExpired
	}
	if s.paymentSvc == nil {
		return nil, ErrPaymentLinkInvalid
	}
	pi, err := s.paymentSvc.FetchPaymentIntent(ctx, *order.PaymentIntentID)
	if err != nil {
		return nil, err
	}
	if pi.ClientSecret == "" {
		return nil, ErrPaymentLinkInvalid
	}

	safe := *order
	safe.CustomerEmail = nil
	safe.CustomerPhone = nil
	safe.CustomerID = nil
	safe.ShippingAddressID = nil
	safe.Notes = nil

	return &PaymentInfoResult{
		Order:          &safe,
		ClientSecret:   pi.ClientSecret,
		PublishableKey: s.paymentSvc.PublishableKey(ctx),
		Mode:           s.paymentSvc.Mode(ctx),
		Currency:       "HKD",
	}, nil
}

// SetupTokenResult is returned to the customer-facing /checkout/success page
// so it can offer a "Create account" CTA wired to a one-time setup-password
// link, skipping the generic registration form.
type SetupTokenResult struct {
	Token      string `json:"token,omitempty"`
	URL        string `json:"url,omitempty"`
	AlreadySet bool   `json:"already_set"`
}

// CreateSetupTokenForOrder mints a setup-password token for the customer
// behind an order, authorizing via the Stripe payment_intent (returned by
// Stripe's redirect to /checkout/success). Returns AlreadySet=true if the
// customer already has a password.
func (s *OrderService) CreateSetupTokenForOrder(ctx context.Context, orderID, paymentIntent string) (*SetupTokenResult, error) {
	if paymentIntent == "" {
		return nil, ErrPaymentLinkInvalid
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentIntentID == nil || *order.PaymentIntentID == "" {
		return nil, ErrPaymentLinkInvalid
	}
	if paymentIntent != *order.PaymentIntentID {
		return nil, ErrPaymentLinkInvalid
	}
	if order.CustomerID == nil || *order.CustomerID == "" {
		return &SetupTokenResult{AlreadySet: true}, nil
	}

	var pwHash sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT password_hash FROM customers WHERE id=$1`, *order.CustomerID).Scan(&pwHash); err != nil {
		return nil, err
	}
	if pwHash.Valid && pwHash.String != "" {
		return &SetupTokenResult{AlreadySet: true}, nil
	}
	if s.customerSvc == nil {
		return &SetupTokenResult{AlreadySet: true}, nil
	}
	token, err := s.customerSvc.CreateSetupToken(ctx, *order.CustomerID)
	if err != nil {
		return nil, err
	}
	base := ""
	if s.emailSvc != nil {
		base = s.emailSvc.PublicBaseURL(ctx)
	}
	return &SetupTokenResult{
		Token: token,
		URL:   fmt.Sprintf("%s/account/setup-password?token=%s", base, token),
	}, nil
}

// MarkPaidByPaymentIntent flips a pending order to `paid` and triggers the
// confirmation email. Called from the Stripe webhook on payment_intent.succeeded.
func (s *OrderService) MarkPaidByPaymentIntent(ctx context.Context, paymentIntentID, pmType, cardBrand, cardLast4 string) error {
	var orderID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE payment_intent_id=$1`, paymentIntentID).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("webhook: no order found for payment_intent %s", paymentIntentID)
		return nil
	}
	if err != nil {
		return err
	}

	// Idempotent: COALESCE/NULLIF preserves previously-stored values if Stripe
	// re-sends the event with a sparser payload.
	_, _ = s.db.ExecContext(ctx,
		`UPDATE orders
		   SET payment_status='succeeded',
		       payment_method = COALESCE(NULLIF($2, ''), payment_method),
		       card_brand     = COALESCE(NULLIF($3, ''), card_brand),
		       card_last4     = COALESCE(NULLIF($4, ''), card_last4)
		 WHERE id=$1`, orderID, pmType, cardBrand, cardLast4)

	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == StatusPending {
		_, err := s.UpdateStatus(ctx, orderID, UpdateStatusRequest{Status: StatusPaid})
		if err != nil {
			log.Printf("webhook: update status for order %s: %v", orderID, err)
		}
		// Empty the source cart now that payment is confirmed. Best-effort:
		// failure here shouldn't block payment processing or email delivery.
		var cartID sql.NullString
		if err := s.db.QueryRowContext(ctx,
			`SELECT cart_id FROM orders WHERE id=$1`, orderID).Scan(&cartID); err == nil && cartID.Valid {
			if _, err := s.db.ExecContext(ctx,
				`DELETE FROM cart_items WHERE cart_id = $1`, cartID.String); err != nil {
				log.Printf("webhook: clear cart for order %s: %v", orderID, err)
			}
		}
		// Send confirmation email (best-effort)
		s.sendConfirmationEmail(ctx, order)

		// Loyalty earn (P3 #24) — fire-and-forget so a slow points write
		// can't delay webhook ack to Stripe. Detach the request context so
		// downstream callees see a stable context.
		if s.onPaid != nil {
			paid := *order
			paid.Status = StatusPaid
			go s.onPaid(context.Background(), &paid)
		}
	}
	return nil
}

// RecordPaymentFailure logs a declined/failed payment attempt against the
// matching order. Called from the Stripe webhook on
// payment_intent.payment_failed. The order is left pending so the shopper can
// retry; this only records the attempt (system notice + payment_status) so an
// admin can see it. The payment_status update is guarded against an
// already-succeeded order in case Stripe delivers a stale failed event after a
// successful retry on the same PaymentIntent.
func (s *OrderService) RecordPaymentFailure(ctx context.Context, paymentIntentID, reason string) error {
	var orderID, prevStatus string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(payment_status,'') FROM orders WHERE payment_intent_id=$1`, paymentIntentID).Scan(&orderID, &prevStatus)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("webhook: payment_failed but no order found for payment_intent %s", paymentIntentID)
		return nil
	}
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET payment_status='failed'
		 WHERE id=$1 AND payment_status IS DISTINCT FROM 'succeeded'`, orderID); err != nil {
		return err
	}
	_ = CreateSystemNoticeTx(ctx, tx, orderID, nil, "Payment attempt failed: "+reason)

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("webhook: recorded payment failure for order %s (payment_intent %s): %s", orderID, paymentIntentID, reason)

	// On the FIRST transition into a failed state, email the shopper a link to
	// complete payment so they know the order exists and how to retry. Guarded
	// to fire once: a stale/repeat failed event (prevStatus already 'failed') or
	// a post-success event ('succeeded') won't re-send. Best-effort, off the
	// webhook's request path.
	if s.emailSvc != nil && s.paymentSvc != nil && prevStatus != "succeeded" && prevStatus != "failed" {
		go func() {
			bg := context.Background()
			order, err := s.GetByID(bg, orderID)
			if err != nil || order.Status != StatusPending {
				return
			}
			pi, err := s.paymentSvc.FetchPaymentIntent(bg, paymentIntentID)
			if err != nil || pi.ClientSecret == "" {
				log.Printf("payment-failed email: fetch intent for order %s: %v", orderID, err)
				return
			}
			s.sendPaymentLinkEmail(bg, order, pi.ClientSecret)
		}()
	}
	return nil
}

func (s *OrderService) sendConfirmationEmail(ctx context.Context, order *Order) {
	if s.emailSvc == nil || order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}

	// Shipping address from the order's frozen snapshot (ship_* columns), not a
	// live join — the confirmation reflects what was shipped, immutably.
	var line1, line2, city, state, postal, country string
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(ship_line1,''), COALESCE(ship_line2,''), COALESCE(ship_city,''),
		        COALESCE(ship_state,''), COALESCE(ship_postal_code,''), COALESCE(ship_country,'')
		 FROM orders WHERE id=$1`, order.ID).
		Scan(&line1, &line2, &city, &state, &postal, &country)

	items := buildOrderEmailItems(order.Items)

	setupURL := ""
	if order.CustomerID != nil && s.customerSvc != nil {
		// Generate a setup-password token only if the customer has no password yet.
		var pwHash sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT password_hash FROM customers WHERE id=$1`, *order.CustomerID).Scan(&pwHash)
		if err == nil && (!pwHash.Valid || pwHash.String == "") {
			token, err := s.customerSvc.CreateSetupToken(ctx, *order.CustomerID)
			if err == nil {
				base := s.emailSvc.PublicBaseURL(ctx)
				setupURL = fmt.Sprintf("%s/account/setup-password?token=%s", base, token)
			} else {
				log.Printf("create setup token for customer %s: %v", *order.CustomerID, err)
			}
		}
	}

	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}

	taxLabel := ""
	if s.taxSvc != nil {
		taxLabel = s.taxSvc.Calculate(ctx, 0).Label
	}

	emailPromos := make([]email.EmailPromotion, 0, len(order.AppliedPromotions))
	for _, p := range order.AppliedPromotions {
		desc := ""
		if p.Description != nil {
			desc = *p.Description
		}
		emailPromos = append(emailPromos, email.EmailPromotion{Name: p.Name, Description: desc})
	}

	err := s.emailSvc.SendOrderConfirmation(ctx, email.OrderEmailParams{
		OrderID:           order.ID,
		OrderNumber:       order.OrderNumber,
		CustomerName:      name,
		CustomerEmail:     *order.CustomerEmail,
		Items:             items,
		Subtotal:          order.Subtotal,
		ShippingFee:       order.ShippingFee,
		ShippingLabel:     ShippingLabel(order, "zh-Hant"),
		DiscountAmount:    order.DiscountAmount,
		AppliedPromotions: emailPromos,
		TaxAmount:         order.TaxAmount,
		TaxLabel:          taxLabel,
		Total:             order.Total,
		Currency:          "HKD",
		ShippingLine1:     line1,
		ShippingLine2:     line2,
		ShippingCity:      city,
		ShippingPostal:    postal,
		ShippingCountry:   country,
		SetupURL:          setupURL,
	})
	if err != nil {
		log.Printf("send order confirmation email for order %s: %v", order.ID, err)
	}
}

// sendBankTransferOnHoldEmail emails an installer the bank-transfer instructions
// for an order placed on hold. Best-effort — a send failure must not fail
// checkout (the storefront also shows the same details on the success page).
func (s *OrderService) sendBankTransferOnHoldEmail(ctx context.Context, order *Order) {
	if s.emailSvc == nil || order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}

	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}

	emailPromos := make([]email.EmailPromotion, 0, len(order.AppliedPromotions))
	for _, p := range order.AppliedPromotions {
		desc := ""
		if p.Description != nil {
			desc = *p.Description
		}
		emailPromos = append(emailPromos, email.EmailPromotion{Name: p.Name, Description: desc})
	}

	orderURL := ""
	if base := s.emailSvc.PublicBaseURL(ctx); base != "" {
		orderURL = fmt.Sprintf("%s/account/orders/%s", base, order.ID)
	}

	bank := s.bankTransferSettings(ctx)

	err := s.emailSvc.SendBankTransferOnHold(ctx, email.BankTransferOnHoldParams{
		OrderID:           order.ID,
		OrderNumber:       order.OrderNumber,
		CustomerName:      name,
		CustomerEmail:     *order.CustomerEmail,
		Items:             buildOrderEmailItems(order.Items),
		Subtotal:          order.Subtotal,
		ShippingLabel:     ShippingLabel(order, "zh-Hant"),
		DiscountAmount:    order.DiscountAmount,
		AppliedPromotions: emailPromos,
		Total:             order.Total,
		Currency:          "HKD",
		OrderURL:          orderURL,
		BankAccountName:   bank.accountName,
		BankName:          bank.bankName,
		BankAccountNumber: bank.accountNumber,
		WhatsAppDisplay:   bank.whatsappDisplay,
		WhatsAppURL:       bank.whatsappURL,
	})
	if err != nil {
		log.Printf("send bank-transfer on-hold email for order %s: %v", order.ID, err)
	}
}

type bankTransferDetails struct {
	accountName     string
	bankName        string
	accountNumber   string
	whatsappDisplay string
	whatsappURL     string
}

// bankTransferSettings reads the admin-configured bank-transfer account details
// from site_settings (best-effort; missing rows yield empty strings).
func (s *OrderService) bankTransferSettings(ctx context.Context) bankTransferDetails {
	var d bankTransferDetails
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM site_settings WHERE key = ANY($1)`,
		pq.Array([]string{
			"bank_transfer_account_name", "bank_transfer_bank_name", "bank_transfer_account_number",
			"bank_transfer_whatsapp_display", "bank_transfer_whatsapp_url",
		}))
	if err != nil {
		return d
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		switch k {
		case "bank_transfer_account_name":
			d.accountName = v
		case "bank_transfer_bank_name":
			d.bankName = v
		case "bank_transfer_account_number":
			d.accountNumber = v
		case "bank_transfer_whatsapp_display":
			d.whatsappDisplay = v
		case "bank_transfer_whatsapp_url":
			d.whatsappURL = v
		}
	}
	return d
}

type lowStockDec struct {
	VariantID string
	Quantity  int
}

// checkLowStockCrossings fires a low-stock alert email when a variant's stock
// drops to or below its threshold for the first time after a checkout
// decrement. We use the "just crossed" rule to avoid spamming on every order:
// fire only when previous_stock > threshold && new_stock <= threshold.
func (s *OrderService) checkLowStockCrossings(ctx context.Context, decs []lowStockDec) {
	if s.emailSvc == nil || len(decs) == 0 {
		return
	}

	var enabled string
	s.db.QueryRowContext(ctx, `SELECT value FROM site_settings WHERE key = 'low_stock_alert_enabled'`).Scan(&enabled)
	if enabled != "true" {
		return
	}

	var defaultThresholdStr string
	s.db.QueryRowContext(ctx, `SELECT value FROM site_settings WHERE key = 'low_stock_threshold_default'`).Scan(&defaultThresholdStr)
	defaultThreshold, _ := strconv.Atoi(defaultThresholdStr)
	if defaultThreshold <= 0 {
		defaultThreshold = 5
	}

	base := s.emailSvc.PublicBaseURL(ctx)

	for _, d := range decs {
		var newStock int
		var threshold sql.NullInt64
		var productID string
		var productName string
		var variantName sql.NullString
		var sku string
		err := s.db.QueryRowContext(ctx,
			`SELECT v.stock_qty, v.low_stock_threshold, v.product_id, p.name, v.name, v.sku
			 FROM product_variants v JOIN products p ON p.id = v.product_id
			 WHERE v.id = $1`, d.VariantID).
			Scan(&newStock, &threshold, &productID, &productName, &variantName, &sku)
		if err != nil {
			continue
		}

		eff := defaultThreshold
		if threshold.Valid {
			eff = int(threshold.Int64)
		}
		prevStock := newStock + d.Quantity
		// Only fire when we just crossed the threshold.
		if prevStock <= eff || newStock > eff {
			continue
		}

		vName := ""
		if variantName.Valid {
			vName = variantName.String
		}
		err = s.emailSvc.SendLowStockAlert(ctx, email.LowStockParams{
			ProductName:     productName,
			VariantName:     vName,
			SKU:             sku,
			StockQty:        newStock,
			Threshold:       eff,
			AdminProductURL: fmt.Sprintf("%s/admin/products/%s", base, productID),
		})
		if err != nil {
			log.Printf("send low-stock alert for variant %s: %v", d.VariantID, err)
		}
	}
}

// RefundRequest is the admin payload for issuing a refund.
type RefundRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

var ErrRefundExceedsTotal = errors.New("refund amount exceeds remaining refundable total")
var ErrOrderNotRefundable = errors.New("order is not in a refundable state")

// IssueRefund triggers a Stripe refund and updates the order. Supports partial
// refunds: each call adds to the existing refund_amount; when the cumulative
// refund equals the order total, the order moves to status `refunded`.
func (s *OrderService) IssueRefund(ctx context.Context, orderID string, req RefundRequest) (*Order, error) {
	if s.paymentSvc == nil {
		return nil, fmt.Errorf("payment service unavailable")
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	// Snapshot the pre-refund order for audit. `order` is read-only after this
	// point in IssueRefund; we'll re-fetch the post-refund state at the end.
	var before *Order
	if s.audit != nil {
		copy := *order
		before = &copy
	}
	if order.PaymentIntentID == nil || *order.PaymentIntentID == "" {
		return nil, fmt.Errorf("order has no payment intent")
	}
	switch order.Status {
	case StatusPaid, StatusProcessing, StatusShipped, StatusDelivered:
		// refundable
	default:
		return nil, ErrOrderNotRefundable
	}

	totalCents := int64(order.Total*100 + 0.5)
	alreadyRefunded := int64(order.RefundAmount*100 + 0.5)
	remaining := totalCents - alreadyRefunded
	if remaining <= 0 {
		return nil, ErrRefundExceedsTotal
	}

	amount := req.AmountCents
	if amount <= 0 || amount > remaining {
		amount = remaining // default to full remaining refund
	}
	if amount > remaining {
		return nil, ErrRefundExceedsTotal
	}

	refundID, err := s.paymentSvc.CreateRefund(ctx, *order.PaymentIntentID, amount, req.Reason)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	newRefundTotal := alreadyRefunded + amount
	newStatus := order.Status
	if newRefundTotal >= totalCents {
		newStatus = StatusRefunded
	}

	var reasonPtr *string
	if req.Reason != "" {
		reasonPtr = &req.Reason
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE orders
		 SET refund_amount    = $2,
		     refund_reason    = COALESCE($3, refund_reason),
		     refunded_at      = NOW(),
		     stripe_refund_id = $4,
		     status           = $5
		 WHERE id = $1`,
		orderID, float64(newRefundTotal)/100.0, reasonPtr, refundID, newStatus)
	if err != nil {
		return nil, err
	}

	noteText := fmt.Sprintf("Refund issued: %.2f", float64(amount)/100.0)
	if req.Reason != "" {
		noteText += " — " + req.Reason
	}
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status, note) VALUES ($1, $2, $3)`,
		orderID, newStatus, noteText)
	statusForNotice := newStatus
	_ = CreateSystemNoticeTx(ctx, tx, orderID, &statusForNotice, noteText)

	// On full refund, restock every order_items line and write one
	// inventory_history row per variant inside the same tx.
	if newStatus == StatusRefunded && order.Status != StatusRefunded && order.Status != StatusCancelled {
		if err := s.restockOrderItemsTx(ctx, tx, orderID, "order.refund", reasonPtr); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	go s.sendRefundEmail(context.Background(), orderID, float64(amount)/100.0, req.Reason, newStatus == StatusRefunded)

	// On a full refund the receipt is no longer accurate — drop any cache so
	// the storefront stops showing a "fast download" lightning icon for a
	// receipt that's about to fail the receiptable-status check.
	if newStatus == StatusRefunded {
		s.invalidateReceiptCache(orderID)
	}

	after, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "order.refund", orderID, before, after)
	return after, nil
}

func (s *OrderService) sendRefundEmail(ctx context.Context, orderID string, refundAmount float64, reason string, isFull bool) {
	if s.emailSvc == nil {
		return
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		log.Printf("send refund email: load order %s: %v", orderID, err)
		return
	}
	if order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}
	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}
	base := s.emailSvc.PublicBaseURL(ctx)

	if err := s.emailSvc.SendOrderRefunded(ctx, email.RefundEmailParams{
		OrderID:       order.ID,
		OrderNumber:   order.OrderNumber,
		CustomerName:  name,
		CustomerEmail: *order.CustomerEmail,
		Currency:      "HKD",
		RefundAmount:  refundAmount,
		OrderTotal:    order.Total,
		Reason:        reason,
		IsFullRefund:  isFull,
		OrderURL:      fmt.Sprintf("%s/account/orders/%s", base, order.ID),
	}); err != nil {
		log.Printf("send refund email for order %s: %v", order.ID, err)
	}
}

// sendShippedEmail looks up the latest shipment for the order (if any) and
// sends the customer a "your order has shipped" notification. Best-effort —
// callers don't fail on email errors.
func (s *OrderService) sendShippedEmail(ctx context.Context, orderID string) {
	if s.emailSvc == nil {
		return
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		log.Printf("send shipped email: load order %s: %v", orderID, err)
		return
	}
	if order.CustomerEmail == nil || *order.CustomerEmail == "" {
		return
	}

	var carrier, service, trackingNumber, trackingURL string
	s.db.QueryRowContext(ctx,
		`SELECT COALESCE(carrier,''), COALESCE(service,''), COALESCE(tracking_number,''), COALESCE(tracking_url,'')
		 FROM shipments WHERE order_id = $1
		 ORDER BY created_at DESC LIMIT 1`, orderID).
		Scan(&carrier, &service, &trackingNumber, &trackingURL)

	name := ""
	if order.CustomerName != nil {
		name = *order.CustomerName
	}
	base := s.emailSvc.PublicBaseURL(ctx)
	orderURL := fmt.Sprintf("%s/account/orders/%s", base, order.ID)

	if err := s.emailSvc.SendOrderShipped(ctx, email.ShippedEmailParams{
		OrderID:        order.ID,
		OrderNumber:    order.OrderNumber,
		CustomerName:   name,
		CustomerEmail:  *order.CustomerEmail,
		Carrier:        carrier,
		Service:        service,
		TrackingNumber: trackingNumber,
		TrackingURL:    trackingURL,
		OrderURL:       orderURL,
	}); err != nil {
		log.Printf("send shipped email for order %s: %v", order.ID, err)
	}
}

func splitName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.SplitN(full, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// nullStrPtr converts a sql.NullString into a *string, returning nil for NULL
// or empty values (the JSON shape omits these via omitempty).
func nullStrPtr(n sql.NullString) *string {
	if n.Valid && n.String != "" {
		s := n.String
		return &s
	}
	return nil
}

// shipSnapshot is the frozen shipping-address copy written onto an order's
// ship_* columns at write time, so order history stays independent of the
// (editable, deduped) customer address book. Field order matches the ship_*
// column order used by every order INSERT/UPDATE.
type shipSnapshot struct {
	FirstName  string
	LastName   string
	Phone      *string
	Line1      string
	Line2      *string
	City       string
	State      *string
	PostalCode string
	Country    string
}

// args returns the 9 ship_* values in column order for an INSERT/UPDATE.
func (snap shipSnapshot) args() []any {
	return []any{snap.FirstName, snap.LastName, snap.Phone, snap.Line1, snap.Line2,
		snap.City, snap.State, snap.PostalCode, snap.Country}
}

// loadShipSnapshot reads the resolved address-book row to freeze onto an order.
// Returns a zero snapshot (empty Line1 → NULL ship_* / no captured address)
// when addressID is nil/empty or the row is gone, preserving the prior
// "no shipping address" behavior for guest / legacy orders.
func (s *OrderService) loadShipSnapshot(ctx context.Context, addressID *string) (shipSnapshot, error) {
	var snap shipSnapshot
	if addressID == nil || *addressID == "" {
		return snap, nil
	}
	var first, last, phone, line1, line2, city, state, postal, country sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT first_name, last_name, phone, line1, line2, city, state, postal_code, country
		 FROM addresses WHERE id = $1`, *addressID).
		Scan(&first, &last, &phone, &line1, &line2, &city, &state, &postal, &country)
	if errors.Is(err, sql.ErrNoRows) {
		return snap, nil
	}
	if err != nil {
		return snap, err
	}
	return shipSnapshot{
		FirstName:  first.String,
		LastName:   last.String,
		Phone:      nullStrPtr(phone),
		Line1:      line1.String,
		Line2:      nullStrPtr(line2),
		City:       city.String,
		State:      nullStrPtr(state),
		PostalCode: postal.String,
		Country:    country.String,
	}, nil
}

// refreshOrderShipSnapshot re-freezes an order's ship_* columns from the given
// address when the shopper changed address mid-resume. A nil/empty addressID
// means the address was unchanged, so the existing frozen snapshot is left
// intact (symmetric with the shipping_address_id = COALESCE($2, …) pattern).
func (s *OrderService) refreshOrderShipSnapshot(ctx context.Context, orderID string, addressID *string) error {
	if addressID == nil || *addressID == "" {
		return nil
	}
	snap, err := s.loadShipSnapshot(ctx, addressID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE orders SET
		    ship_first_name=$2, ship_last_name=$3, ship_phone=$4, ship_line1=$5, ship_line2=$6,
		    ship_city=$7, ship_state=$8, ship_postal_code=$9, ship_country=$10, updated_at=now()
		 WHERE id=$1`,
		append([]any{orderID}, snap.args()...)...)
	return err
}

// GetByIDForCustomer returns the order only if it belongs to the given
// customer. Returns ErrOrderNotFound on a miss so the caller cannot
// distinguish "wrong owner" from "non-existent" via the response.
func (s *OrderService) GetByIDForCustomer(ctx context.Context, orderID, customerID string) (*Order, error) {
	if customerID == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.CustomerID == nil || *order.CustomerID != customerID {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// GetByIDForPaymentIntent returns a redacted order if the supplied
// payment_intent matches the one stored on the order. Used by the public
// checkout success page where the visitor isn't necessarily logged in but
// holds the PI returned by Stripe's redirect. PII (email, phone, address,
// customer_id, notes) is stripped.
func (s *OrderService) GetByIDForPaymentIntent(ctx context.Context, orderID, paymentIntent string) (*Order, error) {
	if paymentIntent == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.PaymentIntentID == nil || *order.PaymentIntentID != paymentIntent {
		return nil, ErrOrderNotFound
	}
	safe := *order
	safe.CustomerEmail = nil
	safe.CustomerPhone = nil
	safe.CustomerID = nil
	safe.ShippingAddressID = nil
	safe.ShippingAddress = nil
	safe.Notes = nil
	return &safe, nil
}

func (s *OrderService) GetByID(ctx context.Context, id string) (*Order, error) {
	var order Order
	var appliedPromosRaw []byte
	var shipFirst, shipLast, shipPhone, shipLine1, shipLine2, shipCity, shipState, shipPostal, shipCountry sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, COALESCE(order_number, ''), customer_id, status, shipping_address_id,
		        subtotal, shipping_fee, shipping_free, discount_amount, applied_promotions, tax_amount, total, notes,
		        customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		        card_brand, card_last4,
		        selected_carrier, selected_service, pickup_point_id, pickup_point_label,
		        refund_amount, refund_reason, refunded_at, stripe_refund_id,
		        ship_first_name, ship_last_name, ship_phone, ship_line1, ship_line2,
		        ship_city, ship_state, ship_postal_code, ship_country,
		        created_at, updated_at
		 FROM orders WHERE id = $1`, id).
		Scan(&order.ID, &order.Number, &order.OrderNumber, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.ShippingFree, &order.DiscountAmount, &appliedPromosRaw, &order.TaxAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.CardBrand, &order.CardLast4,
			&order.SelectedCarrier, &order.SelectedService, &order.PickupPointID, &order.PickupPointLabel,
			&order.RefundAmount, &order.RefundReason, &order.RefundedAt, &order.StripeRefundID,
			&shipFirst, &shipLast, &shipPhone, &shipLine1, &shipLine2,
			&shipCity, &shipState, &shipPostal, &shipCountry,
			&order.CreatedAt, &order.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	order.AppliedPromotions = scanAppliedPromotions(appliedPromosRaw)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, order_id, variant_id, parent_item_id, product_name, product_subtitle, variant_sku, unit_price, quantity, line_total
		 FROM order_items WHERE order_id = $1
		 ORDER BY parent_item_id NULLS FIRST, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OrderItem
		rows.Scan(&item.ID, &item.OrderID, &item.VariantID, &item.ParentItemID, &item.ProductName, &item.ProductSubtitle,
			&item.VariantSKU, &item.UnitPrice, &item.Quantity, &item.LineTotal)
		order.Items = append(order.Items, item)
	}

	// Shipping address is a frozen snapshot on the order (ship_* columns), not a
	// live join — editing or deleting the customer's address-book row never
	// alters a placed order. nil when no address was captured (guest / legacy).
	if shipLine1.Valid && shipLine1.String != "" {
		order.ShippingAddress = &ShippingAddress{
			FirstName:  shipFirst.String,
			LastName:   shipLast.String,
			Phone:      nullStrPtr(shipPhone),
			Line1:      shipLine1.String,
			Line2:      nullStrPtr(shipLine2),
			City:       shipCity.String,
			State:      nullStrPtr(shipState),
			PostalCode: shipPostal.String,
			Country:    shipCountry.String,
		}
	}

	var paidAt sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT created_at FROM order_status_history
		 WHERE order_id = $1 AND status = 'paid'
		 ORDER BY created_at ASC LIMIT 1`, id).Scan(&paidAt)
	if err == nil && paidAt.Valid {
		s := paidAt.String
		order.PaidAt = &s
	}

	return &order, nil
}

// ListFilters narrows the admin order list. All fields are optional; the zero
// value of each is treated as "no filter on this dimension".
type ListFilters struct {
	Search    string        // substring match across order_number / customer_*
	Statuses  []OrderStatus // OR — only orders whose status is in the slice
	From      *time.Time    // inclusive lower bound on created_at
	To        *time.Time    // exclusive upper bound on created_at (caller passes start-of-day-after to make a calendar-day range inclusive)
	HasUnread bool          // only orders with at least one unread customer notice
	Roles     []string      // OR — customers.role IN (...); empty = any
	Carrier   string        // exact match on orders.selected_carrier
	HasPickup *bool         // nil = ignore; true = pickup_point_id IS NOT NULL; false = IS NULL
	HasNotes  bool          // true = notes IS NOT NULL AND notes <> ''
}

var orderSearchFields = []string{"COALESCE(order_number, '')", "COALESCE(customer_name, '')", "COALESCE(customer_email, '')", "COALESCE(customer_phone, '')"}

func (s *OrderService) List(ctx context.Context, f ListFilters, limit, offset int) ([]Order, int, error) {
	conds := []string{}
	args := []any{}

	if clause, arg := util.BuildSearchClause(f.Search, orderSearchFields, len(args)+1); clause != "" {
		conds = append(conds, clause)
		args = append(args, arg)
	}
	if len(f.Statuses) > 0 {
		raw := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			raw[i] = string(st)
		}
		conds = append(conds, fmt.Sprintf("status = ANY($%d::order_status[])", len(args)+1))
		args = append(args, pq.Array(raw))
	}
	if f.From != nil {
		// Qualify with `orders.` — the query LEFT JOINs customers, which also has
		// a created_at column, so a bare reference is ambiguous (Postgres errors).
		conds = append(conds, fmt.Sprintf("orders.created_at >= $%d", len(args)+1))
		args = append(args, *f.From)
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("orders.created_at < $%d", len(args)+1))
		args = append(args, *f.To)
	}
	if f.HasUnread {
		// Mirrors UnreadCountsForAdmin: only customer-authored notices count
		// as "unread admin attention required".
		conds = append(conds, `EXISTS (SELECT 1 FROM order_notices n WHERE n.order_id = orders.id AND n.role = 'customer' AND n.read_at IS NULL)`)
	}
	if len(f.Roles) > 0 {
		conds = append(conds, fmt.Sprintf("c.role::text = ANY($%d::text[])", len(args)+1))
		args = append(args, pq.Array(f.Roles))
	}
	if f.Carrier != "" {
		conds = append(conds, fmt.Sprintf("orders.selected_carrier = $%d", len(args)+1))
		args = append(args, f.Carrier)
	}
	if f.HasPickup != nil {
		if *f.HasPickup {
			conds = append(conds, "orders.pickup_point_id IS NOT NULL")
		} else {
			conds = append(conds, "orders.pickup_point_id IS NULL")
		}
	}
	if f.HasNotes {
		conds = append(conds, "orders.notes IS NOT NULL AND orders.notes <> ''")
	}

	whereSQL := ""
	if len(conds) > 0 {
		whereSQL = " WHERE " + strings.Join(conds, " AND ")
	}

	fromSQL := " FROM orders LEFT JOIN customers c ON c.id = orders.customer_id"

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+fromSQL+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	query := `SELECT orders.id, orders.number, COALESCE(orders.order_number, ''), orders.customer_id, orders.status,
		        orders.subtotal, orders.shipping_fee, orders.shipping_free, orders.discount_amount, orders.tax_amount, orders.total,
		        orders.customer_email, orders.customer_phone, orders.customer_name, orders.payment_intent_id, orders.payment_status,
		        orders.notes, orders.selected_carrier, orders.selected_service, orders.pickup_point_id, orders.pickup_point_label,
		        orders.refund_amount, orders.refunded_at,
		        c.role,
		        (SELECT COUNT(*) FROM order_items oi WHERE oi.order_id = orders.id AND oi.parent_item_id IS NULL) AS items_count,
		        orders.created_at, orders.updated_at` +
		fromSQL + whereSQL + fmt.Sprintf(` ORDER BY orders.created_at DESC LIMIT $%d OFFSET $%d`, limitIdx, offsetIdx)

	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Number, &o.OrderNumber, &o.CustomerID, &o.Status,
			&o.Subtotal, &o.ShippingFee, &o.ShippingFree, &o.DiscountAmount, &o.TaxAmount, &o.Total,
			&o.CustomerEmail, &o.CustomerPhone, &o.CustomerName, &o.PaymentIntentID, &o.PaymentStatus,
			&o.Notes, &o.SelectedCarrier, &o.SelectedService, &o.PickupPointID, &o.PickupPointLabel,
			&o.RefundAmount, &o.RefundedAt,
			&o.CustomerRole,
			&o.ItemsCount,
			&o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// CarrierOption is one entry in the carrier-filter dropdown.
type CarrierOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ListCarriers returns the distinct, non-empty selected_carrier values across
// all orders, sorted by frequency descending. Used to populate the admin
// orders list carrier filter without hardcoding the carrier set on the client.
func (s *OrderService) ListCarriers(ctx context.Context) ([]CarrierOption, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT selected_carrier, COUNT(*) FROM orders
		  WHERE selected_carrier IS NOT NULL AND selected_carrier <> ''
		  GROUP BY selected_carrier
		  ORDER BY COUNT(*) DESC, selected_carrier ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CarrierOption, 0)
	for rows.Next() {
		var c CarrierOption
		if err := rows.Scan(&c.Value, &c.Count); err != nil {
			return nil, err
		}
		c.Label = c.Value
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetIDByNumber resolves a sequential display number to its UUID.
// Returns sql.ErrNoRows if no row matches.
func (s *OrderService) GetIDByNumber(ctx context.Context, n int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM orders WHERE number = $1`, n).Scan(&id)
	return id, err
}

// Delete removes an order and its dependent rows (cascade on order_items
// and order_status_history). Used by the admin order list "Delete" action.
func (s *OrderService) Delete(ctx context.Context, id string) error {
	var before *Order
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id); err == nil {
			before = prev
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM orders WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrOrderNotFound
	}
	// Receipt cache lives on disk, not in the DB — the FK cascade can't reach
	// it. Unlink files explicitly so a recycled order ID can never read back
	// a stale prior receipt.
	s.invalidateReceiptCache(id)
	s.record(ctx, "order.delete", id, before, nil)
	return nil
}

func (s *OrderService) UpdateStatus(ctx context.Context, id string, req UpdateStatusRequest) (*Order, error) {
	var before *Order
	if s.audit != nil {
		if prev, err := s.GetByID(ctx, id); err == nil {
			before = prev
		}
	}
	var current OrderStatus
	err := s.db.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = $1`, id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	allowed := false
	for _, next := range allowedTransitions[current] {
		if next == req.Status {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("cannot transition from %s to %s", current, req.Status)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var order Order
	err = tx.QueryRowContext(ctx,
		`UPDATE orders SET status = $2 WHERE id = $1
		 RETURNING id, number, COALESCE(order_number, ''), customer_id, status, shipping_address_id,
		           subtotal, shipping_fee, shipping_free, discount_amount, tax_amount, total, notes,
		           customer_email, customer_phone, customer_name, payment_intent_id, payment_status, payment_method,
		           selected_carrier, selected_service, pickup_point_id, pickup_point_label,
		           created_at, updated_at`,
		id, req.Status).
		Scan(&order.ID, &order.Number, &order.OrderNumber, &order.CustomerID, &order.Status, &order.ShippingAddressID,
			&order.Subtotal, &order.ShippingFee, &order.ShippingFree, &order.DiscountAmount, &order.TaxAmount, &order.Total,
			&order.Notes, &order.CustomerEmail, &order.CustomerPhone, &order.CustomerName,
			&order.PaymentIntentID, &order.PaymentStatus, &order.PaymentMethod,
			&order.SelectedCarrier, &order.SelectedService, &order.PickupPointID, &order.PickupPointLabel,
			&order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}

	tx.ExecContext(ctx,
		`INSERT INTO order_status_history (order_id, status, note) VALUES ($1, $2, $3)`,
		id, req.Status, req.Note)

	// Mirror the status change as a system notice so it shows up in the user-
	// visible timeline alongside admin/customer messages. Use the supplied
	// note when present, otherwise synthesize a default.
	noticeBody := fmt.Sprintf("Status updated to %s", req.Status)
	if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
		noticeBody = *req.Note
	}
	status := req.Status
	_ = CreateSystemNoticeTx(ctx, tx, id, &status, noticeBody)

	// Restock the order's items when transitioning into a terminal "stock
	// returned" state. Guard with the prior `current` status so a no-op
	// re-transition can't double-restock (also disallowed by
	// allowedTransitions, but defensive).
	if (req.Status == StatusCancelled || req.Status == StatusRefunded) &&
		current != StatusCancelled && current != StatusRefunded {
		restockReason := "order.cancel"
		if req.Status == StatusRefunded {
			restockReason = "order.refund"
		}
		if err := s.restockOrderItemsTx(ctx, tx, id, restockReason, req.Note); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Notify the customer only on the 處理中 → 已發貨 transition: send the
	// shipped email and post the SF Express tracking notice (via onShipped).
	// Both are best-effort and fire once, since the state machine only reaches
	// shipped from processing.
	if current == StatusProcessing && req.Status == StatusShipped {
		go s.sendShippedEmail(context.Background(), id)
		if s.onShipped != nil {
			go s.onShipped(context.Background(), &order)
		}
	}
	// A refund renders any previously cached receipt misleading (it claims
	// a paid balance that's no longer owed). Clear so the next download
	// either re-renders or shows the not-receiptable error.
	if req.Status == StatusRefunded {
		s.invalidateReceiptCache(id)
	}

	s.record(ctx, "order.update_status", order.ID, before, order)
	return &order, nil
}
