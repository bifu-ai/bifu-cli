// Package forex implements the `bifu-cli forex` command group.
// Manages MT5 forex orders via the payment service HTTP endpoints.
package forex

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	paymentapi "bifu-cli/internal/api/payment"
	"bifu-cli/internal/clifconfig"
	"bifu-cli/internal/output"
)

// LoadFn resolves the active profile and printer.
type LoadFn func() (*clifconfig.Profile, *output.Printer, error)

// NewForexCmd builds the `forex` command tree.
func NewForexCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forex",
		Short: "Forex/MT5 order management",
		Long: `Manage MT5 forex orders: market and pending orders, positions, and history.

Order types:
  buy        Market buy (instant execution)
  sell       Market sell (instant execution)
  buyLimit   Buy limit — execute when price ≤ specified price
  sellLimit  Sell limit — execute when price ≥ specified price
  buyStop    Buy stop   — execute when price ≥ specified price
  sellStop   Sell stop  — execute when price ≤ specified price

Examples:
  bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01
  bifu-cli forex order modify --login-id 90390034 --order-id 12345 --sl 1.03 --tp 1.09
  bifu-cli forex order close --login-id 90390034 --order-id 12345
  bifu-cli forex order cancel --login-id 90390034 --order-id 12345
  bifu-cli forex order history --login-id 90390034 --from 2026-01-01 --to 2026-12-31`,
	}
	cmd.AddCommand(newOrderCmd(load))
	cmd.AddCommand(newPositionsCmd(load))
	cmd.AddCommand(newAccountCmd(load))
	return cmd
}

// ── forex account ─────────────────────────────────────────────────────────────

func newAccountCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Manage forex accounts (create)"}
	cmd.AddCommand(newAccountCreate(load))
	return cmd
}

