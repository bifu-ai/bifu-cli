// Package spot implements the `bifu-cli spot` command group.
package spot

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	spotapi "bifu-cli/internal/api/spot"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewSpotCmd builds the `spot` command tree.
func NewSpotCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spot",
		Short: "Spot trading (orders, balances)",
	}
	cmd.AddCommand(newOrderCmd(load))
	cmd.AddCommand(newBalanceCmd(load))
	return cmd
}

func newclient(load LoadFn) (*spotapi.Client, *output.Printer, error) {
	p, pr, err := load()
	if err != nil {
		return nil, nil, err
	}
	c := spotapi.New(p)
	c.SetVerbose(pr.Verbose)
	return c, pr, nil
}

func newOrderCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Manage spot orders"}
	cmd.AddCommand(newOrderCreate(load))
	cmd.AddCommand(newOrderCancel(load))
	cmd.AddCommand(newOrderGet(load))
	cmd.AddCommand(newOrderList(load))
	return cmd
}

func newOrderCreate(load LoadFn) *cobra.Command {
	var symbol, side, typ, price, size, clientID, tif string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a spot order",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, pr, err := load()
			if err != nil {
				return err
			}
			c := spotapi.New(p)
			c.SetVerbose(pr.Verbose)
			if clientID == "" {
				uid := p.Auth.UserID
				if uid == "" {
					uid = "anon"
				}
				ts := time.Now().UTC().Format("20060102150405")
				sym := strings.ToLower(symbol)
				clientID = fmt.Sprintf("%s-%s-%s-%s", uid, sym, strings.ToLower(side), ts)
				if len(clientID) > 64 {
					clientID = clientID[:64]
				}
			}
			resp, err := c.CreateOrder(&spotapi.CreateOrderReq{
				SymbolID:      symbol,
				OrderSide:     side,
				Type:          typ,
				Price:         price,
				Size:          size,
				ClientOrderID: clientID,
				TimeInForce:   tif,
			})
			if err != nil {
				return err
			}
			pr.OK("Order created")
			pr.PrintKV([]output.KV{
				{Key: "Order ID", Value: resp.OrderID},
				{Key: "Client Order ID", Value: resp.ClientOrderID},
				{Key: "Status", Value: resp.Status},
			})
			return nil
		},
	}
	cmd.Flags().StringVarP(&symbol, "symbol", "s", "", "Trading pair symbol")
	cmd.Flags().StringVar(&side, "side", "", "BUY | SELL")
	cmd.Flags().StringVar(&typ, "type", "MARKET", "MARKET | LIMIT | STOP_LIMIT")
	cmd.Flags().StringVar(&price, "price", "0", "Limit price")
	cmd.Flags().StringVar(&size, "size", "", "Order quantity")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client order ID")
	cmd.Flags().StringVar(&tif, "tif", "GOOD_TIL_CANCEL", "Time-in-force")
	_ = cmd.MarkFlagRequired("symbol")
	_ = cmd.MarkFlagRequired("side")
	_ = cmd.MarkFlagRequired("size")
	return cmd
}

func newOrderCancel(load LoadFn) *cobra.Command {
	var orderID, clientID, symbol string
	var all bool
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a spot order (or all orders)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newclient(load)
			if err != nil {
				return err
			}
			if all {
				if err := c.CancelAllOrders(symbol); err != nil {
					return err
				}
				pr.OK("All orders cancelled")
				return nil
			}
			if err := c.CancelOrder(&spotapi.CancelOrderReq{
				OrderID:       orderID,
				ClientOrderID: clientID,
			}); err != nil {
				return err
			}
			pr.OK("Order cancelled")
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client order ID")
	cmd.Flags().StringVarP(&symbol, "symbol", "s", "", "Symbol (used with --all)")
	cmd.Flags().BoolVar(&all, "all", false, "Cancel all open orders")
	return cmd
}

func newOrderGet(load LoadFn) *cobra.Command {
	var orderID string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get order details",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newclient(load)
			if err != nil {
				return err
			}
			o, err := c.GetOrder(orderID)
			if err != nil {
				return err
			}
			pr.PrintKV([]output.KV{
				{Key: "Order ID", Value: o.OrderID},
				{Key: "Symbol ID", Value: fmt.Sprintf("%v", o.SymbolID)},
				{Key: "Side", Value: o.OrderSide},
				{Key: "Type", Value: o.Type},
				{Key: "Price", Value: o.Price},
				{Key: "Size", Value: o.Size},
				{Key: "Filled", Value: o.FilledQuantity},
				{Key: "Avg Price", Value: o.AveragePrice},
				{Key: "Status", Value: o.Status},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&orderID, "order-id", "", "Order ID")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

func newOrderList(load LoadFn) *cobra.Command {
	var symbol string
	var history bool
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List open (or historical) spot orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newclient(load)
			if err != nil {
				return err
			}
			var orders []spotapi.Order
			var apiErr error
			if history {
				orders, apiErr = c.ListOrderHistory(symbol, limit)
			} else {
				orders, apiErr = c.ListOpenOrders(symbol)
			}
			if apiErr != nil {
				return apiErr
			}
			if len(orders) == 0 {
				pr.Line("No orders found.")
				return nil
			}
			var rows [][]string
			for _, o := range orders {
				rows = append(rows, []string{
					o.OrderID, fmt.Sprintf("%v", o.SymbolID),
					o.OrderSide, o.Type, o.Price, o.Size, o.FilledQuantity, o.Status,
				})
			}
			pr.PrintTable([]string{"ORDER_ID", "SYMBOL", "SIDE", "TYPE", "PRICE", "SIZE", "FILLED", "STATUS"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVarP(&symbol, "symbol", "s", "", "Filter by symbol ID")
	cmd.Flags().BoolVar(&history, "history", false, "Show order history")
	cmd.Flags().IntVar(&limit, "limit", 50, "Limit (history only)")
	return cmd
}

func newBalanceCmd(load LoadFn) *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show spot account balances",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newclient(load)
			if err != nil {
				return err
			}
			items, err := c.GetBalance()
			if err != nil {
				return err
			}
			if len(items) == 0 {
				pr.Line("No balances found.")
				return nil
			}
			var rows [][]string
			for _, b := range items {
				rows = append(rows, []string{b.CoinID.String(), b.Amount, b.PendingDepositAmount, b.PendingWithdrawAmount})
			}
			pr.PrintTable([]string{"COIN_ID", "AMOUNT", "PENDING_DEPOSIT", "PENDING_WITHDRAW"}, rows)
			return nil
		},
	}
}


