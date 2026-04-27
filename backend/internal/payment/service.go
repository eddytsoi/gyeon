package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
	"gyeon/backend/internal/settings"
)

var ErrNotConfigured = errors.New("stripe is not configured")

type Service struct {
	settings *settings.Service
}

func NewService(s *settings.Service) *Service {
	return &Service{settings: s}
}

type Config struct {
	PublishableKey string `json:"publishable_key"`
	Mode           string `json:"mode"`
}

// PublicConfig returns the publishable key + mode for the storefront (no secrets).
func (s *Service) PublicConfig(ctx context.Context) Config {
	mode := s.mode(ctx)
	pk := s.read(ctx, "stripe_"+mode+"_publishable_key")
	return Config{PublishableKey: pk, Mode: mode}
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

// VerifyWebhook validates the Stripe-Signature header and parses the event.
func (s *Service) VerifyWebhook(ctx context.Context, body []byte, sigHeader string) (stripe.Event, error) {
	secret := s.WebhookSecret(ctx)
	if secret == "" {
		return stripe.Event{}, ErrNotConfigured
	}
	return webhook.ConstructEvent(body, sigHeader, secret)
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