func newAccountCreate(load LoadFn) *cobra.Command {
	var platform, accType, subType, currency, password string
	var leverage int64

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a forex account (MT5 or TradFi/Fortex)",
		Long: `Create a new forex trading account.

--platform mt5     → MT5 account (mt_type=2)
--platform tradfi  → TradFi/Fortex account (mt_type=3; requires the user to be
                     in the tradfi whitelist, otherwise the backend rejects it).`,
		Example: `  bifu-cli forex account create --platform tradfi --type demo --currency USD --leverage 100 --password 'Pass123!'
  bifu-cli forex account create --platform mt5 --type demo --currency USD --leverage 100 --password 'Pass123!'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			mtType := int32(2)
			switch platform {
			case "tradfi", "fortex", "TradFi":
				mtType = 3
			case "mt5", "MT5", "":
				mtType = 2
			default:
				return fmt.Errorf("unknown --platform %q (use: mt5 | tradfi)", platform)
			}
			acct, err := c.CreateForexAccount(&paymentapi.CreateForexAccountReq{
				Type:     accType,
				Currency: currency,
				Leverage: leverage,
				Password: password,
				SubType:  subType,
				MtType:   mtType,
			})
			if err != nil {
				return err
			}
			pr.OK("Forex account created")
			pr.PrintKV([]output.KV{
				{Key: "Login", Value: acct.Login},
				{Key: "Platform", Value: acct.PlatformName()},
				{Key: "Type", Value: acct.Type + "/" + acct.SubType},
				{Key: "Currency", Value: acct.Currency},
				{Key: "Leverage", Value: acct.Leverage},
				{Key: "Status", Value: acct.Status},
			})
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "mt5", "Platform: mt5 | tradfi")
	cmd.Flags().StringVar(&accType, "type", "demo", "Account type: live | demo")
	cmd.Flags().StringVar(&subType, "sub-type", "normal", "Sub-type: normal | signal | copyTrade")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Account currency (e.g. USD)")
	cmd.Flags().Int64Var(&leverage, "leverage", 100, "Leverage")
	cmd.Flags().StringVar(&password, "password", "", "Account password (required)")
	_ = cmd.MarkFlagRequired("password")
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

// ── forex order ───────────────────────────────────────────────────────────────

func newOrderCmd(load LoadFn) *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Manage MT5 forex orders"}
	cmd.AddCommand(newOrderCreate(load))
	cmd.AddCommand(newOrderModify(load))
	cmd.AddCommand(newOrderClose(load))
	cmd.AddCommand(newOrderCancel(load))
	cmd.AddCommand(newOrderHistory(load))
	cmd.AddCommand(newBatchClose(load))
	cmd.AddCommand(newBatchCancel(load))
	return cmd
}

func newPositionsCmd(load LoadFn) *cobra.Command {
	var loginID int64
	cmd := &cobra.Command{
		Use:     "positions",
		Short:   "List open forex positions and pending orders",
		Example: `  bifu-cli forex positions --login-id 90390034`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			orders, err := c.GetForexOpenOrders(loginID)
			if err != nil {
				return err
			}
			if len(orders) == 0 {
				pr.Line("No open positions or pending orders.")
				return nil
			}
			var rows [][]string
			for _, o := range orders {
				rows = append(rows, []string{
					o.Ticket, o.Symbol, o.OrderType, o.Volume,
					o.OpenPrice, o.StopLoss, o.TakeProfit, o.Profit, o.State,
				})
			}
			pr.PrintTable([]string{"TICKET", "SYMBOL", "TYPE", "VOLUME", "OPEN_PRICE", "SL", "TP", "PROFIT", "STATE"}, rows)
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	_ = cmd.MarkFlagRequired("login-id")
	return cmd
}

func newOrderCreate(load LoadFn) *cobra.Command {
	var loginID int64
	var symbol, typ, expiration string
	var volume, sl, tp, price float64
	var orderType, side, lots, fillPolicy, stopLimitPrice, expirationType string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Place a forex order (MT5 or TradFi — routed by account platform)",
		Example: `  # MT5 / TradFi 通用（按 login-id 的账户平台自动路由）
  bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01 --sl 1.02 --tp 1.08
  bifu-cli forex order create --login-id 90390034 --symbol GBPUSD --type buyLimit --volume 0.01 --price 1.25
  # TradFi 专用字段（仅 mt_type=3 账户生效；不传则由 --type 推导 orderType/side）
  bifu-cli forex order create --login-id 800000175 --symbol EURUSD --order-type Market --side Buy --lots 0.01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			req := &paymentapi.CreateForexOrderReq{
				LoginID: loginID,
				Symbol:  symbol,
				Volume:  volume,
				Type:    typ,
				Price:   price,
				SL:      sl,
				TP:      tp,
			}
			if cmd.Flags().Changed("expiration") {
				req.Expiration = expiration
			}
			// TradFi-only overrides (ignored by MT5 accounts).
			req.OrderType = orderType
			req.Side = side
			req.Lots = lots
			req.FillPolicy = fillPolicy
			req.StopLimitPrice = stopLimitPrice
			req.ExpirationType = expirationType
			resp, err := c.CreateForexOrder(req)
			if err != nil {
				return err
			}
			pr.OK("Forex order placed")
			pr.PrintKV([]output.KV{
				{Key: "Login ID", Value: strconv.FormatInt(loginID, 10)},
				{Key: "Symbol", Value: symbol},
				{Key: "Type", Value: typ},
				{Key: "Volume", Value: strconv.FormatFloat(volume, 'f', -1, 64)},
				{Key: "Order ID", Value: fmt.Sprintf("%v", resp.OrderID)},
			})
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (e.g. EURUSD)")
	cmd.Flags().StringVar(&typ, "type", "", "Order type: buy|sell|buyLimit|sellLimit|buyStop|sellStop")
	cmd.Flags().Float64Var(&volume, "volume", 0.01, "Trade volume in lots")
	cmd.Flags().Float64Var(&price, "price", 0, "Pending order price (limit/stop orders)")
	cmd.Flags().Float64Var(&sl, "sl", 0, "Stop loss price")
	cmd.Flags().Float64Var(&tp, "tp", 0, "Take profit price")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Expiration time (RFC3339, e.g. 2026-12-31T18:00:00.000Z)")
	// TradFi(Fortex)-only flags (mt_type=3 accounts); ignored by MT5.
	cmd.Flags().StringVar(&orderType, "order-type", "", "TradFi order type: Market|Limit|Stop|StopLimit (overrides --type)")
	cmd.Flags().StringVar(&side, "side", "", "TradFi side: Buy|Sell (overrides --type)")
	cmd.Flags().StringVar(&lots, "lots", "", "TradFi lots (alternative to --volume)")
	cmd.Flags().StringVar(&fillPolicy, "fill-policy", "", "TradFi fill policy")
	cmd.Flags().StringVar(&stopLimitPrice, "stop-limit-price", "", "TradFi StopLimit trigger price")
	cmd.Flags().StringVar(&expirationType, "expiration-type", "", "TradFi expiration type")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("symbol")
	// --type is optional: TradFi accounts may instead use --order-type + --side.
	return cmd
}

