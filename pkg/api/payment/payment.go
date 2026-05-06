// Package payment provides API clients for the payment service:
// balance query, deposit, withdrawal, and inter-account transfers.
package payment

import (
	"fmt"

	"bifu-cli/pkg/client"
	"bifu-cli/pkg/clifconfig"
)

// Client is the Payment API client (cookie auth).
type Client struct {
	http    *client.HTTPClient
	profile *clifconfig.Profile
}

// New creates a Payment API client.
func New(profile *clifconfig.Profile) *Client {
	return &Client{
		http:    client.NewPaymentHTTPClient(profile),
		profile: profile,
	}
}

// SetVerbose enables HTTP request/response logging.
func (c *Client) SetVerbose(v bool) { c.http.SetVerbose(v) }

// ── Types ─────────────────────────────────────────────────────────────────────

type SavingBalanceResult struct {
	Items    []SavingAccountItem `json:"items"`
	TotalUSD string              `json:"totalUSD"`
}

type SavingAccountItem struct {
	ID               string `json:"id"`
	Currency         string `json:"currency"`
	Balance          string `json:"balance"`
	FrozenBalance    string `json:"frozenBalance"`
	AvailableBalance string `json:"availableBalance"`
}

type TotalBalanceResult struct {
	Balance  string `json:"balance"`
	Currency struct {
		Code string `json:"code"`
	} `json:"currency"`
	Saving    BalanceBreakdown `json:"saving"`
	Forex     BalanceBreakdown `json:"forex"`
	CopyTrade BalanceBreakdown `json:"copyTrade"`
}

type BalanceBreakdown struct {
	Balance string `json:"balance"`
	Ratio   string `json:"ratio"`
}

type ForexAccountItem struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	SubType     string `json:"subType"`
	Leverage    string `json:"leverage"`
	Balance     string `json:"balance"`
	Equity      string `json:"equity"`
	MarginFree  string `json:"marginFree"`
	Margin      string `json:"margin"`
	MarginLevel string `json:"marginLevel"`
	Group       string `json:"group"`
	GroupType   string `json:"groupType"`
	IsDefault   bool   `json:"isDefault"`
	Enable      bool   `json:"enable"`
}

type CreateForexOrderReq struct {
	LoginID    int64   `json:"loginId"`
	Symbol     string  `json:"symbol"`
	Volume     float64 `json:"volume"`
	Type       string  `json:"type"`  // buy|sell|buyLimit|sellLimit|buyStop|sellStop
	Price      float64 `json:"price"` // 0 = market order
	SL         float64 `json:"sl"`    // 0 = no stop loss
	TP         float64 `json:"tp"`    // 0 = no take profit
	Comment    string  `json:"comment"`
	Expiration string  `json:"expiration,omitempty"`
}

type CreateForexOrderResult struct {
	OrderID interface{} `json:"orderId"`
}

type ModifyForexOrderReq struct {
	LoginID int64   `json:"loginId"`
	OrderID int64   `json:"orderId"`
	SL      float64 `json:"sl,omitempty"`
	TP      float64 `json:"tp,omitempty"`
	Price   float64 `json:"price,omitempty"`
}

type CloseForexOrderReq struct {
	LoginID int64   `json:"loginId"`
	OrderID int64   `json:"orderId"`
	Volume  float64 `json:"volume"` // 0 = full close
}

type CancelForexOrderReq struct {
	LoginID int64 `json:"login_id"`
	OrderID int64 `json:"order_id"`
}

type BatchCloseForexOrderReq struct {
	LoginID  int64   `json:"loginId"`
	OrderIDs []int64 `json:"orderIds"`
	Volume   float64 `json:"volume"` // 0 = full close
}

type BatchCancelForexOrderReq struct {
	LoginID  int64   `json:"loginId"`
	OrderIDs []int64 `json:"orderIds"`
}

type BatchOrderResult struct {
	OrderID interface{} `json:"orderId"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
}

type ForexOpenOrder struct {
	Ticket     string `json:"ticket"`
	Symbol     string `json:"symbol"`
	OrderType  string `json:"orderType"`
	Volume     string `json:"volume"`
	OpenPrice  string `json:"openPrice"`
	OpenTime   string `json:"openTime"`
	StopLoss   string `json:"stopLoss"`
	TakeProfit string `json:"takeProfit"`
	Profit     string `json:"profit"`
	State      string `json:"state"`
}

type ForexHistoryOrder struct {
	Ticket     int64   `json:"ticket"`
	Symbol     string  `json:"symbol"`
	Type       string  `json:"type"`
	Volume     float64 `json:"volume"`
	OpenPrice  float64 `json:"openPrice"`
	ClosePrice float64 `json:"closePrice"`
	Profit     float64 `json:"profit"`
	OpenTime   string  `json:"openTime"`
	CloseTime  string  `json:"closeTime"`
}

type TransferReq struct {
	SavingAccountID int64  `json:"saving_account_id"`
	ForexAccountID  int64  `json:"forex_account_id"` // MT5 login ID
	Amount          string `json:"amount"`
	Currency        string `json:"currency"`
	Type            int64  `json:"type"` // 1=forex_to_saving, 2=saving_to_forex
}

type DepositCheckoutReq struct {
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount"`
	GatewayID string  `json:"paymentGatewayId"`
}

// ── Methods ───────────────────────────────────────────────────────────────────

