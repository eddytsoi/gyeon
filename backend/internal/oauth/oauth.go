// Package oauth implements storefront customer social login (Google + Apple)
// via the OAuth 2.0 / OpenID Connect authorization-code flow. Provider
// credentials live in site_settings (read on each call, mirroring the
// recaptcha / email packages), so the feature is fully admin-toggleable and a
// fresh install works without any credentials. The whole handshake runs
// server-side; the handler sets the customer_token cookie itself.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"gyeon/backend/internal/settings"
)

const (
	ProviderGoogle = "google"
	ProviderApple  = "apple"
)

var (
	ErrProviderDisabled = errors.New("oauth provider not enabled")
	ErrUnknownProvider  = errors.New("unknown oauth provider")
	ErrInvalidState     = errors.New("invalid or expired oauth state")
	ErrNoEmail          = errors.New("provider did not return a verified email")
)

// UserInfo is the normalized identity resolved from a provider after a
// successful authorization-code exchange.
type UserInfo struct {
	Provider      string
	Subject       string // provider-stable user id ("sub")
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
}

type Service struct {
	settings *settings.Service
	db       *sql.DB
	client   *http.Client
}

func New(s *settings.Service, db *sql.DB) *Service {
	return &Service{settings: s, db: db, client: &http.Client{Timeout: 10 * time.Second}}
}

// SetHTTPClient overrides the default 10s client. Used by tests.
func (s *Service) SetHTTPClient(c *http.Client) { s.client = c }

// ValidProvider reports whether p is a provider this package handles.
func ValidProvider(p string) bool { return p == ProviderGoogle || p == ProviderApple }

// Enabled reports whether the admin switched the provider on AND supplied the
// minimum credentials it needs to function.
func (s *Service) Enabled(ctx context.Context, provider string) bool {
	switch provider {
	case ProviderGoogle:
		return s.boolSetting(ctx, "google_oauth_enabled") &&
			s.read(ctx, "google_oauth_client_id") != "" &&
			s.read(ctx, "google_oauth_client_secret") != ""
	case ProviderApple:
		return s.boolSetting(ctx, "apple_oauth_enabled") &&
			s.read(ctx, "apple_oauth_client_id") != "" &&
			s.read(ctx, "apple_oauth_team_id") != "" &&
			s.read(ctx, "apple_oauth_key_id") != "" &&
			s.read(ctx, "apple_oauth_private_key") != ""
	}
	return false
}

// AuthURL persists a fresh single-use login state and returns the provider
// authorize URL the browser should be redirected to.
func (s *Service) AuthURL(ctx context.Context, provider string) (string, error) {
	if !s.Enabled(ctx, provider) {
		return "", ErrProviderDisabled
	}
	switch provider {
	case ProviderGoogle:
		return s.googleAuthURL(ctx)
	case ProviderApple:
		return s.appleAuthURL(ctx)
	}
	return "", ErrUnknownProvider
}

// Exchange validates+consumes the state and exchanges the authorization code,
// returning the authenticated user.
func (s *Service) Exchange(ctx context.Context, provider, code, state string) (*UserInfo, error) {
	if !s.Enabled(ctx, provider) {
		return nil, ErrProviderDisabled
	}
	if code == "" {
		return nil, ErrInvalidState
	}
	st, err := s.consumeState(ctx, state)
	if err != nil {
		return nil, err
	}
	if st.Provider != provider {
		return nil, ErrInvalidState
	}
	switch provider {
	case ProviderGoogle:
		return s.googleExchange(ctx, code, st)
	case ProviderApple:
		return s.appleExchange(ctx, code, st)
	}
	return nil, ErrUnknownProvider
}

// redirectURI must be byte-identical in the authorize request and the token
// exchange, and must match what's registered with the provider. Built from the
// public_base_url setting so both halves of the flow agree.
func (s *Service) redirectURI(ctx context.Context, provider string) string {
	base := strings.TrimRight(s.read(ctx, "public_base_url"), "/")
	return base + "/api/v1/customers/oauth/" + provider + "/callback"
}

// ── settings helpers (mirror recaptcha) ──────────────────────────────────

func (s *Service) read(ctx context.Context, key string) string {
	st, err := s.settings.Get(ctx, key)
	if err != nil || st == nil {
		return ""
	}
	return strings.TrimSpace(st.Value)
}

func (s *Service) boolSetting(ctx context.Context, key string) bool {
	return strings.EqualFold(s.read(ctx, key), "true")
}

// ── random + PKCE helpers ────────────────────────────────────────────────

func randString(nbytes int) (string, error) {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func pkceVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