func newOrderModify(load LoadFn) *cobra.Command {
	var loginID, orderID int64
	var sl, tp, price float64

	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Modify SL/TP or pending order price",
		Example: `  bifu-cli forex order modify --login-id 90390034 --order-id 12345 --sl 1.03 --tp 1.09
  bifu-cli forex order modify --login-id 90390034 --order-id 12345 --price 1.0600`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			req := &paymentapi.ModifyForexOrderReq{
				LoginID: loginID,
				OrderID: orderID,
			}
			if cmd.Flags().Changed("sl") {
				req.SL = sl
			}
			if cmd.Flags().Changed("tp") {
				req.TP = tp
			}
			if cmd.Flags().Changed("price") {
				req.Price = price
			}
			if err := c.ModifyForexOrder(req); err != nil {
				return err
			}
			pr.OK("Order %d modified", orderID)
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().Int64Var(&orderID, "order-id", 0, "Order ticket ID")
	cmd.Flags().Float64Var(&sl, "sl", 0, "New stop loss")
	cmd.Flags().Float64Var(&tp, "tp", 0, "New take profit")
	cmd.Flags().Float64Var(&price, "price", 0, "New pending price")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

func newOrderClose(load LoadFn) *cobra.Command {
	var loginID, orderID int64
	var volume float64

	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close (or partially close) a forex position",
		Example: `  bifu-cli forex order close --login-id 90390034 --order-id 12345           # full close
  bifu-cli forex order close --login-id 90390034 --order-id 12345 --volume 0.01  # partial`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if err := c.CloseForexOrder(&paymentapi.CloseForexOrderReq{
				LoginID: loginID,
				OrderID: orderID,
				Volume:  volume,
			}); err != nil {
				return err
			}
			pr.OK("Position %d closed (volume=%.2f)", orderID, volume)
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().Int64Var(&orderID, "order-id", 0, "Position ticket ID")
	cmd.Flags().Float64Var(&volume, "volume", 0, "Volume to close (0 = full close)")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

func newOrderCancel(load LoadFn) *cobra.Command {
	var loginID, orderID int64
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a pending forex order (limit/stop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if err := c.CancelForexOrder(&paymentapi.CancelForexOrderReq{
				LoginID: loginID,
				OrderID: orderID,
			}); err != nil {
				return err
			}
			pr.OK("Pending order %d cancelled", orderID)
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().Int64Var(&orderID, "order-id", 0, "Order ticket ID")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("order-id")
	return cmd
}

func newOrderHistory(load LoadFn) *cobra.Command {
	var loginID int64
	var from, to string
	var pageSize, pageNum int

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Query closed forex order history",
		Example: `  bifu-cli forex order history --login-id 90390034
  bifu-cli forex order history --login-id 90390034 --from 2026-01-01 --to 2026-06-30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			if from == "" {
				from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
			}
			if to == "" {
				to = time.Now().Format("2006-01-02")
			}
			orders, err := c.GetForexCloseOrders(loginID, from, to, pageSize, pageNum)
			if err != nil {
				return err
			}
			if len(orders) == 0 {
				pr.Line("No closed orders found.")
				return nil
			}
			var rows [][]string
			for _, o := range orders {
				rows = append(rows, []string{
					strconv.FormatInt(o.Ticket, 10),
					o.Symbol, o.Type,
					strconv.FormatFloat(o.Volume, 'f', 2, 64),
					strconv.FormatFloat(o.OpenPrice, 'f', 5, 64),
					strconv.FormatFloat(o.ClosePrice, 'f', 5, 64),
					strconv.FormatFloat(o.Profit, 'f', 2, 64),
					o.OpenTime, o.CloseTime,
				})
			}
			pr.PrintTable([]string{"TICKET", "SYMBOL", "TYPE", "VOLUME", "OPEN", "CLOSE", "PROFIT", "OPEN_TIME", "CLOSE_TIME"}, rows)
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().StringVar(&from, "from", "", "From date YYYY-MM-DD (default: 1 month ago)")
	cmd.Flags().StringVar(&to, "to", "", "To date YYYY-MM-DD (default: today)")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Page size")
	cmd.Flags().IntVar(&pageNum, "page-num", 0, "Page number")
	_ = cmd.MarkFlagRequired("login-id")
	return cmd
}

func newBatchClose(load LoadFn) *cobra.Command {
	var loginID int64
	var orderIDs []int64
	var volume float64
	cmd := &cobra.Command{
		Use:     "batch-close",
		Short:   "Batch close multiple forex positions",
		Example: `  bifu-cli forex order batch-close --login-id 90390034 --order-ids 111,222,333`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			results, err := c.BatchCloseForexOrder(&paymentapi.BatchCloseForexOrderReq{
				LoginID:  loginID,
				OrderIDs: orderIDs,
				Volume:   volume,
			})
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Success {
					pr.OK("Closed order %v", r.OrderID)
				} else {
					pr.Err("Failed %v: %s", r.OrderID, r.Error)
				}
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().Int64SliceVar(&orderIDs, "order-ids", nil, "Comma-separated order IDs")
	cmd.Flags().Float64Var(&volume, "volume", 0, "Volume to close per order (0 = full)")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("order-ids")
	return cmd
}

func newBatchCancel(load LoadFn) *cobra.Command {
	var loginID int64
	var orderIDs []int64
	cmd := &cobra.Command{
		Use:     "batch-cancel",
		Short:   "Batch cancel multiple pending forex orders",
		Example: `  bifu-cli forex order batch-cancel --login-id 90390034 --order-ids 111,222,333`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, pr, err := newClient(load)
			if err != nil {
				return err
			}
			results, err := c.BatchCancelForexOrder(&paymentapi.BatchCancelForexOrderReq{
				LoginID:  loginID,
				OrderIDs: orderIDs,
			})
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Success {
					pr.OK("Cancelled order %v", r.OrderID)
				} else {
					pr.Err("Failed %v: %s", r.OrderID, r.Error)
				}
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&loginID, "login-id", 0, "MT5 account login ID")
	cmd.Flags().Int64SliceVar(&orderIDs, "order-ids", nil, "Comma-separated order IDs")
	_ = cmd.MarkFlagRequired("login-id")
	_ = cmd.MarkFlagRequired("order-ids")
	return cmd
}
