// Package orion implements the `bifu-cli orion` command group: read-only
// access to orion signal subscription market data (pricing, signals, history).
package orion

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	orionapi "bifu-cli/internal/api/orion"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewOrionCmd builds the `orion` command tree.
func NewOrionCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orion",
		Short: "Orion signal subscription — pricing, signals, history (read-only)",
		Long: `Read-only access to the orion signal product.

  price          subscription pricing (public)
  signal         current signal + active buy/sell calls (needs subscription)
  signal-history past signals (details need a subscription)
  subscription   your current subscription status (needs login)`,
	}
	cmd.AddCommand(newPriceCmd(load), newSignalCmd(load), newHistoryCmd(load), newSubscriptionCmd(load))
	return cmd
}

func newClient(load LoadFn) (*orionapi.Client, *output.Printer, error) {
	p, pr, err := load()
	if err != nil {
		return nil, nil, err
	}
	c := orionapi.New(p)
	c.SetVerbose(pr.Verbose)
	return c, pr, nil
}

func newPriceCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:     "price",
		Short:   "List subscription pricing tiers (public)",
		Example: "  bifu-cli orion price",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			tiers, err := c.GetPrice()
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(tiers))
			for _, t := range tiers {
				rows = append(rows, []string{
					t.Type,
					strconv.Itoa(t.Quantity) + " " + t.Unit,
					strconv.FormatFloat(t.USD, 'f', -1, 64),
					yesNo(t.IsFree),
					yesNo(t.CanUsePromotion),
					t.Status,
				})
			}
			pr.PrintTable([]string{"TYPE", "PERIOD", "USD", "FREE", "PROMO", "STATUS"}, rows)
			return nil
		},
	}
}

func newSignalCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:     "signal",
		Short:   "Show the current signal and active buy/sell calls (needs subscription)",
		Example: "  bifu-cli orion signal",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			res, err := c.GetSignal()
			if err != nil {
				return err
			}
			if !res.HasSubscription {
				pr.Line("No active subscription — subscribe to view signals (see `bifu-cli orion price`).")
				return nil
			}
			if res.Signal != nil {
				pr.PrintKV([]output.KV{
					{Key: "Signal ID", Value: res.Signal.ID},
					{Key: "Status", Value: res.Signal.Status},
					{Key: "Window", Value: unixDate(res.Signal.StartDate) + " → " + unixDate(res.Signal.EndDate)},
					{Key: "Timezone", Value: res.Signal.Timezone},
				})
			}
			if len(res.SignalPolicy) == 0 {
				pr.Line("No active calls right now.")
				return nil
			}
			rows := make([][]string, 0, len(res.SignalPolicy))
			for _, s := range res.SignalPolicy {
				rows = append(rows, []string{
					productSymbol(s.Product), s.Type, s.Entry, s.SL, s.PT1, s.PT2, s.Trend, s.RealtimeState,
				})
			}
			pr.Header("Active calls")
			pr.PrintTable([]string{"PRODUCT", "SIDE", "ENTRY", "SL", "PT1", "PT2", "TREND", "STATE"}, rows)
			return nil
		},
	}
}

func newHistoryCmd(load LoadFn) *cobra.Command {
	var page, size int
	cmd := &cobra.Command{
		Use:     "signal-history",
		Short:   "List past signals (details need a subscription)",
		Example: "  bifu-cli orion signal-history --page 1 --size 20",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			res, err := c.GetSignalHistory(page, size)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(res.Items))
			for _, s := range res.Items {
				rows = append(rows, []string{
					unixDate(s.CreatedAt), productSymbol(s.Product), s.Type, s.Entry, s.SL, s.PT1, s.PT2, s.Status,
				})
			}
			pr.PrintTable([]string{"DATE", "PRODUCT", "SIDE", "ENTRY", "SL", "PT1", "PT2", "STATUS"}, rows)
			if len(res.Items) == 0 && res.Total != "" && res.Total != "0" {
				pr.Line("\n%s past signals exist, but details require an active subscription.", res.Total)
			} else {
				pr.Line("\nTotal: %s", res.Total)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&size, "size", 20, "Page size")
	return cmd
}

func newSubscriptionCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:     "subscription",
		Short:   "Show your current orion subscription (needs login)",
		Example: "  bifu-cli orion subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			s, err := c.GetSubscription()
			if err != nil {
				return err
			}
			if s == nil || s.ID == "" || s.ID == "0" {
				pr.Line("No active subscription.")
				return nil
			}
			pr.PrintKV([]output.KV{
				{Key: "Subscription ID", Value: s.ID},
				{Key: "Plan", Value: s.SubscribePriceType + " (" + s.SubscribePriceUnit + ")"},
				{Key: "Valid", Value: yesNo(s.IsValid)},
				{Key: "Expired", Value: yesNo(s.IsExpiry)},
				{Key: "Window", Value: unixDate(s.StartDate) + " → " + unixDate(s.EndDate)},
				{Key: "Auto payment", Value: yesNo(s.AutoPayment)},
			})
			return nil
		},
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func productSymbol(p *orionapi.Product) string {
	if p == nil {
		return ""
	}
	if p.Symbol != "" {
		return p.Symbol
	}
	return p.Source + p.Dest
}

// unixDate formats a protojson int64-as-string unix seconds value as a date.
func unixDate(s string) string {
	if s == "" || s == "0" {
		return ""
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return s
	}
	return time.Unix(n, 0).Format("2006-01-02 15:04")
}
