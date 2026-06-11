// Package mcp exposes bifu-cli's trading capabilities as a Model Context
// Protocol (MCP) server, so AI agents (Claude Desktop, Cursor, VS Code, …) can
// query balances/positions and place/cancel orders through the active profile.
package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	contractapi "bifu-cli/internal/api/contract"
	paymentapi "bifu-cli/internal/api/payment"
	spotapi "bifu-cli/internal/api/spot"
	"bifu-cli/internal/clifconfig"
)

// NewServer builds the MCP server with all bifu tools bound to the given profile.
func NewServer(profile *clifconfig.Profile, version string) *server.MCPServer {
	spot := spotapi.New(profile)
	contract := contractapi.New(profile)
	payment := paymentapi.New(profile)

	s := server.NewMCPServer("bifu-cli", version,
		server.WithToolCapabilities(true),
		server.WithInstructions(
			"BifuFX trading tools. Read balances/positions/orders and place or "+
				"cancel spot & contract orders for the configured profile. Sizes and "+
				"prices are strings; symbolId/contractId are numeric IDs (see getMetaData)."),
	)

	// ── Read tools ────────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("get_spot_balance",
		mcp.WithDescription("Get spot account balances (per coin).")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(spot.GetBalance())
		})

	s.AddTool(mcp.NewTool("get_payment_balance",
		mcp.WithDescription("Get aggregated fund balance across accounts."),
		mcp.WithString("currency", mcp.Description("Fiat currency, e.g. USD"), mcp.DefaultString("USD"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(payment.GetTotalBalance(r.GetString("currency", "USD")))
		})

	s.AddTool(mcp.NewTool("get_contract_account",
		mcp.WithDescription("Get contract/futures account assets and equity.")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(contract.GetAccount())
		})

	s.AddTool(mcp.NewTool("list_contract_positions",
		mcp.WithDescription("List open contract positions."),
		mcp.WithString("contractId", mcp.Description("Filter by numeric contractId (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(contract.ListPositions(r.GetString("contractId", "")))
		})

	s.AddTool(mcp.NewTool("list_spot_open_orders",
		mcp.WithDescription("List open spot orders."),
		mcp.WithString("symbolId", mcp.Description("Filter by numeric symbolId (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(spot.ListOpenOrders(r.GetString("symbolId", "")))
		})

	s.AddTool(mcp.NewTool("list_contract_open_orders",
		mcp.WithDescription("List open contract orders."),
		mcp.WithString("contractId", mcp.Description("Filter by numeric contractId (optional)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(contract.ListOpenOrders(r.GetString("contractId", "")))
		})

	s.AddTool(mcp.NewTool("list_forex_accounts",
		mcp.WithDescription("List the user's forex (MT5/TradFi) accounts.")),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return jsonResult(payment.GetForexAccountList())
		})

	// ── Write tools (place / cancel orders) ───────────────────────────────────
	s.AddTool(mcp.NewTool("create_spot_order",
		mcp.WithDescription("Place a spot order."),
		mcp.WithString("symbolId", mcp.Required(), mcp.Description("Numeric symbolId")),
		mcp.WithString("side", mcp.Required(), mcp.Description("BUY or SELL"), mcp.Enum("BUY", "SELL")),
		mcp.WithString("size", mcp.Required(), mcp.Description("Order size in base asset")),
		mcp.WithString("type", mcp.Description("MARKET or LIMIT"), mcp.DefaultString("MARKET"), mcp.Enum("MARKET", "LIMIT")),
		mcp.WithString("price", mcp.Description("Limit price (0 for market)"), mcp.DefaultString("0")),
		mcp.WithString("timeInForce", mcp.Description("GOOD_TIL_CANCEL | IMMEDIATE_OR_CANCEL | FILL_OR_KILL"), mcp.DefaultString("GOOD_TIL_CANCEL"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return jsonResult(spot.CreateOrder(&spotapi.CreateOrderReq{
				SymbolID:    symbol,
				OrderSide:   side,
				Type:        r.GetString("type", "MARKET"),
				Price:       r.GetString("price", "0"),
				Size:        size,
				TimeInForce: r.GetString("timeInForce", "GOOD_TIL_CANCEL"),
			}))
		})

	s.AddTool(mcp.NewTool("create_contract_order",
		mcp.WithDescription("Place a contract/futures order. Open long = LONG+BUY, close long = LONG+SELL+reduceOnly, open short = SHORT+SELL."),
		mcp.WithString("contractId", mcp.Required(), mcp.Description("Numeric contractId")),
		mcp.WithString("positionSide", mcp.Required(), mcp.Description("LONG or SHORT"), mcp.Enum("LONG", "SHORT")),
		mcp.WithString("orderSide", mcp.Required(), mcp.Description("BUY or SELL"), mcp.Enum("BUY", "SELL")),
		mcp.WithString("size", mcp.Required(), mcp.Description("Order size")),
		mcp.WithString("type", mcp.Description("MARKET or LIMIT"), mcp.DefaultString("MARKET"), mcp.Enum("MARKET", "LIMIT")),
		mcp.WithString("price", mcp.Description("Limit price (0 for market)"), mcp.DefaultString("0")),
		mcp.WithString("marginMode", mcp.Description("SHARED or ISOLATED"), mcp.DefaultString("SHARED")),
		mcp.WithBoolean("reduceOnly", mcp.Description("Reduce-only (closing)"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return jsonResult(contract.CreateOrder(&contractapi.CreateOrderReq{
				ContractID:           contractID,
				MarginMode:           r.GetString("marginMode", "SHARED"),
				SeparatedMode:        "COMBINED",
				SeparatedOpenOrderID: "0",
				PositionSide:         posSide,
				OrderSide:            ordSide,
				Price:                r.GetString("price", "0"),
				Size:                 size,
				Type:                 r.GetString("type", "MARKET"),
				TimeInForce:          "GOOD_TIL_CANCEL",
				ReduceOnly:           r.GetBool("reduceOnly", false),
			}))
		})

	s.AddTool(mcp.NewTool("cancel_spot_order",
		mcp.WithDescription("Cancel a spot order by id."),
		mcp.WithString("orderId", mcp.Required(), mcp.Description("Order id"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := r.RequireString("orderId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := spot.CancelOrder(&spotapi.CancelOrderReq{OrderID: id}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(`{"ok":true,"orderId":"` + id + `"}`), nil
		})

	s.AddTool(mcp.NewTool("cancel_contract_order",
		mcp.WithDescription("Cancel a contract order by id."),
		mcp.WithString("orderId", mcp.Required(), mcp.Description("Order id"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := r.RequireString("orderId")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := contract.CancelOrder(id, ""); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(`{"ok":true,"orderId":"` + id + `"}`), nil
		})

	return s
}

// Serve runs the MCP server over stdio (blocking).
func Serve(profile *clifconfig.Profile, version string) error {
	return server.ServeStdio(NewServer(profile, version))
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
