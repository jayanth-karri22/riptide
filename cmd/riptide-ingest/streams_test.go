package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestStreamURL(t *testing.T) {
	const spotBTC = "btcusdt@trade/btcusdt@bookTicker/btcusdt@depth@100ms"

	tests := []struct {
		name    string
		venue   Venue
		symbols []string
		want    string
		wantErr bool
	}{
		{
			name:    "spot pairs every symbol with every suffix",
			venue:   Spot,
			symbols: []string{"btcusdt", "ethusdt"},
			want: "wss://stream.binance.com:9443/stream?streams=" + spotBTC +
				"/ethusdt@trade/ethusdt@bookTicker/ethusdt@depth@100ms",
		},
		{
			name:    "perp uses aggTrade and adds markPrice",
			venue:   Perp,
			symbols: []string{"btcusdt"},
			want: "wss://fstream.binance.com/public/stream?streams=" +
				"btcusdt@aggTrade/btcusdt@bookTicker/btcusdt@depth@100ms/btcusdt@markPrice@1s",
		},
		{
			name:    "symbols are trimmed and lowercased",
			venue:   Spot,
			symbols: []string{"  BTCUSDT\n"},
			want:    "wss://stream.binance.com:9443/stream?streams=" + spotBTC,
		},
		{
			name:    "blank symbols are dropped, not subscribed",
			venue:   Spot,
			symbols: []string{"btcusdt", "", "   "},
			want:    "wss://stream.binance.com:9443/stream?streams=" + spotBTC,
		},
		{
			name:    "no usable symbols is an error",
			venue:   Spot,
			symbols: []string{"", "  "},
			wantErr: true,
		},
		{
			name:    "nil symbols is an error",
			venue:   Perp,
			symbols: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.venue.StreamURL(tt.symbols)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestStreamURLDoesNotMutateInput(t *testing.T) {
	symbols := []string{" BTCUSDT ", "ethusdt"}
	if _, err := Spot.StreamURL(symbols); err != nil {
		t.Fatal(err)
	}
	if symbols[0] != " BTCUSDT " {
		t.Errorf("caller's slice was modified: %q", symbols[0])
	}
}

func TestStreamURLStreamCount(t *testing.T) {
	symbols := []string{"btcusdt", "ethusdt", "solusdt", "xrpusdt", "bnbusdt",
		"dogeusdt", "adausdt", "avaxusdt", "linkusdt", "ltcusdt"}

	for _, tt := range []struct {
		venue Venue
		want  int
	}{
		{Spot, 30},
		{Perp, 40},
	} {
		t.Run(tt.venue.Name, func(t *testing.T) {
			got, err := tt.venue.StreamURL(symbols)
			if err != nil {
				t.Fatal(err)
			}
			_, query, ok := strings.Cut(got, "?streams=")
			if !ok {
				t.Fatalf("no ?streams= in %q", got)
			}
			if n := len(strings.Split(query, "/")); n != tt.want {
				t.Errorf("got %d streams, want %d", n, tt.want)
			}
		})
	}
}

func TestStreamURLEnforcesPerVenueCap(t *testing.T) {
	symbols := make([]string, 51) // 153 spot streams, 204 perp streams
	for i := range symbols {
		symbols[i] = fmt.Sprintf("sym%dusdt", i)
	}
	if _, err := Spot.StreamURL(symbols); err != nil {
		t.Errorf("spot should accept 153 streams under its 1024 cap: %v", err)
	}
	if _, err := Perp.StreamURL(symbols); err == nil {
		t.Error("perp should reject 204 streams over its 200 cap")
	}
}
