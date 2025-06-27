package scrapers

import (
	"strconv"
	"sync"
	"testing"
	"time"

	models "github.com/diadata-org/lumina-library/models"
)

func TestBinanceParseWSResponse(t *testing.T) {
	cases := []struct {
		name   string
		input  binanceWSResponse
		expect models.Trade
	}{
		{
			name: "valid buy trade",
			input: binanceWSResponse{
				Timestamp:      1721923200000, // example ms value
				Price:          "40000.5",
				Volume:         "0.1",
				ForeignTradeID: 1234,
				ForeignName:    "BTCUSDT",
				Type:           "trade",
				Buy:            true,
			},
			expect: models.Trade{
				Exchange:       Exchanges[BINANCE_EXCHANGE],
				Time:           time.Unix(0, 1721923200000*1e6),
				Price:          40000.5,
				Volume:         0.1,
				ForeignTradeID: strconv.Itoa(1234),
			},
		},
		{
			name: "invalid price (should be 0)",
			input: binanceWSResponse{
				Timestamp:      1000000000000,
				Price:          "bad-price",
				Volume:         "2.1",
				ForeignTradeID: 42,
				ForeignName:    "XRPUSDT",
				Type:           "trade",
				Buy:            true,
			},
			expect: models.Trade{
				Exchange:       Exchanges[BINANCE_EXCHANGE],
				Time:           time.Unix(0, 1000000000000*1e6),
				Price:          0,
				Volume:         2.1,
				ForeignTradeID: strconv.Itoa(42),
			},
		},
		{
			name: "invalid volume (should be 0)",
			input: binanceWSResponse{
				Timestamp:      2000000000000,
				Price:          "12345.6",
				Volume:         "bad-volume",
				ForeignTradeID: 55,
				ForeignName:    "BNBUSDT",
				Type:           "trade",
				Buy:            true,
			},
			expect: models.Trade{
				Exchange:       Exchanges[BINANCE_EXCHANGE],
				Time:           time.Unix(0, 2000000000000*1e6),
				Price:          12345.6,
				Volume:         0,
				ForeignTradeID: strconv.Itoa(55),
			},
		},
		{
			name: "nil type (should skip, but parser doesn't check)",
			input: binanceWSResponse{
				Timestamp:      2000000000000,
				Price:          "12345.6",
				Volume:         "1.23",
				ForeignTradeID: 88,
				ForeignName:    "LTCUSDT",
				Type:           nil,
				Buy:            true,
			},
			expect: models.Trade{
				Exchange:       Exchanges[BINANCE_EXCHANGE],
				Time:           time.Unix(0, 2000000000000*1e6),
				Price:          12345.6,
				Volume:         1.23,
				ForeignTradeID: strconv.Itoa(88),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := binanceParseWSResponse(tc.input)
			// Compare main fields
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

func TestSubscribe(t *testing.T) {
	lock := &sync.RWMutex{}
	mockWs := &mockWsConn{}
	scraper := &binanceScraper{
		wsClient: mockWs,
	}
	pair := models.ExchangePair{
		ForeignName: "BTC-USDT",
	}
	// Test subscribe
	err := scraper.subscribe(pair, true, lock)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(mockWs.writeJSONCalls) != 1 {
		t.Errorf("expected WriteJSON to be called once")
	}
	// Test unsubscribe
	err = scraper.subscribe(pair, false, lock)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(mockWs.writeJSONCalls) != 2 {
		t.Errorf("expected WriteJSON to be called twice")
	}
}

func TestFetchTrades(t *testing.T) {
	// Prepare the mock websocket with a valid trade message
	mockWs := &mockWsConn{
		readJSONQueue: []interface{}{
			binanceWSResponse{
				Timestamp:      1620000000000, // ms timestamp
				Price:          "123.45",
				Volume:         "6.78",
				ForeignTradeID: 99,
				ForeignName:    "BTCUSDT",
				Type:           "trade", // just needs to be non-nil
				Buy:            true,
			},
		},
	}

	// Add a dummy mapping for tickerPairMap
	pair := models.Pair{}
	tickerMap := map[string]models.Pair{
		"BTCUSDT": pair,
	}

	scraper := &binanceScraper{
		wsClient:         mockWs,
		tradesChannel:    make(chan models.Trade, 1),
		tickerPairMap:    tickerMap,
		lastTradeTimeMap: make(map[string]time.Time),
	}

	var lock sync.RWMutex
	go scraper.fetchTrades(&lock)

	select {
	case trade := <-scraper.tradesChannel:
		// Assert the main trade fields
		if trade.Price != 123.45 {
			t.Errorf("expected price 123.45, got %v", trade.Price)
		}
		if trade.Volume != 6.78 {
			t.Errorf("expected volume 6.78, got %v", trade.Volume)
		}
		if trade.ForeignTradeID != "99" {
			t.Errorf("expected ForeignTradeID '99', got %v", trade.ForeignTradeID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected trade, got none")
	}
}
