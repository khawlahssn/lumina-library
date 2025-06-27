package scrapers

import (
	"testing"
	"time"
)

func TestParseKuCoinTradeMessage(t *testing.T) {
	cases := []struct {
		name        string
		input       kuCoinWSResponse
		wantPrice   float64
		wantVolume  float64
		wantTime    time.Time
		wantTradeID string
		expectErr   bool
	}{
		{
			name: "valid buy trade",
			input: kuCoinWSResponse{
				Data: kuCoinWSData{
					Price:   "33225.1",
					Size:    "0.23",
					Side:    "buy",
					TradeID: "abc123",
					Time:    "1721923200000", // ms since epoch
				},
			},
			wantPrice:   33225.1,
			wantVolume:  0.23,
			wantTime:    time.Unix(0, 1721923200000),
			wantTradeID: "abc123",
			expectErr:   false,
		},
		{
			name: "invalid price",
			input: kuCoinWSResponse{
				Data: kuCoinWSData{
					Price:   "bad-price",
					Size:    "0.2",
					Side:    "buy",
					TradeID: "abc456",
					Time:    "1721923200000",
				},
			},
			expectErr: true,
		},
		{
			name: "invalid volume",
			input: kuCoinWSResponse{
				Data: kuCoinWSData{
					Price:   "33225.1",
					Size:    "bad-size",
					Side:    "buy",
					TradeID: "abc789",
					Time:    "1721923200000",
				},
			},
			expectErr: true,
		},
		{
			name: "invalid timestamp",
			input: kuCoinWSResponse{
				Data: kuCoinWSData{
					Price:   "10000.1",
					Size:    "1.0",
					Side:    "buy",
					TradeID: "badts",
					Time:    "bad-timestamp",
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, volume, ts, tradeID, err := parseKuCoinTradeMessage(tc.input)
			if tc.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectErr {
				if price != tc.wantPrice {
					t.Errorf("Price: got %v, want %v", price, tc.wantPrice)
				}
				if volume != tc.wantVolume {
					t.Errorf("Volume: got %v, want %v", volume, tc.wantVolume)
				}
				if !ts.Equal(tc.wantTime) {
					t.Errorf("Time: got %v, want %v", ts, tc.wantTime)
				}
				if tradeID != tc.wantTradeID {
					t.Errorf("TradeID: got %v, want %v", tradeID, tc.wantTradeID)
				}
			}
		})
	}
}
