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
		req.Header.Set("Cookie", "user_auth_name="+am.profile.AuthCookie)
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
	var bodyReader io.Reader
	var bodyStr string
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyStr = string(b)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply auth — all authenticated endpoints use the session cookie.
	c.auth.ApplyCookie(req)

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[HTTP] %s %s\n", method, u.String())
		for k, vs := range req.Header {
			val := strings.Join(vs, ", ")
			// Never log session credentials (would leak into shell history / CI logs).
			if h := strings.ToLower(k); h == "cookie" || h == "authorization" {
				val = "<redacted>"
			}
			fmt.Fprintf(os.Stderr, "[HTTP]   %s: %s\n", k, val)
		}
		if bodyStr != "" {
			fmt.Fprintf(os.Stderr, "[HTTP]   body: %s\n", bodyStr)
		}
	}

	stop := startSpinner(method+" "+u.Path, c.Verbose)
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		stop()
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	stop()
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[HTTP] <- %d (%dms) body: %s\n",
			resp.StatusCode, time.Since(start).Milliseconds(), truncate(string(data), 500))
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed (HTTP 401): session expired or invalid — run `bifu-cli auth login`")
	}
	if resp.StatusCode == 403 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = "access denied"
		}
		return nil, fmt.Errorf("access denied (HTTP 403): %s", msg)
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("endpoint not found (HTTP 404): %s", rawURL)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       data,
		Duration:   time.Since(start),
	}, nil
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
