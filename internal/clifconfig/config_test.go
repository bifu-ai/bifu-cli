package clifconfig

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ActiveProfile != "default" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "default")
	}
	if _, ok := cfg.Profiles["default"]; !ok {
		t.Error("expected a default profile")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())

	cfg := defaultConfig()
	p := cfg.Active()
	p.BaseURL = "https://api.example.dev"
	p.Auth.AuthCookie = "cookie=="
	p.Auth.UserID = "123"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gp := got.Active()
	if gp.BaseURL != "https://api.example.dev" {
		t.Errorf("BaseURL = %q, want %q", gp.BaseURL, "https://api.example.dev")
	}
	if gp.Auth.AuthCookie != "cookie==" {
		t.Errorf("AuthCookie = %q, want %q", gp.Auth.AuthCookie, "cookie==")
	}
	if gp.Auth.UserID != "123" {
		t.Errorf("UserID = %q, want %q", gp.Auth.UserID, "123")
	}
}

func TestConfigFilePermissions(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	cfg := defaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(ConfigPath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perm = %o, want 600", perm)
	}
}

func TestSetActive(t *testing.T) {
	cfg := defaultConfig()
	cfg.EnsureProfile("staging")

	if err := cfg.SetActive("staging"); err != nil {
		t.Fatalf("SetActive(staging): %v", err)
	}
	if cfg.ActiveProfile != "staging" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "staging")
	}
	if err := cfg.SetActive("nope"); err == nil {
		t.Error("SetActive(nope): expected error for missing profile")
	}
}

func TestEnsureProfileIsIdempotent(t *testing.T) {
	cfg := defaultConfig()
	p1 := cfg.EnsureProfile("dev")
	p1.BaseURL = "https://dev.example"
	p2 := cfg.EnsureProfile("dev")
	if p1 != p2 {
		t.Error("EnsureProfile returned a different profile on second call")
	}
	if p2.BaseURL != "https://dev.example" {
		t.Errorf("BaseURL = %q, want existing profile preserved", p2.BaseURL)
	}
}

func TestActiveCreatesMissingProfile(t *testing.T) {
	cfg := &CLIConfig{ActiveProfile: "ghost", Profiles: map[string]*Profile{}}
	p := cfg.Active()
	if p == nil {
		t.Fatal("Active returned nil")
	}
	if p.Name != "ghost" {
		t.Errorf("Name = %q, want %q", p.Name, "ghost")
	}
	if p.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout = %v, want default 30s", p.HTTPTimeout)
	}
}

func TestGenerateClientOrderID(t *testing.T) {
	ts := time.Date(2026, 6, 28, 12, 30, 45, 0, time.UTC)

	t.Run("with user id", func(t *testing.T) {
		p := &Profile{Auth: AuthProfile{UserID: "109150807"}}
		got := p.GenerateClientOrderID("BTCUSDT", "BUY", ts)
		want := "109150807-btcusdt-buy-20260628123045"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("anonymous fallback", func(t *testing.T) {
		p := &Profile{}
		got := p.GenerateClientOrderID("ETHUSDT", "SELL", ts)
		want := "anon-ethusdt-sell-20260628123045"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("truncated to 64 chars", func(t *testing.T) {
		p := &Profile{Auth: AuthProfile{UserID: strings.Repeat("X", 80)}}
		got := p.GenerateClientOrderID("BTCUSDT", "BUY", ts)
		if len(got) != maxClientOrderIDLen {
			t.Errorf("len = %d, want %d", len(got), maxClientOrderIDLen)
		}
	})
}

func TestURLBuilders(t *testing.T) {
	p := defaultProfile("default")
	p.BaseURL = "https://api.bifu.dev"

	cases := map[string]string{
		p.GetPublicURL("/ticker"): "https://api.bifu.dev/api/v1/public/ticker",
		p.GetPrivateURL("/order"): "https://api.bifu.dev/api/v1/private/order",
		p.GetPaymentURL("/bal"):   "https://api.bifu.dev/payment/bal",
		p.GetOrionURL("/sig"):     "https://api.bifu.dev/orion/sig",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("url = %q, want %q", got, want)
		}
	}
}

func TestWSURLBuilders(t *testing.T) {
	p := defaultProfile("default")
	p.WebSocketURL = "wss://api.bifu.dev"

	// Relative path is appended to the WS base.
	if got, want := p.GetWSMarketURL(), "wss://api.bifu.dev/api/v1/public/ws"; got != want {
		t.Errorf("GetWSMarketURL = %q, want %q", got, want)
	}

	// An absolute ws:// override is returned as-is.
	p.WSPrivate = "wss://other.host/ws"
	if got, want := p.GetWSPrivateURL(), "wss://other.host/ws"; got != want {
		t.Errorf("GetWSPrivateURL = %q, want %q", got, want)
	}
}

func TestTradfiWSFallback(t *testing.T) {
	p := defaultProfile("default")

	// Explicit value wins.
	p.Pushgw.TradfiWS = "wss://explicit/tradfi/ws"
	if got := p.GetTradfiWSURL(); got != "wss://explicit/tradfi/ws" {
		t.Errorf("explicit TradfiWS = %q", got)
	}

	// Falls back to <pushgw endpoint>/tradfi/ws.
	p.Pushgw.TradfiWS = ""
	p.Pushgw.WSEndpoint = "wss://push.bifu.dev"
	if got, want := p.GetTradfiWSURL(), "wss://push.bifu.dev/tradfi/ws"; got != want {
		t.Errorf("fallback TradfiWS = %q, want %q", got, want)
	}

	// Empty when nothing configured.
	p.Pushgw.WSEndpoint = ""
	if got := p.GetTradfiWSURL(); got != "" {
		t.Errorf("unconfigured TradfiWS = %q, want empty", got)
	}
}
