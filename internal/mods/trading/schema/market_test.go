package schema

import "testing"

func TestMarketKlinesQueryValidate(t *testing.T) {
	params := &MarketKlinesQuery{
		MarketSymbolQuery: MarketSymbolQuery{Exchange: " OKX ", Market: "FUTURES", Symbol: "btc-usdt-swap"},
		Interval:          "1H",
	}
	if err := params.Validate(); err != nil {
		t.Fatal(err)
	}
	if params.Exchange != "okx" || params.Market != "futures" || params.Symbol != "BTC-USDT-SWAP" || params.Interval != "1h" || params.Limit != 300 {
		t.Fatalf("unexpected normalized query: %#v", params)
	}
}

func TestMarketKlinesQueryRejectsUnsupportedInterval(t *testing.T) {
	params := &MarketKlinesQuery{
		MarketSymbolQuery: MarketSymbolQuery{Exchange: "binance", Market: "spot", Symbol: "BTCUSDT"},
		Interval:          "2h",
	}
	if err := params.Validate(); err == nil {
		t.Fatal("expected unsupported interval error")
	}
}
