// Package spot provides API client methods for spot trading.
package spot

import (
	"encoding/json"
	"fmt"
	"strconv"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// Client is the Spot trading API client.
type Client struct {
	http    *client.HTTPClient
	profile *clifconfig.Profile
}

// New creates a Spot API client from the active profile.
func New(profile *clifconfig.Profile) *Client {
	return &Client{
		http:    client.NewHTTPClient(profile),
		profile: profile,
	}
}

// SetVerbose enables HTTP request/response logging.
func (c *Client) SetVerbose(v bool) { c.http.SetVerbose(v) }

// ── Order types ───────────────────────────────────────────────────────────────

type CreateOrderReq struct {
	SymbolID      string `json:"symbolId"`
	OrderSide     string `json:"orderSide"` // BUY | SELL
	Type          string `json:"type"`      // LIMIT | MARKET
	Price         string `json:"price"`     // "0" for market
	Size          string `json:"size"`
	IsQuoteSize   bool   `json:"isQuoteSize"`
	TimeInForce   string `json:"timeInForce"` // GOOD_TIL_CANCEL | IMMEDIATE_OR_CANCEL | FILL_OR_KILL
	ClientOrderID string `json:"clientOrderId"`
	ReduceOnly    bool   `json:"reduceOnly,omitempty"`
}

type OrderResp struct {
	OrderID       string `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
}

type Order struct {
	OrderID        string      `json:"orderId"`
	SymbolID       interface{} `json:"symbolId"`
	OrderSide      string      `json:"orderSide"`
	Type           string      `json:"type"`
	Status         string      `json:"status"`
	Price          string      `json:"price"`
	Size           string      `json:"size"`
	FilledQuantity string      `json:"filledQuantity"`
	AveragePrice   string      `json:"averagePrice"`
	CreatedTime    interface{} `json:"createdTime"`
}

type CancelOrderReq struct {
	OrderID       string `json:"orderId,omitempty"`
	ClientOrderID string `json:"clientOrderId,omitempty"`
}

type AccountInfo struct {
	AccountID   interface{} `json:"accountId"`
	AccountType int         `json:"accountType"`
	Status      string      `json:"status"`
}

type BalanceItem struct {
	CoinID                json.Number `json:"coinId"`
	Amount                string      `json:"amount"`
	PendingDepositAmount  string      `json:"pendingDepositAmount"`
	PendingWithdrawAmount string      `json:"pendingWithdrawAmount"`
	CreatedTime           string      `json:"createdTime"`
	UpdatedTime           string      `json:"updatedTime"`
}

type AccountAsset struct {
	AccountID   json.Number   `json:"accountId"`
	BalanceList []BalanceItem `json:"balanceList"`
	Version     json.Number   `json:"version"`
}

// ── Transfer types ─────────────────────────────────────────────────────────────

type TransferReq struct {
	CoinID       int    `json:"coinId"`
	Amount       string `json:"amount"`
	FromAccount  string `json:"fromAccount"`
	ToAccount    string `json:"toAccount"`
	TransferType string `json:"transferType"`
}

type TransferResp struct {
	TransferID string `json:"transferId"`
	Status     string `json:"status"`
}

// ── Methods ───────────────────────────────────────────────────────────────────

// CreateOrder places a new spot order.
func (c *Client) CreateOrder(req *CreateOrderReq) (*OrderResp, error) {
	u := c.profile.GetPrivateURL("/spot/order/createOrder")
	raw, err := c.http.PostSpot(u, req)
	if err != nil {
		return nil, err
	}
	var dst OrderResp
	if err := client.ParseAPIResponse(raw.Body, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

// CancelOrder cancels an existing spot order by order ID or client order ID.
func (c *Client) CancelOrder(req *CancelOrderReq) error {
	if req.OrderID != "" {
		u := c.profile.GetPrivateURL("/spot/order/cancelOrderById")
		body := map[string]string{"orderId": req.OrderID}
		raw, err := c.http.PostSpot(u, body)
		if err != nil {
			return err
		}
		return client.ParseAPIResponse(raw.Body, nil)
	}
	if req.ClientOrderID != "" {
		u := c.profile.GetPrivateURL("/spot/order/cancelOrderByClientOrderId")
		body := map[string]interface{}{"clientOrderIdList": []string{req.ClientOrderID}}
		raw, err := c.http.PostSpot(u, body)
		if err != nil {
			return err
		}
		return client.ParseAPIResponse(raw.Body, nil)
	}
	return fmt.Errorf("orderId or clientOrderId is required")
}

// CancelAllOrders cancels all open orders, optionally filtered by instrument ID.
func (c *Client) CancelAllOrders(symbolID string) error {
	req := map[string]interface{}{}
	if symbolID != "" {
		if id, err := strconv.Atoi(symbolID); err == nil {
			req["instrumentId"] = id
		}
	}
	u := c.profile.GetPrivateURL("/spot/order/cancelAllOrders")
	raw, err := c.http.PostSpot(u, req)
	if err != nil {
		return err
	}
	return client.ParseAPIResponse(raw.Body, nil)
}

// GetOrder retrieves a single order by ID.
func (c *Client) GetOrder(orderID string) (*Order, error) {
	u := c.profile.GetPrivateURL("/spot/order/getOrderById")
	raw, err := c.http.GetSpot(u, map[string]string{"orderIdList": orderID})
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := client.ParseAPIResponse(raw.Body, &orders); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	return &orders[0], nil
}

// ListOpenOrders returns paginated open orders, optionally filtered by instrument ID.
func (c *Client) ListOpenOrders(symbolID string) ([]Order, error) {
	params := map[string]string{
		"pageSize": "100",
		"pageNo":   "0",
	}
	if symbolID != "" {
		if id, err := strconv.Atoi(symbolID); err == nil {
			params["instrumentId"] = strconv.Itoa(id)
		}
	}
	u := c.profile.GetPrivateURL("/spot/order/getActiveOrderPage2")
	raw, err := c.http.GetSpot(u, params)
	if err != nil {
		return nil, err
	}
	return parsePageDataOrders(raw.Body)
}

// ListOrderHistory returns paginated order history, optionally filtered by instrument ID.
func (c *Client) ListOrderHistory(symbolID string, limit int) ([]Order, error) {
	pageSize := 100
	if limit > 0 {
		pageSize = limit
	}
	params := map[string]string{
		"pageSize": strconv.Itoa(pageSize),
		"pageNo":   "0",
	}
	if symbolID != "" {
		if id, err := strconv.Atoi(symbolID); err == nil {
			params["instrumentId"] = strconv.Itoa(id)
		}
	}
	u := c.profile.GetPrivateURL("/spot/order/getHistoryOrderPage")
	raw, err := c.http.GetSpot(u, params)
	if err != nil {
		return nil, err
	}
	return parsePageDataOrders(raw.Body)
}

// GetBalance returns the spot account balance list.
// Uses cookie auth when no spot API key is configured.
func (c *Client) GetBalance() ([]BalanceItem, error) {
	u := c.profile.GetPrivateURL("/spot/account/getAccountAsset")
	raw, err := c.http.GetSpot(u, nil)
	if err != nil {
		return nil, err
	}
	var asset AccountAsset
	if err := client.ParseAPIResponse(raw.Body, &asset); err != nil {
		return nil, err
	}
	return asset.BalanceList, nil
}

// Transfer moves funds between spot sub-accounts.
func (c *Client) Transfer(req *TransferReq) (*TransferResp, error) {
	u := c.profile.GetPrivateURL("/spot/transfer/transfer")
	raw, err := c.http.PostSpot(u, req)
	if err != nil {
		return nil, err
	}
	var dst TransferResp
	if err := client.ParseAPIResponse(raw.Body, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// tryParseOrders handles both {"orders":[...]} and [...] response shapes.
func tryParseOrders(raw []byte, dst *[]Order) error {
	// Try direct parse via standard APIResponse wrapper
	if err := client.ParseAPIResponse(raw, dst); err == nil {
		return nil
	}
	// Fallback: maybe data is wrapped in {orders:[...]}
	var envelope struct {
		Code string `json:"code"`
		Data struct {
			Orders []Order `json:"orders"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("parse orders: %w (body: %.200s)", err, raw)
	}
	*dst = envelope.Data.Orders
	return nil
}

// parsePageDataOrders parses a paginated API response and extracts orders from dataList.
func parsePageDataOrders(raw []byte) ([]Order, error) {
	var wrapper struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			DataList []Order `json:"dataList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse page response: %w (body: %.200s)", err, raw)
	}
	if wrapper.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s - %s", wrapper.Code, wrapper.Message)
	}
	if wrapper.Data.DataList == nil {
		return []Order{}, nil
	}
	return wrapper.Data.DataList, nil
}
