// Package contract implements the `bifu-cli contract` command group.
package contract

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	contractapi "bifu-cli/internal/api/contract"
	metaapi "bifu-cli/internal/api/meta"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// entryPrice derives the average entry price from open value / size.
func entryPrice(openValue, size string) string {
	ov, err1 := strconv.ParseFloat(openValue, 64)
	sz, err2 := strconv.ParseFloat(size, 64)
	if err1 != nil || err2 != nil || sz == 0 {
		return ""
	}
	return strconv.FormatFloat(ov/sz, 'f', -1, 64)
}

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewContractCmd builds the `contract` command tree.
func NewContractCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Contract/futures trading (orders, positions, account)",
	}
	cmd.AddCommand(newOrderCmd(load))
	cmd.AddCommand(newPositionCmd(load))
	cmd.AddCommand(newAccountCmd(load))
	return cmd
}

func newClient(load LoadFn) (*contractapi.Client, *output.Printer, *clifconfig.Profile, error) {
	p, pr, err := load()
	if err != nil {
		return nil, nil, nil, err
	}
	c := contractapi.New(p)
	c.SetVerbose(pr.Verbose)
	return c, pr, p, nil
}

// resolveContract turns a contract name (e.g. "BTCUSDT") into its numeric
// contractId, printing the mapping. Empty and numeric values pass through.
func resolveContract(p *clifconfig.Profile, pr *output.Printer, s string) (string, error) {
	id, err := metaapi.ResolveContractSymbol(p, pr.Verbose, s)
	if err != nil {
		return "", err
	}
	if s != "" && id != s {
		pr.Line("%s", output.Dim(fmt.Sprintf("  %s → contract %s", s, id)))
	}
	return id, nil
}

// ── contract order ────────────────────────────────────────────────────────────

func newOrderCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Manage contract orders"}
	cmd.AddCommand(newOrderCreate(load))
	cmd.AddCommand(newOrderCancel(load))
	cmd.AddCommand(newOrderGet(load))
	cmd.AddCommand(newOrderList(load))
	return cmd
}

