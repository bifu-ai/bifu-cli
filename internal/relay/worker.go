// Package relay implements a signal-relay gateway worker: it keeps a
// WebSocket to the relay gateway (gw.relaysignal.dev), receives parsed
// trading signals, and executes them on a bifu forex (MT5) account via the
// payment API. Protocol peer: signal-relay cloudflare-gateway /worker/stream
// (frames: signal entries {seq,ts,signal}, control downlink, telemetry uplink).
package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	paymentapi "bifu-cli/internal/api/payment"
	"bifu-cli/internal/output"
)

// staleExecutionSecs: replayed signals older than this only advance last_seq
// and are never executed (guards against re-firing history after a restart).
const staleExecutionSecs = 90

const (
	reconnectInterval = 5 * time.Second
	telemetryInterval = 30 * time.Second
	pingInterval      = 20 * time.Second
	maxTracedIDs      = 500
)

// ForexAPI is the slice of the payment client the worker needs.
type ForexAPI interface {
	CreateForexOrder(req *paymentapi.CreateForexOrderReq) (*paymentapi.CreateForexOrderResult, error)
	CloseForexOrder(req *paymentapi.CloseForexOrderReq) error
	ModifyForexOrder(req *paymentapi.ModifyForexOrderReq) error
	BatchCloseForexOrder(req *paymentapi.BatchCloseForexOrderReq) ([]paymentapi.BatchOrderResult, error)
	BatchCancelForexOrder(req *paymentapi.BatchCancelForexOrderReq) ([]paymentapi.BatchOrderResult, error)
	GetForexOpenOrders(loginID int64) ([]paymentapi.ForexOpenOrder, error)
}

// Config wires one worker instance.
type Config struct {
	GatewayURL          string // ws:// or wss:// base, no trailing slash
	APIKey              string // gwk_… issued by the gateway admin
	WorkerID            string
	LoginID             int64
	Symbol              string  // MT5 symbol on the bifu account, e.g. XAUUSD
	Volume              float64 // default lot size when the signal has none
	Live                bool    // master switch: false = dry-run everything
	DefaultSourceStatus string  // test | live — status for newly seen sources
	StateFile           string
}

// Signal mirrors the gateway's build_signal_dict payload.
type Signal struct {
	TraceID       string   `json:"trace_id"`
	SignalType    string   `json:"signal_type"`
	IsValid       bool     `json:"is_valid"`
	Symbol        string   `json:"symbol"`
	EntryPrice    *float64 `json:"entry_price"`
	EntryLow      *float64 `json:"entry_low"`
	EntryHigh     *float64 `json:"entry_high"`
	StopLoss      *float64 `json:"stop_loss"`
	TakeProfit    *float64 `json:"take_profit"`
	Amount        *float64 `json:"amount"`
	CloseRatio    float64  `json:"close_ratio"`
	TargetTicket  string   `json:"target_ticket"`
	Tag           string   `json:"tag"`
	RawMessage    string   `json:"raw_message"`
	SourceName    string   `json:"source_name"`
	CollectorName string   `json:"collector_name"`
	SourceType    string   `json:"source_type"`
	Ts            float64  `json:"ts"`
}

type persistedState struct {
	LastSeq     int64             `json:"last_seq"`
	SourceRules map[string]string `json:"source_rules,omitempty"` // name → disabled|test|live
	UpdatedAt   int64             `json:"updated_at"`
}

// Worker is a live gateway connection plus execution state.
type Worker struct {
	cfg Config
	api ForexAPI
	pr  *output.Printer

	mu          sync.Mutex // guards ws writes + mutable state below
	conn        *websocket.Conn
	lastSeq     int64
	paused      bool
	sourceRules map[string]string
	tracedIDs   map[string]struct{}
	traceOrder  []string
	apiHealthy  bool
}


// logf prints a timestamped operational line (always visible; the worker is a
// long-running daemon, so these are not verbose-gated).
func (w *Worker) logf(format string, args ...any) {
	w.pr.Line(time.Now().Format("15:04:05")+" "+format, args...)
}

