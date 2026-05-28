package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleAuthEndpoint  = "https://appleid.apple.com/auth/authorize"
	appleTokenEndpoint = "https://appleid.apple.com/auth/token"
	appleKeysEndpoint  = "https://appleid.apple.com/auth/keys"
	appleIssuer        = "https://appleid.apple.com"
)

func (s *Service) appleAuthURL(ctx context.Context) (string, error) {
	state, err := randString(32)
	if err != nil {
		return "", err
	}
	nonce, err := randString(16)
	if err != nil {
		return "", err
	}
	if err := s.saveState(ctx, loginState{State: state, Provider: ProviderApple, Nonce: nonce}); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", s.read(ctx, "apple_oauth_client_id"))
	q.Set("redirect_uri", s.redirectURI(ctx, ProviderApple))
	q.Set("response_type", "code")
	q.Set("scope", "name email")
	q.Set("state", state)
	q.Set("nonce", nonce)
	// Requesting name/email forces form_post: Apple POSTs the callback from
	// its own origin (cross-site), which is why login state lives server-side
	// rather than in a SameSite cookie.
	q.Set("response_mode", "form_post")
	return appleAuthEndpoint + "?" + q.Encode(), nil
}

func (s *Service) appleExchange(ctx context.Context, code string, st *loginState) (*UserInfo, error) {
	secret, err := s.appleClientSecret(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", s.read(ctx, "apple_oauth_client_id"))
	form.Set("client_secret", secret)
	form.Set("redirect_uri", s.redirectURI(ctx, ProviderApple))

	var tok struct {
		IDToken   string `json:"id_token"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if err := s.postForm(ctx, appleTokenEndpoint, form, &tok); err != nil {
		return nil, err
	}
	if tok.IDToken == "" {
		return nil, fmt.Errorf("apple token exchange failed: %s %s", tok.Error, tok.ErrorDesc)
	}
	return s.appleVerifyIDToken(ctx, tok.IDToken, st.Nonce)
}

// appleClientSecret builds the short-lived ES256 JWT Apple requires in place of
// a static client secret.
func (s *Service) appleClientSecret(ctx context.Context) (string, error) {
	key, err := parseECPrivateKey(s.read(ctx, "apple_oauth_private_key"))
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    s.read(ctx, "apple_oauth_team_id"),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Minute)),
		Audience:  jwt.ClaimStrings{appleIssuer},
		Subject:   s.read(ctx, "apple_oauth_client_id"),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	t.Header["kid"] = s.read(ctx, "apple_oauth_key_id")
	return t.SignedString(key)
}

func (s *Service) appleVerifyIDToken(ctx context.Context, idToken, nonce string) (*UserInfo, error) {
	clientID := s.read(ctx, "apple_oauth_client_id")
	var claims struct {
		jwt.RegisteredClaims
		Email string `json:"email"`
		Nonce string `json:"nonce"`
	}
	_, err := jwt.ParseWithClaims(idToken, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return s.appleKey(ctx, kid)
	}, jwt.WithIssuer(appleIssuer), jwt.WithAudience(clientID))
	if err != nil {
		return nil, fmt.Errorf("apple id_token verify: %w", err)
	}
	if nonce != "" && claims.Nonce != nonce {
		return nil, errors.New("apple id_token nonce mismatch")
	}
	if claims.Subject == "" || claims.Email == "" {
		return nil, ErrNoEmail
	}
	return &UserInfo{
		Provider:      ProviderApple,
		Subject:       claims.Subject,
		Email:         strings.ToLower(claims.Email),
		EmailVerified: true, // Apple only ever returns verified emails
	}, nil
}

// ── Apple JWKS (RSA public keys) ─────────────────────────────────────────

type appleKeyCache struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

var appleKeys = &appleKeyCache{keys: map[string]*rsa.PublicKey{}}

func (s *Service) appleKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	appleKeys.mu.RLock()
	k, ok := appleKeys.keys[kid]
	fresh := time.Since(appleKeys.fetched) < time.Hour
	appleKeys.mu.RUnlock()
	if ok && fresh {
		return k, nil
	}
	if err := s.refreshAppleKeys(ctx); err != nil {
		if ok { // fall back to a possibly-stale key rather than hard-fail
			return k, nil
		}
		return nil, err
	}
	appleKeys.mu.RLock()
	defer appleKeys.mu.RUnlock()
	if k, ok = appleKeys.keys[kid]; !ok {
		return nil, fmt.Errorf("apple jwks: no key for kid %q", kid)
	}
	return k, nil
}

func (s *Service) refreshAppleKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appleKeysEndpoint, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple jwks http %d", resp.StatusCode)
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return err
	}
	m := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		pub, perr := jwkToRSA(k.N, k.E)
		if perr != nil {
			continue
		}
		m[k.Kid] = pub
	}
	if len(m) == 0 {
		return errors.New("apple jwks: no usable keys")
	}
	appleKeys.mu.Lock()
	appleKeys.keys = m
	appleKeys.fetched = time.Now()
	appleKeys.mu.Unlock()
	return nil
}

func jwkToRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, errors.New("apple jwks: invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func parseECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("apple private key: invalid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("apple private key: not an EC key")
		}
		return ec, nil
	}
	// Fall back to SEC1 in case the key was exported in that form.
	return x509.ParseECPrivateKey(block.Bytes)
}

// ParseAppleUserName extracts the first/last name from Apple's first-login
// `user` form field (JSON). Apple only sends this on the very first
// authorization; returns empty strings when absent or unparseable.
func ParseAppleUserName(userJSON string) (first, last string) {
	if strings.TrimSpace(userJSON) == "" {
		return "", ""
	}
	var u struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil {
		return "", ""
	}
	return u.Name.FirstName, u.Name.LastName
}
