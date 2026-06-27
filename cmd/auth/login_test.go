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
		`{"Name":"user_auth_name","Value":"abc123=="}`: "abc123==", // JSON http.Cookie
		`rawCookieValue==`: "rawCookieValue==", // not JSON → as-is
		``:                 "",
	}
	for in, want := range cases {
		if got := extractCookieValue(in); got != want {
			t.Errorf("extractCookieValue(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEmailLoginSavesToResolvedProfile verifies that `auth login` writes the
// session cookie to the profile resolved by load() (i.e. the one --profile
// selected) and NOT to the on-disk active profile. Regression test for a bug
// where logging in with --profile dev authenticated against dev but saved the
// cookie to the active "default" profile.
func TestEmailLoginSavesToResolvedProfile(t *testing.T) {
	// Mock backend returns a cookie inline (no 2FA) so no stdin is read.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/login" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"retCode": "0",
			"result": map[string]any{
				"cookieStr": `{"Name":"user_auth_name","Value":"DEVcookie=="}`,
				"user":      map[string]any{"userId": "42"},
			},
		})
	}))
	defer srv.Close()

	// Config has two profiles; active is "default", but we log in as "dev".
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	cfg, _ := clifconfig.Load()
	cfg.EnsureProfile("default")
	dev := cfg.EnsureProfile("dev")
	dev.BaseURL = srv.URL
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// load() simulates `--profile dev`: it returns the dev profile.
	load := func() (*clifconfig.Profile, *output.Printer, error) {
		c, _ := clifconfig.Load()
		return c.Profiles["dev"], output.NewPrinter(output.FormatPlain, false), nil
	}

	cmd := newLoginCmd(load)
	cmd.SetArgs([]string{"--username", "user@example.com", "--password", "secret"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}

	got, _ := clifconfig.Load()
	if c := got.Profiles["dev"].Auth.AuthCookie; c != "DEVcookie==" {
		t.Errorf("dev cookie = %q, want %q", c, "DEVcookie==")
	}
	if c := got.Profiles["default"].Auth.AuthCookie; c != "" {
		t.Errorf("default cookie = %q, want empty (cookie leaked to wrong profile)", c)
	}
	// Logging into "dev" should also make it the active profile.
	if got.ActiveProfile != "dev" {
		t.Errorf("active profile = %q, want %q after logging into dev", got.ActiveProfile, "dev")
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

	// Shorten polling so the test runs fast.
	origInterval := devicePollInterval
	devicePollInterval = 10 * time.Millisecond
	defer func() { devicePollInterval = origInterval }()

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
