// Package clifconfig manages bifu-cli configuration stored at ~/.bifu-cli/config.yaml.
// This is the single source of truth for all CLI settings: endpoints, auth, profiles.
package clifconfig

import (
	"crypto/rand"
	"encoding/hex"
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
// Supports multiple named profiles (like OKX CLI profiles).
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
	WebURL       string `yaml:"web_url"`       // Web app URL (e.g. https://bifu.dev), informational
	WebSocketURL string `yaml:"websocket_url"` // WS base (e.g. wss://api.bifu.dev)
	GrpcSpot     string `yaml:"grpc_spot"`     // Spot gRPC addr (host:port)
	GrpcContract string `yaml:"grpc_contract"` // Contract gRPC addr (host:port)

	// ── API path prefixes ─────────────────────────────────────────────────────
	PublicPath    string `yaml:"public_path"`     // default: /api/v1/public
	PrivatePath   string `yaml:"private_path"`    // default: /api/v1/private
	WSMarket      string `yaml:"ws_market"`       // default: /api/v1/public/ws
	WSPrivate     string `yaml:"ws_private"`      // default: /api/v1/private/contract/ws
	WSPrivateSpot string `yaml:"ws_private_spot"` // default: /api/v1/private/spot/ws

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
	// Session cookie name, captured from the login response. It is
	// environment-specific (dev=user_auth_name, staging/prod differ) per the
	// backend's CookieNameMap, so it must be sent under the right name.
	AuthCookieName string `yaml:"auth_cookie_name"`

	// User identity
	UserID string `yaml:"user_id"`

	// Trading account identifiers (informational; auth is via AuthCookie)
	SpotAccountID     string `yaml:"spot_account_id"`
	ContractAccountID string `yaml:"contract_account_id"`

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
	WSPath     string `yaml:"ws_path"`   // MT5 push gateway, e.g. /pushgw/ws
	TradfiWS   string `yaml:"tradfi_ws"` // TradFi(Fortex) push WS full URL, e.g. wss://fxapi.bifu.dev/tradfi/ws
}

// ── Defaults ──────────────────────────────────────────────────────────────────

func defaultProfile(name string) *Profile {
	return &Profile{
		Name:          name,
		PublicPath:    "/api/v1/public",
		PrivatePath:   "/api/v1/private",
		WSMarket:      "/api/v1/public/ws",
		WSPrivate:     "/api/v1/private/contract/ws",
		WSPrivateSpot: "/api/v1/private/spot/ws",
		HTTPTimeout:   30 * time.Second,
		Auth: AuthProfile{
			Locale:       "en",
			TerminalType: "API",
		},
	}
}

// ── File helpers ──────────────────────────────────────────────────────────────

// ConfigDir returns the directory where the config file lives.
// Override with BIFU_CLI_HOME env var.
//
// The override is validated (BIFU-CLI-202606-025): a symlinked target, or a
// world/group-writable directory, is rejected so a poisoned env var can't point
// the CLI at an attacker-controlled config (with a forged base_url / cookie).
// On validation failure we fall back to the default ~/.bifu-cli with a warning.
func ConfigDir() string {
	if v := os.Getenv("BIFU_CLI_HOME"); v != "" {
		if err := validateConfigDir(v); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring BIFU_CLI_HOME=%q: %v\n", v, err)
		} else {
			return v
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, DefaultConfigDir)
}

// validateConfigDir rejects a BIFU_CLI_HOME that is a symlink or has loose
// permissions. A non-existent dir is allowed (it will be created 0700 on Save).
func validateConfigDir(dir string) error {
	fi, err := os.Lstat(dir) // #nosec G703 -- this IS the BIFU_CLI_HOME validation gate; Lstat only inspects the path (no open/read) to reject symlinks/loose perms
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path is a symlink")
	}
	if !fi.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("directory is group/world-writable (%#o)", fi.Mode().Perm())
	}
	return nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), DefaultConfigFile)
}

// Load reads the CLI config file.  If it doesn't exist a default config with
// a "default" profile is returned (not written to disk).
func Load() (*CLIConfig, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path) // #nosec G304 -- our own config path (~/.bifu-cli or $BIFU_CLI_HOME), not untrusted input
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
	// Transparently decrypt at-rest secrets so the rest of the code works with
	// plaintext in memory (BIFU-CLI-202606-004). A field that can't be decrypted
	// (e.g. config copied from another machine) is cleared so the user is asked
	// to log in again rather than sending a garbage cookie.
	for _, p := range cfg.Profiles {
		if p == nil {
			continue
		}
		if dec, err := decryptSecret(p.Auth.AuthCookie); err == nil {
			p.Auth.AuthCookie = dec
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not decrypt session for profile %q (run `bifu-cli auth login`): %v\n", p.Name, err)
			p.Auth.AuthCookie = ""
		}
	}
	return &cfg, nil
}

