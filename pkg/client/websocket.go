package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"bifu-cli/pkg/clifconfig"

	"github.com/gorilla/websocket"
)

// WSMessage is a generic inbound WebSocket message.
type WSMessage struct {
	Raw []byte
}

// WSClient manages a single WebSocket connection with auto-ping and reconnect.
type WSClient struct {
	mu        sync.Mutex
	conn      *websocket.Conn
	profile   *clifconfig.Profile
	url       string
	cookie    string
	messages  chan []byte
	done      chan struct{}
	connected bool
}

// NewWSClient creates a client pointed at the given full URL.
func NewWSClient(profile *clifconfig.Profile, wsURL string) *WSClient {
	return &WSClient{
		profile:  profile,
		url:      wsURL,
		messages: make(chan []byte, 256),
		done:     make(chan struct{}),
	}
}

// NewWSMarketClient creates a client for the public market WebSocket.
func NewWSMarketClient(profile *clifconfig.Profile) *WSClient {
	return NewWSClient(profile, profile.GetWSMarketURL())
}

// NewWSPrivateClient creates a client for the private trading WebSocket.
func NewWSPrivateClient(profile *clifconfig.Profile) *WSClient {
	c := NewWSClient(profile, profile.GetWSPrivateURL())
	c.cookie = profile.Auth.AuthCookie
	return c
}

// NewForexWSClient creates a client for MT5 events WebSocket.
func NewForexWSClient(profile *clifconfig.Profile, sessionToken string) *WSClient {
	return NewWSClient(profile, profile.GetForexWSURL(sessionToken))
}

// NewPushgwWSClient creates a client for the real-time forex quote WebSocket.
func NewPushgwWSClient(profile *clifconfig.Profile) *WSClient {
	return NewWSClient(profile, profile.GetPushgwWSURL())
}

// Connect opens the WebSocket connection.
func (c *WSClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: false},
		HandshakeTimeout: 10 * time.Second,
	}

	var headers http.Header
	if c.cookie != "" {
		headers = http.Header{}
		headers.Set("Cookie", "user_auth_name="+c.cookie)
	}

	conn, _, err := dialer.Dial(c.url, headers)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", c.url, err)
	}
	c.conn = conn
	c.connected = true

	go c.readLoop()
	return nil
}

// Subscribe sends a subscription message.
func (c *WSClient) Subscribe(channels ...string) error {
	msg := map[string]interface{}{
		"op":   "subscribe",
		"args": channels,
	}
	return c.WriteJSON(msg)
}

// Unsubscribe sends an unsubscribe message.
func (c *WSClient) Unsubscribe(channels ...string) error {
	msg := map[string]interface{}{
		"op":   "unsubscribe",
		"args": channels,
	}
	return c.WriteJSON(msg)
}

// WriteJSON sends a JSON-encoded message.
func (c *WSClient) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteJSON(v)
}

// Messages returns a read-only channel of inbound raw messages.
func (c *WSClient) Messages() <-chan []byte {
	return c.messages
}

// Close terminates the connection.
func (c *WSClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.done)
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
		c.connected = false
	}
}

// URL returns the connected WebSocket URL.
func (c *WSClient) URL() string { return c.url }

func (c *WSClient) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		select {
		case c.messages <- msg:
		default:
			// drop if channel full
		}
	}
}

// ── Public WebSocket response helpers ─────────────────────────────────────────

// PublicWSResp is the envelope for public market WebSocket messages.
type PublicWSResp struct {
	Event   string          `json:"event"`
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
	Code    string          `json:"code"`
	Msg     string          `json:"msg"`
}

// ParsePublicMsg parses a raw message as a PublicWSResp.
func ParsePublicMsg(raw []byte) (*PublicWSResp, error) {
	var resp PublicWSResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
