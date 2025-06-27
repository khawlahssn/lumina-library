package scrapers

import (
	"strconv"
	"testing"
	"time"
)

func TestParseKrakenTradeMessage(t *testing.T) {
	cases := []struct {
		name    string
		input   krakenWSResponseData
		want    float64
		wantVol float64
		wantTs  string // string for time parsing
		wantID  string
		wantErr bool
	}{
		{
			name: "valid buy trade",
			input: krakenWSResponseData{
				Symbol:    "BTC/USD",
				Side:      "buy",
				Price:     31500.5,
				Size:      0.123,
				OrderType: "market",
				TradeID:   456,
				Time:      "2024-06-26T23:50:55.000000Z",
			},
			want:    31500.5,
			wantVol: 0.123,
			wantTs:  "2024-06-26T23:50:55.000000Z",
			wantID:  strconv.Itoa(456),
			wantErr: false,
		},
		{
			name: "invalid timestamp",
			input: krakenWSResponseData{
				Symbol:    "ETH/USD",
				Side:      "buy",
				Price:     2500.0,
				Size:      1.0,
				OrderType: "limit",
				TradeID:   789,
				Time:      "bad-timestamp",
			},
			want:    2500.0,
			wantVol: 1.0,
			wantTs:  "",
			wantID:  "",
			wantErr: true,
		},
	}

	layout := "2006-01-02T15:04:05.000000Z"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, volume, timestamp, foreignTradeID, err := parseKrakenTradeMessage(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseKrakenTradeMessage() error = %v, wantErr %v", err, tc.wantErr)
			}
			if price != tc.want {
				t.Errorf("price = %v, want %v", price, tc.want)
			}
			if volume != tc.wantVol {
				t.Errorf("volume = %v, want %v", volume, tc.wantVol)
			}
			if tc.wantTs != "" {
				wantTime, err2 := time.Parse(layout, tc.wantTs)
				if err2 != nil {
					t.Fatalf("bad wantTs in test: %v", err2)
				}
				if !timestamp.Equal(wantTime) {
					t.Errorf("timestamp = %v, want %v", timestamp, wantTime)
				}
			} else if !timestamp.IsZero() {
				t.Errorf("timestamp should be zero, got %v", timestamp)
			}
			if foreignTradeID != tc.wantID {
				t.Errorf("foreignTradeID = %v, want %v", foreignTradeID, tc.wantID)
			}
		})
	}
}
