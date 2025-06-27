package scrapers

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/diadata-org/lumina-library/models"
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

func TestKrakenFetchTrades(t *testing.T) {
	mockWs := &mockWsConn{}
	tradesCh := make(chan models.Trade, 1)
	pairMap := map[string]models.Pair{
		"BTCUSD": {
			QuoteToken: models.Asset{Symbol: "BTC"},
			BaseToken:  models.Asset{Symbol: "USD"},
		},
	}

	// Simulate a trade message as received from Kraken.
	mockWs.readJSONQueue = []interface{}{
		krakenWSResponse{
			Channel: "trade",
			Data: []krakenWSResponseData{
				{
					Symbol:    "BTC/USD",
					Side:      "buy",
					Price:     55555.55,
					Size:      1.23,
					TradeID:   987654,
					Time:      time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
					OrderType: "market",
				},
			},
		},
	}

	scraper := &krakenScraper{
		wsClient:         mockWs,
		tradesChannel:    tradesCh,
		tickerPairMap:    pairMap,
		lastTradeTimeMap: map[string]time.Time{},
	}

	var lock sync.RWMutex

	go scraper.fetchTrades(&lock)

	select {
	case trade := <-tradesCh:
		if trade.Price != 55555.55 {
			t.Errorf("expected price 55555.55, got %v", trade.Price)
		}
		if trade.Volume != 1.23 {
			t.Errorf("expected volume 1.23, got %v", trade.Volume)
		}
		if trade.QuoteToken.Symbol != "BTC" || trade.BaseToken.Symbol != "USD" {
			t.Errorf("unexpected tokens: %+v, %+v", trade.QuoteToken, trade.BaseToken)
		}
		if trade.ForeignTradeID != "987654" {
			t.Errorf("unexpected ForeignTradeID: %v", trade.ForeignTradeID)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive trade")
	}
}
