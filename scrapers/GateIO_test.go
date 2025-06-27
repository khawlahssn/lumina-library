package scrapers

import (
	"strconv"
	"testing"
	"time"

	models "github.com/diadata-org/lumina-library/models"
)

var dummyPair = models.Pair{
	QuoteToken: models.Asset{Symbol: "USDT", Blockchain: "Ethereum", Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7"},
	BaseToken:  models.Asset{Symbol: "BTC", Blockchain: "Bitcoin", Address: "0x0000000000000000000000000000000000000000"},
}

func TestHandleWSResponse_GateIO(t *testing.T) {
	scraper := &gateIOScraper{
		tickerPairMap: map[string]models.Pair{
			"BTCUSDT": dummyPair,
		},
	}

	cases := []struct {
		name   string
		input  GateIOResponseTrade
		expect models.Trade
	}{
		{
			name: "valid buy trade",
			input: GateIOResponseTrade{
				Result: struct {
					ID           int    `json:"id"`
					CreateTime   int    `json:"create_time"`
					CreateTimeMs string `json:"create_time_ms"`
					Side         string `json:"side"`
					CurrencyPair string `json:"currency_pair"`
					Amount       string `json:"amount"`
					Price        string `json:"price"`
				}{
					ID:           123456,
					CreateTime:   1721923200,
					CreateTimeMs: "1721923200000",
					Side:         "buy",
					CurrencyPair: "BTC_USDT",
					Amount:       "0.123",
					Price:        "42000.99",
				},
			},
			expect: models.Trade{
				QuoteToken:     dummyPair.QuoteToken,
				BaseToken:      dummyPair.BaseToken,
				Price:          42000.99,
				Volume:         0.123,
				Time:           time.Unix(1721923200, 0),
				Exchange:       Exchanges[GATEIO_EXCHANGE],
				ForeignTradeID: strconv.FormatInt(123456, 16),
			},
		},
		{
			name: "invalid price (should return zero trade)",
			input: GateIOResponseTrade{
				Result: struct {
					ID           int    `json:"id"`
					CreateTime   int    `json:"create_time"`
					CreateTimeMs string `json:"create_time_ms"`
					Side         string `json:"side"`
					CurrencyPair string `json:"currency_pair"`
					Amount       string `json:"amount"`
					Price        string `json:"price"`
				}{
					ID:           77,
					CreateTime:   1000000000,
					CreateTimeMs: "1000000000000",
					Side:         "buy",
					CurrencyPair: "BTC_USDT",
					Amount:       "0.5",
					Price:        "bad-price",
				},
			},
			expect: models.Trade{}, // zero trade on error
		},
		{
			name: "invalid amount (should return zero trade)",
			input: GateIOResponseTrade{
				Result: struct {
					ID           int    `json:"id"`
					CreateTime   int    `json:"create_time"`
					CreateTimeMs string `json:"create_time_ms"`
					Side         string `json:"side"`
					CurrencyPair string `json:"currency_pair"`
					Amount       string `json:"amount"`
					Price        string `json:"price"`
				}{
					ID:           88,
					CreateTime:   2000000000,
					CreateTimeMs: "2000000000000",
					Side:         "buy",
					CurrencyPair: "BTC_USDT",
					Amount:       "bad-amount",
					Price:        "1000.99",
				},
			},
			expect: models.Trade{}, // zero trade on error
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scraper.handleWSResponse(tc.input)
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
			if got.QuoteToken != tc.expect.QuoteToken {
				t.Errorf("QuoteToken: got %+v, want %+v", got.QuoteToken, tc.expect.QuoteToken)
			}
			if got.BaseToken != tc.expect.BaseToken {
				t.Errorf("BaseToken: got %+v, want %+v", got.BaseToken, tc.expect.BaseToken)
			}
		})
	}
}
