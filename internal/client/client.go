// Package client provides low-level HTTP and WebSocket clients for bifu-cli.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"

	"bifu-cli/internal/clifconfig"
)

// ShowSpinner controls whether a progress spinner is shown on stderr during
// HTTP requests. It is disabled for JSON output (set from the root command) and
// auto-suppressed for non-terminals / verbose mode.
var ShowSpinner = true

// startSpinner shows a spinner on stderr and returns a stop function. It is a
// no-op when spinners are disabled, output is not a terminal, or verbose mode
// is on (verbose prints its own request logs).
func startSpinner(label string, verbose bool) func() {
	if !ShowSpinner || verbose || !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = " " + label
	s.Start()
	return s.Stop
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// AuthManager resolves credentials from a profile. All authenticated endpoints
// (spot / contract / payment / forex) use the same user_auth_name session cookie
// obtained via `bifu-cli auth login`.
type AuthManager struct {
	mu      sync.RWMutex
	profile *clifconfig.AuthProfile
}

// NewAuthManager creates an AuthManager from the active profile auth section.
func NewAuthManager(auth *clifconfig.AuthProfile) *AuthManager {
	return &AuthManager{profile: auth}
}

// ApplyCookie sets the user_auth_name cookie header.
func (am *AuthManager) ApplyCookie(req *http.Request) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	am.applyCookieLocked(req)
}

func (am *AuthManager) applyCookieLocked(req *http.Request) {
	if am.profile.AuthCookie != "" {
		req.Header.Set("Cookie", am.profile.AuthCookieName+"="+am.profile.AuthCookie)
	}
}

// ── HTTP Client ───────────────────────────────────────────────────────────────

// HTTPClient wraps net/http.Client with profile-aware auth.
type HTTPClient struct {
	http    *http.Client
	profile *clifconfig.Profile
	auth    *AuthManager
	Verbose bool
}

// SetVerbose enables or disables HTTP request/response logging.
func (c *HTTPClient) SetVerbose(v bool) { c.Verbose = v }

// NewHTTPClient creates a profile-aware HTTP client. All authenticated
// endpoints (spot / contract / payment / forex) share the same session-cookie
// auth, so a single constructor serves every API client.
func NewHTTPClient(profile *clifconfig.Profile) *HTTPClient {
	timeout := profile.HTTPTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &HTTPClient{
		http:    &http.Client{Timeout: timeout},
		profile: profile,
		auth:    NewAuthManager(&profile.Auth),
	}
}

// HTTPResponse wraps the raw HTTP response.
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Duration   time.Duration
}

// GetSpot performs an authenticated GET for Spot endpoints.
func (c *HTTPClient) GetSpot(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil)
}

// GetContract performs an authenticated GET for Contract endpoints.
func (c *HTTPClient) GetContract(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil)
}

// GetPayment performs a cookie-authenticated GET for Payment endpoints.
func (c *HTTPClient) GetPayment(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil)
}

// PostSpot performs an authenticated POST for Spot endpoints.
func (c *HTTPClient) PostSpot(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body)
}

// PostContract performs an authenticated POST for Contract endpoints.
func (c *HTTPClient) PostContract(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body)
}

// PostPayment performs a cookie-authenticated POST for Payment endpoints.
func (c *HTTPClient) PostPayment(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body)
}

