// Package client provides low-level HTTP and WebSocket clients for bifu-cli.
package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"bifu-cli/pkg/clifconfig"
)

// ── Auth ──────────────────────────────────────────────────────────────────────

// AuthMode defines which credential style is used for a request.
type AuthMode int

const (
	AuthNone    AuthMode = iota
	AuthCookie           // user_auth_name cookie (payment / forex)
	AuthAPIKey           // ACCESS-KEY + HMAC-SHA256 signature (spot / contract)
	AuthUToken           // u-token header (new gateway)
)

// Auth holds resolved credentials for a single request.
type Auth struct {
	AccessKey string
	Timestamp string
	Signature string
}

// AuthManager resolves credentials from a profile.
type AuthManager struct {
	mu      sync.RWMutex
	profile *clifconfig.AuthProfile
	mode    AuthMode
}

// NewAuthManager creates an AuthManager from the active profile auth section.
func NewAuthManager(auth *clifconfig.AuthProfile) *AuthManager {
	mode := AuthNone
	switch {
	case auth.SpotAccessKey != "":
		mode = AuthAPIKey
	case auth.UToken != "":
		mode = AuthUToken
	case auth.AuthCookie != "":
		mode = AuthCookie
	}
	return &AuthManager{profile: auth, mode: mode}
}

// NewContractAuthManager is like NewAuthManager but prefers contract keys.
func NewContractAuthManager(auth *clifconfig.AuthProfile) *AuthManager {
	am := NewAuthManager(auth)
	if auth.ContractAccessKey != "" {
		am.mode = AuthAPIKey
	}
	return am
}

// SignAPIKey builds an HMAC-SHA256 signature: sign(timestamp, method, path, body).
func SignAPIKey(secretKey, timestamp, method, path, body string) string {
	msg := timestamp + method + path
	if body != "" {
		msg += body
	}
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// ApplySpot sets Spot API-Key auth headers on an existing request.
func (am *AuthManager) ApplySpot(req *http.Request, bodyStr string) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := SignAPIKey(am.profile.SpotSecretKey, ts, req.Method, req.URL.Path, bodyStr)
	req.Header.Set("ACCESS-KEY", am.profile.SpotAccessKey)
	req.Header.Set("ACCESS-TIMESTAMP", ts)
	req.Header.Set("ACCESS-SIGN", sig)
	req.Header.Set("terminalType", am.profile.TerminalType)
	req.Header.Set("locale", am.profile.Locale)
}

// ApplyContract sets Contract API-Key auth headers.
func (am *AuthManager) ApplyContract(req *http.Request, bodyStr string) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	key := am.profile.ContractAccessKey
	sec := am.profile.ContractSecretKey
	if key == "" {
		key = am.profile.SpotAccessKey
		sec = am.profile.SpotSecretKey
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := SignAPIKey(sec, ts, req.Method, req.URL.Path, bodyStr)
	req.Header.Set("ACCESS-KEY", key)
	req.Header.Set("ACCESS-TIMESTAMP", ts)
	req.Header.Set("ACCESS-SIGN", sig)
	req.Header.Set("terminalType", am.profile.TerminalType)
	req.Header.Set("locale", am.profile.Locale)
}

// ApplyCookie sets the user_auth_name cookie header.
func (am *AuthManager) ApplyCookie(req *http.Request) {
	am.mu.RLock()
	defer am.mu.RUnlock()
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
}

// NewHTTPClient creates a client using the Spot credential set.
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

// NewContractHTTPClient creates a client using the Contract credential set.
func NewContractHTTPClient(profile *clifconfig.Profile) *HTTPClient {
	c := NewHTTPClient(profile)
	c.auth = NewContractAuthManager(&profile.Auth)
	return c
}

// NewPaymentHTTPClient creates a client using cookie auth (payment/forex endpoints).
func NewPaymentHTTPClient(profile *clifconfig.Profile) *HTTPClient {
	c := NewHTTPClient(profile)
	return c
}

// HTTPResponse wraps the raw HTTP response.
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Duration   time.Duration
}

// GetSpot performs an authenticated GET for Spot endpoints.
func (c *HTTPClient) GetSpot(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil, true, false)
}

// GetContract performs an authenticated GET for Contract endpoints.
func (c *HTTPClient) GetContract(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil, false, true)
}

// GetPayment performs a cookie-authenticated GET for Payment endpoints.
func (c *HTTPClient) GetPayment(rawURL string, params map[string]string) (*HTTPResponse, error) {
	return c.do("GET", rawURL, params, nil, false, false)
}

// PostSpot performs an authenticated POST for Spot endpoints.
func (c *HTTPClient) PostSpot(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body, true, false)
}

// PostContract performs an authenticated POST for Contract endpoints.
func (c *HTTPClient) PostContract(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body, false, true)
}

// PostPayment performs a cookie-authenticated POST for Payment endpoints.
func (c *HTTPClient) PostPayment(rawURL string, body interface{}) (*HTTPResponse, error) {
	return c.do("POST", rawURL, nil, body, false, false)
}

func (c *HTTPClient) do(method, rawURL string, params map[string]string, body interface{}, signSpot, signContract bool) (*HTTPResponse, error) {
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

	// Apply auth
	switch {
	case signSpot:
		c.auth.ApplySpot(req, bodyStr)
	case signContract:
		c.auth.ApplyContract(req, bodyStr)
	default:
		c.auth.ApplyCookie(req)
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
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