// Save writes the config back to disk, creating the directory if needed.
// Secret fields (session cookie) are encrypted at rest (BIFU-CLI-202606-004);
// the in-memory config keeps plaintext, so a shallow clone is encrypted just
// for serialization.
func (c *CLIConfig) Save() error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	out, err := c.encryptedClone()
	if err != nil {
		return fmt.Errorf("encrypt secrets: %w", err)
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(ConfigPath(), data, 0o600)
}

// encryptedClone returns a copy of the config with secret fields encrypted,
// without mutating the live in-memory profiles.
func (c *CLIConfig) encryptedClone() (*CLIConfig, error) {
	clone := &CLIConfig{
		ActiveProfile: c.ActiveProfile,
		Profiles:      make(map[string]*Profile, len(c.Profiles)),
	}
	for name, p := range c.Profiles {
		if p == nil {
			continue
		}
		cp := *p
		enc, err := encryptSecret(cp.Auth.AuthCookie)
		if err != nil {
			return nil, err
		}
		cp.Auth.AuthCookie = enc
		clone.Profiles[name] = &cp
	}
	return clone, nil
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

// maxClientOrderIDLen bounds generated client order IDs (exchange limit).
const maxClientOrderIDLen = 64

// GenerateClientOrderID builds a client order ID of the form
// "<symbol>-<side>-<UTCtimestamp>-<rand>", truncated to the exchange's 64-char
// limit. It is shared by the spot and contract order commands so the format and
// length rule stay in one place.
//
// The user_id is intentionally NOT embedded (BIFU-CLI-202606-009): it leaked the
// account identifier to the exchange on every order. A crypto/rand suffix makes
// the id unique even for two orders placed in the same second (the timestamp is
// only 1s-precision), preventing server-side dedup from dropping a legitimate
// order. ts is passed in (not read from the clock) to keep the timestamp
// component testable.
func (p *Profile) GenerateClientOrderID(symbol, side string, ts time.Time) string {
	id := fmt.Sprintf("%s-%s-%s-%s",
		strings.ToLower(symbol), strings.ToLower(side), ts.UTC().Format("20060102150405"), randomSuffix())
	if len(id) > maxClientOrderIDLen {
		id = id[:maxClientOrderIDLen]
	}
	return id
}

// randomSuffix returns 8 hex chars of crypto-random entropy for order-id
// uniqueness. On the (practically impossible) RNG failure it falls back to the
// nanosecond clock so an id is always produced.
func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b)
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

// GetOrionURL builds a URL for orion (signal subscription) service endpoints.
func (p *Profile) GetOrionURL(path string) string {
	return p.BaseURL + "/orion" + path
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

// GetWSPrivateURL builds the full WebSocket URL for private contract trading events.
// If WSPrivate is already a full URL it is returned (with ws:// upgraded to wss://);
// otherwise it is appended to WebSocketURL. Private streams carry the session
// cookie and trading events, so plaintext ws:// is force-upgraded to wss://
// (BIFU-CLI-202606-007).
func (p *Profile) GetWSPrivateURL() string {
	if strings.HasPrefix(p.WSPrivate, "ws://") || strings.HasPrefix(p.WSPrivate, "wss://") {
		return forceWSS(p.WSPrivate)
	}
	return p.WebSocketURL + p.WSPrivate
}

// GetWSPrivateSpotURL builds the full WebSocket URL for private spot trading events.
// If WSPrivateSpot is already a full URL it is returned (with ws:// upgraded to
// wss://); otherwise it is appended to WebSocketURL. See GetWSPrivateURL.
func (p *Profile) GetWSPrivateSpotURL() string {
	if strings.HasPrefix(p.WSPrivateSpot, "ws://") || strings.HasPrefix(p.WSPrivateSpot, "wss://") {
		return forceWSS(p.WSPrivateSpot)
	}
	return p.WebSocketURL + p.WSPrivateSpot
}

// forceWSS upgrades a plaintext ws:// private-stream URL to wss://.
func forceWSS(u string) string {
	if strings.HasPrefix(u, "ws://") {
		return "wss://" + strings.TrimPrefix(u, "ws://")
	}
	return u
}

// GetPushgwWSURL returns the MT5 pushgw WebSocket URL.
func (p *Profile) GetPushgwWSURL() string {
	return p.Pushgw.WSEndpoint + p.Pushgw.WSPath
}

// GetTradfiWSURL returns the TradFi(Fortex) push WebSocket URL.
// Falls back to <pushgw endpoint>/tradfi/ws when not explicitly configured.
func (p *Profile) GetTradfiWSURL() string {
	if p.Pushgw.TradfiWS != "" {
		return p.Pushgw.TradfiWS
	}
	if p.Pushgw.WSEndpoint != "" {
		return p.Pushgw.WSEndpoint + "/tradfi/ws"
	}
	return ""
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
