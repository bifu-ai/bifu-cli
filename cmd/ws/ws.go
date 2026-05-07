// Package ws implements the `bifu-cli ws` command group.
// WebSocket config and real-time streaming.
package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	wsclient "bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewWSCmd builds the `ws` command tree.
func NewWSCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ws",
		Short: "WebSocket streaming and configuration",
		Long: `Subscribe to real-time market data or private trading events over WebSocket.

Config subcommand manages WebSocket endpoints stored in the active profile.

  bifu-cli ws config set --market-url wss://api.bifu.dev --private-url wss://api.bifu.dev
  bifu-cli ws market --channels ticker.BTCUSDT,depth.BTCUSDT
  bifu-cli ws private
  bifu-cli ws forex
  bifu-cli ws pushgw`,
	}
	cmd.AddCommand(newWSConfigCmd(load))
	cmd.AddCommand(newWSMarketCmd(load))
	cmd.AddCommand(newWSPrivateCmd(load))
	cmd.AddCommand(newWSPushgwCmd(load))
	return cmd
}

// ── ws config ─────────────────────────────────────────────────────────────────

func newWSConfigCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage WebSocket endpoint configuration",
	}

	// ws config show
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current WebSocket endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			pr.Header("WebSocket Endpoints")
			pr.PrintKV([]output.KV{
				{Key: "Market WS", Value: p.GetWSMarketURL()},
				{Key: "Private WS (contract)", Value: p.GetWSPrivateURL()},
				{Key: "Private WS (spot)", Value: p.GetWSPrivateSpotURL()},
				{Key: "Pushgw WS", Value: p.GetPushgwWSURL()},
			})
			return nil
		},
	})

	// ws config set
	var marketURL, privateURL, pushgwWS, pushgwPath, wsMarket, wsPrivate, wsPrivateSpot string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set WebSocket endpoints in the active profile",
		Example: `  bifu-cli ws config set --market-url wss://api.bifu.dev
  bifu-cli ws config set --pushgw-ws wss://api.bifu.dev --pushgw-path /pushgw/ws`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			p := cfg.Active()
			if cmd.Flags().Changed("market-url") && cmd.Flags().Changed("ws-market") {
				// Both provided: combine into full URL stored in WSMarket
				p.WSMarket = marketURL + wsMarket
			} else if cmd.Flags().Changed("market-url") {
				p.WebSocketURL = marketURL
			} else if cmd.Flags().Changed("ws-market") {
				p.WSMarket = wsMarket
			}
			if cmd.Flags().Changed("private-url") && cmd.Flags().Changed("ws-private") {
				// Both provided: combine into full URL
				p.WSPrivate = privateURL + wsPrivate
			} else if cmd.Flags().Changed("private-url") {
				// Full URL goes directly to WSPrivate (never touches shared WebSocketURL)
				p.WSPrivate = privateURL
			} else if cmd.Flags().Changed("ws-private") {
				p.WSPrivate = wsPrivate
			}
			if cmd.Flags().Changed("ws-private-spot") {
				p.WSPrivateSpot = wsPrivateSpot
			}
			if cmd.Flags().Changed("pushgw-ws") {
				p.Pushgw.WSEndpoint = pushgwWS
			}
			if cmd.Flags().Changed("pushgw-path") {
				p.Pushgw.WSPath = pushgwPath
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			_, pr, _ := load()
			pr.OK("WebSocket config updated")
			return nil
		},
	}
	setCmd.Flags().StringVar(&marketURL, "market-url", "", "Market WebSocket base URL")
	setCmd.Flags().StringVar(&privateURL, "private-url", "", "Private WebSocket base URL")
	setCmd.Flags().StringVar(&wsMarket, "ws-market", "", "Market WS path")
	setCmd.Flags().StringVar(&wsPrivate, "ws-private", "", "Private WS path (contract)")
	setCmd.Flags().StringVar(&wsPrivateSpot, "ws-private-spot", "", "Private WS path or full URL (spot)")
	setCmd.Flags().StringVar(&pushgwWS, "pushgw-ws", "", "Pushgw WS base URL")
	setCmd.Flags().StringVar(&pushgwPath, "pushgw-path", "", "Pushgw WS path")
	cmd.AddCommand(setCmd)
	return cmd
}

// ── ws market ─────────────────────────────────────────────────────────────────

