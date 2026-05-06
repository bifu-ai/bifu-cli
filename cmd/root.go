// Package cmd wires up all bifu-cli cobra commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"bifu-cli/cmd/config"
	"bifu-cli/cmd/contract"
	"bifu-cli/cmd/forex"
	"bifu-cli/cmd/payment"
	"bifu-cli/cmd/spot"
	"bifu-cli/cmd/ws"
	"bifu-cli/pkg/clifconfig"
	"bifu-cli/pkg/output"
)

const version = "1.0.0"

var (
	globalProfile string
	globalOutput  string
	globalVerbose bool
)

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
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// LoadFn is the shared context-loader signature used by every subcommand.
type LoadFn = func() (*clifconfig.Profile, *output.Printer, error)

func init() {
	rootCmd.PersistentFlags().StringVarP(&globalProfile, "profile", "p", "default",
		"Config profile to use (see: bifu-cli config list)")
	rootCmd.PersistentFlags().StringVarP(&globalOutput, "output", "o", "table",
		"Output format: table | json | plain")
	rootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false,
		"Enable verbose/debug output")

	load := loadCtx

	rootCmd.AddCommand(config.NewConfigCmd())
	rootCmd.AddCommand(spot.NewSpotCmd(load))
	rootCmd.AddCommand(contract.NewContractCmd(load))
	rootCmd.AddCommand(payment.NewPaymentCmd(load))
	rootCmd.AddCommand(forex.NewForexCmd(load))
	rootCmd.AddCommand(ws.NewWSCmd(load))
	rootCmd.AddCommand(newVersionCmd())
}

func loadCtx() (*clifconfig.Profile, *output.Printer, error) {
	cfg, err := clifconfig.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	if globalProfile != "" && globalProfile != "default" {
		if err := cfg.SetActive(globalProfile); err != nil {
				fmt.Fprintf(os.Stderr, "warning: profile %q not found, using defaults\n", globalProfile)
			cfg.EnsureProfile(globalProfile)
			cfg.ActiveProfile = globalProfile
		}
	}

	profile := cfg.Active()
	printer := output.NewPrinter(output.Format(globalOutput), globalVerbose)
	return profile, printer, nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print bifu-cli version",
		Run: func(cmd *cobra.Command, args []string) {
				fmt.Printf("bifu-cli %s\n", version)
		},
	}
}
