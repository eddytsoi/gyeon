package email

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newResendTestService wires a Service whose Resend transport points at the
// given httptest server. sendViaResend doesn't touch settings, so a nil
// settings.Service is fine here.
func newResendTestService(srv *httptest.Server) *Service {
	return &Service{httpClient: srv.Client(), resendBaseURL: srv.URL}
}

func resendTestConfig() Config {
	return Config{
		Provider:  ProviderResend,
		APIKey:    "re_test_key",
		FromEmail: "noreply@gyeon.hk",
		FromName:  "GYEON",
	}
}

func TestSendViaResend_Success(t *testing.T) {
	var gotAuth, gotCT, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc-123"}`))
	}))
	defer srv.Close()

	s := newResendTestService(srv)
	err := s.sendViaResend(resendTestConfig(), "cust@example.com", "reply@gyeon.hk", "Hi", "plain", "<b>html</b>")
	if err != nil {
		t.Fatalf("sendViaResend: unexpected error: %v", err)
	}

	if gotPath != "/emails" {
		t.Errorf("path = %q, want /emails", gotPath)
	}
	if gotAuth != "Bearer re_test_key" {
		t.Errorf("Authorization = %q, want Bearer re_test_key", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if gotBody["from"] != "GYEON <noreply@gyeon.hk>" {
		t.Errorf("from = %v, want GYEON <noreply@gyeon.hk>", gotBody["from"])
	}
	to, ok := gotBody["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "cust@example.com" {
		t.Errorf("to = %v, want [cust@example.com]", gotBody["to"])
	}
	if gotBody["subject"] != "Hi" || gotBody["html"] != "<b>html</b>" || gotBody["text"] != "plain" {
		t.Errorf("subject/html/text mismatch: %v", gotBody)
	}
	if gotBody["reply_to"] != "reply@gyeon.hk" {
		t.Errorf("reply_to = %v, want reply@gyeon.hk", gotBody["reply_to"])
	}
}

func TestSendViaResend_OmitsEmptyReplyTo(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc-123"}`))
	}))
	defer srv.Close()

	s := newResendTestService(srv)
	if err := s.sendViaResend(resendTestConfig(), "cust@example.com", "", "Hi", "plain", "<b>html</b>"); err != nil {
		t.Fatalf("sendViaResend: unexpected error: %v", err)
	}
	if _, present := gotBody["reply_to"]; present {
		t.Errorf("reply_to should be omitted when empty, got %v", gotBody["reply_to"])
	}
}

func TestSendViaResend_ErrorIncludesResendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"name":"validation_error","message":"The gyeon.hk domain is not verified."}`))
	}))
	defer srv.Close()

	s := newResendTestService(srv)
	err := s.sendViaResend(resendTestConfig(), "cust@example.com", "", "Hi", "plain", "<b>html</b>")
	if err == nil {
		t.Fatal("sendViaResend: expected error on 4xx, got nil")
	}
	if !strings.Contains(err.Error(), "domain is not verified") {
		t.Errorf("error should carry Resend message, got: %v", err)
	}
}
