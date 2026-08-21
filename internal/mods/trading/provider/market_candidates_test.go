package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBinancePublicCandidateRankings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/fapi/v1/ticker/24hr":
			_, _ = fmt.Fprint(w, `[{"symbol":"ETHUSDT","quoteVolume":"200"},{"symbol":"BTCUSDT","quoteVolume":"300"},{"symbol":"BNBUSDT","quoteVolume":"100"}]`)
		case "/fapi/v1/openInterest":
			values := map[string]string{"BTCUSDT": "10", "ETHUSDT": "30", "BNBUSDT": "20"}
			symbol := request.URL.Query().Get("symbol")
			_, _ = fmt.Fprintf(w, `{"symbol":%q,"openInterest":%q}`, symbol, values[symbol])
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	market := &MarketData{client: server.Client(), binanceSpotURL: server.URL, binanceFuturesURL: server.URL, okxURL: server.URL}
	liquidity, err := market.LiquidityCandidates(context.Background(), "binance", "futures", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(liquidity) != 2 || liquidity[0] != "BTCUSDT" || liquidity[1] != "ETHUSDT" {
		t.Fatalf("unexpected liquidity ranking: %v", liquidity)
	}
	oi, err := market.OpenInterestCandidates(context.Background(), "binance", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(oi) != 2 || oi[0] != "ETHUSDT" || oi[1] != "BNBUSDT" {
		t.Fatalf("unexpected OI ranking: %v", oi)
	}
}

func TestOKXPublicCandidateRankingsNormalizeSymbols(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v5/market/tickers":
			_, _ = fmt.Fprint(w, `{"code":"0","data":[{"instId":"BTC-USDT-SWAP","volCcy24h":"100"},{"instId":"ETH-USDT-SWAP","volCcy24h":"200"}]}`)
		case "/api/v5/public/open-interest":
			_, _ = fmt.Fprint(w, `{"code":"0","data":[{"instId":"BTC-USDT-SWAP","oi":"10","oiCcy":"15"},{"instId":"ETH-USDT-SWAP","oi":"20","oiCcy":"5"}]}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	market := &MarketData{client: server.Client(), binanceSpotURL: server.URL, binanceFuturesURL: server.URL, okxURL: server.URL}
	liquidity, err := market.LiquidityCandidates(context.Background(), "okx", "futures", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(liquidity) != 2 || liquidity[0] != "ETHUSDT" || liquidity[1] != "BTCUSDT" {
		t.Fatalf("unexpected OKX liquidity ranking: %v", liquidity)
	}
	oi, err := market.OpenInterestCandidates(context.Background(), "okx", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(oi) != 2 || oi[0] != "BTCUSDT" || oi[1] != "ETHUSDT" {
		t.Fatalf("unexpected OKX OI ranking: %v", oi)
	}
}
