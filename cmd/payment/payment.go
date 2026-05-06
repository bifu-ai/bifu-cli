// Package payment implements the `bifu-cli payment` command group.
package payment

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	paymentapi "bifu-cli/pkg/api/payment"
	"bifu-cli/pkg/clifconfig"
	"bifu-cli/pkg/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewPaymentCmd builds the `payment` command tree.
func NewPaymentCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payment",
		Short: "Payment and fund management (balances, deposits, withdrawals, transfers)",
	}
	cmd.AddCommand(newBalanceCmd(load))
	cmd.AddCommand(newTransferCmd(load))
	cmd.AddCommand(newForexAccountsCmd(load))
	return cmd
}

func newClient(load LoadFn) (*paymentapi.Client, *output.Printer, error) {
	p, pr, err := load()
	if err != nil {
		return nil, nil, err
	}
	return paymentapi.New(p), pr, nil
}

// ── payment balance ───────────────────────────────────────────────────────────

func newBalanceCmd(load LoadFn) *cobra.Command {
	var currency string
	var total bool
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show fiat saving balance (or total aggregated balance)",
		Example: `  bifu-cli payment balance
  bifu-cli payment balance --currency USD
  bifu-cli payment balance --total`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if total {
				res, err := c.GetTotalBalance(currency)
				if err != nil {
					return err
				}
				pr.Header("Total Balance")
				pr.PrintKV([]output.KV{
					{Key: "Total (USD)", Value: res.Balance},
					{Key: "Saving", Value: res.Saving.Balance + " (" + res.Saving.Ratio + "%)"},
					{Key: "Forex", Value: res.Forex.Balance + " (" + res.Forex.Ratio + "%)"},
					{Key: "CopyTrade", Value: res.CopyTrade.Balance + " (" + res.CopyTrade.Ratio + "%)"},
				})
				return nil
			}
			res, err := c.GetSavingBalance(currency)
			if err != nil {
				return err
			}
			pr.Header("Saving Balance")
			var rows [][]string
			for _, item := range res.Items {
				rows = append(rows, []string{
					item.Currency, item.Balance, item.AvailableBalance, item.FrozenBalance,
				})
			}
			pr.PrintTable([]string{"CURRENCY", "BALANCE", "AVAILABLE", "FROZEN"}, rows)
			pr.Line("Total (USD): %s", output.Bold(res.TotalUSD))
			return nil
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "", "Filter by currency (e.g. USD)")
	cmd.Flags().BoolVar(&total, "total", false, "Show aggregated total across all account types")
	return cmd
}

// ── payment transfer ──────────────────────────────────────────────────────────

func newTransferCmd(load LoadFn) *cobra.Command {
	var fromType, toType int
	var amount, currency string
	var loginID int64
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer funds between account types",
		Long: `Transfer funds between account types.

Account types:
  1 = Saving account
  2 = Forex commission
  3 = Forex trading account (requires --login-id)`,
		Example: `  bifu-cli payment transfer --from 1 --to 3 --amount 1000 --currency USD --login-id 90390034
  bifu-cli payment transfer --from 3 --to 1 --amount 500 --currency USD --login-id 90390034`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if err := c.Transfer(&paymentapi.TransferReq{
				FromType: fromType,
				ToType:   toType,
				Amount:   amount,
				Currency: currency,
				LoginID:  loginID,
			}); err != nil {
				return err
			}
			pr.OK("Transfer submitted: %s %s  (type %d → type %d)", amount, currency, fromType, toType)
			return nil
		},
	}
	cmd.Flags().IntVar(&fromType, "from", 1, "Source account type (1=saving, 2=commission, 3=forex)")
	cmd.Flags().IntVar(&toType, "to", 3, "Destination account type")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount to transfer")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency")
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "Forex login ID (required for type 3)")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

// ── payment forex-accounts ────────────────────────────────────────────────────

func newForexAccountsCmd(load LoadFn) *cobra.Command {
	var loginID int64
	cmd := &cobra.Command{
		Use:   "forex-accounts",
		Short: "List linked forex (MT5) accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, pr, err := load()
			if err != nil {
				return err
			}
			// The forex account group endpoint is handled by the forex command group.
			// Here we show a helpful redirect.
			_ = loginID
			pr.Line("Use: %s", output.Bold("bifu-cli forex account list"))
			pr.Line("     %s", output.Bold("bifu-cli forex account get --login-id <id>"))
			fmt.Println()
			pr.Line("Or for fund operations between saving and forex accounts:")
			pr.Line("  bifu-cli payment transfer --from 1 --to 3 --amount 1000 --currency USD --login-id <id>")
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "Forex account login ID")
	return cmd
}

// fmtFloat is a helper to format a float64 as string for display.
func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
