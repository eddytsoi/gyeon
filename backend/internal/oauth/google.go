package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	googleAuthEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint = "https://oauth2.googleapis.com/token"
	googleUserInfo      = "https://openidconnect.googleapis.com/v1/userinfo"
)

func (s *Service) googleAuthURL(ctx context.Context) (string, error) {
	state, err := randString(32)
	if err != nil {
		return "", err
	}
	verifier, err := pkceVerifier()
	if err != nil {
		return "", err
	}
	if err := s.saveState(ctx, loginState{State: state, Provider: ProviderGoogle, CodeVerifier: verifier}); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", s.read(ctx, "google_oauth_client_id"))
	q.Set("redirect_uri", s.redirectURI(ctx, ProviderGoogle))
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("code_challenge", pkceChallenge(verifier))
	q.Set("code_challenge_method", "S256")
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return googleAuthEndpoint + "?" + q.Encode(), nil
}

func (s *Service) googleExchange(ctx context.Context, code string, st *loginState) (*UserInfo, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", s.read(ctx, "google_oauth_client_id"))
	form.Set("client_secret", s.read(ctx, "google_oauth_client_secret"))
	form.Set("redirect_uri", s.redirectURI(ctx, ProviderGoogle))
	form.Set("code_verifier", st.CodeVerifier)

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := s.postForm(ctx, googleTokenEndpoint, form, &tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("google token exchange failed: %s %s", tok.Error, tok.ErrorDesc)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfo, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var ui struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}
	if err := json.Unmarshal(body, &ui); err != nil {
		return nil, err
	}
	if ui.Sub == "" || ui.Email == "" || !ui.EmailVerified {
		return nil, ErrNoEmail
	}
	return &UserInfo{
		Provider:      ProviderGoogle,
		Subject:       ui.Sub,
		Email:         strings.ToLower(ui.Email),
		EmailVerified: true,
		FirstName:     ui.GivenName,
		LastName:      ui.FamilyName,
	}, nil
}

// postForm posts x-www-form-urlencoded data and decodes the JSON body into out.
// Token endpoints return JSON error fields on 4xx too, so the body is decoded
// regardless of status code.
func (s *Service) postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode token response (http %d): %w", resp.StatusCode, err)
	}
	return nil
}
