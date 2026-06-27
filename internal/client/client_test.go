package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bifu-cli/internal/clifconfig"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"}, // shorter than limit
		{"hello", 5, "hello"},  // exactly at limit
		{"hello", 3, "hel..."}, // truncated
		{"", 5, ""},            // empty
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestAPIResponseGetMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  interface{}
		want string
	}{
		{"string", "boom", "boom"},
		{"nested map", map[string]interface{}{"message": "nested"}, "nested"},
		{"nil", nil, ""},
		{"unexpected type", 42, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &APIResponse{Message: c.msg}
			if got := r.GetMessage(); got != c.want {
				t.Errorf("GetMessage() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseAPIResponse(t *testing.T) {
	t.Run("success with data", func(t *testing.T) {
		var dst struct {
			OrderID string `json:"orderId"`
		}
		err := ParseAPIResponse([]byte(`{"code":"SUCCESS","data":{"orderId":"42"}}`), &dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dst.OrderID != "42" {
			t.Errorf("OrderID = %q, want %q", dst.OrderID, "42")
		}
	})

	t.Run("empty code treated as success", func(t *testing.T) {
		if err := ParseAPIResponse([]byte(`{"data":null}`), nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error code surfaces message", func(t *testing.T) {
		err := ParseAPIResponse([]byte(`{"code":"BAD_REQUEST","message":"invalid symbol"}`), nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "BAD_REQUEST") || !strings.Contains(err.Error(), "invalid symbol") {
			t.Errorf("error = %q, want it to mention code and message", err.Error())
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		if err := ParseAPIResponse([]byte(`{not json`), nil); err == nil {
			t.Error("expected error for malformed json")
		}
	})
}

func TestParsePaymentResponse(t *testing.T) {
	t.Run("success retCode 0", func(t *testing.T) {
		var dst struct {
			Balance string `json:"balance"`
		}
		err := ParsePaymentResponse([]byte(`{"retCode":"0","result":{"balance":"100"}}`), &dst)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dst.Balance != "100" {
			t.Errorf("Balance = %q, want %q", dst.Balance, "100")
		}
	})

	t.Run("numeric retCode 0", func(t *testing.T) {
		if err := ParsePaymentResponse([]byte(`{"retCode":0,"result":null}`), nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error retCode", func(t *testing.T) {
		err := ParsePaymentResponse([]byte(`{"retCode":"10001","retMsg":"insufficient funds"}`), nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "10001") || !strings.Contains(err.Error(), "insufficient funds") {
			t.Errorf("error = %q, want it to mention code and message", err.Error())
		}
	})

	t.Run("missing result is ok", func(t *testing.T) {
		var dst struct{ X string }
		if err := ParsePaymentResponse([]byte(`{"retCode":"0"}`), &dst); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// newTestClient builds an HTTPClient pointed at a test server.
func newTestClient(baseURL string) *HTTPClient {
	return NewHTTPClient(&clifconfig.Profile{
		BaseURL: baseURL,
		Auth:    clifconfig.AuthProfile{AuthCookie: "test-cookie", AuthCookieName: "user_auth_name"},
	})
}

func TestHTTPErrorMapping(t *testing.T) {
	cases := []struct {
		status  int
		body    string
		wantSub string // substring expected in the error
	}{
		{http.StatusUnauthorized, "", "auth login"},
		{http.StatusForbidden, "nope", "access denied"},
		{http.StatusNotFound, "", "not found"},
		{http.StatusInternalServerError, "kaboom", "server error (HTTP 500)"},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		_, err := newTestClient(srv.URL).GetSpot(srv.URL, nil)
		srv.Close()
		if err == nil {
			t.Errorf("status %d: expected error, got nil", c.status)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("status %d: error = %q, want substring %q", c.status, err.Error(), c.wantSub)
		}
	}
}

func TestHTTPRequestSendsCookieAndParsesBody(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"SUCCESS","data":{"orderId":"7"}}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).PostSpot(srv.URL, map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCookie != "user_auth_name=test-cookie" {
		t.Errorf("Cookie header = %q, want %q", gotCookie, "user_auth_name=test-cookie")
	}
	var dst struct {
		OrderID string `json:"orderId"`
	}
	if err := ParseAPIResponse(resp.Body, &dst); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if dst.OrderID != "7" {
		t.Errorf("OrderID = %q, want %q", dst.OrderID, "7")
	}
}

func TestGETRetriesTransientUnknown(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 { // first two attempts return the transient UNKNOWN envelope
			_, _ = w.Write([]byte(`{"code":"UNKNOWN","data":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":"SUCCESS","data":{"orderId":"9"}}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).GetSpot(srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("server calls = %d, want 3 (two transient + one success)", calls)
	}
	var dst struct {
		OrderID string `json:"orderId"`
	}
	if err := ParseAPIResponse(resp.Body, &dst); err != nil || dst.OrderID != "9" {
		t.Errorf("final response not the success body: %v / %q", err, dst.OrderID)
	}
}

func TestPOSTNotRetried(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError) // transient 5xx
	}))
	defer srv.Close()

	// A POST (e.g. order creation) must NOT be replayed, even on a 5xx.
	_, _ = newTestClient(srv.URL).PostSpot(srv.URL, map[string]string{"a": "b"})
	if calls != 1 {
		t.Errorf("POST attempted %d times, want 1 (no replay of a write)", calls)
	}
}

func TestHTTPParamsAppendedToQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("symbol")
		_, _ = w.Write([]byte(`{"code":"SUCCESS"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).GetSpot(srv.URL, map[string]string{"symbol": "BTCUSDT"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "BTCUSDT" {
		t.Errorf("query symbol = %q, want %q", gotQuery, "BTCUSDT")
	}
}
