// Package clifconfig manages bifu-cli configuration stored at ~/.bifu-cli/config.yaml.
// This is the single source of truth for all CLI settings: endpoints, auth, profiles.
package clifconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigDir  = ".bifu-cli"
	DefaultConfigFile = "config.yaml"
)

// CLIConfig is the root configuration for bifu-cli.
// Supports multiple named profiles (like AWS CLI profiles).
type CLIConfig struct {
	// Active profile name
	ActiveProfile string `yaml:"active_profile"`
	// Named profiles (key = profile name)
	Profiles map[string]*Profile `yaml:"profiles"`
}

// Profile holds all settings for a single environment/account context.
type Profile struct {
	Name string `yaml:"name"` // Human-readable label

	// ── Endpoints ────────────────────────────────────────────────────────────
	BaseURL      string `yaml:"base_url"`      // HTTP API base (e.g. https://api.bifu.dev)
	WebSocketURL string `yaml:"websocket_url"` // WS base (e.g. wss://api.bifu.dev)
	GrpcSpot     string `yaml:"grpc_spot"`     // Spot gRPC addr (host:port)
	GrpcContract string `yaml:"grpc_contract"` // Contract gRPC addr (host:port)

	// ── API path prefixes ─────────────────────────────────────────────────────
	PublicPath  string `yaml:"public_path"`  // default: /api/v1/public
	PrivatePath string `yaml:"private_path"` // default: /api/v1/private
        WSMarket    string `yaml:"ws_market"`    // default: /api/v1/public/ws
        WSPrivate   string `yaml:"ws_private"`   // default: /api/v1/private/contract/ws (or /api/v1/private/spot/ws)

	// ── Authentication ────────────────────────────────────────────────────────
	Auth AuthProfile `yaml:"auth"`

	// ── MT5 / Forex ───────────────────────────────────────────────────────────
	Forex ForexProfile `yaml:"forex"`

	// ── Pushgw (realtime forex quotes) ───────────────────────────────────────
	Pushgw PushgwProfile `yaml:"pushgw"`

	// ── HTTP client ───────────────────────────────────────────────────────────
	HTTPTimeout time.Duration `yaml:"http_timeout"` // default: 30s
}

// AuthProfile holds credential settings for a profile.
type AuthProfile struct {
	// Cookie auth (copied from browser DevTools)
	AuthCookie string `yaml:"auth_cookie"`

	// User identity
	UserID string `yaml:"user_id"`

	// Spot trading API keys (HMAC-SHA256)
	SpotAccessKey string `yaml:"spot_access_key"`
	SpotSecretKey string `yaml:"spot_secret_key"`
	SpotAccountID string `yaml:"spot_account_id"`

	// Contract trading API keys
	ContractAccessKey string `yaml:"contract_access_key"`
	ContractSecretKey string `yaml:"contract_secret_key"`
	ContractAccountID string `yaml:"contract_account_id"`

	// Token-based auth (gateway)
	UToken string `yaml:"u_token"`
	VToken string `yaml:"v_token"`

	// Common headers
	Locale       string `yaml:"locale"`        // e.g. "en", "zh-CN"
	TerminalType string `yaml:"terminal_type"` // e.g. "API"
}

// ForexProfile holds MT5 connection settings.
type ForexProfile struct {
	HTTPEndpoint    string `yaml:"http_endpoint"`
	ManagerGrpcAddr string `yaml:"manager_grpc_addr"`
}

// PushgwProfile holds real-time forex quote WebSocket settings.
type PushgwProfile struct {
	WSEndpoint string `yaml:"ws_endpoint"`
	WSPath     string `yaml:"ws_path"` // e.g. /pushgw/ws
}

// ── Defaults ──────────────────────────────────────────────────────────────────

func defaultProfile(name string) *Profile {
	return &Profile{
		Name:         name,
		PublicPath:   "/api/v1/public",
		PrivatePath:  "/api/v1/private",
		WSMarket:     "/api/v1/public/ws",
		WSPrivate:    "/api/v1/private/contract/ws",
		HTTPTimeout:  30 * time.Second,
		Auth: AuthProfile{
			Locale:       "en",
			TerminalType: "API",
		},
	}
}

// ── File helpers ──────────────────────────────────────────────────────────────

// ConfigDir returns the directory where the config file lives.
// Override with BIFU_CLI_HOME env var.
func ConfigDir() string {
	if v := os.Getenv("BIFU_CLI_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultConfigDir)
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultConfigFile)
}

// Load reads the CLI config file.  If it doesn't exist a default config with
// a "default" profile is returned (not written to disk).
func Load() (*CLIConfig, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg CLIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = "default"
	}
	return &cfg, nil
}

// Save writes the config back to disk, creating the directory if needed.
func (c *CLIConfig) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(ConfigPath(), data, 0o600)
}

// Active returns the currently active profile, creating a default if absent.
func (c *CLIConfig) Active() *Profile {
	p, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		p = defaultProfile(c.ActiveProfile)
		c.Profiles[c.ActiveProfile] = p
	}
	return p
}

// SetActive switches the active profile.
func (c *CLIConfig) SetActive(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile %q does not exist", name)
	}
	c.ActiveProfile = name
	return nil
}

// EnsureProfile creates a profile if it doesn't exist and returns it.
func (c *CLIConfig) EnsureProfile(name string) *Profile {
	if p, ok := c.Profiles[name]; ok {
		return p
	}
	p := defaultProfile(name)
	c.Profiles[name] = p
	return p
}

// ── Derived helpers used by API clients ───────────────────────────────────────

// GetPublicURL builds a full URL for a public path.
func (p *Profile) GetPublicURL(path string) string {
	return p.BaseURL + p.PublicPath + path
}

// GetPrivateURL builds a full URL for a private path.
func (p *Profile) GetPrivateURL(path string) string {
	return p.BaseURL + p.PrivatePath + path
}

// GetPaymentURL builds a URL for payment service endpoints (different prefix).
func (p *Profile) GetPaymentURL(path string) string {
	return p.BaseURL + "/payment" + path
}

// GetWSMarketURL builds the full WebSocket URL for market data.
// If WSMarket is already a full URL (starts with ws:// or wss://), it is returned as-is;
// otherwise it is appended to WebSocketURL.
func (p *Profile) GetWSMarketURL() string {
	if strings.HasPrefix(p.WSMarket, "ws://") || strings.HasPrefix(p.WSMarket, "wss://") {
		return p.WSMarket
	}
	return p.WebSocketURL + p.WSMarket
}

// GetWSPrivateURL builds the full WebSocket URL for private trading events.
// If WSPrivate is already a full URL (starts with ws:// or wss://), it is returned as-is;
// otherwise it is appended to WebSocketURL.
func (p *Profile) GetWSPrivateURL() string {
	if strings.HasPrefix(p.WSPrivate, "ws://") || strings.HasPrefix(p.WSPrivate, "wss://") {
		return p.WSPrivate
	}
	return p.WebSocketURL + p.WSPrivate
}

// GetPushgwWSURL returns the pushgw WebSocket URL.
func (p *Profile) GetPushgwWSURL() string {
	return p.Pushgw.WSEndpoint + p.Pushgw.WSPath
}

// ── Internal ──────────────────────────────────────────────────────────────────

func defaultConfig() *CLIConfig {
	return &CLIConfig{
		ActiveProfile: "default",
		Profiles: map[string]*Profile{
			"default": defaultProfile("default"),
		},
	}
}
