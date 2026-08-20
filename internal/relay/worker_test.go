package relay

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	paymentapi "bifu-cli/internal/api/payment"
	"bifu-cli/internal/output"
)

// fakeForex records calls and serves canned open orders.
type fakeForex struct {
	openOrders []paymentapi.ForexOpenOrder
	created    []*paymentapi.CreateForexOrderReq
	closed     []*paymentapi.CloseForexOrderReq
	modified   []*paymentapi.ModifyForexOrderReq
	batchClose []*paymentapi.BatchCloseForexOrderReq
	batchCanc  []*paymentapi.BatchCancelForexOrderReq
}

func (f *fakeForex) CreateForexOrder(req *paymentapi.CreateForexOrderReq) (*paymentapi.CreateForexOrderResult, error) {
	f.created = append(f.created, req)
	return &paymentapi.CreateForexOrderResult{OrderID: "1001"}, nil
}
func (f *fakeForex) CloseForexOrder(req *paymentapi.CloseForexOrderReq) error {
	f.closed = append(f.closed, req)
	return nil
}
func (f *fakeForex) ModifyForexOrder(req *paymentapi.ModifyForexOrderReq) error {
	f.modified = append(f.modified, req)
	return nil
}
func (f *fakeForex) BatchCloseForexOrder(req *paymentapi.BatchCloseForexOrderReq) ([]paymentapi.BatchOrderResult, error) {
	f.batchClose = append(f.batchClose, req)
	return nil, nil
}
func (f *fakeForex) BatchCancelForexOrder(req *paymentapi.BatchCancelForexOrderReq) ([]paymentapi.BatchOrderResult, error) {
	f.batchCanc = append(f.batchCanc, req)
	return nil, nil
}
func (f *fakeForex) GetForexOpenOrders(int64) ([]paymentapi.ForexOpenOrder, error) {
	return f.openOrders, nil
}

func newTestWorker(t *testing.T, api ForexAPI, live bool) *Worker {
	t.Helper()
	return New(Config{
		GatewayURL:          "wss://example.invalid",
		APIKey:              "gwk_test",
		WorkerID:            "test",
		LoginID:             2002,
		Symbol:              "XAUUSD",
		Volume:              0.01,
		Live:                live,
		DefaultSourceStatus: "live",
		StateFile:           filepath.Join(t.TempDir(), "state.json"),
	}, api, output.NewPrinter(output.FormatPlain, false))
}

func fp(v float64) *float64 { return &v }

func sig(typ string) *Signal {
	return &Signal{
		TraceID:       "tid_" + typ,
		SignalType:    typ,
		IsValid:       true,
		Symbol:        "XAUUSD",
		CollectorName: "TG",
		Ts:            float64(time.Now().Unix()),
	}
}

func TestOpenLongMarketOrder(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	s := sig("open_long")
	s.StopLoss, s.TakeProfit = fp(4490.0), fp(4510.0)
	w.executeSignal(s)
	if len(api.created) != 1 {
		t.Fatalf("expected 1 order, got %d", len(api.created))
	}
	req := api.created[0]
	if req.Type != "buy" || req.Price != 0 || req.Volume != 0.01 || req.SL != 4490 || req.TP != 4510 {
		t.Fatalf("bad order: %+v", req)
	}
}

func TestOpenShortWithEntryRangeIsLimit(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	s := sig("open_short")
	s.EntryLow, s.EntryHigh = fp(4500.0), fp(4510.0)
	s.Amount = fp(0.05)
	w.executeSignal(s)
	req := api.created[0]
	if req.Type != "sellLimit" || req.Price != 4505 || req.Volume != 0.05 {
		t.Fatalf("bad pending order: %+v", req)
	}
}

func TestDryRunPlacesNothing(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, false) // --live not set
	w.executeSignal(sig("open_long"))
	if len(api.created) != 0 {
		t.Fatalf("dry-run must not place orders")
	}
}

func TestSourceRuleTestModePlacesNothing(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	w.sourceRules["TG"] = "test"
	w.executeSignal(sig("open_long"))
	if len(api.created) != 0 {
		t.Fatalf("test-mode source must not place orders")
	}
}

func TestCloseLongOnlyClosesBuys(t *testing.T) {
	api := &fakeForex{openOrders: []paymentapi.ForexOpenOrder{
		{Ticket: "11", Symbol: "XAUUSD", OrderType: "Market", Side: "Buy", Lots: "0.02"},
		{Ticket: "12", Symbol: "XAUUSD", OrderType: "Market", Side: "Sell", Lots: "0.02"},
		{Ticket: "13", Symbol: "EURUSD", OrderType: "Market", Side: "Buy", Lots: "0.02"},
		{Ticket: "14", Symbol: "XAUUSD", OrderType: "Limit", Side: "Buy", Lots: "0.02"},
	}}
	w := newTestWorker(t, api, true)
	w.executeSignal(sig("close_long"))
	if len(api.batchClose) != 1 {
		t.Fatalf("expected 1 batch close, got %d", len(api.batchClose))
	}
	ids := api.batchClose[0].OrderIDs
	if len(ids) != 1 || ids[0] != 11 {
		t.Fatalf("close_long should target only XAUUSD buy positions, got %v", ids)
	}
}

