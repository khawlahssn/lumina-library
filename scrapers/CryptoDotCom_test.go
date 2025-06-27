package scrapers

import (
	"testing"
	"time"

	models "github.com/diadata-org/lumina-library/models"
)

func TestCryptodotcomParseTradeMessage(t *testing.T) {
	cases := []struct {
		name   string
		input  cryptodotcomWSResponse
		expect []models.Trade
	}{
		{
			name: "single valid trade",
			input: cryptodotcomWSResponse{
				Result: cryptodotcomWSResponseResult{
					Data: []cryptodotcomWSResponseData{
						{
							TradeID:   "abc123",
							Timestamp: 1721923200000,
							Price:     "12345.67",
							Volume:    "0.01",
							Side:      "BUY",
						},
					},
				},
			},
			expect: []models.Trade{
				{
					Price:          12345.67,
					Volume:         0.01,
					Time:           time.Unix(0, 1721923200000*1e6),
					Exchange:       Exchanges[CRYPTODOTCOM_EXCHANGE],
					ForeignTradeID: "abc123",
				},
			},
		},
		{
			name: "invalid price (should return no trades)",
			input: cryptodotcomWSResponse{
				Result: cryptodotcomWSResponseResult{
					Data: []cryptodotcomWSResponseData{
						{
							TradeID:   "badprice",
							Timestamp: 1721923200000,
							Price:     "not-a-number",
							Volume:    "0.01",
							Side:      "BUY",
						},
					},
				},
			},
			expect: nil,
		},
		{
			name: "invalid volume (should return no trades)",
			input: cryptodotcomWSResponse{
				Result: cryptodotcomWSResponseResult{
					Data: []cryptodotcomWSResponseData{
						{
							TradeID:   "badvol",
							Timestamp: 1721923200000,
							Price:     "222.2",
							Volume:    "not-a-number",
							Side:      "BUY",
						},
					},
				},
			},
			expect: nil,
		},
		{
			name: "multiple valid trades",
			input: cryptodotcomWSResponse{
				Result: cryptodotcomWSResponseResult{
					Data: []cryptodotcomWSResponseData{
						{
							TradeID:   "one",
							Timestamp: 1000000000000,
							Price:     "10",
							Volume:    "1",
							Side:      "BUY",
						},
						{
							TradeID:   "two",
							Timestamp: 2000000000000,
							Price:     "20",
							Volume:    "2",
							Side:      "BUY",
						},
					},
				},
			},
			expect: []models.Trade{
				{
					Price:          10,
					Volume:         1,
					Time:           time.Unix(0, 1000000000000*1e6),
					Exchange:       Exchanges[CRYPTODOTCOM_EXCHANGE],
					ForeignTradeID: "one",
				},
				{
					Price:          20,
					Volume:         2,
					Time:           time.Unix(0, 2000000000000*1e6),
					Exchange:       Exchanges[CRYPTODOTCOM_EXCHANGE],
					ForeignTradeID: "two",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := cryptodotcomParseTradeMessage(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %d trades, got %d", len(tc.expect), len(got))
			}
			for i := range got {
				if got[i].Exchange != tc.expect[i].Exchange {
					t.Errorf("Exchange: got %v, want %v", got[i].Exchange, tc.expect[i].Exchange)
				}
				if !got[i].Time.Equal(tc.expect[i].Time) {
					t.Errorf("Time: got %v, want %v", got[i].Time, tc.expect[i].Time)
				}
				if got[i].Price != tc.expect[i].Price {
					t.Errorf("Price: got %v, want %v", got[i].Price, tc.expect[i].Price)
				}
				if got[i].Volume != tc.expect[i].Volume {
					t.Errorf("Volume: got %v, want %v", got[i].Volume, tc.expect[i].Volume)
				}
				if got[i].ForeignTradeID != tc.expect[i].ForeignTradeID {
					t.Errorf("ForeignTradeID: got %v, want %v", got[i].ForeignTradeID, tc.expect[i].ForeignTradeID)
				}
			}
		})
	}
}
