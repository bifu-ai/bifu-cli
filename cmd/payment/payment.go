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
	c := paymentapi.New(p)
	c.SetVerbose(pr.Verbose)
	return c, pr, nil
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
	var direction string
	var amount, currency string
	var savingAccountID, loginID, forexInternalID int64
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer funds between saving and forex accounts",
		Long: `Transfer funds between a saving account and a forex (MT5) account.

Direction:
  to-forex   Transfer from saving → forex account
  to-saving  Transfer from forex  → saving account`,
		Example: `  bifu-cli payment transfer --direction to-forex  --login-id 90390034 --amount 1000 --currency USD
  bifu-cli payment transfer --direction to-saving --login-id 90390034 --amount 500  --currency USD`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			var transferType int64
			switch direction {
			case "to-forex", "2":
				transferType = 2
			case "to-saving", "1":
				transferType = 1
			default:
				return fmt.Errorf("--direction must be 'to-forex' or 'to-saving'")
			}
			if loginID == 0 {
				return fmt.Errorf("--login-id is required")
			}
			// Look up internal forex account ID from MT5 login.
			if forexInternalID == 0 {
				accounts, err := c.GetForexAccountList()
				if err != nil {
					return fmt.Errorf("lookup forex accounts: %w", err)
				}
				loginStr := strconv.FormatInt(loginID, 10)
				for _, a := range accounts {
					if a.Login == loginStr && a.ID != "" {
						if id, err2 := strconv.ParseInt(a.ID, 10, 64); err2 == nil {
							forexInternalID = id
							break
						}
					}
				}
				if forexInternalID == 0 {
					return fmt.Errorf("forex account with login %d not found", loginID)
				}
			}
			// Auto-lookup saving account ID by currency if not provided.
			if savingAccountID == 0 {
				bal, err := c.GetSavingBalance(currency)
				if err != nil {
					return fmt.Errorf("lookup saving account: %w", err)
				}
				for _, item := range bal.Items {
					if item.Currency == currency && item.ID != "" {
						if id, err2 := strconv.ParseInt(item.ID, 10, 64); err2 == nil {
							savingAccountID = id
							break
						}
					}
				}
				if savingAccountID == 0 {
					return fmt.Errorf("no %s saving account found; use --saving-id to specify manually", currency)
				}
			}
			if err := c.Transfer(&paymentapi.TransferReq{
				SavingAccountID: savingAccountID,
				ForexAccountID:  forexInternalID,
				Amount:          amount,
				Currency:        currency,
				Type:            transferType,
			}); err != nil {
				return err
			}
			pr.OK("Transfer submitted: %s %s (%s)", amount, currency, direction)
			return nil
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "", "Transfer direction: to-forex | to-saving")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount to transfer")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency")
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "Forex (MT5) login ID (used to look up internal account ID)")
	cmd.Flags().Int64Var(&forexInternalID, "forex-id", 0, "Forex internal account ID (overrides --login-id lookup)")
	cmd.Flags().Int64Var(&savingAccountID, "saving-id", 0, "Saving account ID (optional, auto-detected from --currency)")
	_ = cmd.MarkFlagRequired("direction")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}

// ── payment forex-accounts ────────────────────────────────────────────────────

func newForexAccountsCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forex-accounts",
		Short: "List linked forex (MT5) accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			c := paymentapi.New(p)
			c.SetVerbose(mustBool(cmd.Root().PersistentFlags().GetBool("verbose")))
			items, err := c.GetForexAccountList()
			if err != nil {
				return err
			}
			if len(items) == 0 {
				pr.Line("No forex accounts found.")
				return nil
			}
			rows := make([][]string, 0, len(items))
			for _, a := range items {
				rows = append(rows, []string{a.Login, a.Type + "/" + a.SubType, a.Status, a.Balance, a.Equity, a.MarginFree, a.Leverage, a.GroupType})
			}
			pr.PrintTable([]string{"LOGIN", "TYPE", "STATUS", "BALANCE", "EQUITY", "FREE MARGIN", "LEVERAGE", "GROUP"}, rows)
			return nil
		},
	}
	return cmd
}

func mustBool(v bool, _ error) bool { return v }

// fmtFloat is a helper to format a float64 as string for display.
func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