func newWSMarketCmd(load LoadFn) *cobra.Command {
	var channels []string
	var pretty bool
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Subscribe to public market data stream",
		Example: `  bifu-cli ws market --channels ticker.10000001
  bifu-cli ws market --channels ticker.all
  bifu-cli ws market --channels ticker.10000001,depth.10000001.15 --pretty`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			url := p.GetWSMarketURL()
			pr.Header("Market WebSocket: " + url)
			pr.Line("Channels: %s", strings.Join(channels, ", "))
			pr.Line("Press Ctrl+C to stop\n")

			ws := wsclient.NewWSMarketClient(p)
			if err := ws.Connect(); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ws.Close()

			if err := ws.Subscribe(channels...); err != nil {
				return err
			}
			return streamMarketMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().StringSliceVar(&channels, "channels", nil, "Channel(s) to subscribe (e.g. ticker.10000001, ticker.all, depth.10000001.15)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
	_ = cmd.MarkFlagRequired("channels")
	return cmd
}

// ── ws private ────────────────────────────────────────────────────────────────

func newWSPrivateCmd(load LoadFn) *cobra.Command {
	var pretty bool
	var spot bool
	cmd := &cobra.Command{
		Use:   "private",
		Short: "Subscribe to private trading events stream (contract by default)",
		Example: `  bifu-cli ws private
  bifu-cli ws private --spot
  bifu-cli ws private --pretty`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			var ws *wsclient.WSClient
			if spot {
				ws = wsclient.NewWSPrivateSpotClient(p)
			} else {
				ws = wsclient.NewWSPrivateClient(p)
			}
			pr.Header("Private WebSocket: " + ws.URL())
			pr.Line("Press Ctrl+C to stop\n")

			if err := ws.Connect(); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ws.Close()

			return streamPrivateMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
	cmd.Flags().BoolVar(&spot, "spot", false, "Connect to spot private WS instead of contract")
	return cmd
}

// ── ws pushgw ─────────────────────────────────────────────────────────────────

func newWSPushgwCmd(load LoadFn) *cobra.Command {
	var pretty bool
	var symbols []string
	var loginIDs []int64
	var marketWatch bool
	cmd := &cobra.Command{
		Use:   "pushgw",
		Short: "Subscribe to push gateway real-time events",
		Long: `Connect to the push gateway WebSocket and subscribe to market or trading events.

Three subscription modes (can be combined):
  --market-watch       全品种行情快照 + 增量推送 (market_watch event)
  --symbols EURUSD,... 指定品种 tick 推送
  --login-ids 123,...  MT5 账户持仓/订单实时推送 (orderEvent)`,
		Example: `  # 全品种行情
  bifu-cli ws pushgw --market-watch

  # 指定品种 tick
  bifu-cli ws pushgw --symbols EURUSD,XAUUSD,BTCUSD

  # MT5 账户订单/持仓事件
  bifu-cli ws pushgw --login-ids 90390034

  # 组合：行情 + 账户事件
  bifu-cli ws pushgw --market-watch --login-ids 90390034 --pretty`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			url := p.GetPushgwWSURL()
			if url == "" {
				return fmt.Errorf("pushgw WebSocket URL not configured — run: bifu-cli ws config set --pushgw-ws wss://...")
			}
			pr.Header("Pushgw WebSocket: " + url)
			if marketWatch {
				pr.Line("Subscriptions: market_watch (all symbols)")
			}
			if len(symbols) > 0 {
				pr.Line("Subscriptions: symbols=%s", strings.Join(symbols, ","))
			}
			if len(loginIDs) > 0 {
				ids := make([]string, len(loginIDs))
				for i, id := range loginIDs {
					ids[i] = fmt.Sprintf("%d", id)
				}
				pr.Line("Subscriptions: orderEvent login_ids=%s", strings.Join(ids, ","))
			}
			pr.Line("Press Ctrl+C to stop\n")

			ws := wsclient.NewPushgwWSClient(p)
			if err := ws.Connect(); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ws.Close()

			// Send subscriptions
			if marketWatch {
				if err := ws.WriteJSON(map[string]string{"type": "sub", "event": "market_watch"}); err != nil {
					return fmt.Errorf("subscribe market_watch: %w", err)
				}
			}
			if len(symbols) > 0 {
				if err := ws.WriteJSON(map[string]interface{}{
					"type":  "sub",
					"event": "symbol_update_batch",
					"args":  symbols,
				}); err != nil {
					return fmt.Errorf("subscribe symbols: %w", err)
				}
			}
			if len(loginIDs) > 0 {
				if err := ws.WriteJSON(map[string]interface{}{
					"type":      "sub",
					"event":     "orderEvent",
					"login_ids": loginIDs,
				}); err != nil {
					return fmt.Errorf("subscribe orderEvent: %w", err)
				}
			}

			return streamPushgwMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
	cmd.Flags().BoolVar(&marketWatch, "market-watch", false, "Subscribe to market_watch (all symbols snapshot + updates)")
	cmd.Flags().StringSliceVar(&symbols, "symbols", nil, "Symbol(s) to subscribe for tick updates (e.g. EURUSD,XAUUSD)")
	cmd.Flags().Int64SliceVar(&loginIDs, "login-ids", nil, "MT5 login ID(s) for orderEvent subscription")
	return cmd
}

// streamPushgwMessages is like streamMessages but uses pushgw ping format {"type":"ping"}.
func streamPushgwMessages(ws *wsclient.WSClient, pr *output.Printer, pretty bool) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Println("\nDisconnected.")
			return nil
		case <-ticker.C:
			_ = ws.WriteJSON(map[string]string{"type": "ping"})
		case msg, ok := <-ws.Messages():
			if !ok {
				return fmt.Errorf("WebSocket connection closed")
			}
			// Skip empty/null heartbeat responses
			trimmed := strings.TrimSpace(string(msg))
			if trimmed == "null" || trimmed == "" {
				continue
			}
			if pretty {
				var v interface{}
				if err := json.Unmarshal(msg, &v); err == nil {
					b, _ := json.MarshalIndent(v, "", "  ")
					fmt.Println(string(b))
					continue
				}
			}
			ts := time.Now().Format("15:04:05.000")
			pr.Line("[%s] %s", output.Dim(ts), string(msg))
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// streamMarketMessages reads market WS messages. Sends {event:ping} every 30s to keep alive.
// Responds to server pings: {event:"ping",time:"..."} → {event:"pong",time:"..."}
func streamMarketMessages(ws *wsclient.WSClient, pr *output.Printer, pretty bool) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Println("\nDisconnected.")
			return nil
		case <-ticker.C:
			_ = ws.WriteJSON(map[string]string{"event": "ping"})
		case msg, ok := <-ws.Messages():
			if !ok {
				return fmt.Errorf("WebSocket connection closed")
			}
			// Auto-respond to server-initiated pings
			var m struct {
				Event string `json:"event"`
				Time  string `json:"time"`
			}
			if json.Unmarshal(msg, &m) == nil && m.Event == "ping" {
				_ = ws.WriteJSON(map[string]string{"event": "pong", "time": m.Time})
				continue
			}
			printMessage(msg, pr, pretty)
		}
	}
}

