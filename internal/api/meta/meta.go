// Package meta fetches public market metadata (GET /api/v1/public/meta/getMetaData)
// and resolves human-friendly symbol names (e.g. "BTCUSDT", "BTC/USDT") to the
// numeric IDs the platform expects (spot symbolId / contract contractId).
//
// Results are cached on disk per environment (keyed by base URL) so repeated
// commands don't re-fetch, and symbol resolution works without a round-trip
// once warmed.
package meta

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// cacheTTL bounds how long a cached metadata snapshot is reused before refetch.
const cacheTTL = 6 * time.Hour

// Instrument is one tradable symbol or contract with its numeric ID.
type Instrument struct {
	ID   string `json:"id"`   // numeric ID as a string (e.g. "10000001")
	Name string `json:"name"` // display name (e.g. "BTC/USDT" or "BTC-USDT")
}

// Meta is the resolved metadata: spot symbols and contracts.
type Meta struct {
	Symbols   []Instrument `json:"symbols"`   // spot (symbolId / symbolName)
	Contracts []Instrument `json:"contracts"` // contract (contractId / contractName)
}

// Client fetches and caches market metadata.
type Client struct {
	http    *client.HTTPClient
	profile *clifconfig.Profile
}

// New creates a metadata client for the active profile.
func New(profile *clifconfig.Profile) *Client {
	return &Client{http: client.NewHTTPClient(profile), profile: profile}
}

// SetVerbose toggles HTTP logging on the underlying client.
func (c *Client) SetVerbose(v bool) { c.http.SetVerbose(v) }

// rawMeta mirrors the relevant slice of the getMetaData response.
type rawMeta struct {
	SymbolList []struct {
		SymbolID   json.Number `json:"symbolId"`
		SymbolName string      `json:"symbolName"`
	} `json:"symbolList"`
	ContractList []struct {
		ContractID   json.Number `json:"contractId"`
		ContractName string      `json:"contractName"`
	} `json:"contractList"`
}

// Load returns metadata, preferring a fresh on-disk cache and falling back to a
// network fetch (which then repopulates the cache).
func (c *Client) Load() (*Meta, error) {
	if m, ok := c.readCache(); ok {
		return m, nil
	}
	m, err := c.fetch()
	if err != nil {
		// A stale cache is more useful than a hard failure.
		if m, ok := c.readCache(); ok {
			return m, nil
		}
		return nil, err
	}
	c.writeCache(m)
	return m, nil
}

// fetch retrieves metadata over HTTP.
func (c *Client) fetch() (*Meta, error) {
	u := c.profile.GetPublicURL("/meta/getMetaData")
	resp, err := c.http.GetPayment(u, nil) // plain GET; public endpoint
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	var raw rawMeta
	if err := client.ParseAPIResponse(resp.Body, &raw); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	m := &Meta{}
	for _, s := range raw.SymbolList {
		m.Symbols = append(m.Symbols, Instrument{ID: s.SymbolID.String(), Name: s.SymbolName})
	}
	for _, ct := range raw.ContractList {
		m.Contracts = append(m.Contracts, Instrument{ID: ct.ContractID.String(), Name: ct.ContractName})
	}
	return m, nil
}

// ── Resolution ────────────────────────────────────────────────────────────────

// Normalize folds a symbol name to a comparison key: uppercase, separators
// stripped. "btc/usdt", "BTC-USDT" and "BTCUSDT" all become "BTCUSDT".
func Normalize(s string) string {
	r := strings.NewReplacer("-", "", "/", "", "_", "", " ", "")
	return r.Replace(strings.ToUpper(strings.TrimSpace(s)))
}

// isNumeric reports whether s is a bare numeric ID (already resolved).
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func lookup(list []Instrument, key string) (Instrument, bool) {
	for _, it := range list {
		if Normalize(it.Name) == key {
			return it, true
		}
	}
	return Instrument{}, false
}

