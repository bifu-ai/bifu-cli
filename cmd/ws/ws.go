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
	cmd.AddCommand(newWSForexCmd(load))
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
				{Key: "Private WS", Value: p.GetWSPrivateURL()},
				{Key: "Forex WS", Value: p.GetForexWSURL("")},
				{Key: "Pushgw WS", Value: p.GetPushgwWSURL()},
			})
			return nil
		},
	})

	// ws config set
	var marketURL, privateURL, forexWS, forexPath, pushgwWS, pushgwPath string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set WebSocket endpoints in the active profile",
		Example: `  bifu-cli ws config set --market-url wss://api.bifu.dev --market-path /api/v1/public/market/ws
  bifu-cli ws config set --forex-ws wss://mt.api.com --forex-path /mt5/Events`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			p := cfg.Active()
			if cmd.Flags().Changed("market-url") {
				p.WebSocketURL = marketURL
			}
			if cmd.Flags().Changed("private-url") {
				p.WebSocketURL = privateURL
			}
			if cmd.Flags().Changed("ws-market") {
				p.WSMarket = marketURL
			}
			if cmd.Flags().Changed("ws-private") {
				p.WSPrivate = privateURL
			}
			if cmd.Flags().Changed("forex-ws") {
				p.Forex.WSEndpoint = forexWS
			}
			if cmd.Flags().Changed("forex-path") {
				p.Forex.WSPath = forexPath
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
	setCmd.Flags().StringVar(&marketURL, "ws-market", "", "Market WS path")
	setCmd.Flags().StringVar(&privateURL, "ws-private", "", "Private WS path")
	setCmd.Flags().StringVar(&forexWS, "forex-ws", "", "Forex WS base URL")
	setCmd.Flags().StringVar(&forexPath, "forex-path", "", "Forex WS path")
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
		Example: `  bifu-cli ws market --channels ticker.BTCUSDT
  bifu-cli ws market --channels ticker.BTCUSDT,depth.ETHUSDT --pretty`,
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
			return streamMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().StringSliceVar(&channels, "channels", nil, "Channel(s) to subscribe (e.g. ticker.BTCUSDT)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
	_ = cmd.MarkFlagRequired("channels")
	return cmd
}

// ── ws private ────────────────────────────────────────────────────────────────

func newWSPrivateCmd(load LoadFn) *cobra.Command {
	var channels []string
	var pretty bool
	cmd := &cobra.Command{
		Use:   "private",
		Short: "Subscribe to private trading events stream",
		Example: `  bifu-cli ws private
  bifu-cli ws private --channels order,position --pretty`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			pr.Header("Private WebSocket: " + p.GetWSPrivateURL())
			pr.Line("Press Ctrl+C to stop\n")

			ws := wsclient.NewWSPrivateClient(p)
			if err := ws.Connect(); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ws.Close()

			if len(channels) > 0 {
				if err := ws.Subscribe(channels...); err != nil {
					return err
				}
			}
			return streamMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().StringSliceVar(&channels, "channels", nil, "Channel(s) to subscribe")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
	return cmd
}

// ── ws forex ──────────────────────────────────────────────────────────────────

func newWSForexCmd(load LoadFn) *cobra.Command {
	var loginID int64
	var pretty bool
	cmd := &cobra.Command{
		Use:   "forex",
		Short: "Subscribe to forex (MT5) real-time events",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			url := p.GetForexWSURL("")
			if url == "" {
				return fmt.Errorf("forex WebSocket URL not configured — run: bifu-cli ws config set --forex-ws wss://...")
			}
			pr.Header("Forex WebSocket: " + url)
			if loginID > 0 {
				pr.Line("Login ID: %d", loginID)
			}
			pr.Line("Press Ctrl+C to stop\n")

			ws := wsclient.NewForexWSClient(p, "")
			if err := ws.Connect(); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer ws.Close()

			if loginID > 0 {
				_ = ws.WriteJSON(map[string]interface{}{
					"action":  "subscribe",
					"loginId": loginID,
				})
			}
			return streamMessages(ws, pr, pretty)
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 login ID to subscribe to")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Pretty-print JSON messages")
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
