package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func TestDecimalChangeLimitsPercentagePrecision(t *testing.T) {
	change, rate := decimalChange("63533.2", "64339.1")
	if change != "-805.9" || rate != "-1.252582" {
		t.Fatalf("unexpected decimal change: change=%s rate=%s", change, rate)
	}
}

func TestBinancePublicMarketData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/ticker/24hr":
			if r.URL.Query().Get("symbol") == "BTCUSDT" {
				_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","lastPrice":"65000.10","openPrice":"64000","highPrice":"66000","lowPrice":"63000","priceChange":"1000.10","priceChangePercent":"1.562","volume":"12.3000","quoteVolume":"800000","closeTime":1720000000000}`))
				return
			}
			if r.URL.Query().Get("type") != "MINI" {
				t.Fatalf("unexpected Binance ticker query %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT"},{"symbol":"BTCFDUSD"}]`))
		case "/api/v3/klines":
			_, _ = w.Write([]byte(`[[1720000000000,"64000","66000","63000","65000.10","12.3",1720000059999,"800000",100,"6","400000","0"]]`))
		case "/api/v3/exchangeInfo":
			_, _ = w.Write([]byte(`{"symbols":[{"symbol":"BTCUSDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.10"},{"filterType":"LOT_SIZE","minQty":"0.001","stepSize":"0.001"},{"filterType":"NOTIONAL","minNotional":"10"}]}]}`))
		default:
			t.Fatalf("unexpected Binance market path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewMarketData(server.Client())
	client.binanceSpotURL = server.URL
	instruments, err := client.Instruments(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeSpot)
	if err != nil || len(instruments) != 1 || instruments[0].Symbol != "BTCUSDT" {
		t.Fatalf("unexpected Binance instruments: %#v, err=%v", instruments, err)
	}
	ticker, err := client.Ticker(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeSpot, "BTCUSDT")
	if err != nil || ticker.LastPrice != "65000.10" || ticker.PriceChangeRate != "1.562" {
		t.Fatalf("unexpected Binance ticker: %#v, err=%v", ticker, err)
	}
	klines, err := client.Klines(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeSpot, "BTCUSDT", "1m", 1)
	if err != nil || len(klines) != 1 || klines[0].Close != "65000.10" || klines[0].Time != 1720000000 {
		t.Fatalf("unexpected Binance klines: %#v, err=%v", klines, err)
	}
	rules, err := client.InstrumentRules(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeSpot, "BTCUSDT")
	if err != nil || rules.StepSize != "0.001" || rules.TickSize != "0.10" || rules.MinNotional != "10" {
		t.Fatalf("unexpected Binance rules: %#v, err=%v", rules, err)
	}
}

func TestOKXPublicMarketData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			if r.URL.Query().Get("instId") != "" {
				_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","minSz":"1","lotSz":"1","tickSz":"0.1"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","state":"live","settleCcy":"USDT"},{"instId":"BTC-USD-SWAP","state":"live","settleCcy":"USD"}]}`))
		case "/api/v5/market/ticker":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","last":"102","open24h":"100","high24h":"104","low24h":"98","vol24h":"10","volCcy24h":"1000","volCcyQuote24h":"102000","ts":"1720000000000"}]}`))
		case "/api/v5/market/candles":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[["1720000060000","102","103","101","102.5","2","205","205","0"],["1720000000000","100","103","99","102","3","306","306","1"]]}`))
		default:
			t.Fatalf("unexpected OKX market path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewMarketData(server.Client())
	client.okxURL = server.URL
	instruments, err := client.Instruments(context.Background(), schema.MarketExchangeOKX, schema.MarketTypeFutures)
	if err != nil || len(instruments) != 1 || instruments[0].BaseAsset != "BTC" {
		t.Fatalf("unexpected OKX instruments: %#v, err=%v", instruments, err)
	}
	ticker, err := client.Ticker(context.Background(), schema.MarketExchangeOKX, schema.MarketTypeFutures, "BTC-USDT-SWAP")
	if err != nil || ticker.PriceChange != "2" || ticker.PriceChangeRate != "2" || ticker.BaseVolume != "1000" || ticker.QuoteVolume != "102000" {
		t.Fatalf("unexpected OKX ticker: %#v, err=%v", ticker, err)
	}
	klines, err := client.Klines(context.Background(), schema.MarketExchangeOKX, schema.MarketTypeFutures, "BTC-USDT-SWAP", "1m", 2)
	if err != nil || len(klines) != 2 || klines[0].Time != 1720000000 || !klines[0].Closed || klines[1].Closed {
		t.Fatalf("unexpected OKX klines: %#v, err=%v", klines, err)
	}
	rules, err := client.InstrumentRules(context.Background(), schema.MarketExchangeOKX, schema.MarketTypeFutures, "BTC-USDT-SWAP")
	if err != nil || rules.MinQuantity != "1" || rules.TickSize != "0.1" {
		t.Fatalf("unexpected OKX rules: %#v, err=%v", rules, err)
	}
}

func TestBinanceFuturesMarketContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/ticker/24hr":
			_, _ = w.Write([]byte(`{"symbol":"BTCUSDT","lastPrice":"100","openPrice":"99","highPrice":"102","lowPrice":"98","priceChange":"1","priceChangePercent":"1.01","volume":"100","quoteVolume":"10000","closeTime":1720000000000}`))
		case "/fapi/v1/depth":
			_, _ = w.Write([]byte(`{"lastUpdateId":1,"bids":[["99.9","8"],["99.8","2"]],"asks":[["100.1","2"],["100.2","3"]]}`))
		case "/fapi/v1/trades":
			_, _ = w.Write([]byte(`[{"qty":"3","isBuyerMaker":false},{"qty":"1","isBuyerMaker":true}]`))
		case "/fapi/v1/openInterest":
			_, _ = w.Write([]byte(`{"openInterest":"12.5"}`))
		case "/fapi/v1/premiumIndex":
			_, _ = w.Write([]byte(`{"markPrice":"100","lastFundingRate":"0.0005","nextFundingTime":1720003600000}`))
		default:
			t.Fatalf("unexpected Binance context path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewMarketData(server.Client())
	client.binanceFuturesURL = server.URL
	result, err := client.Context(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeFutures, "BTCUSDT", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderBook.SpreadBPS <= 0 || result.OrderBook.Imbalance <= 0 {
		t.Fatalf("unexpected order book indicators: %#v", result.OrderBook)
	}
	if result.Derivatives.OpenInterest != "12.5" || result.Derivatives.OpenInterestValue != "1250" || result.Derivatives.FundingRate != "0.0005" {
		t.Fatalf("unexpected derivatives: %#v", result.Derivatives)
	}
	if !result.Derivatives.FlowIsProxy || result.Derivatives.TakerBuySellRatio != 3 {
		t.Fatalf("unexpected taker flow: %#v", result.Derivatives)
	}
	if result.RiskRadar.FundingRatePct != 0.05 || result.RiskRadar.CrowdingRisk != "medium" {
		t.Fatalf("unexpected risk radar: %#v", result.RiskRadar)
	}
}

func TestOKXFuturesMarketContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v5/market/ticker":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","last":"100","open24h":"99","high24h":"102","low24h":"98","vol24h":"100","volCcy24h":"100","volCcyQuote24h":"10000","ts":"1720000000000"}]}`))
		case "/api/v5/market/books":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"bids":[["99.9","4","0","1"]],"asks":[["100.1","6","0","1"]],"ts":"1720000000000"}]}`))
		case "/api/v5/market/trades":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"side":"buy","sz":"2"},{"side":"sell","sz":"4"}]}`))
		case "/api/v5/public/open-interest":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"oi":"50","oiCcy":"5000"}]}`))
		case "/api/v5/public/funding-rate":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"fundingRate":"-0.0002","nextFundingTime":"1720003600000"}]}`))
		default:
			t.Fatalf("unexpected OKX context path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewMarketData(server.Client())
	client.okxURL = server.URL
	result, err := client.Context(context.Background(), schema.MarketExchangeOKX, schema.MarketTypeFutures, "BTC-USDT-SWAP", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Derivatives.OpenInterest != "50" || result.Derivatives.OpenInterestValue != "5000" || result.Derivatives.TakerBuySellRatio != 0.5 {
		t.Fatalf("unexpected OKX derivatives: %#v", result.Derivatives)
	}
	if result.OrderBook.Imbalance >= 0 || result.RiskRadar.RiskScore < 0 {
		t.Fatalf("unexpected OKX context: %#v", result)
	}
}
