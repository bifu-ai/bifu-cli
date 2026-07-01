package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bifu-cli/internal/clifconfig"
)

// startMockBackend serves the meta endpoint plus spot/contract createOrder,
// capturing the most recent createOrder request body.
func startMockBackend(captured *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/meta/getMetaData"):
			_, _ = w.Write([]byte(`{"code":"SUCCESS","data":{
				"symbolList":[{"symbolId":"90000001","symbolName":"BTC-USDT"}],
				"contractList":[{"contractId":"10000001","contractName":"BTC/USDT"}]
			}}`))
		case strings.Contains(r.URL.Path, "/order/createOrder"):
			b, _ := io.ReadAll(r.Body)
			*captured = string(b)
			_, _ = w.Write([]byte(`{"code":"SUCCESS","data":{"orderId":"777","status":"NEW"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func mockProfile(baseURL string) *clifconfig.Profile {
	p := &clifconfig.Profile{
		BaseURL:     baseURL,
		PublicPath:  "/api/v1/public",
		PrivatePath: "/api/v1/private",
	}
	p.Auth.UserID = "42"
	return p
}

// invoke sends a tools/call and returns the decoded JSON-RPC response.
func invoke(t *testing.T, baseURL, tool string, args map[string]any) map[string]any {
	t.Helper()
	srv := NewServer(mockProfile(baseURL), "test")
	reqObj := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	}
	raw, _ := json.Marshal(reqObj)
	resp := srv.HandleMessage(context.Background(), raw)
	out, _ := json.Marshal(resp)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

// resultText pulls the first text content out of a tools/call response and
// reports whether the tool flagged an error.
func resultText(m map[string]any) (text string, isErr bool) {
	res, _ := m["result"].(map[string]any)
	if res == nil {
		return "", true
	}
	if e, ok := res["isError"].(bool); ok {
		isErr = e
	}
	if arr, ok := res["content"].([]any); ok {
		for _, c := range arr {
			if cm, ok := c.(map[string]any); ok {
				if s, ok := cm["text"].(string); ok {
					text += s
				}
			}
		}
	}
	return text, isErr
}

// TestCreateSpotOrderSendsClientOrderID is the regression guard for the bug
// where MCP create tools sent an empty clientOrderId (INVALID_CLIENT_ORDER_ID).
func TestCreateSpotOrderSendsClientOrderID(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	t.Setenv("BIFU_MCP_ALLOW_TRADE", "1")
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	m := invoke(t, srv.URL, "create_spot_order", map[string]any{
		"symbolId": "90000001", "side": "BUY", "type": "LIMIT",
		"price": "10000", "size": "0.0001",
	})
	if _, isErr := resultText(m); isErr {
		txt, _ := resultText(m)
		t.Fatalf("create_spot_order returned error: %s", txt)
	}

	var req struct {
		SymbolID      string `json:"symbolId"`
		ClientOrderID string `json:"clientOrderId"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("parse captured body %q: %v", body, err)
	}
	if req.ClientOrderID == "" {
		t.Error("clientOrderId is empty — would be rejected as INVALID_CLIENT_ORDER_ID")
	}
}

// TestCreateContractOrderSendsClientOrderID guards the contract create path.
func TestCreateContractOrderSendsClientOrderID(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	t.Setenv("BIFU_MCP_ALLOW_TRADE", "1")
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	m := invoke(t, srv.URL, "create_contract_order", map[string]any{
		"contractId": "10000001", "positionSide": "LONG", "orderSide": "BUY",
		"type": "LIMIT", "price": "10000", "size": "0.001",
	})
	if _, isErr := resultText(m); isErr {
		txt, _ := resultText(m)
		t.Fatalf("create_contract_order returned error: %s", txt)
	}

	var req struct {
		ClientOrderID string `json:"clientOrderId"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("parse captured body %q: %v", body, err)
	}
	if req.ClientOrderID == "" {
		t.Error("contract clientOrderId is empty")
	}
}

// TestMCPResolvesSymbolName checks that a symbol name (BTCUSDT) is resolved to
// its numeric id before the order is sent.
func TestMCPResolvesSymbolName(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	t.Setenv("BIFU_MCP_ALLOW_TRADE", "1")
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	m := invoke(t, srv.URL, "create_spot_order", map[string]any{
		"symbolId": "BTCUSDT", "side": "BUY", "type": "LIMIT",
		"price": "10000", "size": "0.0001",
	})
	if txt, isErr := resultText(m); isErr {
		t.Fatalf("create_spot_order(BTCUSDT) errored: %s", txt)
	}
	var req struct {
		SymbolID string `json:"symbolId"`
	}
	_ = json.Unmarshal([]byte(body), &req)
	if req.SymbolID != "90000001" {
		t.Errorf("symbolId sent = %q, want resolved 90000001", req.SymbolID)
	}
}

// TestWriteToolsDisabledByDefault verifies order placement is refused unless
// BIFU_MCP_ALLOW_TRADE is set (BIFU-CLI-202606-001).
func TestWriteToolsDisabledByDefault(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	// Deliberately NOT setting BIFU_MCP_ALLOW_TRADE.
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	m := invoke(t, srv.URL, "create_spot_order", map[string]any{
		"symbolId": "90000001", "side": "BUY", "type": "MARKET", "size": "0.1",
	})
	txt, isErr := resultText(m)
	if !isErr {
		t.Fatalf("expected error result when trading disabled, got: %s", txt)
	}
	if !strings.Contains(txt, "BIFU_MCP_ALLOW_TRADE") {
		t.Fatalf("error should mention the enable flag, got: %s", txt)
	}
	if body != "" {
		t.Fatalf("a backend request was made despite trading being disabled: %s", body)
	}
}

// TestInvalidSizeRejected verifies client-side numeric validation (001).
func TestInvalidSizeRejected(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	t.Setenv("BIFU_MCP_ALLOW_TRADE", "1")
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	m := invoke(t, srv.URL, "create_spot_order", map[string]any{
		"symbolId": "90000001", "side": "BUY", "type": "MARKET", "size": "-5",
	})
	if _, isErr := resultText(m); !isErr {
		t.Fatalf("expected error for negative size")
	}
	if body != "" {
		t.Fatalf("negative size reached backend: %s", body)
	}
}

// TestCloseRequiresReduceOnly verifies the contract cross-field invariant (017).
func TestCloseRequiresReduceOnly(t *testing.T) {
	t.Setenv("BIFU_CLI_HOME", t.TempDir())
	t.Setenv("BIFU_MCP_ALLOW_TRADE", "1")
	var body string
	srv := startMockBackend(&body)
	defer srv.Close()

	// LONG + SELL without reduceOnly = closing a long without the flag → rejected.
	m := invoke(t, srv.URL, "create_contract_order", map[string]any{
		"contractId": "10000001", "positionSide": "LONG", "orderSide": "SELL",
		"type": "MARKET", "size": "0.001",
	})
	txt, isErr := resultText(m)
	if !isErr {
		t.Fatalf("expected error for LONG+SELL without reduceOnly, got: %s", txt)
	}
	if !strings.Contains(txt, "reduceOnly") {
		t.Fatalf("error should mention reduceOnly, got: %s", txt)
	}
}
