package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"bifu-cli/internal/clifconfig"

	"github.com/coder/websocket"
)

// WSMessage is a generic inbound WebSocket message.
type WSMessage struct {
	Raw []byte
}

// Keepalive tuning: the client sends a ping every pingPeriod and treats a peer
// that doesn't answer within pongWait as dead. coder/websocket's Ping blocks
// until the pong arrives (or the context expires), so a missed pong tears the
// connection down. Reconnect is intentionally NOT handled here — the caller
// decides whether to redial.
const (
	pingPeriod = 20 * time.Second
	pongWait   = 60 * time.Second
	// maxMessageBytes bounds a single inbound frame so a hostile/buggy peer can't
	// force unbounded memory growth.
	maxMessageBytes = 4 << 20 // 4 MiB
)

// WSClient manages a single WebSocket connection with an auto-ping keepalive.
type WSClient struct {
	mu         sync.Mutex
	conn       *websocket.Conn
	profile    *clifconfig.Profile
	url        string
	cookie     string
	cookieName string
	messages   chan []byte
	done       chan struct{}
	closeOnce  sync.Once
	connected  bool
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

// NewWSPrivateClient creates a client for the private contract trading WebSocket.
func NewWSPrivateClient(profile *clifconfig.Profile) *WSClient {
	c := NewWSClient(profile, profile.GetWSPrivateURL())
	c.cookie = profile.Auth.AuthCookie
	c.cookieName = profile.Auth.AuthCookieName
	return c
}

// NewWSPrivateSpotClient creates a client for the private spot trading WebSocket.
func NewWSPrivateSpotClient(profile *clifconfig.Profile) *WSClient {
	c := NewWSClient(profile, profile.GetWSPrivateSpotURL())
	c.cookie = profile.Auth.AuthCookie
	c.cookieName = profile.Auth.AuthCookieName
	return c
}

// NewPushgwWSClient creates a client for the real-time forex quote WebSocket (MT5 push gateway).
func NewPushgwWSClient(profile *clifconfig.Profile) *WSClient {
	return NewWSClient(profile, profile.GetPushgwWSURL())
}

// NewTradfiWSClient creates a client for the TradFi(Fortex) push WebSocket.
func NewTradfiWSClient(profile *clifconfig.Profile) *WSClient {
	return NewWSClient(profile, profile.GetTradfiWSURL())
}

// Connect opens the WebSocket connection.
func (c *WSClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	opts := &websocket.DialOptions{}
	if c.cookie != "" {
		opts.HTTPHeader = http.Header{}
		opts.HTTPHeader.Set("Cookie", c.cookieName+"="+c.cookie)
	}

	// Bound the handshake with a context timeout (replaces gorilla's
	// HandshakeTimeout). The dial context is independent of the connection's
	// lifetime — coder/websocket does not retain it after the handshake.
	dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, c.url, opts)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", c.url, err)
	}
	conn.SetReadLimit(maxMessageBytes)

	c.conn = conn
	c.connected = true

	go c.readLoop()
	go c.pingLoop()
	return nil
}

// pingLoop sends a periodic ping so idle connections are not dropped by the
// peer or an intermediary, and so a dead peer (no pong within pongWait) trips a
// teardown.
func (c *WSClient) pingLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()
			if conn == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), pongWait)
			err := conn.Ping(ctx)
			cancel()
			if err != nil {
				// Dead peer — close so the read loop unblocks and the caller can
				// observe the disconnect.
				c.Close()
				return
			}
		}
	}
}

// Subscribe sends subscription messages for the given channels.
// Market WS expects one message per channel: {"event":"subscribe","channel":"..."}
func (c *WSClient) Subscribe(channels ...string) error {
	for _, ch := range channels {
		msg := map[string]string{
			"event":   "subscribe",
			"channel": ch,
		}
		if err := c.WriteJSON(msg); err != nil {
			return err
		}
	}
	return nil
}

// Unsubscribe sends unsubscribe messages for the given channels.
func (c *WSClient) Unsubscribe(channels ...string) error {
	for _, ch := range channels {
		msg := map[string]string{
			"event":   "unsubscribe",
			"channel": ch,
		}
		if err := c.WriteJSON(msg); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON sends a JSON-encoded text message.
func (c *WSClient) WriteJSON(v interface{}) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

// Messages returns a read-only channel of inbound raw messages.
func (c *WSClient) Messages() <-chan []byte {
	return c.messages
}

// Close terminates the connection. It is safe to call more than once and from
// multiple goroutines — only the first call has any effect.
func (c *WSClient) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusNormalClosure, "")
			c.connected = false
		}
	})
}

// URL returns the connected WebSocket URL.
func (c *WSClient) URL() string { return c.url }

// Connected reports whether the connection is currently open.
func (c *WSClient) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *WSClient) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}
		// Each read is bounded by pongWait: any inbound frame proves the
		// connection is live, and a silent peer trips the deadline.
		ctx, cancel := context.WithTimeout(context.Background(), pongWait)
		_, msg, err := c.conn.Read(ctx)
		cancel()
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
