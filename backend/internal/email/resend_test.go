package email

import (
	"encoding/json"
	"errors"
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

func TestSendBatchViaResend_Success(t *testing.T) {
	var gotAuth, gotCT, gotPath string
	var gotBody []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		// data order matches the request order.
		_, _ = w.Write([]byte(`{"data":[{"id":"id-1"},{"id":"id-2"}]}`))
	}))
	defer srv.Close()

	s := newResendTestService(srv)
	msgs := []resendBatchMsg{
		{From: "GYEON <noreply@gyeon.hk>", To: []string{"a@example.com"}, Subject: "S1", HTML: "<b>1</b>", Text: "t1"},
		{From: "GYEON <noreply@gyeon.hk>", To: []string{"b@example.com"}, Subject: "S2", HTML: "<b>2</b>", Text: "t2"},
	}
	ids, err := s.sendBatchViaResend(resendTestConfig(), msgs)
	if err != nil {
		t.Fatalf("sendBatchViaResend: unexpected error: %v", err)
	}

	if gotPath != "/emails/batch" {
		t.Errorf("path = %q, want /emails/batch", gotPath)
	}
	if gotAuth != "Bearer re_test_key" {
		t.Errorf("Authorization = %q, want Bearer re_test_key", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	// Body must be a TOP-LEVEL array of two distinct messages.
	if len(gotBody) != 2 {
		t.Fatalf("body should be an array of 2 messages, got %d: %v", len(gotBody), gotBody)
	}
	if gotBody[0]["subject"] != "S1" || gotBody[1]["subject"] != "S2" {
		t.Errorf("subjects out of order/mismatch: %v", gotBody)
	}
	to0, ok := gotBody[0]["to"].([]any)
	if !ok || len(to0) != 1 || to0[0] != "a@example.com" {
		t.Errorf("msg0 to = %v, want [a@example.com]", gotBody[0]["to"])
	}
	if gotBody[1]["html"] != "<b>2</b>" || gotBody[1]["text"] != "t2" {
		t.Errorf("msg1 html/text mismatch: %v", gotBody[1])
	}
	// IDs parsed back in request order.
	if len(ids) != 2 || ids[0] != "id-1" || ids[1] != "id-2" {
		t.Errorf("ids = %v, want [id-1 id-2]", ids)
	}
}

func TestSendBatchViaResend_ErrorIncludesResendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"name":"validation_error","message":"too many emails in batch"}`))
	}))
	defer srv.Close()

	s := newResendTestService(srv)
	_, err := s.sendBatchViaResend(resendTestConfig(), []resendBatchMsg{
		{From: "GYEON <noreply@gyeon.hk>", To: []string{"a@example.com"}, Subject: "S", HTML: "<b>h</b>", Text: "t"},
	})
	if err == nil {
		t.Fatal("sendBatchViaResend: expected error on 4xx, got nil")
	}
	if !strings.Contains(err.Error(), "too many emails in batch") {
		t.Errorf("error should carry Resend message, got: %v", err)
	}
	if errors.Is(err, ErrAuth) {
		t.Errorf("422 should NOT be classified as ErrAuth (must stay retryable), got: %v", err)
	}
}

// authErrServer serves the given status with a Resend-style auth error body.
func authErrServer(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"name":"validation_error","message":"API key is invalid"}`))
	}))
}

func TestResend_AuthStatusesAreErrAuth(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := authErrServer(status)
		s := newResendTestService(srv)

		// single-send
		err := s.sendViaResend(resendTestConfig(), "cust@example.com", "", "Hi", "plain", "<b>h</b>")
		if !errors.Is(err, ErrAuth) {
			t.Errorf("sendViaResend status %d: want ErrAuth, got %v", status, err)
		}
		if !strings.Contains(err.Error(), "API key is invalid") {
			t.Errorf("sendViaResend status %d: error should carry Resend message, got %v", status, err)
		}

		// batch
		_, berr := s.sendBatchViaResend(resendTestConfig(), []resendBatchMsg{
			{From: "GYEON <noreply@gyeon.hk>", To: []string{"a@example.com"}, Subject: "S", HTML: "<b>h</b>", Text: "t"},
		})
		if !errors.Is(berr, ErrAuth) {
			t.Errorf("sendBatchViaResend status %d: want ErrAuth, got %v", status, berr)
		}
		srv.Close()
	}
}
