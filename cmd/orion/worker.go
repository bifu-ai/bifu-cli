package orion

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	paymentapi "bifu-cli/internal/api/payment"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/relay"
)

func newWorkerCmd(load LoadFn) *cobra.Command {
	var (
		gatewayURL          string
		apiKey              string
		workerID            string
		loginID             int64
		symbol              string
		volume              float64
		live                bool
		defaultSourceStatus string
		stateFile           string
	)

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run as a signal-relay worker: receive gateway signals and trade them on a bifu forex account",
		Long: `Connects to the signal-relay gateway over WebSocket, receives parsed trading
signals (open/close/modify/cancel), and executes them on the given MT5 login
via the bifu forex API.

Safety: without --live every action is logged as a dry-run. Per-source rules
(test/live/disabled) are remembered in the state file and can be switched from
the gateway admin UI; a source only trades for real when BOTH --live is set
and its rule is "live".`,
		Example: `  # dry-run first (recommended)
  bifu-cli orion worker --api-key gwk_xxx --login-id 90390034 --symbol XAUUSD

  # real orders, 0.01 lots per signal unless the signal carries its own size
  bifu-cli orion worker --api-key gwk_xxx --login-id 90390034 --symbol XAUUSD --volume 0.01 --live`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			if apiKey == "" {
				apiKey = os.Getenv("BIFU_RELAY_API_KEY")
			}
			if apiKey == "" {
				return fmt.Errorf("missing gateway worker key: pass --api-key or set BIFU_RELAY_API_KEY")
			}
			if loginID <= 0 {
				return fmt.Errorf("--login-id is required")
			}
			if volume <= 0 || math.IsNaN(volume) || math.IsInf(volume, 0) {
				return fmt.Errorf("--volume must be a positive number")
			}
			if defaultSourceStatus != "test" && defaultSourceStatus != "live" {
				return fmt.Errorf("--default-source-status must be test or live")
			}
			if workerID == "" {
				host, _ := os.Hostname()
				workerID = "bifu-cli-" + host
			}
			if stateFile == "" {
				stateFile = filepath.Join(clifconfig.ConfigDir(), "orion-worker-state.json")
			}

			api := paymentapi.New(p)
			api.SetVerbose(pr.Verbose)
			w := relay.New(relay.Config{
				GatewayURL:          gatewayURL,
				APIKey:              apiKey,
				WorkerID:            workerID,
				LoginID:             loginID,
				Symbol:              symbol,
				Volume:              volume,
				Live:                live,
				DefaultSourceStatus: defaultSourceStatus,
				StateFile:           stateFile,
			}, api, pr)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := w.Run(ctx); err != nil && ctx.Err() == nil {
				return err
			}
			pr.Line("worker stopped.")
			return nil
		},
	}

	cmd.Flags().StringVar(&gatewayURL, "gateway-url", "wss://gw.relaysignal.dev", "signal-relay gateway WebSocket base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "gateway worker key (gwk_…, or env BIFU_RELAY_API_KEY)")
	cmd.Flags().StringVar(&workerID, "worker-id", "", "worker id shown in the gateway admin UI (default bifu-cli-<hostname>)")
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID to trade on")
	cmd.Flags().StringVar(&symbol, "symbol", "XAUUSD", "MT5 symbol to trade")
	cmd.Flags().Float64Var(&volume, "volume", 0.01, "default lot size when the signal has none")
	cmd.Flags().BoolVar(&live, "live", false, "place real orders (default: dry-run, only log actions)")
	cmd.Flags().StringVar(&defaultSourceStatus, "default-source-status", "test", "rule for newly seen signal sources: test|live")
	cmd.Flags().StringVar(&stateFile, "state-file", "", "path for last_seq/source-rule state (default <config-dir>/orion-worker-state.json)")
	_ = cmd.MarkFlagRequired("login-id")
	return cmd
}