func New(cfg Config, api ForexAPI, pr *output.Printer) *Worker {
	w := &Worker{
		cfg:         cfg,
		api:         api,
		pr:          pr,
		sourceRules: map[string]string{},
		tracedIDs:   map[string]struct{}{},
	}
	w.loadState()
	return w
}

// Run blocks until ctx is cancelled, reconnecting on any transport error.
func (w *Worker) Run(ctx context.Context) error {
	mode := "DRY-RUN"
	if w.cfg.Live {
		mode = "LIVE"
	}
	w.logf("relay worker starting [%s] gateway=%s worker_id=%s symbol=%s login=%d last_seq=%d",
		mode, w.cfg.GatewayURL, w.cfg.WorkerID, w.cfg.Symbol, w.cfg.LoginID, w.lastSeq)
	for {
		if err := w.connectAndReceive(ctx); err != nil && ctx.Err() == nil {
			w.pr.Err("gateway connection: %v (reconnect in %s)", err, reconnectInterval)
		}
		select {
		case <-ctx.Done():
			w.saveState()
			return ctx.Err()
		case <-time.After(reconnectInterval):
		}
	}
}

func (w *Worker) connectAndReceive(ctx context.Context) error {
	qs := url.Values{}
	qs.Set("api_key", w.cfg.APIKey)
	qs.Set("last_seq", strconv.FormatInt(w.lastSeq, 10))
	qs.Set("worker_id", w.cfg.WorkerID)
	wsURL := strings.TrimRight(w.cfg.GatewayURL, "/") + "/worker/stream?" + qs.Encode()

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	conn.SetReadLimit(1 << 20)
	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.conn = nil
		w.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
	}()
	w.pr.OK("gateway connected (last_seq=%d)", w.lastSeq)

	loopCtx, stop := context.WithCancel(ctx)
	defer stop()
	go w.telemetryLoop(loopCtx)
	go w.pingLoop(loopCtx, conn)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}
		var frame map[string]json.RawMessage
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}
		var event string
		if raw, ok := frame["event"]; ok {
			_ = json.Unmarshal(raw, &event)
		}
		switch event {
		case "connected":
			continue
		case "disconnect":
			w.logf("gateway sent disconnect; reconnecting")
			return nil
		case "control":
			w.handleControl(ctx, data)
		default:
			w.handleEntry(data)
		}
	}
}

func (w *Worker) pingLoop(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_ = conn.Ping(pingCtx)
			cancel()
		}
	}
}

// send serializes one JSON frame to the gateway (single-writer via mu).
func (w *Worker) send(ctx context.Context, v any) error {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, data)
}

// ── control downlink ────────────────────────────────────────────────

func (w *Worker) handleControl(ctx context.Context, data []byte) {
	var frame struct {
		ControlID string            `json:"control_id"`
		Action    string            `json:"action"`
		Params    map[string]string `json:"params"`
	}
	if err := json.Unmarshal(data, &frame); err != nil {
		return
	}
	ok, detail := w.applyControl(frame.Action, frame.Params)
	w.logf("control %s(%v) → ok=%v %s", frame.Action, frame.Params, ok, detail)
	_ = w.send(ctx, map[string]any{
		"event": "control_result", "control_id": frame.ControlID,
		"ok": ok, "detail": detail,
	})
}

func (w *Worker) applyControl(action string, params map[string]string) (bool, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	switch action {
	case "pause":
		w.paused = true
		return true, "paused"
	case "resume":
		w.paused = false
		return true, "resumed"
	case "set_source_rule":
		source, status := params["source"], params["status"]
		if status != "disabled" && status != "test" && status != "live" {
			return false, "invalid status: " + status
		}
		if _, known := w.sourceRules[source]; !known {
			return false, "unknown source: " + source
		}
		w.sourceRules[source] = status
		w.saveStateLocked()
		return true, source + " → " + status
	}
	return false, "unknown action: " + action
}

// ── telemetry uplink ────────────────────────────────────────────────

