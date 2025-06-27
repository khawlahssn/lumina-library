package scrapers

import (
	"strconv"
	"testing"
	"time"

	models "github.com/diadata-org/lumina-library/models"
)

func mustParseTime(t *testing.T, layout, value string) time.Time {
	ts, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("failed to parse time in test: %v", err)
	}
	return ts
}

func TestCoinbaseParseTradeMessage(t *testing.T) {
	cases := []struct {
		name   string
		input  coinBaseWSResponse
		expect models.Trade
	}{
		{
			name: "valid buy trade",
			input: coinBaseWSResponse{
				Price:   "30000.00",
				Size:    "0.5",
				Time:    "2024-06-26T01:23:45.123456Z",
				TradeID: 789,
				Side:    "buy",
			},
			expect: models.Trade{
				Price:          30000.00,
				Volume:         0.5,
				Time:           mustParseTime(t, "2006-01-02T15:04:05.000000Z", "2024-06-26T01:23:45.123456Z"),
				Exchange:       Exchanges[COINBASE_EXCHANGE],
				ForeignTradeID: strconv.Itoa(789),
			},
		},
		{
			name: "invalid price (should return zero trade)",
			input: coinBaseWSResponse{
				Price:   "not-a-number",
				Size:    "2.2",
				Time:    "2024-07-01T10:01:00.000000Z",
				TradeID: 456,
				Side:    "buy",
			},
			expect: models.Trade{}, // all fields zero
		},
		{
			name: "invalid volume (should return zero trade)",
			input: coinBaseWSResponse{
				Price:   "1337.99",
				Size:    "bad-size",
				Time:    "2024-07-01T10:02:00.000000Z",
				TradeID: 999,
				Side:    "buy",
			},
			expect: models.Trade{}, // all fields zero
		},
		{
			name: "invalid time (should return zero trade)",
			input: coinBaseWSResponse{
				Price:   "10.00",
				Size:    "1.0",
				Time:    "bad-time-format",
				TradeID: 100,
				Side:    "sell",
			},
			expect: models.Trade{}, // all fields zero
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := coinbaseParseTradeMessage(tc.input)
			if got.Exchange != tc.expect.Exchange {
				t.Errorf("Exchange: got %v, want %v", got.Exchange, tc.expect.Exchange)
			}
			if !got.Time.Equal(tc.expect.Time) {
				t.Errorf("Time: got %v, want %v", got.Time, tc.expect.Time)
			}
			if got.Price != tc.expect.Price {
				t.Errorf("Price: got %v, want %v", got.Price, tc.expect.Price)
			}
			if got.Volume != tc.expect.Volume {
				t.Errorf("Volume: got %v, want %v", got.Volume, tc.expect.Volume)
			}
			if got.ForeignTradeID != tc.expect.ForeignTradeID {
				t.Errorf("ForeignTradeID: got %v, want %v", got.ForeignTradeID, tc.expect.ForeignTradeID)
			}
		})
	}
}
