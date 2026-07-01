// Package mcp exposes bifu-cli's trading capabilities as a Model Context
// Protocol (MCP) server, so AI agents (Claude Desktop, Cursor, VS Code, …) can
// query balances/positions and place/cancel orders through the active profile.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	contractapi "bifu-cli/internal/api/contract"
	metaapi "bifu-cli/internal/api/meta"
	paymentapi "bifu-cli/internal/api/payment"
	spotapi "bifu-cli/internal/api/spot"
	"bifu-cli/internal/clifconfig"
)

// ── MCP safety gates ──────────────────────────────────────────────────────────
//
// The MCP server has no per-request auth: every tool call acts as the configured
// profile's logged-in session. Because an AI agent driving these tools can be
// steered by untrusted content (prompt injection), trading is gated behind
// explicit opt-in env vars rather than enabled by default (BIFU-CLI-202606-001 /
// 024 / 028).
const (
	// envAllowTrade must be truthy to enable the write tools (place/cancel).
	envAllowTrade = "BIFU_MCP_ALLOW_TRADE"
	// envDetailed must be truthy for read tools to return precise monetary
	// figures; otherwise amounts are masked before entering the agent context.
	envDetailed = "BIFU_MCP_DETAILED"
	// maxOrderSize is a client-side sanity ceiling on order size.
	maxOrderSize = 1e9
)

