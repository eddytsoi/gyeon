package printnode

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gyeon/backend/internal/settings"
)

// fakeSettings is an in-memory settingsReader for tests.
type fakeSettings map[string]string

func (f fakeSettings) Get(_ context.Context, key string) (*settings.Setting, error) {
	return &settings.Setting{Key: key, Value: f[key]}, nil
}

func newTestClient(srv *httptest.Server, s fakeSettings) *Client {
	c := NewClient(nil)
	c.settings = s
	c.baseURL = srv.URL
	return c
}

func TestSubmitPDF_AuthAndBareIntegerResponse(t *testing.T) {
	var gotAuthUser, gotAuthPass string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/printjobs" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		// PrintNode returns 201 with a bare integer job id.
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "623")
	}))
	defer srv.Close()

	c := newTestClient(srv, fakeSettings{settingAPIKey: "test-key", settingPrinterID: "34"})
	jobID, err := c.SubmitPDF(context.Background(), 34, "Receipt 1001", []byte("%PDF-1.4 fake"), 2)
	if err != nil {
		t.Fatalf("SubmitPDF: %v", err)
	}
	if jobID != 623 {
		t.Errorf("jobID = %d, want 623", jobID)
	}
	// API key is the basic-auth username; password empty.
	if gotAuthUser != "test-key" || gotAuthPass != "" {
		t.Errorf("basic auth = %q:%q, want test-key:(empty)", gotAuthUser, gotAuthPass)
	}
	if gotBody["contentType"] != "pdf_base64" {
		t.Errorf("contentType = %v, want pdf_base64", gotBody["contentType"])
	}
	// content must be base64 of the raw PDF bytes.
	want := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 fake"))
	if gotBody["content"] != want {
		t.Errorf("content not base64 of pdf bytes")
	}
	opts, _ := gotBody["options"].(map[string]any)
	if opts["fit_to_page"] != true {
		t.Errorf("options.fit_to_page = %v, want true", opts["fit_to_page"])
	}
	if opts["copies"] != float64(2) {
		t.Errorf("options.copies = %v, want 2", opts["copies"])
	}
}

func TestListPrinters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":34,"name":"Front desk","state":"online","computer":{"id":1,"name":"POS","state":"connected"}}]`)
	}))
	defer srv.Close()

	c := newTestClient(srv, fakeSettings{settingAPIKey: "k"})
	printers, err := c.ListPrinters(context.Background())
	if err != nil {
		t.Fatalf("ListPrinters: %v", err)
	}
	if len(printers) != 1 || printers[0].ID != 34 || printers[0].State != "online" {
		t.Fatalf("unexpected printers: %+v", printers)
	}
}

func TestUnauthorizedSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"code":"Unauthorized","message":"bad api key","uid":"abc"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv, fakeSettings{settingAPIKey: "wrong"})
	_, err := c.ListPrinters(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
	var apiErr *APIError
	if !errorAs(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusUnauthorized || apiErr.Code != "Unauthorized" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

func TestNotConfiguredWhenNoKey(t *testing.T) {
	c := newTestClient(httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})), fakeSettings{})
	if _, err := c.ListPrinters(context.Background()); err != ErrNotConfigured {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

func TestTestPagePDFIsWellFormed(t *testing.T) {
	pdf := testPagePDF()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("missing %PDF- header")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Error("missing EOF trailer")
	}
	if !bytes.Contains(pdf, []byte("startxref")) {
		t.Error("missing startxref")
	}
}

// errorAs is a tiny stand-in for errors.As to avoid an extra import line just
// for the test (kept local so the file's intent stays obvious).
func errorAs(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
