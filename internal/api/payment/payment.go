// Package payment provides API clients for the payment service:
// balance query, deposit, withdrawal, and inter-account transfers.
package payment

import (
	"encoding/json"
	"strings"

	"fmt"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// flexStr unmarshals from a JSON string OR number into a Go string.
// The forex close-orders endpoint returns numeric-looking fields as plain
// numbers for MT5 but as quoted strings for TradFi; this accepts both.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = flexStr(str)
		return nil
	}
	*f = flexStr(s)
	return nil
}

func (f flexStr) String() string { return string(f) }

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
	MtType      int    `json:"mtType"`   // 1=MT4, 2=MT5, 3=TradFi(Fortex)
	Currency    string `json:"currency"`
	IsDefault   bool   `json:"isDefault"`
	Enable      bool   `json:"enable"`
}

// PlatformName maps the mtType code to a human-readable trading platform name.
func (a ForexAccountItem) PlatformName() string {
	switch a.MtType {
	case 1:
		return "MT4"
	case 2:
		return "MT5"
	case 3:
		return "TradFi"
	default:
		return fmt.Sprintf("mt%d", a.MtType)
	}
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

	// TradFi(Fortex)-only optional fields (ignored by MT5 accounts).
	// When the account's mt_type=3, the backend routes to TradFi and, if these
	// are empty, derives orderType/side from Type.
	OrderType      string `json:"orderType,omitempty"`      // Market|Limit|Stop|StopLimit
	Side           string `json:"side,omitempty"`           // Buy|Sell
	Lots           string `json:"lots,omitempty"`           // alternative to volume
	StopLimitPrice string `json:"stopLimitPrice,omitempty"` // StopLimit trigger price
	ExpirationType string `json:"expirationType,omitempty"`
	FillPolicy     string `json:"fillPolicy,omitempty"`
}

// CreateForexAccountReq is the request body for POST /payment/mt5/create-forex-account.
// mt_type selects the platform: 2=MT5, 3=TradFi(Fortex).
type CreateForexAccountReq struct {
	Type      string `json:"type"`     // live | demo
	Currency  string `json:"currency"` // e.g. USD
	Leverage  int64  `json:"leverage"`
	Password  string `json:"password"`
	SubType   string `json:"subType,omitempty"` // normal | signal | copyTrade
	MtType    int32  `json:"mtType"`            // 2=MT5, 3=TradFi
	IsPremier bool   `json:"isPremier,omitempty"`
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
	Ticket     flexStr `json:"ticket"`
	Symbol     string  `json:"symbol"`
	Type       flexStr `json:"orderType"`
	Volume     flexStr `json:"lots"`
	OpenPrice  flexStr `json:"openPrice"`
	ClosePrice flexStr `json:"closePrice"`
	Profit     flexStr `json:"profit"`
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

type setUserAttributeReq struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SetUserAttribute sets an attribute on the current (cookie-authenticated) user.
// Used to self-enroll into the tradfi whitelist: SetUserAttribute("tradfi-whitelist", "1").
func (c *Client) SetUserAttribute(name, value string) error {
	u := c.profile.BaseURL + "/user/set_user_attribute"
	raw, err := c.http.PostPayment(u, setUserAttributeReq{Name: name, Value: value})
	if err != nil {
		return err
	}
	return client.ParsePaymentResponse(raw.Body, nil)
}

// CreateForexAccount creates a new forex (MT5 or TradFi) account.
// Returns the newly created account. Requires the user to be in the tradfi
// whitelist when MtType=3.
func (c *Client) CreateForexAccount(req *CreateForexAccountReq) (*ForexAccountItem, error) {
	u := c.profile.GetPaymentURL("/mt5/create-forex-account")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return nil, err
	}
	var result struct {
		Item ForexAccountItem `json:"item"`
	}
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return &result.Item, nil
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

// ── Unified Transfer ──────────────────────────────────────────────────────────

// TransferAccountType mirrors the proto enum TransferAccountType.
type TransferAccountType int32

const (
	TransferAccountTypeSaving          TransferAccountType = 1
	TransferAccountTypeForexMT5        TransferAccountType = 2
	TransferAccountTypeCryptoFunding   TransferAccountType = 5
	TransferAccountTypeCryptoSpot      TransferAccountType = 6
	TransferAccountTypeCryptoFuture    TransferAccountType = 7
	TransferAccountTypeCopyTrading     TransferAccountType = 9
	TransferAccountTypeEarn            TransferAccountType = 11
)

// UnifiedTransferReq is the request body for POST /payment/v2/transfer.
type UnifiedTransferReq struct {
	FromAccountType TransferAccountType `json:"from_account_type"`
	FromAccountID   int64               `json:"from_account_id,omitempty"`
	ToAccountType   TransferAccountType `json:"to_account_type"`
	ToAccountID     int64               `json:"to_account_id,omitempty"`
	Amount          string              `json:"amount"`
	Currency        string              `json:"currency,omitempty"`
	CoinID          int32               `json:"coin_id,omitempty"`
	MtType          int32               `json:"mt_type,omitempty"` // forex transfers: 2=MT5, 3=TradFi (0→MT5)
	Comment         string              `json:"comment,omitempty"`
}

// UnifiedTransferResult is the result embedded in the unified transfer response.
type UnifiedTransferResult struct {
	Ticket     string `json:"ticket"`
	Status     string `json:"status"`
	FromAmount string `json:"from_amount"`
	ToAmount   string `json:"to_amount"`
	Fee        string `json:"fee"`
}

// UnifiedTransfer calls POST /payment/v2/transfer (the unified transfer API).
func (c *Client) UnifiedTransfer(req *UnifiedTransferReq) (*UnifiedTransferResult, error) {
	u := c.profile.GetPaymentURL("/v2/transfer")
	raw, err := c.http.PostPayment(u, req)
	if err != nil {
		return nil, err
	}
	var result UnifiedTransferResult
	if err := client.ParsePaymentResponse(raw.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
