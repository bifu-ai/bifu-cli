package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

func TestExtractCookieValue(t *testing.T) {
	cases := map[string]string{
		`{"Name":"user_auth_name","Value":"abc123=="}`: "abc123==", // JSON http.Cookie
		`rawCookieValue==`:                             "rawCookieValue==", // not JSON → as-is
		``:                                             "",
	}
	for in, want := range cases {
		if got := extractCookieValue(in); got != want {
			t.Errorf("extractCookieValue(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDeviceLoginFlow drives runDeviceLogin end to end against a mock backend
// that implements the device-flow contract: device_code issues a code, and
// device_token returns "pending" once before "success".
func TestDeviceLoginFlow(t *testing.T) {
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/device_code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result": map[string]any{
					"deviceCode":              "dev-code-xyz",
					"userCode":                "ABCD-1234",
					"verificationUri":         srv0(r) + "/device",
					"verificationUriComplete": srv0(r) + "/device?code=ABCD-1234",
					"expiresIn":               600,
					"interval":                1, // poll fast in the test
				},
			})
		case "/user/device_token":
			n := atomic.AddInt32(&polls, 1)
			if n < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"retCode": "0",
					"result":  map[string]any{"status": "pending"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result": map[string]any{
					"status":    "success",
					"cookieStr": `{"Name":"user_auth_name","Value":"REALcookie=="}`,
					"user":      map[string]any{"userId": "109150807"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Stub the browser opener so no real browser launches.
	origOpen := openBrowser
	var openedURL string
	openBrowser = func(u string) error { openedURL = u; return nil }
	defer func() { openBrowser = origOpen }()

	// Point config at a temp dir and a profile using the mock server.
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	cfg, _ := clifconfig.Load()
	p := cfg.EnsureProfile("default")
	p.BaseURL = srv.URL
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	load := func() (*clifconfig.Profile, *output.Printer, error) {
		c, _ := clifconfig.Load()
		return c.Active(), output.NewPrinter(output.FormatPlain, false), nil
	}

	if err := runDeviceLogin(load); err != nil {
		t.Fatalf("runDeviceLogin: %v", err)
	}

	if openedURL == "" {
		t.Error("browser opener was not called")
	}

	// Verify the cookie + user id were persisted.
	got, _ := clifconfig.Load()
	if c := got.Active().Auth.AuthCookie; c != "REALcookie==" {
		t.Errorf("saved cookie = %q, want %q", c, "REALcookie==")
	}
	if uid := got.Active().Auth.UserID; uid != "109150807" {
		t.Errorf("saved user_id = %q, want %q", uid, "109150807")
	}
	if polls < 2 {
		t.Errorf("expected at least 2 polls (pending then success), got %d", polls)
	}
}

// srv0 builds an absolute base URL from the inbound request (scheme is http in
// httptest), used only to populate verification URLs in the mock response.
func srv0(r *http.Request) string {
	return "http://" + r.Host
}
