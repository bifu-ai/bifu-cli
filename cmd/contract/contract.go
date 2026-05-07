// Package contract implements the `bifu-cli contract` command group.
package contract

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	contractapi "bifu-cli/internal/api/contract"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

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

func newClient(load LoadFn) (*contractapi.Client, *output.Printer, error) {
	p, pr, err := load()
	if err != nil {
		return nil, nil, err
	}
	c := contractapi.New(p)
	c.SetVerbose(pr.Verbose)
	return c, pr, nil
}

// ── contract order ────────────────────────────────────────────────────────────

func newOrderCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Manage contract orders"}
	cmd.AddCommand(newOrderCreate(load))
	cmd.AddCommand(newOrderCancel(load))
	cmd.AddCommand(newOrderModify(load))
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
			c, pr, err := newClient(load)
			if err != nil {
				return err
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
	cmd.Flags().StringVar(&contractID, "contract", "", "Contract ID (e.g. BTCUSDT)")
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
	return cmd
}

func newOrderCancel(load LoadFn) *cobra.Command {
	var orderID, clientID, contractID string
	var all bool
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel contract order(s)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if all {
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
	cmd.Flags().StringVar(&contractID, "contract", "", "Cancel all orders for contract")
	cmd.Flags().BoolVar(&all, "all", false, "Cancel all open orders")
	return cmd
}

func newOrderModify(load LoadFn) *cobra.Command {
	var orderID, clientID, price, qty string
	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Modify a contract order price or size",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if err := c.ModifyOrder(&contractapi.ModifyOrderReq{
				OrderID:       orderID,
				ClientOrderID: clientID,
				NewPrice:      price,
				NewQuantity:   qty,
			}); err != nil {
				return err
			}
			pr.OK("Order modified")
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client order ID")
	cmd.Flags().StringVar(&price, "price", "", "New price")
	cmd.Flags().StringVar(&qty, "size", "", "New size")
	return cmd
}

func newOrderGet(load LoadFn) *cobra.Command {
	var orderID string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get contract order details",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
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
				{Key: "Avg Price", Value: o.AveragePrice},
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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open contract orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			orders, err := c.ListOpenOrders(contractID)
			if err != nil {
				return err
			}
			if len(orders) == 0 {
				pr.Line("No open orders.")
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
	cmd.Flags().StringVar(&contractID, "contract", "", "Filter by contract ID")
	return cmd
}

// ── contract position ─────────────────────────────────────────────────────────

func newPositionCmd(load LoadFn) *cobra.Command {
	var contractID string
	cmd := &cobra.Command{
		Use:   "position",
		Short: "View open positions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
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
					p.EntryPrice, p.MarkPrice, p.Pnl,
					p.Leverage, p.LiqPrice,
				})
			}
			pr.PrintTable([]string{"CONTRACT", "SIDE", "MARGIN", "SIZE", "ENTRY", "MARK", "UPNL", "LEVERAGE", "LIQ_PRICE"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&contractID, "contract", "", "Filter by contract ID")
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List open positions",
		RunE:  cmd.RunE,
	})
	return cmd
}

// ── contract account ──────────────────────────────────────────────────────────

func newAccountCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:   "account",
		Short: "Show contract account summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
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
