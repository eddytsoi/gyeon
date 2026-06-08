// Package printnode integrates with the PrintNode REST API
// (https://www.printnode.com) to remote-print receipt PDFs on a physical
// printer attached to a machine running the PrintNode client software.
//
// Auth: HTTP Basic with the API key as the username and an empty password
// (req.SetBasicAuth(apiKey, "")). Credentials and the target printer are read
// from site_settings on every call so toggling them in admin takes effect
// without a restart — mirrors the shipany/payment clients.
package printnode

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gyeon/backend/internal/settings"
)

const defaultBaseURL = "https://api.printnode.com"

// Setting keys (all admin-only — never add printnode_api_key to publicSettingKeys).
const (
	settingEnabled   = "printnode_enabled"
	settingAPIKey    = "printnode_api_key"
	settingPrinterID = "printnode_printer_id"
	settingCopies    = "printnode_copies"
)

// ErrNotConfigured is returned when printnode_api_key is empty.
var ErrNotConfigured = errors.New("printnode is not configured")

// settingsReader is the slice of settings.Service the client needs. Narrowed
// to an interface so the client is unit-testable without a database.
type settingsReader interface {
	Get(ctx context.Context, key string) (*settings.Setting, error)
}

// APIError is returned by do() when PrintNode responds with status >= 400.
// PrintNode error bodies look like {code, message, uid}; we best-effort parse
// them and fall back to the verbatim body otherwise.
type APIError struct {
	Status  int
	Code    string
	Message string
	Raw     string
	Method  string
	Route   string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Raw
	}
	if e.Code != "" {
		msg = e.Code + ": " + msg
	}
	return fmt.Sprintf("printnode %s %s: %d %s", e.Method, e.Route, e.Status, strings.TrimSpace(msg))
}

// Printer is the subset of the PrintNode printer object the admin UI needs to
// let an operator pick a printer ID.
type Printer struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Computer struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"computer"`
}

// Client talks to the PrintNode REST API. Safe for concurrent use.
type Client struct {
	settings settingsReader
	hc       *http.Client
	// baseURL is overridable so tests can point at a mock server.
	baseURL string
}

func NewClient(s *settings.Service) *Client {
	return &Client{
		settings: s,
		hc:       &http.Client{Timeout: 20 * time.Second},
		baseURL:  defaultBaseURL,
	}
}

// ── Settings accessors (read fresh per call) ──────────────────────────────

func (c *Client) read(ctx context.Context, key string) string {
	st, err := c.settings.Get(ctx, key)
	if err != nil || st == nil {
		return ""
	}
	return strings.TrimSpace(st.Value)
}

// Enabled reports whether auto-print on paid is switched on.
func (c *Client) Enabled(ctx context.Context) bool {
	return strings.EqualFold(c.read(ctx, settingEnabled), "true")
}

// Configured reports whether the client has the minimum to print: an API key
// and a printer id. Used by the manual-print endpoint for an early 400.
func (c *Client) Configured(ctx context.Context) bool {
	return c.read(ctx, settingAPIKey) != "" && c.PrinterID(ctx) != 0
}

func (c *Client) apiKey(ctx context.Context) string { return c.read(ctx, settingAPIKey) }

// PrinterID returns the configured printer id, or 0 if unset/invalid.
func (c *Client) PrinterID(ctx context.Context) int {
	n, _ := strconv.Atoi(c.read(ctx, settingPrinterID))
	return n
}

// Copies returns the configured copy count, defaulting to 1.
func (c *Client) Copies(ctx context.Context) int {
	n, _ := strconv.Atoi(c.read(ctx, settingCopies))
	if n < 1 {
		return 1
	}
	return n
}

// ── API calls ──────────────────────────────────────────────────────────────

// ListPrinters returns every printer on the account (GET /printers).
func (c *Client) ListPrinters(ctx context.Context) ([]Printer, error) {
	var printers []Printer
	if _, err := c.do(ctx, http.MethodGet, "/printers", nil, &printers); err != nil {
		return nil, err
	}
	return printers, nil
}

// SubmitPDF submits a print job for the given PDF bytes (POST /printjobs) and
// returns the PrintNode job id. The body is base64-encoded with
// contentType "pdf_base64"; fit_to_page lets the printer scale an A4 receipt.
func (c *Client) SubmitPDF(ctx context.Context, printerID int, title string, pdf []byte, copies int) (int64, error) {
	if printerID == 0 {
		return 0, errors.New("printnode: printerID is 0")
	}
	if copies < 1 {
		copies = 1
	}
	body := map[string]any{
		"printerId":   printerID,
		"title":       title,
		"contentType": "pdf_base64",
		"content":     base64.StdEncoding.EncodeToString(pdf),
		"source":      "Gyeon",
		"options": map[string]any{
			"copies":      copies,
			"fit_to_page": true,
		},
	}
	// On success PrintNode returns a bare integer job id.
	var jobID int64
	if _, err := c.do(ctx, http.MethodPost, "/printjobs", body, &jobID); err != nil {
		return 0, err
	}
	return jobID, nil
}

func (c *Client) do(ctx context.Context, method, route string, body, out any) (int, error) {
	key := c.apiKey(ctx)
	if key == "" {
		return 0, ErrNotConfigured
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	full := strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(route, "/")
	req, err := http.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return 0, err
	}
	// PrintNode auth: API key as username, empty password.
	req.SetBasicAuth(key, "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, fmt.Errorf("printnode %s %s: %w", method, route, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		apiErr := &APIError{Status: resp.StatusCode, Method: method, Route: route, Raw: string(respBody)}
		var env struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &env) == nil {
			apiErr.Code = env.Code
			apiErr.Message = env.Message
		}
		return resp.StatusCode, apiErr
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode printnode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}
