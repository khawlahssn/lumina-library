package scrapers

import (
	"strconv"
	"sync"
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

func TestCoinbaseFetchTrades(t *testing.T) {
	// Prepare a valid trade message
	mockWs := &mockWsConn{
		readJSONQueue: []interface{}{
			coinBaseWSResponse{
				Type:      "match",
				TradeID:   123,
				Price:     "2567.01",
				Size:      "0.55",
				Time:      "2024-07-01T10:01:00.000000Z",
				ProductID: "BTC-USD",
				Side:      "buy",
			},
		},
	}

	// If handleWSResponse expects pair["BTCUSD"] etc., add it
	pair := models.Pair{}
	tickerMap := map[string]models.Pair{
		"BTCUSD": pair,
	}

	scraper := &coinbaseScraper{
		wsClient:         mockWs,
		tradesChannel:    make(chan models.Trade, 1),
		tickerPairMap:    tickerMap,
		lastTradeTimeMap: make(map[string]time.Time),
	}

	var lock sync.RWMutex
	go scraper.fetchTrades(&lock)

	select {
	case trade := <-scraper.tradesChannel:
		if trade.Price != 2567.01 {
			t.Errorf("expected price 2567.01, got %v", trade.Price)
		}
		if trade.Volume != 0.55 {
			t.Errorf("expected volume 0.55, got %v", trade.Volume)
		}
		if trade.ForeignTradeID != "123" {
			t.Errorf("expected ForeignTradeID '123', got %v", trade.ForeignTradeID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected trade, got none")
	}
}
