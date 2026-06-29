// Package cmd wires up all bifu-cli cobra commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"bifu-cli/cmd/auth"
	"bifu-cli/cmd/config"
	"bifu-cli/cmd/contract"
	"bifu-cli/cmd/forex"
	"bifu-cli/cmd/mcp"
	"bifu-cli/cmd/orion"
	"bifu-cli/cmd/payment"
	skillscmd "bifu-cli/cmd/skills"
	"bifu-cli/cmd/spot"
	"bifu-cli/cmd/ws"
	"bifu-cli/internal/client"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// version is set at build time via -ldflags "-X bifu-cli/cmd.version=...".
var version = "dev"

var (
	globalProfile string
	globalOutput  string
	globalJSON    bool
	globalVerbose bool
	globalYes     bool
)

// resolvedFormat applies the --json shortcut over --output.
func resolvedFormat() output.Format {
	if globalJSON {
		return output.FormatJSON
	}
	return output.Format(globalOutput)
}

var rootCmd = &cobra.Command{
	Use:   "bifu-cli",
	Short: "BifuFX command-line interface",
	Long: color.New(color.Bold).Sprint("bifu-cli") + " — BifuFX trading platform CLI\n\n" +
		"Manage spot, contract, forex orders, deposits, withdrawals and real-time WebSocket subscriptions.\n\n" +
		"  bifu-cli config init --env dev\n" +
		"  bifu-cli spot order create --symbol BTCUSDT --side BUY --size 0.01\n" +
		"  bifu-cli contract position list\n" +
		"  bifu-cli payment balance\n" +
		"  bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01\n" +
		"  bifu-cli ws market --channels ticker.BTCUSDT",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command, printing any error as a red ✗ line.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, output.ErrText("✗ ")+err.Error())
		return err
	}
	return nil
}

// LoadFn is the shared context-loader signature used by every subcommand.
type LoadFn = func() (*clifconfig.Profile, *output.Printer, error)

func init() {
	rootCmd.PersistentFlags().StringVarP(&globalProfile, "profile", "p", "",
		"Config profile to use (see: bifu-cli config list); defaults to active profile")
	rootCmd.PersistentFlags().StringVarP(&globalOutput, "output", "o", "table",
		"Output format: table | json | plain")
	rootCmd.PersistentFlags().BoolVar(&globalJSON, "json", false,
		"Shortcut for --output json")
	rootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false,
		"Enable verbose/debug output")
	rootCmd.PersistentFlags().BoolVarP(&globalYes, "yes", "y", false,
		"Skip confirmation prompts (assume yes)")

	_ = rootCmd.RegisterFlagCompletionFunc("output", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "plain"}, cobra.ShellCompDirectiveNoFileComp
	})

	load := loadCtx
	// Trading commands require an authenticated session. Gate them at load time
	// so an un-logged-in invocation fails fast client-side with a clear message
	// instead of firing a doomed request and surfacing a raw 401
	// (BIFU-CLI-202606-013).
	tradeLoad := requireAuth(load)

	rootCmd.AddCommand(config.NewConfigCmd())
	rootCmd.AddCommand(auth.NewAuthCmd(load))
	rootCmd.AddCommand(spot.NewSpotCmd(tradeLoad))
	rootCmd.AddCommand(contract.NewContractCmd(tradeLoad))
	rootCmd.AddCommand(payment.NewPaymentCmd(tradeLoad))
	rootCmd.AddCommand(forex.NewForexCmd(tradeLoad))
	rootCmd.AddCommand(ws.NewWSCmd(load))
	// orion endpoints are cookie-authenticated (signal subscriptions can incur
	// charges), so they get the same pre-login gate as the other trading
	// commands (BIFU-CLI-202606-013). ws stays on the plain loader — `ws market`
	// is a public stream.
	rootCmd.AddCommand(orion.NewOrionCmd(tradeLoad))
	rootCmd.AddCommand(mcp.NewMCPCmd(load))
	rootCmd.AddCommand(skillscmd.NewSkillsCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newUpgradeCmd())
}

// requireAuth wraps a LoadFn so commands that need a session fail fast when the
// active profile has no stored cookie (BIFU-CLI-202606-013).
func requireAuth(load LoadFn) LoadFn {
	return func() (*clifconfig.Profile, *output.Printer, error) {
		p, pr, err := load()
		if err != nil {
			return p, pr, err
		}
		if p.Auth.AuthCookie == "" {
			return p, pr, fmt.Errorf("not logged in — run `bifu-cli auth login` (or set the profile cookie with `bifu-cli config set`)")
		}
		return p, pr, nil
	}
}

func loadCtx() (*clifconfig.Profile, *output.Printer, error) {
	cfg, err := clifconfig.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	// If --profile was explicitly set, switch to that profile
	if globalProfile != "" {
		if err := cfg.SetActive(globalProfile); err != nil {
			fmt.Fprintf(os.Stderr,
				"warning: profile %q does not exist — create it with: bifu-cli config init --profile %s --env <dev|staging|prod>\n",
				globalProfile, globalProfile)
			cfg.EnsureProfile(globalProfile)
			cfg.ActiveProfile = globalProfile
		}
	}
	// else: use cfg.ActiveProfile from config file (set via `config use`)

	format := resolvedFormat()
	// Spinners on stderr would interleave badly with machine-readable JSON.
	client.ShowSpinner = format != output.FormatJSON

	profile := cfg.Active()
	printer := output.NewPrinter(format, globalVerbose)
	return profile, printer, nil
}

func newVersionCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print bifu-cli version (use --check to look for updates)",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("bifu-cli %s\n", version)
			if check {
				reportUpdateStatus(os.Stdout)
			}
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check whether a newer release is available")
	return cmd
}