func (c *HTTPClient) do(method, rawURL string, params map[string]string, body interface{}) (*HTTPResponse, error) {
	// Build query string
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %w", rawURL, err)
	}
	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	// Serialise body
	var rawBody []byte
	var bodyStr string
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		rawBody = b
		bodyStr = string(b)
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[HTTP] %s %s\n", method, u.String())
		if bodyStr != "" {
			fmt.Fprintf(os.Stderr, "[HTTP]   body: %s\n", bodyStr)
		}
	}

	// Retry only idempotent GETs — never replay a POST (would risk a double
	// order). Transient failures (network error, 5xx, or a 200 carrying the
	// backend's intermittent "UNKNOWN" code) are retried with a short backoff so
	// a momentary server blip doesn't surface as a hard error to the user.
	attempts := 1
	if method == "GET" {
		attempts = 3
	}

	var data []byte
	var statusCode int
	start := time.Now()
	for attempt := 1; ; attempt++ {
		req, err := http.NewRequest(method, u.String(), bytesReader(rawBody))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		c.auth.ApplyCookie(req)

		stop := startSpinner(method+" "+u.Path, c.Verbose)
		resp, derr := c.http.Do(req)
		if derr != nil {
			stop()
			if attempt < attempts {
				time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("request: %w", derr)
		}
		data, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		stop()
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		statusCode = resp.StatusCode

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[HTTP] <- %d (%dms) body: %s\n",
				statusCode, time.Since(start).Milliseconds(), truncate(string(data), 500))
		}

		if attempt < attempts && isTransient(statusCode, data) {
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
			continue
		}
		break
	}

	switch {
	case statusCode == 401:
		return nil, fmt.Errorf("authentication failed (HTTP 401): session expired or invalid — run `bifu-cli auth login`")
	case statusCode == 403:
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = "access denied"
		}
		return nil, fmt.Errorf("access denied (HTTP 403): %s", msg)
	case statusCode == 404:
		return nil, fmt.Errorf("endpoint not found (HTTP 404): %s", rawURL)
	case statusCode >= 500:
		return nil, fmt.Errorf("server error (HTTP %d) — please retry in a moment", statusCode)
	case statusCode >= 400:
		return nil, fmt.Errorf("HTTP error %d: %s", statusCode, strings.TrimSpace(string(data)))
	}

	return &HTTPResponse{
		StatusCode: statusCode,
		Body:       data,
		Duration:   time.Since(start),
	}, nil
}

// bytesReader returns a fresh reader over b (nil-safe), so a request body can be
// rebuilt for each retry attempt.
func bytesReader(b []byte) io.Reader {
	if b == nil {
		return nil
	}
	return bytes.NewReader(b)
}

// isTransient reports whether a response is worth retrying: a 5xx, or a 200 that
// carries the backend's intermittent "UNKNOWN" envelope code (seen sporadically
// on read endpoints).
func isTransient(status int, body []byte) bool {
	if status >= 500 {
		return true
	}
	if status == 200 {
		return bytes.Contains(body, []byte(`"code":"UNKNOWN"`)) ||
			bytes.Contains(body, []byte(`"code": "UNKNOWN"`))
	}
	return false
}

// ── JSON helpers ──────────────────────────────────────────────────────────────

// APIResponse is the generic envelope used by spot/contract APIs.
type APIResponse struct {
	Code    string      `json:"code"`
	Message interface{} `json:"message"`
	Data    interface{} `json:"data"`
}

func (r *APIResponse) GetMessage() string {
	switch v := r.Message.(type) {
	case string:
		return v
	case map[string]interface{}:
		if m, ok := v["message"].(string); ok {
			return m
		}
	}
	return ""
}

// ParseAPIResponse unmarshals raw bytes into APIResponse and checks for errors.
func ParseAPIResponse(raw []byte, dst interface{}) error {
	var resp APIResponse
	resp.Data = dst
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("unmarshal: %w (body: %s)", err, truncate(string(raw), 200))
	}
	if resp.Code != "SUCCESS" && resp.Code != "" {
		return fmt.Errorf("API error [%s]: %s", resp.Code, resp.GetMessage())
	}
	return nil
}

// PaymentResponse is the generic envelope used by payment service APIs.
type PaymentResponse struct {
	RetCode interface{} `json:"retCode"`
	RetMsg  string      `json:"retMsg"`
	Result  interface{} `json:"result"`
}

// ParsePaymentResponse unmarshals a payment API envelope.
func ParsePaymentResponse(raw []byte, dst interface{}) error {
	// First pass: get retCode/retMsg
	var shell struct {
		RetCode interface{} `json:"retCode"`
		RetMsg  string      `json:"retMsg"`
	}
	if err := json.Unmarshal(raw, &shell); err != nil {
		return fmt.Errorf("unmarshal: %w (body: %s)", err, truncate(string(raw), 200))
	}
	code := fmt.Sprintf("%v", shell.RetCode)
	if code != "0" && code != "<nil>" && code != "" {
		return fmt.Errorf("payment error [%s]: %s", code, shell.RetMsg)
	}
	// Second pass: unmarshal result into dst
	var wrapper struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if dst != nil && len(wrapper.Result) > 0 {
		if err := json.Unmarshal(wrapper.Result, dst); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