func (w *Worker) telemetryLoop(ctx context.Context) {
	t := time.NewTicker(telemetryInterval)
	defer t.Stop()
	for {
		if err := w.send(ctx, map[string]any{"event": "telemetry", "data": w.collectTelemetry()}); err != nil && ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *Worker) collectTelemetry() map[string]any {
	w.mu.Lock()
	status := "running"
	if w.paused {
		status = "paused"
	}
	rules := make(map[string]string, len(w.sourceRules))
	for k, v := range w.sourceRules {
		rules[k] = v
	}
	w.mu.Unlock()

	positions := []map[string]any{}
	equity := 0.0
	healthy := false
	if orders, err := w.api.GetForexOpenOrders(w.cfg.LoginID); err == nil {
		healthy = true
		for _, o := range orders {
			if !isPosition(o.OrderType) {
				continue
			}
			positions = append(positions, map[string]any{
				"ticket":      o.Ticket,
				"symbol":      o.Symbol,
				"direction":   strings.ToLower(o.OrderType),
				"size":        parseF(o.Volume),
				"entry_price": parseF(o.OpenPrice),
				"pnl":         parseF(o.Profit),
			})
			equity += parseF(o.Profit)
		}
	}
	w.mu.Lock()
	w.apiHealthy = healthy
	w.mu.Unlock()
	return map[string]any{
		"ts":            time.Now().Unix(),
		"status":        status,
		"executor":      "bifu-forex",
		"symbol":        w.cfg.Symbol,
		"mt5_connected": healthy,
		"equity":        equity, // floating P&L only; account equity is not exposed here
		"positions":     positions,
		"source_rules":  rules,
	}
}

// ── signal entries ──────────────────────────────────────────────────

func (w *Worker) handleEntry(data []byte) {
	var entry struct {
		Seq    int64   `json:"seq"`
		Ts     float64 `json:"ts"`
		Signal *Signal `json:"signal"`
	}
	if err := json.Unmarshal(data, &entry); err != nil || entry.Signal == nil {
		return
	}
	if entry.Seq > 0 {
		w.mu.Lock()
		w.lastSeq = entry.Seq
		w.mu.Unlock()
	}
	s := entry.Signal

	if s.TraceID != "" && w.seenTrace(s.TraceID) {
		return
	}
	ts := s.Ts
	if ts == 0 {
		ts = entry.Ts
	}
	if ts > 0 && time.Since(time.Unix(int64(ts), 0)) > staleExecutionSecs*time.Second {
		w.logf("skip stale replay (trace=%s seq=%d age=%.0fs)", s.TraceID, entry.Seq, time.Since(time.Unix(int64(ts), 0)).Seconds())
		w.saveState()
		return
	}

	w.executeSignal(s)
	w.saveState()
}

func (w *Worker) seenTrace(traceID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, dup := w.tracedIDs[traceID]; dup {
		return true
	}
	w.tracedIDs[traceID] = struct{}{}
	w.traceOrder = append(w.traceOrder, traceID)
	for len(w.traceOrder) > maxTracedIDs {
		delete(w.tracedIDs, w.traceOrder[0])
		w.traceOrder = w.traceOrder[1:]
	}
	return false
}

func (w *Worker) executeSignal(s *Signal) {
	if !s.IsValid || s.SignalType == "" || s.SignalType == "none" {
		return
	}
	w.mu.Lock()
	if w.paused {
		w.mu.Unlock()
		w.logf("paused, skip %s (trace=%s)", s.SignalType, s.TraceID)
		return
	}
	source := s.CollectorName
	rule, known := w.sourceRules[source]
	if !known && source != "" {
		rule = w.cfg.DefaultSourceStatus
		w.sourceRules[source] = rule
		w.saveStateLocked()
		w.logf("new source %q → %s", source, rule)
	}
	w.mu.Unlock()

	if rule == "disabled" {
		w.logf("source %q disabled, skip %s (trace=%s)", source, s.SignalType, s.TraceID)
		return
	}
	dryRun := !w.cfg.Live || rule != "live"

	tag := ""
	if dryRun {
		tag = " [dry-run]"
	}
	w.logf("signal%s %s trace=%s src=%s raw=%.60q", tag, s.SignalType, s.TraceID, source, s.RawMessage)

	var err error
	switch s.SignalType {
	case "open_long", "open_short":
		err = w.doOpen(s, dryRun)
	case "close_long", "close_short", "close_all":
		err = w.doClose(s, dryRun)
	case "set_stop_loss", "set_take_profit":
		err = w.doModify(s, dryRun)
	case "cancel":
		err = w.doCancel(s, dryRun)
	default:
		w.logf("unsupported signal type %q, skip", s.SignalType)
		return
	}
	if err != nil {
		w.pr.Err("execute %s failed (trace=%s): %v", s.SignalType, s.TraceID, err)
	}
}

func (w *Worker) doOpen(s *Signal, dryRun bool) error {
	volume := w.cfg.Volume
	if s.Amount != nil && *s.Amount > 0 {
		volume = *s.Amount
	}
	typ := "buy"
	if s.SignalType == "open_short" {
		typ = "sell"
	}
	price := 0.0
	if s.EntryLow != nil && s.EntryHigh != nil && *s.EntryLow > 0 && *s.EntryHigh > 0 {
		// entry range → resting limit order at the midpoint
		price = math.Round((*s.EntryLow+*s.EntryHigh)/2*100) / 100
		typ += "Limit"
	}
	req := &paymentapi.CreateForexOrderReq{
		LoginID: w.cfg.LoginID,
		Symbol:  w.cfg.Symbol,
		Volume:  volume,
		Type:    typ,
		Price:   price,
		SL:      deref(s.StopLoss),
		TP:      deref(s.TakeProfit),
		Comment: "relay:" + s.TraceID,
	}
	if dryRun {
		w.logf("would create order: %s %s %.2f lots price=%.2f sl=%.2f tp=%.2f", req.Type, req.Symbol, req.Volume, req.Price, req.SL, req.TP)
		return nil
	}
	res, err := w.api.CreateForexOrder(req)
	if err != nil {
		return err
	}
	w.pr.OK("order placed: %s %s %.2f lots (order=%v trace=%s)", req.Type, req.Symbol, req.Volume, res.OrderID, s.TraceID)
	return nil
}

func (w *Worker) doClose(s *Signal, dryRun bool) error {
	positions, err := w.openPositions()
	if err != nil {
		return err
	}
	var targets []paymentapi.ForexOpenOrder
	for _, p := range positions {
		dir := strings.ToLower(p.OrderType)
		switch s.SignalType {
		case "close_long":
			if dir != "buy" {
				continue
			}
		case "close_short":
			if dir != "sell" {
				continue
			}
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		w.logf("no matching positions to close (%s)", s.SignalType)
		return nil
	}

	// partial close: per-position volume, MT5 lot step 0.01
	if s.CloseRatio > 0 && s.CloseRatio < 1 {
		for _, p := range targets {
			ticket, terr := strconv.ParseInt(p.Ticket, 10, 64)
			if terr != nil {
				continue
			}
			vol := math.Floor(parseF(p.Volume)*s.CloseRatio*100) / 100
			if vol <= 0 {
				continue
			}
			if dryRun {
				w.logf("would partial-close ticket=%d vol=%.2f (%.0f%%)", ticket, vol, s.CloseRatio*100)
				continue
			}
			if cerr := w.api.CloseForexOrder(&paymentapi.CloseForexOrderReq{LoginID: w.cfg.LoginID, OrderID: ticket, Volume: vol}); cerr != nil {
				err = cerr
			} else {
				w.pr.OK("partial-closed ticket=%d vol=%.2f", ticket, vol)
			}
		}
		return err
	}

	ids := ticketIDs(targets)
	if dryRun {
		w.logf("would close %d position(s): %v", len(ids), ids)
		return nil
	}
	results, err := w.api.BatchCloseForexOrder(&paymentapi.BatchCloseForexOrderReq{LoginID: w.cfg.LoginID, OrderIDs: ids, Volume: 0})
	if err != nil {
		return err
	}
	w.pr.OK("closed %d position(s): %v (results=%d)", len(ids), ids, len(results))
	return nil
}

func (w *Worker) doModify(s *Signal, dryRun bool) error {
	positions, err := w.openPositions()
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		w.logf("no open positions to modify (%s)", s.SignalType)
		return nil
	}
	for _, p := range positions {
		ticket, terr := strconv.ParseInt(p.Ticket, 10, 64)
		if terr != nil {
			continue
		}
		// carry the untouched side so the backend doesn't clear it
		sl, tp := parseF(p.StopLoss), parseF(p.TakeProfit)
		if s.SignalType == "set_stop_loss" {
			sl = deref(s.StopLoss)
		} else {
			tp = deref(s.TakeProfit)
		}
		if dryRun {
			w.logf("would modify ticket=%d sl=%.2f tp=%.2f", ticket, sl, tp)
			continue
		}
		if merr := w.api.ModifyForexOrder(&paymentapi.ModifyForexOrderReq{LoginID: w.cfg.LoginID, OrderID: ticket, SL: sl, TP: tp}); merr != nil {
			err = merr
		} else {
			w.pr.OK("modified ticket=%d sl=%.2f tp=%.2f", ticket, sl, tp)
		}
	}
	return err
}

func (w *Worker) doCancel(s *Signal, dryRun bool) error {
	orders, err := w.api.GetForexOpenOrders(w.cfg.LoginID)
	if err != nil {
		return err
	}
	var ids []int64
	for _, o := range orders {
		if o.Symbol != w.cfg.Symbol || isPosition(o.OrderType) {
			continue
		}
		if ticket, terr := strconv.ParseInt(o.Ticket, 10, 64); terr == nil {
			ids = append(ids, ticket)
		}
	}
	if len(ids) == 0 {
		w.logf("no pending orders to cancel")
		return nil
	}
	if dryRun {
		w.logf("would cancel %d pending order(s): %v", len(ids), ids)
		return nil
	}
	if _, err := w.api.BatchCancelForexOrder(&paymentapi.BatchCancelForexOrderReq{LoginID: w.cfg.LoginID, OrderIDs: ids}); err != nil {
		return err
	}
	w.pr.OK("cancelled %d pending order(s): %v", len(ids), ids)
	return nil
}

func (w *Worker) openPositions() ([]paymentapi.ForexOpenOrder, error) {
	orders, err := w.api.GetForexOpenOrders(w.cfg.LoginID)
	if err != nil {
		return nil, err
	}
	var out []paymentapi.ForexOpenOrder
	for _, o := range orders {
		if o.Symbol == w.cfg.Symbol && isPosition(o.OrderType) {
			out = append(out, o)
		}
	}
	return out, nil
}

// ── state persistence ───────────────────────────────────────────────

func (w *Worker) loadState() {
	if w.cfg.StateFile == "" {
		return
	}
	data, err := os.ReadFile(w.cfg.StateFile) // #nosec G304 -- our own state path under the CLI home
	if err != nil {
		return
	}
	var st persistedState
	if json.Unmarshal(data, &st) != nil {
		return
	}
	w.lastSeq = st.LastSeq
	if st.SourceRules != nil {
		w.sourceRules = st.SourceRules
	}
}

func (w *Worker) saveState() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.saveStateLocked()
}

func (w *Worker) saveStateLocked() {
	if w.cfg.StateFile == "" {
		return
	}
	st := persistedState{LastSeq: w.lastSeq, SourceRules: w.sourceRules, UpdatedAt: time.Now().Unix()}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(w.cfg.StateFile), 0o700)
	tmp := w.cfg.StateFile + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, w.cfg.StateFile)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func isPosition(orderType string) bool {
	t := strings.ToLower(orderType)
	return t == "buy" || t == "sell"
}

func parseF(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func ticketIDs(orders []paymentapi.ForexOpenOrder) []int64 {
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		if t, err := strconv.ParseInt(o.Ticket, 10, 64); err == nil {
			ids = append(ids, t)
		}
	}
	return ids
}
