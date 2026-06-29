package contract

import "testing"

func TestCreateOrderReqValidate(t *testing.T) {
	cases := []struct {
		name                string
		ps, os              string
		reduceOnly, wantErr bool
	}{
		{"open long", "LONG", "BUY", false, false},
		{"open short", "SHORT", "SELL", false, false},
		{"close long ok", "LONG", "SELL", true, false},
		{"close short ok", "SHORT", "BUY", true, false},
		{"close long missing reduceOnly", "LONG", "SELL", false, true},
		{"close short missing reduceOnly", "SHORT", "BUY", false, true},
		{"open long with reduceOnly", "LONG", "BUY", true, true},
		{"nonsense combo", "LONG", "LONG", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&CreateOrderReq{PositionSide: c.ps, OrderSide: c.os, ReduceOnly: c.reduceOnly}).Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate(%s/%s reduceOnly=%v) err=%v, wantErr=%v", c.ps, c.os, c.reduceOnly, err, c.wantErr)
			}
		})
	}
}
