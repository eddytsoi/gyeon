package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"gyeon/backend/internal/settings"
)

var ErrNotConfigured = errors.New("stripe is not configured")

type Service struct {
	settings *settings.Service
	db       *sql.DB
}

func NewService(s *settings.Service, db *sql.DB) *Service {
	return &Service{settings: s, db: db}
}

type Config struct {
	PublishableKey string `json:"publishable_key"`
	Mode           string `json:"mode"`
	Country        string `json:"country"`
}

// PublicConfig returns the publishable key + mode + country for the storefront
// (no secrets). Country falls back to "HK" when not configured.
func (s *Service) PublicConfig(ctx context.Context) Config {
	mode := s.mode(ctx)
	pk := s.read(ctx, "stripe_"+mode+"_publishable_key")
	country := s.read(ctx, "stripe_country")
	if country == "" {
		country = "HK"
	}
	return Config{PublishableKey: pk, Mode: mode, Country: country}
}

// Mode returns "test" or "live".
func (s *Service) Mode(ctx context.Context) string { return s.mode(ctx) }

// SecretKey returns the active secret key based on mode.
func (s *Service) SecretKey(ctx context.Context) string {
	return s.read(ctx, "stripe_"+s.mode(ctx)+"_secret_key")
}

// PublishableKey returns the active publishable key based on mode.
func (s *Service) PublishableKey(ctx context.Context) string {
	return s.read(ctx, "stripe_"+s.mode(ctx)+"_publishable_key")
}

// WebhookSecret returns the configured webhook signing secret.
func (s *Service) WebhookSecret(ctx context.Context) string {
	return s.read(ctx, "stripe_webhook_secret")
}

type CreateIntentParams struct {
	AmountCents int64
	Currency    string
	OrderID     string
	Email       string
}

type Intent struct {
	ID           string
	ClientSecret string
}

// CreatePaymentIntent creates a Stripe PaymentIntent using the active secret key.
func (s *Service) CreatePaymentIntent(ctx context.Context, p CreateIntentParams) (*Intent, error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return nil, ErrNotConfigured
	}

	sc := stripe.NewClient(sk)

	if p.Currency == "" {
		p.Currency = "hkd"
	}

	params := &stripe.PaymentIntentCreateParams{
		Amount:   stripe.Int64(p.AmountCents),
		Currency: stripe.String(p.Currency),
		AutomaticPaymentMethods: &stripe.PaymentIntentCreateAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"order_id": p.OrderID,
		},
	}
	if p.Email != "" {
		params.ReceiptEmail = stripe.String(p.Email)
	}

	pi, err := sc.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("stripe create intent: %w", err)
	}
	return &Intent{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

// CreateRefund issues a Stripe refund against an existing PaymentIntent.
// Returns the Stripe refund ID for audit purposes. Pass amountCents=0 for a
// full refund of the remaining captured amount.
func (s *Service) CreateRefund(ctx context.Context, paymentIntentID string, amountCents int64, reason string) (string, error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return "", ErrNotConfigured
	}
	if paymentIntentID == "" {
		return "", fmt.Errorf("payment_intent_id is required")
	}
	sc := stripe.NewClient(sk)

	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(paymentIntentID),
	}
	if amountCents > 0 {
		params.Amount = stripe.Int64(amountCents)
	}
	if reason != "" {
		params.Metadata = map[string]string{"reason": reason}
	}
	rf, err := sc.V1Refunds.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("stripe create refund: %w", err)
	}
	return rf.ID, nil
}

// FetchPaymentMethodDetails retrieves PM type + card brand/last4/exp from Stripe.
// pmType is the Stripe PaymentMethod type ("card", "alipay", etc.); brand/last4/exp
// are populated only when pmType == "card".
func (s *Service) FetchPaymentMethodDetails(ctx context.Context, stripePMID string) (pmType, brand, last4 string, expMonth, expYear int, err error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return "", "", "", 0, 0, ErrNotConfigured
	}
	sc := stripe.NewClient(sk)
	pm, apiErr := sc.V1PaymentMethods.Retrieve(ctx, stripePMID, nil)
	if apiErr != nil {
		return "", "", "", 0, 0, fmt.Errorf("fetch payment method: %w", apiErr)
	}
	pmType = string(pm.Type)
	if pm.Card != nil {
		brand = string(pm.Card.Brand)
		last4 = pm.Card.Last4
		expMonth = int(pm.Card.ExpMonth)
		expYear = int(pm.Card.ExpYear)
	}
	return pmType, brand, last4, expMonth, expYear, nil
}

// VerifyWebhook validates the Stripe-Signature header and parses the event.
// We deliberately ignore the API version mismatch check: webhook endpoints
// are pinned to the API version that was current when they were created in
// the Stripe Dashboard, but the stripe-go SDK we ship here may be newer.
// Treating that as a hard error stops payment_intent.succeeded events from
// flipping orders to paid; the per-event payload we read (id, customer,
// payment_method) is stable across versions, so the bypass is safe.
func (s *Service) VerifyWebhook(ctx context.Context, body []byte, sigHeader string) (stripe.Event, error) {
	secret := s.WebhookSecret(ctx)
	if secret == "" {
		return stripe.Event{}, ErrNotConfigured
	}
	return webhook.ConstructEventWithOptions(body, sigHeader, secret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
}

// FetchPaymentIntent loads a PaymentIntent from Stripe (used as a fallback in tests / debugging).
func (s *Service) FetchPaymentIntent(ctx context.Context, id string) (*stripe.PaymentIntent, error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return nil, ErrNotConfigured
	}
	sc := stripe.NewClient(sk)
	return sc.V1PaymentIntents.Retrieve(ctx, id, nil)
}

func (s *Service) mode(ctx context.Context) string {
	m := s.read(ctx, "stripe_mode")
	if m != "live" {
		return "test"
	}
	return "live"
}

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}