func newOrderCreate(load LoadFn) *cobra.Command {
	var (
		contractID, positionSide, orderSide, typ string
		price, size, tif, clientID               string
		marginMode, separatedMode                string
		reduceOnly                               bool
		triggerPrice, triggerPriceType           string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a contract order",
		Example: `  bifu-cli contract order create --contract BTCUSDT --side LONG --order-side BUY --size 0.01
  bifu-cli contract order create --contract BTCUSDT --side SHORT --order-side SELL --type LIMIT --price 60000 --size 0.01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			c := contractapi.New(p)
			c.SetVerbose(pr.Verbose)
			contractID, err = resolveContract(p, pr, contractID)
			if err != nil {
				return err
			}
			if clientID == "" {
				clientID = p.GenerateClientOrderID(contractID, orderSide, time.Now())
			}
			resp, err := c.CreateOrder(&contractapi.CreateOrderReq{
				ContractID:       strings.ToUpper(contractID),
				MarginMode:       strings.ToUpper(marginMode),
				SeparatedMode:    strings.ToUpper(separatedMode),
				PositionSide:     strings.ToUpper(positionSide),
				OrderSide:        strings.ToUpper(orderSide),
				Price:            price,
				Size:             size,
				Type:             strings.ToUpper(typ),
				TimeInForce:      tif,
				ClientOrderID:    clientID,
				ReduceOnly:       reduceOnly,
				TriggerPrice:     triggerPrice,
				TriggerPriceType: triggerPriceType,
			})
			if err != nil {
				return err
			}
			pr.OK("Contract order created")
			pr.PrintKV([]output.KV{
				{Key: "Order ID", Value: resp.OrderID},
				{Key: "Client Order ID", Value: resp.ClientOrderID},
				{Key: "Status", Value: resp.Status},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&contractID, "contract", "", "Contract: symbol (BTCUSDT) or numeric contractId")
	cmd.Flags().StringVar(&positionSide, "side", "", "Position side: LONG | SHORT")
	cmd.Flags().StringVar(&orderSide, "order-side", "", "Order side: BUY | SELL")
	cmd.Flags().StringVar(&typ, "type", "MARKET", "MARKET | LIMIT | STOP_LIMIT")
	cmd.Flags().StringVar(&price, "price", "0", "Limit price (0 = market)")
	cmd.Flags().StringVar(&size, "size", "", "Order size")
	cmd.Flags().StringVar(&tif, "tif", "GOOD_TIL_CANCEL", "Time-in-force")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client order ID")
	cmd.Flags().StringVar(&marginMode, "margin-mode", "SHARED", "SHARED | ISOLATED")
	cmd.Flags().StringVar(&separatedMode, "separated-mode", "COMBINED", "COMBINED | SEPARATED")
	cmd.Flags().BoolVar(&reduceOnly, "reduce-only", false, "Reduce-only order")
	cmd.Flags().StringVar(&triggerPrice, "trigger-price", "", "Trigger price (for stop orders)")
	cmd.Flags().StringVar(&triggerPriceType, "trigger-type", "LAST_PRICE", "LAST_PRICE | MARK_PRICE")
	_ = cmd.MarkFlagRequired("contract")
	_ = cmd.MarkFlagRequired("side")
	_ = cmd.MarkFlagRequired("order-side")
	_ = cmd.MarkFlagRequired("size")
	fixed := func(vals ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return vals, cobra.ShellCompDirectiveNoFileComp
		}
	}
	_ = cmd.RegisterFlagCompletionFunc("side", fixed("LONG", "SHORT"))
	_ = cmd.RegisterFlagCompletionFunc("order-side", fixed("BUY", "SELL"))
	_ = cmd.RegisterFlagCompletionFunc("type", fixed("MARKET", "LIMIT", "STOP_LIMIT"))
	return cmd
}

func newOrderCancel(load LoadFn) *cobra.Command {
	var orderID, clientID, contractID string
	var all bool
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel contract order(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, p, err := newClient(load)
			if err != nil {
				return err
			}
			if all {
				contractID, err = resolveContract(p, pr, contractID)
				if err != nil {
					return err
				}
				target := "all open contract orders"
				if contractID != "" {
					target += " for contract " + contractID
				}
				if yes, _ := cmd.Root().PersistentFlags().GetBool("yes"); !yes && !pr.Confirm("Cancel "+target+"?") {
					pr.Line("Aborted.")
					return nil
				}
				if err := c.CancelAllOrders(contractID); err != nil {
					return err
				}
				pr.OK("All contract orders cancelled")
				return nil
			}
			if err := c.CancelOrder(orderID, clientID); err != nil {
				return err
			}
			pr.OK("Contract order cancelled")
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client order ID")
	cmd.Flags().StringVar(&contractID, "contract", "", "Cancel all orders for contract: symbol (BTCUSDT) or contractId")
	cmd.Flags().BoolVar(&all, "all", false, "Cancel all open orders")
	return cmd
}

func newOrderGet(load LoadFn) *cobra.Command {
	var orderID string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get contract order details",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, _, err := newClient(load)
			if err != nil {
				return err
			}
			o, err := c.GetOrder(orderID)
			if err != nil {
				return err
			}
			pr.PrintKV([]output.KV{
				{Key: "Order ID", Value: fmt.Sprintf("%v", o.OrderID)},
				{Key: "Contract", Value: fmt.Sprintf("%v", o.ContractID)},
				{Key: "Position Side", Value: o.PositionSide},
				{Key: "Order Side", Value: o.OrderSide},
				{Key: "Type", Value: o.Type},
				{Key: "Price", Value: o.Price},
				{Key: "Size", Value: o.Size},
				{Key: "Filled", Value: o.FilledQuantity},
				{Key: "Avg Price", Value: entryPrice(o.CumFillValue, o.FilledQuantity)},
				{Key: "Status", Value: o.Status},
				{Key: "Reduce Only", Value: fmt.Sprintf("%v", o.ReduceOnly)},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

func newOrderList(load LoadFn) *cobra.Command {
	var contractID string
	var history bool
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open (or historical) contract orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, p, err := newClient(load)
			if err != nil {
				return err
			}
			contractID, err = resolveContract(p, pr, contractID)
			if err != nil {
				return err
			}
			var orders []contractapi.Order
			var apiErr error
			if history {
				orders, apiErr = c.ListOrderHistory(contractID, limit)
			} else {
				orders, apiErr = c.ListOpenOrders(contractID)
			}
			if apiErr != nil {
				return apiErr
			}
			if len(orders) == 0 {
				if history {
					pr.Line("No order history found.")
				} else {
					pr.Line("No open orders.")
				}
				return nil
			}
			var rows [][]string
			for _, o := range orders {
				rows = append(rows, []string{
					fmt.Sprintf("%v", o.OrderID),
					fmt.Sprintf("%v", o.ContractID),
					o.PositionSide, o.OrderSide, o.Type,
					o.Price, o.Size, o.FilledQuantity, o.Status,
				})
			}
			pr.PrintTable([]string{"ORDER_ID", "CONTRACT", "SIDE", "ORDER_SIDE", "TYPE", "PRICE", "SIZE", "FILLED", "STATUS"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&contractID, "contract", "", "Filter by contract: symbol (BTCUSDT) or contractId")
	cmd.Flags().BoolVar(&history, "history", false, "Show order history")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit (history only)")
	return cmd
}

// ── contract position ─────────────────────────────────────────────────────────

func newPositionCmd(load LoadFn) *cobra.Command {
	var contractID string
	cmd := &cobra.Command{
		Use:   "position",
		Short: "View open positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, p, err := newClient(load)
			if err != nil {
				return err
			}
			contractID, err = resolveContract(p, pr, contractID)
			if err != nil {
				return err
			}
			positions, err := c.ListPositions(contractID)
			if err != nil {
				return err
			}
			if len(positions) == 0 {
				pr.Line("No open positions.")
				return nil
			}
			var rows [][]string
			for _, p := range positions {
				rows = append(rows, []string{
					fmt.Sprintf("%v", p.ContractID),
					p.PositionSide, p.MarginMode, p.Size,
					entryPrice(p.OpenValue, p.Size),
					p.OpenValue, p.Leverage, p.OpenFee,
				})
			}
			pr.PrintTable([]string{"CONTRACT", "SIDE", "MARGIN", "SIZE", "ENTRY", "OPEN_VALUE", "LEVERAGE", "OPEN_FEE"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&contractID, "contract", "", "Filter by contract: symbol (BTCUSDT) or contractId")
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List open positions",
		RunE:  cmd.RunE,
	}
	// Bind the same var so `position list --contract X` works like `position --contract X`.
	listCmd.Flags().StringVar(&contractID, "contract", "", "Filter by contract: symbol (BTCUSDT) or contractId")
	cmd.AddCommand(listCmd)
	return cmd
}

// ── contract account ──────────────────────────────────────────────────────────

func newAccountCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:   "account",
		Short: "Show contract account summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, _, err := newClient(load)
			if err != nil {
				return err
			}
			info, err := c.GetAccount()
			if err != nil {
				return err
			}
			pr.PrintKV([]output.KV{
				{Key: "Account ID", Value: info.AccountID},
				{Key: "User ID", Value: info.UserID},
				{Key: "Status", Value: info.Status},
			})
			if len(info.Assets) > 0 {
				pr.Header("Assets:")
				rows := make([][]string, 0, len(info.Assets))
				for _, a := range info.Assets {
					if a.AccountEquity == "0" && a.AccountAvailable == "0" {
						continue
					}
					rows = append(rows, []string{
						a.CoinID, a.AccountEquity, a.AccountAvailable, a.AccountUsed, a.UnrealizePnl,
					})
				}
				if len(rows) > 0 {
					pr.PrintTable([]string{"COIN", "EQUITY", "AVAILABLE", "USED", "UNREALIZED PNL"}, rows)
				} else {
					pr.Line("  No funded assets.")
				}
			}
			return nil
		},
	}
}