func envTruthy(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// tradingDisabledResult is returned by every write tool when trading is not
// explicitly enabled.
func tradingDisabledResult() (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(
		"trading via MCP is disabled. The AI agent could be steered by untrusted " +
			"content; to allow order placement/cancellation set " + envAllowTrade + "=1 " +
			"in the MCP server environment."), nil
}

// validateSize parses and bounds an order size: must be a finite number, > 0,
// and ≤ maxOrderSize (BIFU-CLI-202606-001).
func validateSize(s string) error {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 || f > maxOrderSize {
		return fmt.Errorf("invalid size %q: must be a positive number ≤ %g", s, maxOrderSize)
	}
	return nil
}

// validatePrice parses and bounds a limit price: "0" is allowed (market order);
// otherwise must be a finite number > 0.
func validatePrice(s string) error {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > maxOrderSize {
		return fmt.Errorf("invalid price %q", s)
	}
	return nil
}

// validateContractCombo enforces cross-field invariants on contract orders so a
// "close" can't silently become an "open" (BIFU-CLI-202606-017). Open long =
// LONG+BUY, close long = LONG+SELL+reduceOnly; open short = SHORT+SELL, close
// short = SHORT+BUY+reduceOnly.
func validateContractCombo(positionSide, orderSide string, reduceOnly bool) error {
	closing := (positionSide == "LONG" && orderSide == "SELL") ||
		(positionSide == "SHORT" && orderSide == "BUY")
	opening := (positionSide == "LONG" && orderSide == "BUY") ||
		(positionSide == "SHORT" && orderSide == "SELL")
	switch {
	case !opening && !closing:
		return fmt.Errorf("invalid positionSide/orderSide combination: %s/%s", positionSide, orderSide)
	case closing && !reduceOnly:
		return fmt.Errorf("closing a %s position (orderSide=%s) requires reduceOnly=true", positionSide, orderSide)
	case opening && reduceOnly:
		return fmt.Errorf("opening a %s position must not set reduceOnly=true", positionSide)
	}
	return nil
}

// NewServer builds the MCP server with all bifu tools bound to the given profile.
func NewServer(profile *clifconfig.Profile, version string) *server.MCPServer {
	spot := spotapi.New(profile)
	contract := contractapi.New(profile)
	payment := paymentapi.New(profile)

	// Symbol resolvers: accept a symbol name (e.g. "BTCUSDT") or a numeric id.
	// Numeric/empty values pass through without a network call.
	resolveSpot := func(s string) (string, error) { return metaapi.ResolveSpotSymbol(profile, false, s) }
	resolveContract := func(s string) (string, error) { return metaapi.ResolveContractSymbol(profile, false, s) }

	detailed := envTruthy(envDetailed)
	// fin wraps a read-tool (value, error) return with money-masking unless
	// detailed mode is enabled.
	fin := func(v interface{}, err error) (*mcp.CallToolResult, error) {
		return financialResult(detailed, v, err)
	}

	s := server.NewMCPServer("bifu-cli", version,
		server.WithToolCapabilities(true),
		server.WithInstructions(
			"BifuFX trading tools. These act as the configured profile's logged-in "+
				"session and can move REAL money. Order placement/cancellation is "+
				"DISABLED unless "+envAllowTrade+"=1 is set in the server environment. "+
				"Read tools mask precise balances unless "+envDetailed+"=1. Sizes and "+
				"prices are strings; symbolId/contractId are numeric IDs (see getMetaData)."),
	)

	// ── Read tools ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("get_spot_balance",
		mcp.WithDescription("Get spot account balances (per coin).")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return fin(spot.GetBalance())
		})

	s.AddTool(mcp.NewTool("get_payment_balance",
		mcp.WithDescription("Get aggregated fund balance across accounts."),
		mcp.WithString("currency", mcp.Description("Fiat currency, e.g. USD"), mcp.DefaultString("USD"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return fin(payment.GetTotalBalance(r.GetString("currency", "USD")))
		})

	s.AddTool(mcp.NewTool("get_contract_account",
		mcp.WithDescription("Get contract/futures account assets and equity.")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return fin(contract.GetAccount())
		})

	s.AddTool(mcp.NewTool("list_contract_positions",
		mcp.WithDescription("List open contract positions."),
		mcp.WithString("contractId", mcp.Description("Filter by contractId or symbol name like BTCUSDT (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cid, err := resolveContract(r.GetString("contractId", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return fin(contract.ListPositions(cid))
		})

	s.AddTool(mcp.NewTool("list_spot_open_orders",
		mcp.WithDescription("List open spot orders."),
		mcp.WithString("symbolId", mcp.Description("Filter by symbolId or symbol name like BTCUSDT (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sid, err := resolveSpot(r.GetString("symbolId", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(spot.ListOpenOrders(sid))
		})

	s.AddTool(mcp.NewTool("list_contract_open_orders",
		mcp.WithDescription("List open contract orders."),
		mcp.WithString("contractId", mcp.Description("Filter by contractId or symbol name like BTCUSDT (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cid, err := resolveContract(r.GetString("contractId", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(contract.ListOpenOrders(cid))
		})

	s.AddTool(mcp.NewTool("list_forex_accounts",
		mcp.WithDescription("List the user's forex (MT5/TradFi) accounts.")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return fin(payment.GetForexAccountList())
		})

	// ── Write tools (place / cancel orders) ───────────────────────────────────
	s.AddTool(mcp.NewTool("create_spot_order",
		mcp.WithDescription("Place a spot order."),
		mcp.WithString("symbolId", mcp.Required(), mcp.Description("symbolId or symbol name like BTCUSDT")),
		mcp.WithString("side", mcp.Required(), mcp.Description("BUY or SELL"), mcp.Enum("BUY", "SELL")),
		mcp.WithString("size", mcp.Required(), mcp.Description("Order size in base asset")),
		mcp.WithString("type", mcp.Description("MARKET or LIMIT"), mcp.DefaultString("MARKET"), mcp.Enum("MARKET", "LIMIT")),
		mcp.WithString("price", mcp.Description("Limit price (0 for market)"), mcp.DefaultString("0")),
		mcp.WithString("timeInForce", mcp.Description("GOOD_TIL_CANCEL | IMMEDIATE_OR_CANCEL | FILL_OR_KILL"), mcp.DefaultString("GOOD_TIL_CANCEL"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !envTruthy(envAllowTrade) {
				return tradingDisabledResult()
			}
			symbol, err := r.RequireString("symbolId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			side, err := r.RequireString("side")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			size, err := r.RequireString("size")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := validateSize(size); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			price := r.GetString("price", "0")
			if err := validatePrice(price); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			symbol, err = resolveSpot(symbol)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(spot.CreateOrder(&spotapi.CreateOrderReq{
				SymbolID:      symbol,
				OrderSide:     side,
				Type:          r.GetString("type", "MARKET"),
				Price:         price,
				Size:          size,
				TimeInForce:   r.GetString("timeInForce", "GOOD_TIL_CANCEL"),
				ClientOrderID: profile.GenerateClientOrderID(symbol, side, time.Now()),
			}))
		})

	s.AddTool(mcp.NewTool("create_contract_order",
		mcp.WithDescription("Place a contract/futures order. Open long = LONG+BUY, close long = LONG+SELL+reduceOnly, open short = SHORT+SELL."),
		mcp.WithString("contractId", mcp.Required(), mcp.Description("contractId or symbol name like BTCUSDT")),
		mcp.WithString("positionSide", mcp.Required(), mcp.Description("LONG or SHORT"), mcp.Enum("LONG", "SHORT")),
		mcp.WithString("orderSide", mcp.Required(), mcp.Description("BUY or SELL"), mcp.Enum("BUY", "SELL")),
		mcp.WithString("size", mcp.Required(), mcp.Description("Order size")),
		mcp.WithString("type", mcp.Description("MARKET or LIMIT"), mcp.DefaultString("MARKET"), mcp.Enum("MARKET", "LIMIT")),
		mcp.WithString("price", mcp.Description("Limit price (0 for market)"), mcp.DefaultString("0")),
		mcp.WithString("marginMode", mcp.Description("SHARED or ISOLATED"), mcp.DefaultString("SHARED")),
		mcp.WithBoolean("reduceOnly", mcp.Description("Reduce-only (closing)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !envTruthy(envAllowTrade) {
				return tradingDisabledResult()
			}
			contractID, err := r.RequireString("contractId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			posSide, err := r.RequireString("positionSide")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ordSide, err := r.RequireString("orderSide")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			size, err := r.RequireString("size")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := validateSize(size); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			price := r.GetString("price", "0")
			if err := validatePrice(price); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			reduceOnly := r.GetBool("reduceOnly", false)
			if err := validateContractCombo(posSide, ordSide, reduceOnly); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			contractID, err = resolveContract(contractID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(contract.CreateOrder(&contractapi.CreateOrderReq{
				ContractID:           contractID,
				MarginMode:           r.GetString("marginMode", "SHARED"),
				SeparatedMode:        "COMBINED",
				SeparatedOpenOrderID: "0",
				PositionSide:         posSide,
				OrderSide:            ordSide,
				Price:                price,
				Size:                 size,
				Type:                 r.GetString("type", "MARKET"),
				TimeInForce:          "GOOD_TIL_CANCEL",
				ReduceOnly:           reduceOnly,
				ClientOrderID:        profile.GenerateClientOrderID(contractID, ordSide, time.Now()),
			}))
		})

	s.AddTool(mcp.NewTool("cancel_spot_order",
		mcp.WithDescription("Cancel a spot order by id."),
		mcp.WithString("orderId", mcp.Required(), mcp.Description("Order id"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !envTruthy(envAllowTrade) {
				return tradingDisabledResult()
			}
			id, err := r.RequireString("orderId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := spot.CancelOrder(&spotapi.CancelOrderReq{OrderID: id}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return okResult(id)
		})

	s.AddTool(mcp.NewTool("cancel_contract_order",
		mcp.WithDescription("Cancel a contract order by id."),
		mcp.WithString("orderId", mcp.Required(), mcp.Description("Order id"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !envTruthy(envAllowTrade) {
				return tradingDisabledResult()
			}
			id, err := r.RequireString("orderId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := contract.CancelOrder(id, ""); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return okResult(id)
		})

	return s
}

// Serve runs the MCP server over stdio (blocking).
func Serve(profile *clifconfig.Profile, version string) error {
	return server.ServeStdio(NewServer(profile, version))
}

// ServeHTTP runs the MCP server over Streamable HTTP (blocking). The MCP
// endpoint is mounted at path (default "/mcp") on addr (e.g. "127.0.0.1:8080").
//
// Every tool call acts as the configured profile's logged-in session — there is
// no per-request auth — so bind to localhost unless the network is trusted.
// stateless=true disables per-session state (simpler for serverless/stateless
// clients; the default is stateful with session IDs).
func ServeHTTP(profile *clifconfig.Profile, version, addr, path string, stateless bool) error {
	if path == "" {
		path = "/mcp"
	}
	opts := []server.StreamableHTTPOption{server.WithEndpointPath(path)}
	if stateless {
		opts = append(opts, server.WithStateLess(true))
	}
	httpSrv := server.NewStreamableHTTPServer(NewServer(profile, version), opts...)
	fmt.Fprintf(os.Stderr, "bifu-cli MCP server (Streamable HTTP) listening on http://%s%s\n", addr, path)
	return httpSrv.Start(addr)
}

// jsonResult marshals any (value, error) pair into an MCP tool result.
func jsonResult(v interface{}, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, mErr := json.MarshalIndent(v, "", "  ")
	if mErr != nil {
		return mcp.NewToolResultError(mErr.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// okResult returns a well-formed JSON success result. Built with json.Marshal
// (not string concatenation) so an orderId containing quotes can't produce
// malformed/injected JSON (BIFU-CLI-202606-006).
func okResult(orderID string) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(map[string]any{"ok": true, "orderId": orderID})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// financialResult is jsonResult for read tools: unless detailed mode is on, it
// masks precise monetary figures before they enter the AI agent's context
// (BIFU-CLI-202606-024).
func financialResult(detailed bool, v interface{}, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, mErr := json.MarshalIndent(v, "", "  ")
	if mErr != nil {
		return mcp.NewToolResultError(mErr.Error()), nil
	}
	if detailed {
		return mcp.NewToolResultText(string(b)), nil
	}
	var generic any
	if json.Unmarshal(b, &generic) != nil {
		return mcp.NewToolResultText(string(b)), nil
	}
	maskMoney(generic)
	masked, mErr := json.MarshalIndent(generic, "", "  ")
	if mErr != nil {
		return mcp.NewToolResultText(string(b)), nil
	}
	return mcp.NewToolResultText(
		string(masked) + "\n\n(precise amounts masked; set " + envDetailed + "=1 for full figures)"), nil
}

// moneyKeys are JSON field names carrying precise monetary figures that are
// masked from the agent context by default.
var moneyKeys = map[string]bool{
	"amount": true, "balance": true, "equity": true, "available": true,
	"availablebalance": true, "frozenbalance": true, "frozen": true,
	"margin": true, "marginfree": true, "marginlevel": true,
	"pendingdepositamount": true, "pendingwithdrawamount": true,
	"accountequity": true, "accountavailable": true, "accountused": true,
	"accountfrozen": true, "unrealizepnl": true, "profit": true,
	"openvalue": true, "openfee": true, "totalusd": true, "ratio": true,
	"initialmarginrequirement": true, "maintenancemarginrequirement": true,
}

// maskMoney walks a decoded JSON value and replaces money-bearing fields with
// "***".
func maskMoney(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if moneyKeys[strings.ToLower(k)] {
				if _, isObj := val.(map[string]any); !isObj {
					if _, isArr := val.([]any); !isArr {
						t[k] = "***"
						continue
					}
				}
			}
			maskMoney(val)
		}
	case []any:
		for _, item := range t {
			maskMoney(item)
		}
	}
}
