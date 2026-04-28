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

// SaveCardsEnabled returns true when the stripe_save_cards setting is "true".
func (s *Service) SaveCardsEnabled(ctx context.Context) bool {
	return s.read(ctx, "stripe_save_cards") == "true"
}

type CreateIntentParams struct {
	AmountCents      int64
	Currency         string
	OrderID          string
	Email            string
	StripeCustomerID string // optional; attaches PI to a customer for future use
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
	if p.StripeCustomerID != "" {
		params.Customer = stripe.String(p.StripeCustomerID)
	}

	pi, err := sc.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("stripe create intent: %w", err)
	}
	return &Intent{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

// CreateSetupIntent creates a Stripe SetupIntent so the customer can save a card
// without an immediate charge.
func (s *Service) CreateSetupIntent(ctx context.Context, stripeCustomerID string) (*Intent, error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return nil, ErrNotConfigured
	}
	sc := stripe.NewClient(sk)

	params := &stripe.SetupIntentCreateParams{
		Customer: stripe.String(stripeCustomerID),
		AutomaticPaymentMethods: &stripe.SetupIntentCreateAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	si, err := sc.V1SetupIntents.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("stripe create setup intent: %w", err)
	}
	return &Intent{ID: si.ID, ClientSecret: si.ClientSecret}, nil
}

// EnsureStripeCustomer returns the Stripe customer ID for the given Gyeon customer,
// creating one on Stripe and persisting it locally if it doesn't exist yet.
func (s *Service) EnsureStripeCustomer(ctx context.Context, customerID, email string) (string, error) {
	// Check if we already have one stored
	var stripeID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT stripe_customer_id FROM customers WHERE id=$1`, customerID).Scan(&stripeID)
	if err != nil {
		return "", fmt.Errorf("lookup stripe_customer_id: %w", err)
	}
	if stripeID.Valid && stripeID.String != "" {
		return stripeID.String, nil
	}

	// Create a new Stripe Customer
	sk := s.SecretKey(ctx)
	if sk == "" {
		return "", ErrNotConfigured
	}
	sc := stripe.NewClient(sk)
	cu, err := sc.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"gyeon_customer_id": customerID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}

	// Persist
	_, err = s.db.ExecContext(ctx,
		`UPDATE customers SET stripe_customer_id=$2 WHERE id=$1`, customerID, cu.ID)
	if err != nil {
		return "", fmt.Errorf("persist stripe_customer_id: %w", err)
	}
	return cu.ID, nil
}

// SavedPaymentMethod is a stored card record for a customer.
type SavedPaymentMethod struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	StripePMID string `json:"stripe_pm_id"`
	Brand      string `json:"brand"`
	Last4      string `json:"last4"`
	ExpMonth   int    `json:"exp_month"`
	ExpYear    int    `json:"exp_year"`
	IsDefault  bool   `json:"is_default"`
	CreatedAt  string `json:"created_at"`
}

// ListSavedPaymentMethods returns all saved cards for a customer.
func (s *Service) ListSavedPaymentMethods(ctx context.Context, customerID string) ([]SavedPaymentMethod, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, customer_id, stripe_pm_id, COALESCE(brand,''), COALESCE(last4,''),
		        COALESCE(exp_month,0), COALESCE(exp_year,0), is_default, created_at
		 FROM saved_payment_methods WHERE customer_id=$1 ORDER BY is_default DESC, created_at DESC`,
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SavedPaymentMethod
	for rows.Next() {
		var pm SavedPaymentMethod
		if err := rows.Scan(&pm.ID, &pm.CustomerID, &pm.StripePMID,
			&pm.Brand, &pm.Last4, &pm.ExpMonth, &pm.ExpYear,
			&pm.IsDefault, &pm.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, rows.Err()
}

// StoreSavedPaymentMethod inserts a new saved card record after setup_intent.succeeded.
// Sets is_default=true if this is the customer's first card.
func (s *Service) StoreSavedPaymentMethod(ctx context.Context, customerID, stripePMID, brand, last4 string, expMonth, expYear int) error {
	var count int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM saved_payment_methods WHERE customer_id=$1`, customerID).Scan(&count)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO saved_payment_methods (customer_id, stripe_pm_id, brand, last4, exp_month, exp_year, is_default)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (stripe_pm_id) DO NOTHING`,
		customerID, stripePMID, brand, last4, expMonth, expYear, count == 0)
	return err
}

// DetachPaymentMethod removes a saved card from Stripe and deletes the local record.
// Returns the record so the caller can verify ownership before calling.
func (s *Service) DetachPaymentMethod(ctx context.Context, id, customerID string) error {
	var stripePMID string
	err := s.db.QueryRowContext(ctx,
		`SELECT stripe_pm_id FROM saved_payment_methods WHERE id=$1 AND customer_id=$2`,
		id, customerID).Scan(&stripePMID)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("payment method not found")
	}
	if err != nil {
		return err
	}

	// Detach from Stripe (best-effort — don't block deletion on Stripe errors)
	sk := s.SecretKey(ctx)
	if sk != "" {
		sc := stripe.NewClient(sk)
		_, _ = sc.V1PaymentMethods.Detach(ctx, stripePMID, nil)
	}

	_, err = s.db.ExecContext(ctx,
		`DELETE FROM saved_payment_methods WHERE id=$1 AND customer_id=$2`, id, customerID)
	return err
}

// SetDefaultPaymentMethod marks one card as default and clears the flag on all others.
func (s *Service) SetDefaultPaymentMethod(ctx context.Context, id, customerID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Verify ownership
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM saved_payment_methods WHERE id=$1 AND customer_id=$2)`,
		id, customerID).Scan(&exists); err != nil || !exists {
		return errors.New("payment method not found")
	}

	tx.ExecContext(ctx,
		`UPDATE saved_payment_methods SET is_default=FALSE WHERE customer_id=$1`, customerID)
	tx.ExecContext(ctx,
		`UPDATE saved_payment_methods SET is_default=TRUE WHERE id=$1`, id)

	return tx.Commit()
}

// LookupCustomerByStripeID returns the Gyeon customer ID for a Stripe customer ID.
func (s *Service) LookupCustomerByStripeID(ctx context.Context, stripeCustomerID string) (string, error) {
	var customerID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE stripe_customer_id=$1`, stripeCustomerID).Scan(&customerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("customer not found for stripe id")
	}
	return customerID, err
}

// FetchPaymentMethodDetails retrieves card brand/last4/exp from Stripe.
func (s *Service) FetchPaymentMethodDetails(ctx context.Context, stripePMID string) (brand, last4 string, expMonth, expYear int, err error) {
	sk := s.SecretKey(ctx)
	if sk == "" {
		return "", "", 0, 0, ErrNotConfigured
	}
	sc := stripe.NewClient(sk)
	pm, apiErr := sc.V1PaymentMethods.Retrieve(ctx, stripePMID, nil)
	if apiErr != nil {
		return "", "", 0, 0, fmt.Errorf("fetch payment method: %w", apiErr)
	}
	if pm.Card != nil {
		brand = string(pm.Card.Brand)
		last4 = pm.Card.Last4
		expMonth = int(pm.Card.ExpMonth)
		expYear = int(pm.Card.ExpYear)
	}
	return brand, last4, expMonth, expYear, nil
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
