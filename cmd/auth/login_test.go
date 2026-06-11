package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

func TestExtractCookieValue(t *testing.T) {
	cases := map[string]string{
		`{"Name":"user_auth_name","Value":"abc123=="}`: "abc123==",         // JSON http.Cookie
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
// implementing the QR-login endpoints: qr_code_get issues an issue, and
// qr_code_check returns "processing" once before "success".
func TestDeviceLoginFlow(t *testing.T) {
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/login/qr_code_get":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result": map[string]any{
					"url":     "https://bifu.co/x/issue-xyz",
					"issueId": "issue-xyz",
				},
			})
		case "/user/login/qr_code_check":
			n := atomic.AddInt32(&polls, 1)
			if n < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"retCode": "0",
					"result":  map[string]any{"issueStatus": "processing"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"retCode": "0",
				"result": map[string]any{
					"issueStatus": "success",
					"cookieStr":   `{"Name":"user_auth_name","Value":"REALcookie=="}`,
					"user":        map[string]any{"userId": "109150807"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Stub the browser opener and shorten polling so no real browser launches
	// and the test runs fast.
	origOpen, origInterval := openBrowser, devicePollInterval
	var openedURL string
	openBrowser = func(u string) error { openedURL = u; return nil }
	devicePollInterval = 10 * time.Millisecond
	defer func() { openBrowser, devicePollInterval = origOpen, origInterval }()

	// Point config at a temp dir and a profile using the mock server.
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	cfg, _ := clifconfig.Load()
	p := cfg.EnsureProfile("default")
	p.BaseURL = srv.URL
	p.WebURL = "https://bifu.dev"
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

	// The CLI should open the WebURL host, not the backend's hard-coded prod URL.
	if openedURL != "https://bifu.dev/x/issue-xyz" {
		t.Errorf("openedURL = %q, want %q", openedURL, "https://bifu.dev/x/issue-xyz")
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
		t.Errorf("expected at least 2 polls (processing then success), got %d", polls)
	}
}