// ResolveSpot maps a spot symbol name (or passes through a numeric ID) to its
// symbolId. Returns an error naming the input when it cannot be resolved.
func (m *Meta) ResolveSpot(s string) (string, error) {
	if isNumeric(s) {
		return s, nil
	}
	if it, ok := lookup(m.Symbols, Normalize(s)); ok {
		return it.ID, nil
	}
	return "", fmt.Errorf("unknown spot symbol %q (try a numeric symbolId, or check `getMetaData`)", s)
}

// ResolveContract maps a contract name (or passes through a numeric ID) to its
// contractId.
func (m *Meta) ResolveContract(s string) (string, error) {
	if isNumeric(s) {
		return s, nil
	}
	if it, ok := lookup(m.Contracts, Normalize(s)); ok {
		return it.ID, nil
	}
	return "", fmt.Errorf("unknown contract %q (try a numeric contractId, or check `getMetaData`)", s)
}

// ResolveMarket maps a market-data symbol to a numeric ID for WS ticker/depth
// channels. A "/" hints contract, "-" hints spot; with no separator it prefers
// contract (market ticker is contract-centric) then falls back to spot. It
// returns the resolved ID, the matched display name, and its kind
// ("contract"|"spot"). Numeric input and "all" pass through unchanged.
func (m *Meta) ResolveMarket(s string) (id, name, kind string, err error) {
	if isNumeric(s) || strings.EqualFold(s, "all") {
		return s, s, "", nil
	}
	key := Normalize(s)
	preferContract := !strings.Contains(s, "-") // dash => spot; otherwise contract first
	order := []struct {
		list []Instrument
		kind string
	}{{m.Contracts, "contract"}, {m.Symbols, "spot"}}
	if !preferContract {
		order[0], order[1] = order[1], order[0]
	}
	for _, o := range order {
		if it, ok := lookup(o.list, key); ok {
			return it.ID, it.Name, o.kind, nil
		}
	}
	return "", "", "", fmt.Errorf("unknown market symbol %q (try a numeric instrumentId, `all`, or check `getMetaData`)", s)
}

// ── Convenience resolvers ─────────────────────────────────────────────────────
//
// These one-shot helpers are what command code calls. An empty or already-numeric
// value passes through with no network access; only a real symbol name triggers a
// metadata Load (served from the disk cache when warm).

// ResolveSpotSymbol resolves a spot symbol name to its numeric symbolId.
func ResolveSpotSymbol(p *clifconfig.Profile, verbose bool, s string) (string, error) {
	if s == "" || isNumeric(s) {
		return s, nil
	}
	c := New(p)
	c.SetVerbose(verbose)
	m, err := c.Load()
	if err != nil {
		return "", err
	}
	return m.ResolveSpot(s)
}

// ResolveContractSymbol resolves a contract name to its numeric contractId.
func ResolveContractSymbol(p *clifconfig.Profile, verbose bool, s string) (string, error) {
	if s == "" || isNumeric(s) {
		return s, nil
	}
	c := New(p)
	c.SetVerbose(verbose)
	m, err := c.Load()
	if err != nil {
		return "", err
	}
	return m.ResolveContract(s)
}

// ── Disk cache ──────────────────────────────────────────────────────────────

type cacheFile struct {
	FetchedAt time.Time `json:"fetchedAt"`
	BaseURL   string    `json:"baseUrl"`
	Meta      *Meta     `json:"meta"`
}

// cachePath returns a per-environment cache file path under the config dir.
func (c *Client) cachePath() string {
	sum := sha1.Sum([]byte(c.profile.BaseURL))
	name := "meta-" + hex.EncodeToString(sum[:6]) + ".json"
	return filepath.Join(clifconfig.ConfigDir(), "cache", name)
}

func (c *Client) readCache() (*Meta, bool) {
	data, err := os.ReadFile(c.cachePath())
	if err != nil {
		return nil, false
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Meta == nil {
		return nil, false
	}
	if cf.BaseURL != c.profile.BaseURL || time.Since(cf.FetchedAt) > cacheTTL {
		return nil, false
	}
	return cf.Meta, true
}

func (c *Client) writeCache(m *Meta) {
	cf := cacheFile{FetchedAt: time.Now(), BaseURL: c.profile.BaseURL, Meta: m}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return
	}
	path := c.cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