func TestPartialCloseUsesRatio(t *testing.T) {
	api := &fakeForex{openOrders: []paymentapi.ForexOpenOrder{
		{Ticket: "21", Symbol: "XAUUSD", OrderType: "Market", Side: "Sell", Lots: "0.10"},
	}}
	w := newTestWorker(t, api, true)
	s := sig("close_short")
	s.CloseRatio = 0.5
	w.executeSignal(s)
	if len(api.closed) != 1 || api.closed[0].Volume != 0.05 || api.closed[0].OrderID != 21 {
		t.Fatalf("bad partial close: %+v", api.closed)
	}
}

func TestSetStopLossCarriesExistingTP(t *testing.T) {
	api := &fakeForex{openOrders: []paymentapi.ForexOpenOrder{
		{Ticket: "31", Symbol: "XAUUSD", OrderType: "Market", Side: "Buy", Lots: "0.01", StopLoss: "4480", TakeProfit: "4520"},
	}}
	w := newTestWorker(t, api, true)
	s := sig("set_stop_loss")
	s.StopLoss = fp(4495.0)
	w.executeSignal(s)
	if len(api.modified) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(api.modified))
	}
	m := api.modified[0]
	if m.SL != 4495 || m.TP != 4520 {
		t.Fatalf("modify must carry existing TP: %+v", m)
	}
}

func TestCancelTargetsPendingOnly(t *testing.T) {
	api := &fakeForex{openOrders: []paymentapi.ForexOpenOrder{
		{Ticket: "41", Symbol: "XAUUSD", OrderType: "Market", Side: "Buy", Lots: "0.01"},
		{Ticket: "42", Symbol: "XAUUSD", OrderType: "Stop", Side: "Sell", Lots: "0.01"},
	}}
	w := newTestWorker(t, api, true)
	w.executeSignal(sig("cancel"))
	if len(api.batchCanc) != 1 {
		t.Fatalf("expected 1 batch cancel, got %d", len(api.batchCanc))
	}
	if ids := api.batchCanc[0].OrderIDs; len(ids) != 1 || ids[0] != 42 {
		t.Fatalf("cancel should target pending only, got %v", ids)
	}
}

func TestPausedAndDisabledSkip(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	w.paused = true
	w.executeSignal(sig("open_long"))
	w.paused = false
	w.sourceRules["TG"] = "disabled"
	w.executeSignal(sig("open_long"))
	if len(api.created) != 0 {
		t.Fatalf("paused/disabled must not trade")
	}
}

func TestEntryDedupAndStale(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	fresh := sig("open_long")
	entry := func(seq int64, s *Signal) []byte {
		b, _ := json.Marshal(map[string]any{"seq": seq, "ts": s.Ts, "signal": s})
		return b
	}
	w.handleEntry(entry(1, fresh))
	w.handleEntry(entry(2, fresh)) // duplicate trace_id
	if len(api.created) != 1 {
		t.Fatalf("duplicate trace must execute once, got %d", len(api.created))
	}
	stale := sig("open_short")
	stale.Ts = float64(time.Now().Add(-10 * time.Minute).Unix())
	w.handleEntry(entry(3, stale))
	if len(api.created) != 1 {
		t.Fatalf("stale replay must not execute")
	}
	if w.lastSeq != 3 {
		t.Fatalf("lastSeq should advance past stale entries, got %d", w.lastSeq)
	}
}

func TestControlRoundtrip(t *testing.T) {
	w := newTestWorker(t, &fakeForex{}, true)
	w.sourceRules["TG"] = "live"
	for _, tc := range []struct {
		action string
		params map[string]string
		wantOK bool
	}{
		{"pause", nil, true},
		{"resume", nil, true},
		{"set_source_rule", map[string]string{"source": "TG", "status": "test"}, true},
		{"set_source_rule", map[string]string{"source": "TG", "status": "bogus"}, false},
		{"set_source_rule", map[string]string{"source": "nope", "status": "live"}, false},
		{"selfdestruct", nil, false},
	} {
		ok, detail := w.applyControl(tc.action, tc.params)
		if ok != tc.wantOK {
			t.Fatalf("%s: ok=%v (%s), want %v", tc.action, ok, detail, tc.wantOK)
		}
	}
	if w.sourceRules["TG"] != "test" {
		t.Fatalf("rule not applied: %v", w.sourceRules)
	}
}

func TestStatePersistRoundtrip(t *testing.T) {
	api := &fakeForex{}
	w := newTestWorker(t, api, true)
	w.lastSeq = 42
	w.sourceRules["TG"] = "test"
	w.saveState()

	w2 := New(w.cfg, api, output.NewPrinter(output.FormatPlain, false))
	if w2.lastSeq != 42 || w2.sourceRules["TG"] != "test" {
		t.Fatalf("state roundtrip failed: seq=%d rules=%v", w2.lastSeq, w2.sourceRules)
	}
}

func TestTelemetryShape(t *testing.T) {
	api := &fakeForex{openOrders: []paymentapi.ForexOpenOrder{
		{Ticket: "51", Symbol: "XAUUSD", OrderType: "Market", Side: "Buy", Lots: "0.01", OpenPrice: "4500", Profit: "1.5"},
	}}
	w := newTestWorker(t, api, true)
	data := w.collectTelemetry()
	if data["status"] != "running" || data["mt5_connected"] != true {
		t.Fatalf("bad telemetry: %v", data)
	}
	positions, _ := data["positions"].([]map[string]any)
	if len(positions) != 1 || positions[0]["direction"] != "buy" {
		t.Fatalf("bad positions: %v", positions)
	}
	if _, err := json.Marshal(data); err != nil {
		t.Fatalf("telemetry must be JSON-serializable: %v", err)
	}
	_ = fmt.Sprintf("%v", data)
}