// streamPrivateMessages reads private WS messages. Auto-responds to server pings.
// Server sends {type:"ping",time:"..."} — client must respond {type:"pong",time:"..."}
func streamPrivateMessages(ws *wsclient.WSClient, pr *output.Printer, pretty bool) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			fmt.Println("\nDisconnected.")
			return nil
		case msg, ok := <-ws.Messages():
			if !ok {
				return fmt.Errorf("WebSocket connection closed")
			}
			// Auto-respond to server-initiated pings
			var m struct {
				Type string `json:"type"`
				Time string `json:"time"`
			}
			if json.Unmarshal(msg, &m) == nil && m.Type == "ping" {
				_ = ws.WriteJSON(map[string]string{"type": "pong", "time": m.Time})
				continue
			}
			printMessage(msg, pr, pretty)
		}
	}
}

// streamMessages reads from ws.Messages() until Ctrl-C or ws closes.
func streamMessages(ws *wsclient.WSClient, pr *output.Printer, pretty bool) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Println("\nDisconnected.")
			return nil
		case <-ticker.C:
			_ = ws.WriteJSON(map[string]string{"type": "ping"})
		case msg, ok := <-ws.Messages():
			if !ok {
				return fmt.Errorf("WebSocket connection closed")
			}
			printMessage(msg, pr, pretty)
		}
	}
}

// printMessage prints a WS message to the terminal, optionally pretty-printing JSON.
func printMessage(msg []byte, pr *output.Printer, pretty bool) {
	if pretty {
		var v interface{}
		if err := json.Unmarshal(msg, &v); err == nil {
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(b))
			return
		}
	}
	ts := time.Now().Format("15:04:05.000")
	pr.Line("[%s] %s", output.Dim(ts), string(msg))
}
