// Package config implements the `bifu-cli config` command group.
// Analogous to `solana config` / `near config`.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// NewConfigCmd builds the `config` command tree.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage bifu-cli configuration and profiles",
		Long: `Manage bifu-cli configuration stored at ~/.bifu-cli/config.yaml.

Supports multiple named profiles (like AWS CLI profiles).
Each profile stores endpoints, auth credentials, and connection settings.`,
	}

	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newUseCmd())
	cmd.AddCommand(newDeleteCmd())
	return cmd
}

// ── config get ────────────────────────────────────────────────────────────────

func newGetCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show the active profile configuration",
		Example: `  bifu-cli config get
  bifu-cli config get --profile dev`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			name := cfg.ActiveProfile
			// local --profile flag takes priority, then global --profile/-p flag
			if profile != "" {
				name = profile
			} else if g, _ := cmd.Root().PersistentFlags().GetString("profile"); g != "" && g != "default" {
				name = g
			}
			p, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found (run: bifu-cli config init --profile %s)", name, name)
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.Header(fmt.Sprintf("Profile: %s", output.Bold(name)))
			pr.PrintKV([]output.KV{
				{Key: "Active profile", Value: cfg.ActiveProfile},
				{Key: "Config file", Value: clifconfig.ConfigPath()},
				{Key: "Base URL", Value: p.BaseURL},
				{Key: "WebSocket URL", Value: p.WebSocketURL},
				{Key: "WS Market", Value: p.WSMarket},
				{Key: "WS Private", Value: p.WSPrivate},
				{Key: "WS Private Spot", Value: p.WSPrivateSpot},
				{Key: "gRPC Spot", Value: p.GrpcSpot},
				{Key: "gRPC Contract", Value: p.GrpcContract},
				{Key: "Public path", Value: p.PublicPath},
				{Key: "Private path", Value: p.PrivatePath},
				{Key: "HTTP timeout", Value: p.HTTPTimeout.String()},
				{Key: "Spot account ID", Value: p.Auth.SpotAccountID},
				{Key: "Contract account ID", Value: p.Auth.ContractAccountID},
				{Key: "User ID", Value: p.Auth.UserID},
				{Key: "Cookie auth", Value: maskKey(p.Auth.AuthCookie)},
				{Key: "Forex HTTP", Value: p.Forex.HTTPEndpoint},
				{Key: "Pushgw WS", Value: p.Pushgw.WSEndpoint + p.Pushgw.WSPath},
				{Key: "Locale", Value: p.Auth.Locale},
				{Key: "Terminal type", Value: p.Auth.TerminalType},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name (defaults to active profile)")
	return cmd
}

// ── config set ────────────────────────────────────────────────────────────────

func newSetCmd() *cobra.Command {
	var (
		profile      string
		baseURL      string
		wsURL        string
		grpcSpot     string
		grpcContract string
		publicPath   string
		privatePath  string
		wsMarket     string
		wsPrivate    string
		wsPrivateSpot string
		httpTimeout  string

		// Auth
		authCookie        string
		userID            string
		spotAccountID     string
		contractAccountID string
		uToken            string
		locale            string
		terminalType      string

		// Forex
		forexHTTP string
		forexGrpc string

		// Pushgw
		pushgwWS     string
		pushgwWSPath string
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set configuration values in a profile",
		Example: `  bifu-cli config set --base-url https://api.bifu.dev
  bifu-cli config set --profile dev --base-url https://api.bifu.dev
  bifu-cli config set --auth-cookie <cookie-value>
  bifu-cli config set --forex-http https://api.bifu.dev`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			name := cfg.ActiveProfile
			if profile != "" {
				name = profile
			}
			p := cfg.EnsureProfile(name)

			// Apply only the flags that were explicitly set
			setIfChanged(cmd, "base-url", func() { p.BaseURL = baseURL })
			setIfChanged(cmd, "ws-url", func() { p.WebSocketURL = wsURL })
			setIfChanged(cmd, "grpc-spot", func() { p.GrpcSpot = grpcSpot })
			setIfChanged(cmd, "grpc-contract", func() { p.GrpcContract = grpcContract })
			setIfChanged(cmd, "public-path", func() { p.PublicPath = publicPath })
			setIfChanged(cmd, "private-path", func() { p.PrivatePath = privatePath })
			setIfChanged(cmd, "ws-market", func() { p.WSMarket = wsMarket })
			setIfChanged(cmd, "ws-private", func() { p.WSPrivate = wsPrivate })
			setIfChanged(cmd, "ws-private-spot", func() { p.WSPrivateSpot = wsPrivateSpot })
			setIfChanged(cmd, "http-timeout", func() {
				if d, err := time.ParseDuration(httpTimeout); err == nil {
					p.HTTPTimeout = d
				}
			})
			setIfChanged(cmd, "auth-cookie", func() { p.Auth.AuthCookie = authCookie })
			setIfChanged(cmd, "user-id", func() { p.Auth.UserID = userID })
			setIfChanged(cmd, "spot-account-id", func() { p.Auth.SpotAccountID = spotAccountID })
			setIfChanged(cmd, "contract-account-id", func() { p.Auth.ContractAccountID = contractAccountID })
			setIfChanged(cmd, "u-token", func() { p.Auth.UToken = uToken })
			setIfChanged(cmd, "locale", func() { p.Auth.Locale = locale })
			setIfChanged(cmd, "terminal-type", func() { p.Auth.TerminalType = terminalType })
			setIfChanged(cmd, "forex-http", func() { p.Forex.HTTPEndpoint = forexHTTP })
			setIfChanged(cmd, "forex-grpc", func() { p.Forex.ManagerGrpcAddr = forexGrpc })
			setIfChanged(cmd, "pushgw-ws", func() { p.Pushgw.WSEndpoint = pushgwWS })
			setIfChanged(cmd, "pushgw-ws-path", func() { p.Pushgw.WSPath = pushgwWSPath })

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.OK("Config saved → %s  (profile: %s)", clifconfig.ConfigPath(), name)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile to update (default: active profile)")
	// Endpoint flags
	cmd.Flags().StringVar(&baseURL, "base-url", "", "HTTP API base URL")
	cmd.Flags().StringVar(&wsURL, "ws-url", "", "WebSocket base URL")
	cmd.Flags().StringVar(&grpcSpot, "grpc-spot", "", "Spot gRPC address (host:port)")
	cmd.Flags().StringVar(&grpcContract, "grpc-contract", "", "Contract gRPC address (host:port)")
	cmd.Flags().StringVar(&publicPath, "public-path", "", "Public API path prefix")
	cmd.Flags().StringVar(&privatePath, "private-path", "", "Private API path prefix")
	cmd.Flags().StringVar(&wsMarket, "ws-market", "", "Market WebSocket path")
	cmd.Flags().StringVar(&wsPrivate, "ws-private", "", "Private WebSocket path")
	cmd.Flags().StringVar(&wsPrivateSpot, "ws-private-spot", "", "Spot private WebSocket full URL or path")
	cmd.Flags().StringVar(&httpTimeout, "http-timeout", "", "HTTP timeout (e.g. 30s)")
	// Auth flags
	cmd.Flags().StringVar(&authCookie, "auth-cookie", "", "user_auth_name cookie (from browser DevTools)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID")
	cmd.Flags().StringVar(&spotAccountID, "spot-account-id", "", "Spot account ID")
	cmd.Flags().StringVar(&contractAccountID, "contract-account-id", "", "Contract account ID")
	cmd.Flags().StringVar(&uToken, "u-token", "", "u-token (gateway auth)")
	cmd.Flags().StringVar(&locale, "locale", "", "Locale header (e.g. en, zh-CN)")
	cmd.Flags().StringVar(&terminalType, "terminal-type", "", "Terminal type header (e.g. API, WEB)")
	// Forex flags
	cmd.Flags().StringVar(&forexHTTP, "forex-http", "", "MT5 HTTP endpoint")
	cmd.Flags().StringVar(&forexGrpc, "forex-grpc", "", "MT5 Manager gRPC address")
	// Pushgw flags
	cmd.Flags().StringVar(&pushgwWS, "pushgw-ws", "", "Pushgw WebSocket endpoint")
	cmd.Flags().StringVar(&pushgwWSPath, "pushgw-ws-path", "", "Pushgw WebSocket path (e.g. /pushgw/ws)")
	return cmd
}

// ── config init ───────────────────────────────────────────────────────────────

func newInitCmd() *cobra.Command {
	var profile string
	var env string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new profile with environment defaults",
		Example: `  bifu-cli config init
  bifu-cli config init --profile dev --env dev
  bifu-cli config init --profile prod --env prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			name := profile
			if name == "" {
				name = "default"
			}
			p := cfg.EnsureProfile(name)

			// Apply environment-specific defaults
			switch strings.ToLower(env) {
			case "custom":
				// No preset URLs — user fills in manually via config set
			case "staging":
				p.BaseURL = "https://fxapi.staging.bifu.co"
				p.WebSocketURL = "wss://fxapi.staging.bifu.co"
				p.WSMarket = "wss://quote.staging.bifu.co/api/v1/public/ws"
				p.WSPrivate = "wss://contract.staging.bifu.co/api/v1/private/contract/ws"
				p.WSPrivateSpot = "wss://spot.staging.bifu.co/api/v1/private/spot/ws"
				p.Pushgw.WSEndpoint = "wss://fxapi.staging.bifu.co"
				p.Pushgw.WSPath = "/pushgw/ws"
			case "prod":
				p.BaseURL = "https://fxapi.bifu.co"
				p.WebSocketURL = "wss://fxapi.bifu.co"
				p.WSMarket = "wss://quote.bifu.co/api/v1/public/ws"
				p.WSPrivate = "wss://contract.bifu.live/api/v1/private/contract/ws"
				p.WSPrivateSpot = "wss://spot.bifu.live/api/v1/private/spot/ws"
				p.Pushgw.WSEndpoint = "wss://fxapi.bifu.co"
				p.Pushgw.WSPath = "/pushgw/ws"
			default: // dev (and explicit "dev")
				p.BaseURL = "https://fxapi.bifu.dev"
				p.WebSocketURL = "wss://fxapi.bifu.dev"
				p.WSMarket = "wss://quote.bifu.dev/api/v1/public/ws"
				p.WSPrivate = "wss://contract.bifu.dev/api/v1/private/contract/ws"
				p.WSPrivateSpot = "wss://spot.bifu.dev/api/v1/private/spot/ws"
				p.Pushgw.WSEndpoint = "wss://fxapi.bifu.dev"
				p.Pushgw.WSPath = "/pushgw/ws"
			}

			if err := cfg.Save(); err != nil {
				return err
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.OK("Profile %q initialised for env=%s", name, env)
			pr.Line("  Config: %s", clifconfig.ConfigPath())
			pr.Line("  Run `bifu-cli --profile %s auth login` to authenticate.", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "default", "Profile name to initialise")
	cmd.Flags().StringVar(&env, "env", "dev", "Environment preset: custom | dev | staging | prod")
	return cmd
}

// ── config list ───────────────────────────────────────────────────────────────

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all available profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.Header("Profiles")
			var rows [][]string
			for name, p := range cfg.Profiles {
				active := ""
				if name == cfg.ActiveProfile {
					active = output.Success("✓ active")
				}
				rows = append(rows, []string{name, p.BaseURL, active})
			}
			pr.PrintTable([]string{"PROFILE", "BASE_URL", "STATUS"}, rows)
			pr.Line("\nConfig file: %s", output.Dim(clifconfig.ConfigPath()))
			return nil
		},
	}
}

// ── config use ────────────────────────────────────────────────────────────────

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Switch the active profile",
		Example: `  bifu-cli config use dev
  bifu-cli config use prod`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			if err := cfg.SetActive(args[0]); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.OK("Active profile set to %q", args[0])
			return nil
		},
	}
}

// ── config delete ─────────────────────────────────────────────────────────────

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <profile>",
		Aliases: []string{"rm"},
		Short:   "Delete a profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clifconfig.Load()
			if err != nil {
				return err
			}
			name := args[0]
			if name == cfg.ActiveProfile {
				return fmt.Errorf("cannot delete the active profile %q — switch first with `config use`", name)
			}
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			delete(cfg.Profiles, name)
			if err := cfg.Save(); err != nil {
				return err
			}
			pr := output.NewPrinter(output.FormatTable, false)
			pr.OK("Profile %q deleted", name)
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func setIfChanged(cmd *cobra.Command, flag string, fn func()) {
	if cmd.Flags().Changed(flag) {
		fn()
	}
}

func maskKey(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
