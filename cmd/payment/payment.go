// Package payment implements the `bifu-cli payment` command group.
package payment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	paymentapi "bifu-cli/internal/api/payment"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
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
	cmd.AddCommand(newUnifiedTransferCmd(load))
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
			res, err := c.GetSavingBalance("")
			if err != nil {
				return err
			}
			pr.Header("Saving Balance")
			var rows [][]string
			for _, item := range res.Items {
				if currency != "" && item.Currency != currency {
					continue
				}
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

// ── payment unified-transfer ──────────────────────────────────────────────────

func newUnifiedTransferCmd(load LoadFn) *cobra.Command {
	var fromStr, toStr, amount, currency, comment string
	var coinID int32

	acctType := map[string]paymentapi.TransferAccountType{
		"SAVING":   paymentapi.TransferAccountTypeSaving,
		"FOREX":    paymentapi.TransferAccountTypeForexMT5,
		"FUNDING":  paymentapi.TransferAccountTypeCryptoFunding,
		"SPOT":     paymentapi.TransferAccountTypeCryptoSpot,
		"CONTRACT": paymentapi.TransferAccountTypeCryptoFuture,
		"EARN":     paymentapi.TransferAccountTypeEarn,
	}

	cmd := &cobra.Command{
		Use:   "unified-transfer",
		Short: "Universal fund transfer between any two accounts",
		Long: `Transfer funds between any two account types using the unified transfer API.

Account types:
  SAVING    Fiat saving/wallet account (requires --currency)
  FOREX     MT5 forex account          (requires --currency)
  FUNDING   Crypto funding account     (requires --coin-id)
  SPOT      Crypto spot account        (requires --coin-id)
  CONTRACT  Crypto futures account     (requires --coin-id)
  EARN      Earn/financial account     (requires --coin-id or --currency)`,
		Example: `  bifu-cli payment unified-transfer --from SAVING --to FOREX     --amount 100 --currency USD
  bifu-cli payment unified-transfer --from FOREX  --to SAVING    --amount 100 --currency USD
  bifu-cli payment unified-transfer --from SAVING --to SPOT      --amount 100 --currency USD
  bifu-cli payment unified-transfer --from FUNDING --to SPOT     --amount 10  --coin-id 1
  bifu-cli payment unified-transfer --from SPOT   --to CONTRACT  --amount 10  --coin-id 1
  bifu-cli payment unified-transfer --from CONTRACT --to FUNDING --amount 10  --coin-id 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			fromKey := fmt.Sprintf("%s", fromStr)
			toKey := fmt.Sprintf("%s", toStr)
			// uppercase
			fromKey = strings.ToUpper(fromKey)
			toKey = strings.ToUpper(toKey)

			fromType, ok := acctType[fromKey]
			if !ok {
				return fmt.Errorf("unknown account type %q; valid: SAVING, FOREX, FUNDING, SPOT, CONTRACT, EARN", fromStr)
			}
			toType, ok2 := acctType[toKey]
			if !ok2 {
				return fmt.Errorf("unknown account type %q; valid: SAVING, FOREX, FUNDING, SPOT, CONTRACT, EARN", toStr)
			}
			if fromType == toType {
				return fmt.Errorf("--from and --to must be different account types")
			}

			req := &paymentapi.UnifiedTransferReq{
				FromAccountType: fromType,
				ToAccountType:   toType,
				Amount:          amount,
				Currency:        currency,
				CoinID:          coinID,
				Comment:         comment,
			}
			resp, err := c.UnifiedTransfer(req)
			if err != nil {
				return err
			}
			pr.OK("Transfer submitted")
			pr.PrintKV([]output.KV{
				{Key: "Ticket", Value: resp.Ticket},
				{Key: "Status", Value: resp.Status},
				{Key: "From Amount", Value: resp.FromAmount},
				{Key: "To Amount", Value: resp.ToAmount},
				{Key: "Fee", Value: resp.Fee},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&fromStr, "from", "", "Source account type (SAVING/FOREX/FUNDING/SPOT/CONTRACT/EARN)")
	cmd.Flags().StringVar(&toStr, "to", "", "Destination account type (SAVING/FOREX/FUNDING/SPOT/CONTRACT/EARN)")
	cmd.Flags().StringVar(&amount, "amount", "", "Amount to transfer")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency code for fiat accounts (e.g. USD)")
	cmd.Flags().Int32Var(&coinID, "coin-id", 0, "Coin ID for crypto accounts (e.g. 1=USDT)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional remark")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
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
