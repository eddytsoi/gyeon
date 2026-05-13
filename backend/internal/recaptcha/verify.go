// Package recaptcha verifies Google reCAPTCHA v3 tokens against the
// site-settings-backed secret + score threshold. The verifier is a no-op
// when the `recaptcha_enabled` setting is false, which is the default; this
// makes the entire form pipeline functional on fresh installs without
// requiring Google credentials.
package recaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gyeon/backend/internal/settings"
)

const verifyURL = "https://www.google.com/recaptcha/api/siteverify"

// ErrFailed is returned when reCAPTCHA reports failure (low score, action
// mismatch, expired token, or HTTP-level failures). Callers should treat this
// as a 4xx — the submission is rejected.
var ErrFailed = errors.New("recaptcha verification failed")

type Verifier struct {
	settings *settings.Service
	client   *http.Client
}

func New(s *settings.Service) *Verifier {
	return &Verifier{
		settings: s,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// SetHTTPClient overrides the default 5s-timeout client. Used by tests.
func (v *Verifier) SetHTTPClient(c *http.Client) { v.client = c }

// Result captures the relevant fields of Google's siteverify response.
type Result struct {
	Success     bool      `json:"success"`
	Score       float64   `json:"score"`
	Action      string    `json:"action"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

// Enabled returns true when the admin has flipped `recaptcha_enabled` to
// "true" AND configured a secret key. When false, Verify returns a zero-
// score success without contacting Google — keeps the form pipeline
// functional on local/dev installs.
func (v *Verifier) Enabled(ctx context.Context) bool {
	if !boolSetting(v.settings, ctx, "recaptcha_enabled") {
		return false
	}
	return strings.TrimSpace(read(v.settings, ctx, "recaptcha_secret_key")) != ""
}

// Verify exchanges the token with Google. Returns (score, nil) when valid;
// (score, ErrFailed) when invalid. When the verifier is disabled, returns
// (0, nil) — callers should not treat this as a passing score for any
// audit purpose.
func (v *Verifier) Verify(ctx context.Context, token, expectedAction string) (float64, error) {
	if !v.Enabled(ctx) {
		return 0, nil
	}
	if token == "" {
		return 0, ErrFailed
	}

	secret := read(v.settings, ctx, "recaptcha_secret_key")
	minScore := floatSetting(v.settings, ctx, "recaptcha_min_score", 0.5)

	body := url.Values{"secret": {secret}, "response": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return 0, fmt.Errorf("build recaptcha request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call recaptcha: %w", err)
	}
	defer resp.Body.Close()

	var r Result
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("decode recaptcha response: %w", err)
	}
	if !r.Success {
		return r.Score, ErrFailed
	}
	if expectedAction != "" && r.Action != "" && r.Action != expectedAction {
		return r.Score, ErrFailed
	}
	if r.Score < minScore {
		return r.Score, ErrFailed
	}
	return r.Score, nil
}

func read(s *settings.Service, ctx context.Context, key string) string {
	st, err := s.Get(ctx, key)
	if err != nil {
		return ""
	}
	return st.Value
}

func boolSetting(s *settings.Service, ctx context.Context, key string) bool {
	return strings.EqualFold(strings.TrimSpace(read(s, ctx, key)), "true")
}

func floatSetting(s *settings.Service, ctx context.Context, key string, fallback float64) float64 {
	v := strings.TrimSpace(read(s, ctx, key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
