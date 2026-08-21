package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func TestBinanceFuturesTradingSignsPrimaryAndProtectionOrders(t *testing.T) {
	paths := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if request.Header.Get("X-MBX-APIKEY") != "key" {
			t.Fatal("missing Binance API key header")
		}
		query := request.URL.Query()
		signature := query.Get("signature")
		query.Del("signature")
		mac := hmac.New(sha256.New, []byte("secret"))
		_, _ = mac.Write([]byte(query.Encode()))
		if signature != hex.EncodeToString(mac.Sum(nil)) {
			t.Fatalf("invalid Binance signature")
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/fapi/v1/order" {
			_, _ = writer.Write([]byte(`{"orderId":123,"clientOrderId":"client-a","status":"FILLED","executedQty":"0.01","avgPrice":"100"}`))
		} else {
			_, _ = writer.Write([]byte(`{"algoId":456,"algoStatus":"NEW"}`))
		}
	}))
	defer server.Close()
	client := NewBinance(Credentials{APIKey: "key", SecretKey: "secret", Testnet: true,
		AccountType: BinanceAccountTypeFutures}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.UnixMilli(1234567890000) }
	result, err := client.PlaceOrder(context.Background(), TradingOrderRequest{ClientOrderID: "client-a",
		Market: "futures", Symbol: "BTCUSDT", Side: "buy", PositionSide: "long", OrderType: "market",
		Quantity: "0.01", StopLoss: "90", TakeProfit: "120"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExchangeOrderID != "123" || result.Status != "FILLED" || len(paths) != 3 ||
		paths[0] != "/fapi/v1/order" || paths[1] != "/fapi/v1/algoOrder" || paths[2] != "/fapi/v1/algoOrder" {
		t.Fatalf("unexpected Binance trading result=%#v paths=%#v", result, paths)
	}
}

func TestBinanceSpotProtectedEntryRequiresAtomicLimitOrder(t *testing.T) {
	client := NewBinance(Credentials{AccountType: BinanceAccountTypeSpot}, http.DefaultClient)
	_, err := client.PlaceOrder(context.Background(), TradingOrderRequest{Market: "spot", Symbol: "BTCUSDT",
		Side: "buy", OrderType: "market", Quantity: "0.01", StopLoss: "90", TakeProfit: "120"})
	if err == nil || !strings.Contains(err.Error(), "OTOCO limit") {
		t.Fatalf("expected protected spot market rejection, got %v", err)
	}
}

func TestOKXDemoTradingSignsOrderWithAttachedProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.URL.Path != "/api/v5/trade/order" || request.Header.Get("OK-ACCESS-KEY") != "key" ||
			request.Header.Get("OK-ACCESS-PASSPHRASE") != "pass" || request.Header.Get("x-simulated-trading") != "1" ||
			request.Header.Get("OK-ACCESS-SIGN") == "" {
			t.Fatalf("unexpected OKX request path=%s headers=%v", request.URL.Path, request.Header)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["instId"] != "BTC-USDT-SWAP" || payload["tdMode"] != "cross" || payload["attachAlgoOrds"] == nil {
			t.Fatalf("unexpected OKX order payload: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":"0","msg":"","data":[{"ordId":"789","clOrdId":"client-b","sCode":"0","sMsg":""}]}`))
	}))
	defer server.Close()
	client := NewOKX(Credentials{APIKey: "key", SecretKey: "secret", Passphrase: "pass", Testnet: true}, server.Client())
	client.baseURL = server.URL
	result, err := client.PlaceOrder(context.Background(), TradingOrderRequest{ClientOrderID: "client-b",
		Market: "futures", Symbol: "BTCUSDT", Side: "buy", PositionSide: "long", OrderType: "market",
		Quantity: "1", StopLoss: "90", TakeProfit: "120"})
	if err != nil || result.ExchangeOrderID != "789" || result.Status != "accepted" {
		t.Fatalf("unexpected OKX result=%#v err=%v", result, err)
	}
}

func TestOKXTradingInstrument(t *testing.T) {
	for input, want := range map[string]string{"BTCUSDT": "BTC-USDT-SWAP", "BTC-USDT": "BTC-USDT-SWAP",
		"BTC-USDT-SWAP": "BTC-USDT-SWAP"} {
		if got := okxTradingInstrument(input, "futures"); got != want {
			t.Fatalf("symbol %s: got %s want %s", input, got, want)
		}
	}
}

func TestBuildTradingRequiresGlobalGateAndTestnet(t *testing.T) {
	previousEnabled, previousTestnetOnly := config.C.Trading.Execution.Enabled, config.C.Trading.Execution.TestnetOnly
	t.Cleanup(func() {
		config.C.Trading.Execution.Enabled, config.C.Trading.Execution.TestnetOnly = previousEnabled, previousTestnetOnly
	})
	service := credential.NewService(make([]byte, 32), nil)
	factory := NewFactory(service)
	exchange := &schema.Exchange{ID: "exchange-a", OwnerUserID: "owner-a", ExchangeType: "binance",
		AccountType: "spot", Testnet: false}
	apiKey, _ := service.EncryptField("key", exchange.OwnerUserID, exchange.ID, "api_key")
	secret, _ := service.EncryptField("secret", exchange.OwnerUserID, exchange.ID, "secret_key")
	exchange.APIKeyCiphertext, exchange.SecretKeyCiphertext = apiKey, secret
	config.C.Trading.Execution.Enabled, config.C.Trading.Execution.TestnetOnly = false, true
	if _, err := factory.BuildTrading(exchange); err != ErrTradingProviderDisabled {
		t.Fatalf("disabled global gate must reject provider, got %v", err)
	}
	config.C.Trading.Execution.Enabled = true
	if _, err := factory.BuildTrading(exchange); err == nil || !strings.Contains(err.Error(), "restricted to testnet") {
		t.Fatalf("production exchange must be rejected by testnet-only policy, got %v", err)
	}
	exchange.Testnet = true
	if item, err := factory.BuildTrading(exchange); err != nil || item.Name() != "binance" {
		t.Fatalf("testnet Binance provider was not built: item=%#v err=%v", item, err)
	}
}
