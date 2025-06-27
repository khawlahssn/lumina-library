package scrapers

import (
	"strconv"
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
