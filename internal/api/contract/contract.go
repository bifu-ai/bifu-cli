// Package contract provides API client methods for contract/futures trading.
package contract

import (
	"encoding/json"
	"fmt"
	"strconv"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// Client is the Contract trading API client.
type Client struct {
	http    *client.HTTPClient
	profile *clifconfig.Profile
}

// New creates a Contract API client from the active profile.
func New(profile *clifconfig.Profile) *Client {
	return &Client{
		http:    client.NewContractHTTPClient(profile),
		profile: profile,
	}
}

// SetVerbose enables HTTP request/response logging.
func (c *Client) SetVerbose(v bool) { c.http.SetVerbose(v) }

// ── Types ─────────────────────────────────────────────────────────────────────

type CreateOrderReq struct {
	ContractID           string `json:"contractId"`
	MarginMode           string `json:"marginMode"`                     // SHARED | ISOLATED
	SeparatedMode        string `json:"separatedMode"`                  // COMBINED | SEPARATED
	SeparatedOpenOrderID string `json:"separatedOpenOrderId,omitempty"` // default "0"
	PositionSide         string `json:"positionSide"`                   // LONG | SHORT
	OrderSide            string `json:"orderSide"`                      // BUY | SELL
	Price                string `json:"price"`                          // "0" = market
	Size                 string `json:"size"`
	ClientOrderID        string `json:"clientOrderId"`
	Type                 string `json:"type"`        // LIMIT | MARKET | STOP_LIMIT
	TimeInForce          string `json:"timeInForce"` // GOOD_TIL_CANCEL | IMMEDIATE_OR_CANCEL | FILL_OR_KILL
	ReduceOnly           bool   `json:"reduceOnly"`
	TriggerPrice         string `json:"triggerPrice,omitempty"`
	TriggerPriceType     string `json:"triggerPriceType,omitempty"` // LAST_PRICE | MARK_PRICE
}

type OrderResp struct {
	OrderID       string `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
}

type Order struct {
	OrderID        string      `json:"orderId"`
	ContractID     interface{} `json:"contractId"`
	PositionSide   string      `json:"positionSide"`
	OrderSide      string      `json:"orderSide"`
	Type           string      `json:"type"`
	Status         string      `json:"status"`
	Price          string      `json:"price"`
	Size           string      `json:"size"`
	FilledQuantity string      `json:"filledQuantity"`
	AveragePrice   string      `json:"averagePrice"`
	ReduceOnly     bool        `json:"reduceOnly"`
	CreatedTime    interface{} `json:"createdTime"`
}

type Position struct {
	ContractID   interface{} `json:"contractId"`
	PositionSide string      `json:"positionSide"`
	MarginMode   string      `json:"marginMode"`
	Size         string      `json:"size"`
	EntryPrice   string      `json:"entryPrice"`
	MarkPrice    string      `json:"markPrice"`
	Pnl          string      `json:"unrealizedPnl"`
	Leverage     string      `json:"leverage"`
	LiqPrice     string      `json:"liquidationPrice"`
}

type accountAsset struct {
	AccountID                    string `json:"accountId"`
	CoinID                       string `json:"coinId"`
	AccountEquity                string `json:"accountEquity"`
	AccountAvailable             string `json:"accountAvailable"`
	AccountUsed                  string `json:"accountUsed"`
	AccountFrozen                string `json:"accountFrozen"`
	UnrealizePnl                 string `json:"unrealizePnl"`
	InitialMarginRequirement     string `json:"initialMarginRequirement"`
	MaintenanceMarginRequirement string `json:"maintenanceMarginRequirement"`
}

type AccountInfo struct {
	AccountID string         `json:"id"`
	UserID    string         `json:"userId"`
	Status    string         `json:"status"`
	Assets    []accountAsset // aggregated from getAccountAsset
}

type ModifyOrderReq struct {
	OrderID       string `json:"orderId,omitempty"`
	ClientOrderID string `json:"clientOrderId,omitempty"`
	NewPrice      string `json:"newPrice,omitempty"`
	NewQuantity   string `json:"newQuantity,omitempty"`
}

// ── Methods ───────────────────────────────────────────────────────────────────

// CreateOrder places a new contract order.
func (c *Client) CreateOrder(req *CreateOrderReq) (*OrderResp, error) {
	u := c.profile.GetPrivateURL("/contract/order/createOrder")
	raw, err := c.http.PostContract(u, req)
	if err != nil {
		return nil, err
	}
	var dst OrderResp
	if err := client.ParseAPIResponse(raw.Body, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

// CancelOrder cancels an open contract order by order ID or client order ID.
func (c *Client) CancelOrder(orderID, clientOrderID string) error {
	if orderID != "" {
		u := c.profile.GetPrivateURL("/contract/order/cancelOrderById")
		body := map[string]string{"orderId": orderID}
		raw, err := c.http.PostContract(u, body)
		if err != nil {
			return err
		}
		return client.ParseAPIResponse(raw.Body, nil)
	}
	if clientOrderID != "" {
		u := c.profile.GetPrivateURL("/contract/order/cancelOrderByClientOrderId")
		body := map[string]interface{}{"clientOrderIdList": []string{clientOrderID}}
		raw, err := c.http.PostContract(u, body)
		if err != nil {
			return err
		}
		return client.ParseAPIResponse(raw.Body, nil)
	}
	return fmt.Errorf("orderId or clientOrderId is required")
}

// CancelAllOrders cancels all open contract orders, optionally filtered by instrument ID.
func (c *Client) CancelAllOrders(contractID string) error {
	req := map[string]interface{}{}
	if contractID != "" {
		if id, err := strconv.Atoi(contractID); err == nil {
			req["instrumentId"] = id
		}
	}
	u := c.profile.GetPrivateURL("/contract/order/cancelAllOrders")
	raw, err := c.http.PostContract(u, req)
	if err != nil {
		return err
	}
	return client.ParseAPIResponse(raw.Body, nil)
}

// GetOrder retrieves a contract order by ID.
func (c *Client) GetOrder(orderID string) (*Order, error) {
	u := c.profile.GetPrivateURL("/contract/order/getOrderById")
	raw, err := c.http.GetContract(u, map[string]string{"orderIdList": orderID})
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

// ListOpenOrders returns active contract orders with pagination.
func (c *Client) ListOpenOrders(contractID string) ([]Order, error) {
	params := map[string]string{
		"size": "100",
	}
	if contractID != "" {
		if id, err := strconv.Atoi(contractID); err == nil {
			params["instrumentId"] = strconv.Itoa(id)
		}
	}
	u := c.profile.GetPrivateURL("/contract/order/getActiveOrderPage")
	raw, err := c.http.GetContract(u, params)
	if err != nil {
		return nil, err
	}
	return parsePageDataOrders(raw.Body)
}

// ListOrderHistory returns historical contract orders with pagination.
func (c *Client) ListOrderHistory(contractID string, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 50
	}
	params := map[string]string{
		"pageSize": strconv.Itoa(limit),
		"pageNo":   "0",
	}
	if contractID != "" {
		if id, err := strconv.Atoi(contractID); err == nil {
			params["contractId"] = strconv.Itoa(id)
		}
	}
	u := c.profile.GetPrivateURL("/contract/order/getHistoryOrderPage")
	raw, err := c.http.GetContract(u, params)
	if err != nil {
		return nil, err
	}
	return parsePageDataOrders(raw.Body)
}

// ListPositions returns paginated open positions.
func (c *Client) ListPositions(contractID string) ([]Position, error) {
	params := map[string]string{
		"pageSize": "100",
	}
	if contractID != "" {
		params["contractId"] = contractID
	}
	u := c.profile.GetPrivateURL("/contract/account/getPositionPage")
	raw, err := c.http.GetContract(u, params)
	if err != nil {
		return nil, err
	}
	return parsePageDataPositions(raw.Body)
}

// GetAccount returns the contract account summary using getAccountAsset endpoint.
func (c *Client) GetAccount() (*AccountInfo, error) {
	u := c.profile.GetPrivateURL("/contract/account/getAccountAsset")
	raw, err := c.http.GetContract(u, nil)
	if err != nil {
		return nil, err
	}
	var outer struct {
		Account AccountInfo    `json:"account"`
		Assets  []accountAsset `json:"accountAssetList"`
	}
	if err := client.ParseAPIResponse(raw.Body, &outer); err != nil {
		return nil, err
	}
	outer.Account.Assets = outer.Assets
	return &outer.Account, nil
}

// ModifyOrder modifies an existing contract order (price/size) by order ID.
func (c *Client) ModifyOrder(req *ModifyOrderReq) error {
	u := c.profile.GetPrivateURL("/contract/order/modifyOrderById")
	raw, err := c.http.PostContract(u, req)
	if err != nil {
		return err
	}
	return client.ParseAPIResponse(raw.Body, nil)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

// parsePageDataPositions parses a paginated API response and extracts positions from dataList.
func parsePageDataPositions(raw []byte) ([]Position, error) {
	var wrapper struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			DataList []Position `json:"dataList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse page response: %w (body: %.200s)", err, raw)
	}
	if wrapper.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s - %s", wrapper.Code, wrapper.Message)
	}
	if wrapper.Data.DataList == nil {
		return []Position{}, nil
	}
	return wrapper.Data.DataList, nil
}
