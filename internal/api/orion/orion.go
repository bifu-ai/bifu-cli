// Package orion provides a read-only API client for the orion signal
// subscription service (market signals + pricing).
package orion

import (
	"fmt"

	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
)

// Client is the orion API client (cookie auth; signal/subscription endpoints
// require an active subscription, price/signal-history are public).
type Client struct {
	http    *client.HTTPClient
	profile *clifconfig.Profile
}

// New creates an orion API client from the active profile.
func New(profile *clifconfig.Profile) *Client {
	return &Client{http: client.NewHTTPClient(profile), profile: profile}
}

// SetVerbose enables HTTP request/response logging.
func (c *Client) SetVerbose(v bool) { c.http.SetVerbose(v) }

// ── Types (protojson: int64 encoded as strings, double as numbers) ───────────

// Price is a subscription pricing tier.
type Price struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Unit            string  `json:"unit"`
	Quantity        int     `json:"quantity"`
	IsFree          bool    `json:"isFree"`
	Type            string  `json:"type"`
	USD             float64 `json:"usd"`
	Decode          float64 `json:"decode"`
	CanUseFiat      bool    `json:"canUseFiat"`
	CanUsePromotion bool    `json:"canUsePromotion"`
}

// Product is the instrument a signal targets.
type Product struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Source string `json:"source"`
	Dest   string `json:"dest"`
	IsPair bool   `json:"isPair"`
	Digits int    `json:"digits"`
}

// Signal is the current signal window.
type Signal struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timezone  string `json:"timezone"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	IsTodayUse bool  `json:"isTodayUse"`
}

// SignalPolicy is an actual buy/sell call (entry / stop / targets).
type SignalPolicy struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	Type          string   `json:"type"` // buy | sell
	Entry         string   `json:"entry"`
	SL            string   `json:"sl"`
	PT1           string   `json:"pt1"`
	PT2           string   `json:"pt2"`
	Trend         string   `json:"trend"`
	RealtimeState string   `json:"realtimeState"`
	Product       *Product `json:"product"`
}

// SignalResult is the /orion/signal payload.
type SignalResult struct {
	HasSubscription bool           `json:"hasSubscription"`
	Signal          *Signal        `json:"signal"`
	SignalPolicy    []SignalPolicy `json:"signalPolicy"`
}

// SignalHistoryItem is one past signal.
type SignalHistoryItem struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	Type      string   `json:"type"`
	Entry     string   `json:"entry"`
	SL        string   `json:"sl"`
	PT1       string   `json:"pt1"`
	PT2       string   `json:"pt2"`
	CreatedAt string   `json:"createdAt"`
	Product   *Product `json:"product"`
}

// SignalHistoryResult is the /orion/signal-history payload.
type SignalHistoryResult struct {
	Items []SignalHistoryItem `json:"items"`
	Total string              `json:"total"`
}

// Subscription is the user's current subscription.
type Subscription struct {
	ID                 string `json:"id"`
	SubscribePriceType string `json:"subscribePriceType"`
	SubscribePriceUnit string `json:"subscribePriceUnit"`
	IsValid            bool   `json:"isValid"`
	IsExpiry           bool   `json:"isExpiry"`
	IsStart            bool   `json:"isStart"`
	StartDate          string `json:"startDate"`
	EndDate            string `json:"endDate"`
	AutoPayment        bool   `json:"autoPayment"`
}

// ── Endpoints ────────────────────────────────────────────────────────────────

// GetPrice returns the subscription pricing tiers (public).
func (c *Client) GetPrice() ([]Price, error) {
	resp, err := c.http.GetPayment(c.profile.GetOrionURL("/price"), nil)
	if err != nil {
		return nil, err
	}
	var out []Price
	if err := client.ParsePaymentResponse(resp.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetSignal returns the current signal + active policies (needs subscription).
func (c *Client) GetSignal() (*SignalResult, error) {
	resp, err := c.http.GetPayment(c.profile.GetOrionURL("/signal"), nil)
	if err != nil {
		return nil, err
	}
	var out SignalResult
	if err := client.ParsePaymentResponse(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSignalHistory returns past signals (items require a subscription).
func (c *Client) GetSignalHistory(pageNo, pageSize int) (*SignalHistoryResult, error) {
	params := map[string]string{
		"pageNo":   fmt.Sprintf("%d", pageNo),
		"pageSize": fmt.Sprintf("%d", pageSize),
	}
	resp, err := c.http.GetPayment(c.profile.GetOrionURL("/signal-history"), params)
	if err != nil {
		return nil, err
	}
	var out SignalHistoryResult
	if err := client.ParsePaymentResponse(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSubscription returns the user's current subscription (needs login).
func (c *Client) GetSubscription() (*Subscription, error) {
	resp, err := c.http.GetPayment(c.profile.GetOrionURL("/current-subscription"), nil)
	if err != nil {
		return nil, err
	}
	var out Subscription
	if err := client.ParsePaymentResponse(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
