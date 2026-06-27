package meta

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bifu-cli/internal/clifconfig"
)

func sampleMeta() *Meta {
	return &Meta{
		Symbols: []Instrument{
			{ID: "90000001", Name: "BTC-USDT"},
			{ID: "90000002", Name: "ETH-USDT"},
		},
		Contracts: []Instrument{
			{ID: "10000001", Name: "BTC/USDT"},
			{ID: "10000003", Name: "SOL/USDT"},
		},
	}
}

func TestNormalize(t *testing.T) {
	for _, in := range []string{"btc/usdt", "BTC-USDT", "BTCUSDT", " btc_usdt "} {
		if got := Normalize(in); got != "BTCUSDT" {
			t.Errorf("Normalize(%q) = %q, want BTCUSDT", in, got)
		}
	}
}

func TestResolveSpot(t *testing.T) {
	m := sampleMeta()
	cases := map[string]string{
		"BTC-USDT": "90000001",
		"btcusdt":  "90000001",
		"ETH/USDT": "90000002", // wrong separator still resolves
		"90000001": "90000001", // numeric passthrough
	}
	for in, want := range cases {
		got, err := m.ResolveSpot(in)
		if err != nil || got != want {
			t.Errorf("ResolveSpot(%q) = %q,%v; want %q", in, got, err, want)
		}
	}
	if _, err := m.ResolveSpot("NOPEUSDT"); err == nil {
		t.Error("expected error for unknown spot symbol")
	}
}

func TestResolveContract(t *testing.T) {
	m := sampleMeta()
	got, err := m.ResolveContract("SOLUSDT")
	if err != nil || got != "10000003" {
		t.Errorf("ResolveContract(SOLUSDT) = %q,%v; want 10000003", got, err)
	}
	if _, err := m.ResolveContract("ETHUSDT"); err == nil {
		t.Error("ETHUSDT has no contract — expected error")
	}
}

func TestResolveMarketDisambiguation(t *testing.T) {
	m := sampleMeta()
	cases := []struct {
		in       string
		wantID   string
		wantKind string
	}{
		{"BTCUSDT", "10000001", "contract"},  // no separator → contract first
		{"BTC/USDT", "10000001", "contract"}, // slash → contract
		{"BTC-USDT", "90000001", "spot"},     // dash → spot
		{"ETH-USDT", "90000002", "spot"},     // dash, only spot exists
		{"ETHUSDT", "90000002", "spot"},      // no contract → fall back to spot
		{"10000001", "10000001", ""},         // numeric passthrough
		{"all", "all", ""},                   // wildcard passthrough
	}
	for _, c := range cases {
		id, _, kind, err := m.ResolveMarket(c.in)
		if err != nil {
			t.Errorf("ResolveMarket(%q) error: %v", c.in, err)
			continue
		}
		if id != c.wantID || kind != c.wantKind {
			t.Errorf("ResolveMarket(%q) = id=%q kind=%q; want id=%q kind=%q",
				c.in, id, kind, c.wantID, c.wantKind)
		}
	}
	if _, _, _, err := m.ResolveMarket("FAKEUSDT"); err == nil {
		t.Error("expected error for unknown market symbol")
	}
}

func TestFetchAndCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/meta/getMetaData" {
			http.NotFound(w, r)
			return
		}
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"SUCCESS","data":{
			"symbolList":[{"symbolId":"90000001","symbolName":"BTC-USDT"}],
			"contractList":[{"contractId":"10000001","contractName":"BTC/USDT"}]
		}}`))
	}))
	defer srv.Close()

	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	c := New(&clifconfig.Profile{BaseURL: srv.URL, PublicPath: "/api/v1/public"})

	m, err := c.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if id, _ := m.ResolveContract("BTCUSDT"); id != "10000001" {
		t.Errorf("contract BTCUSDT = %q, want 10000001", id)
	}

	// Second Load should hit the disk cache, not the server.
	if _, err := c.Load(); err != nil {
		t.Fatalf("Load (cached): %v", err)
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (second Load should use cache)", hits)
	}
}