// GetSavingBalance queries the fiat saving account balance.
func (c *Client) GetSavingBalance(currency string) (*SavingBalanceResult, error) {
	params := map[string]string{}
	if currency != "" {
		params["currency"] = currency
	}
	u := c.profile.GetPaymentURL("/saving-balance")
	raw, err := c.http.GetPayment(u, params)
	if err != nil {
		return nil, err
	}
	var result SavingBalanceResult
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTotalBalance queries the total aggregated balance across all account types.
func (c *Client) GetTotalBalance(currency string) (*TotalBalanceResult, error) {
	params := map[string]string{}
	if currency != "" {
		params["currency"] = currency
	}
	u := c.profile.GetPaymentURL("/total-balance")
	raw, err := c.http.GetPayment(u, params)
	if err != nil {
		return nil, err
	}
	var result TotalBalanceResult
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateForexOrder places a forex order (market/limit/stop).
func (c *Client) CreateForexOrder(req *CreateForexOrderReq) (*CreateForexOrderResult, error) {
	u := c.profile.GetPaymentURL("/forex/create-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return nil, err
	}
	var result CreateForexOrderResult
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ModifyForexOrder changes SL/TP or pending price on an existing order.
func (c *Client) ModifyForexOrder(req *ModifyForexOrderReq) error {
	u := c.profile.GetPaymentURL("/forex/modify-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return err
	}
	return client.ParsePaymentResponse(raw.Body, nil)
}

// CloseForexOrder closes (or partially closes) an open forex position.
func (c *Client) CloseForexOrder(req *CloseForexOrderReq) error {
	u := c.profile.GetPaymentURL("/forex/close-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return err
	}
	return client.ParsePaymentResponse(raw.Body, nil)
}

// CancelForexOrder cancels a pending forex order (limit/stop).
func (c *Client) CancelForexOrder(req *CancelForexOrderReq) error {
	u := c.profile.GetPaymentURL("/forex/cancel-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return err
	}
	return client.ParsePaymentResponse(raw.Body, nil)
}

// GetForexCloseOrders fetches historical closed forex orders.
func (c *Client) GetForexCloseOrders(loginID int64, from, to string, pageSize, pageNum int) ([]ForexHistoryOrder, error) {
	params := map[string]string{
		"login_id":  fmt.Sprintf("%d", loginID),
		"from_date": from,
		"to_date":   to,
		"page_size": fmt.Sprintf("%d", pageSize),
		"page_num":  fmt.Sprintf("%d", pageNum),
	}
	u := c.profile.GetPaymentURL("/forex/close-orders")
	raw, err := c.http.GetPayment(u, params)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Orders []ForexHistoryOrder `json:"orders"`
	}
	if err := client.ParsePaymentResponse(raw.Body, &envelope); err != nil {
		return nil, err
	}
	return envelope.Orders, nil
}

// GetForexOpenOrders fetches currently open forex positions and pending orders.
func (c *Client) GetForexOpenOrders(loginID int64) ([]ForexOpenOrder, error) {
	params := map[string]string{
		"login_id": fmt.Sprintf("%d", loginID),
	}
	u := c.profile.GetPaymentURL("/forex/open-orders")
	raw, err := c.http.GetPayment(u, params)
	if err != nil {
		return nil, err
	}
	// API returns result as a direct array: {"result": [...]}
	var orders []ForexOpenOrder
	if err := client.ParsePaymentResponse(raw.Body, &orders); err != nil {
		// Fallback: try object wrapper {"orders": [...]}
		var envelope struct {
			Orders []ForexOpenOrder `json:"orders"`
		}
		if err2 := client.ParsePaymentResponse(raw.Body, &envelope); err2 != nil {
			return nil, err
		}
		return envelope.Orders, nil
	}
	return orders, nil
}

// BatchCloseForexOrder closes multiple positions in a single API call.
func (c *Client) BatchCloseForexOrder(req *BatchCloseForexOrderReq) ([]BatchOrderResult, error) {
	u := c.profile.GetPaymentURL("/forex/batch-close-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Results []BatchOrderResult `json:"results"`
	}
	if err := client.ParsePaymentResponse(raw.Body, &envelope); err != nil {
		return nil, err
	}
	return envelope.Results, nil
}

// BatchCancelForexOrder cancels multiple pending orders in a single API call.
func (c *Client) BatchCancelForexOrder(req *BatchCancelForexOrderReq) ([]BatchOrderResult, error) {
	u := c.profile.GetPaymentURL("/forex/batch-cancel-order")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Results []BatchOrderResult `json:"results"`
	}
	if err := client.ParsePaymentResponse(raw.Body, &envelope); err != nil {
		return nil, err
	}
	return envelope.Results, nil
}

// GetForexAccountList fetches all forex (MT5) accounts linked to the user.
func (c *Client) GetForexAccountList() ([]ForexAccountItem, error) {
	u := c.profile.GetPaymentURL("/forex-account/list")
	raw, err := c.http.GetPayment(u, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Items []ForexAccountItem `json:"items"`
	}
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Transfer moves funds between saving and forex accounts.
// req.Type: 1=forex_to_saving, 2=saving_to_forex.
func (c *Client) Transfer(req *TransferReq) error {
	u := c.profile.GetPaymentURL("/forex-saving/transfer")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return err
	}
	return client.ParsePaymentResponse(raw.Body, nil)
}
