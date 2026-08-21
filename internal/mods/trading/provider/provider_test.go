package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func TestFactoryRejectsIncompleteCredentials(t *testing.T) {
	factory := NewFactory(credential.NewService(make([]byte, 32), nil))
	_, err := factory.Build(&schema.Exchange{ID: "exchange-1", OwnerUserID: "user-1", ExchangeType: "binance"})
	if !errors.Is(err, ErrCredentialsIncomplete) {
		t.Fatalf("expected incomplete credentials error, got %v", err)
	}
}

func TestBinanceSnapshotIsReadOnlyAndNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
			t.Fatal("missing Binance signature")
		}
		switch r.URL.Path {
		case "/fapi/v2/account":
			_, _ = w.Write([]byte(`{"totalMarginBalance":"110","totalWalletBalance":"100","availableBalance":"80","totalUnrealizedProfit":"10"}`))
		case "/fapi/v2/positionRisk":
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","positionSide":"LONG","positionAmt":"0.1","entryPrice":"100","markPrice":"110","unRealizedProfit":"1","liquidationPrice":"50","leverage":"5"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewBinance(Credentials{APIKey: "key", SecretKey: "secret"}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	account, positions, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.TotalEquity != "110" || len(positions) != 1 || positions[0].Side != "long" {
		t.Fatalf("unexpected snapshot: %#v %#v", account, positions)
	}
}

func TestNewBinanceSelectsEndpointByAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		testnet     bool
		baseURL     string
	}{
		{name: "spot production", accountType: BinanceAccountTypeSpot, baseURL: "https://api.binance.com"},
		{name: "spot testnet", accountType: BinanceAccountTypeSpot, testnet: true, baseURL: "https://testnet.binance.vision"},
		{name: "futures production", accountType: BinanceAccountTypeFutures, baseURL: "https://fapi.binance.com"},
		{name: "futures testnet", accountType: BinanceAccountTypeFutures, testnet: true, baseURL: "https://testnet.binancefuture.com"},
		{name: "unified production", accountType: BinanceAccountTypeUnified, baseURL: "https://papi.binance.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewBinance(Credentials{AccountType: tt.accountType, Testnet: tt.testnet}, http.DefaultClient)
			if client.baseURL != tt.baseURL || client.accountType != tt.accountType {
				t.Fatalf("unexpected Binance client: baseURL=%q accountType=%q", client.baseURL, client.accountType)
			}
		})
	}
}

func TestBinanceHealthUsesPublicEndpointForAccountType(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		path        string
	}{
		{name: "spot", accountType: BinanceAccountTypeSpot, path: "/api/v3/ping"},
		{name: "futures", accountType: BinanceAccountTypeFutures, path: "/fapi/v1/ping"},
		{name: "unified", accountType: BinanceAccountTypeUnified, path: "/papi/v1/ping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Fatalf("unexpected Binance health request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("X-MBX-APIKEY") != "" || r.URL.Query().Get("signature") != "" {
					t.Fatal("Binance health request must not include credentials")
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			client := NewBinance(Credentials{AccountType: tt.accountType}, server.Client())
			client.baseURL = server.URL
			if err := client.Health(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBinanceValidateCredentialsUsesSignedReadOnlyAccountEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		path        string
	}{
		{name: "spot", accountType: BinanceAccountTypeSpot, path: "/api/v3/account"},
		{name: "futures", accountType: BinanceAccountTypeFutures, path: "/fapi/v2/account"},
		{name: "unified", accountType: BinanceAccountTypeUnified, path: "/papi/v1/balance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Fatalf("unexpected Binance credential request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
					t.Fatal("Binance credential request must be signed")
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			client := NewBinance(Credentials{
				APIKey: "key", SecretKey: "secret", AccountType: tt.accountType,
			}, server.Client())
			client.baseURL = server.URL
			client.now = func() time.Time { return time.Unix(1, 0) }
			if err := client.ValidateCredentials(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBinanceSignedGetDoesNotReuseOrMutateSignature(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		query := r.URL.Query()
		signature, err := hex.DecodeString(query.Get("signature"))
		if err != nil {
			t.Fatalf("invalid signature encoding: %v", err)
		}
		query.Del("signature")
		mac := hmac.New(sha256.New, []byte("secret"))
		_, _ = mac.Write([]byte(query.Encode()))
		if !hmac.Equal(signature, mac.Sum(nil)) {
			t.Fatalf("signature mismatch for request %d: %s", requests, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewBinance(Credentials{APIKey: "key", SecretKey: "secret", AccountType: BinanceAccountTypeSpot}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	query := url.Values{"symbol": {"BTCUSDT"}}
	for range 2 {
		var response map[string]interface{}
		if err := client.signedGet(context.Background(), "/api/v3/myTrades", query, &response); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 2 || query.Get("signature") != "" || query.Get("timestamp") != "" || query.Get("recvWindow") != "" {
		t.Fatalf("signedGet mutated caller query after %d requests: %s", requests, query.Encode())
	}
}

func TestBinanceSpotSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/account":
			if r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
				t.Fatal("missing Binance spot signature")
			}
			if r.URL.Query().Get("omitZeroBalances") != "true" {
				t.Fatalf("missing omitZeroBalances: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"balances":[{"asset":"USDT","free":"10","locked":"2"},{"asset":"BTC","free":"0.1","locked":"0"},{"asset":"LDBTC","free":"0.2","locked":"0"},{"asset":"LDDOGE","free":"3","locked":"0"},{"asset":"UNKNOWN","free":"2","locked":"0"}]}`))
		case "/api/v3/ticker/price":
			if r.URL.Query().Get("signature") != "" {
				t.Fatal("public Binance ticker request must not be signed")
			}
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","price":"100"},{"symbol":"DOGEUSDT","price":"2"},{"symbol":"BNBUSDT","price":"10"},{"symbol":"SOLUSDT","price":"5"}]`))
		case "/sapi/v1/simple-earn/flexible/position":
			if r.Method != http.MethodGet || r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
				t.Fatal("Binance Simple Earn flexible request must be a signed GET")
			}
			_, _ = w.Write([]byte(`{"rows":[{"asset":"BTC","totalAmount":"0.25"},{"asset":"DOGE","totalAmount":"4"},{"asset":"BNB","totalAmount":"2"}],"total":3}`))
		case "/sapi/v1/simple-earn/locked/position":
			if r.Method != http.MethodGet || r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
				t.Fatal("Binance Simple Earn locked request must be a signed GET")
			}
			_, _ = w.Write([]byte(`{"rows":[{"asset":"SOL","amount":"3"}],"total":1}`))
		case "/sapi/v1/asset/get-funding-asset":
			if r.Method != http.MethodPost || r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
				t.Fatal("Binance Funding Wallet request must be a signed POST")
			}
			_, _ = w.Write([]byte(`[{"asset":"USDT","free":"1","locked":"0","freeze":"0","withdrawing":"0"}]`))
		default:
			t.Fatalf("unexpected Binance spot path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewBinance(Credentials{
		APIKey: "key", SecretKey: "secret", AccountType: BinanceAccountTypeSpot,
	}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	account, positions, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.TotalEquity != "91" || account.AvailableBalance != "21" || len(account.Assets) != 6 || len(positions) != 0 {
		t.Fatalf("unexpected spot snapshot: %#v %#v", account, positions)
	}
	if account.Assets[0].Asset != "BTC" || account.Assets[0].Total != "0.35" || account.Assets[0].Locked != "0.25" || account.Assets[0].ValueUSDT != "35" {
		t.Fatalf("unexpected merged Simple Earn BTC valuation: %#v", account.Assets[0])
	}
	for _, asset := range account.Assets {
		if asset.Asset == "LDBTC" || asset.Asset == "LDDOGE" {
			t.Fatalf("LD collateral must be merged into its underlying asset: %#v", asset)
		}
	}
	if account.Assets[5].Asset != "UNKNOWN" || account.Assets[5].ValueUSDT != "0" {
		t.Fatalf("unknown assets must be retained without blocking the snapshot: %#v", account.Assets[5])
	}
	if _, ok := client.symbols["BTCUSDT"]; !ok {
		t.Fatal("expected BTCUSDT history symbol")
	}
	if _, ok := client.symbols["DOGEUSDT"]; !ok {
		t.Fatal("expected DOGEUSDT history symbol derived from LDDOGE")
	}
}

func TestBinanceUnifiedSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
			t.Fatal("missing Binance Portfolio Margin signature")
		}
		switch r.URL.Path {
		case "/papi/v1/account":
			_, _ = w.Write([]byte(`{"accountEquity":"120","actualEquity":"125","totalAvailableBalance":"","virtualMaxWithdrawAmount":"90"}`))
		case "/papi/v1/balance":
			_, _ = w.Write([]byte(`[{"asset":"USDT","totalWalletBalance":"125","crossMarginBorrowed":"2","crossMarginFree":"90","crossMarginInterest":"0.1","crossMarginLocked":"5","umUnrealizedPNL":"1.25","cmUnrealizedPNL":"0.25"}]`))
		case "/papi/v1/um/account":
			_, _ = w.Write([]byte(`{"positions":[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.2","entryPrice":"100","markPrice":"95","unrealizedProfit":"1.5","liquidationPrice":"150","leverage":"4"}]}`))
		default:
			t.Fatalf("unexpected Binance Portfolio Margin path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewBinance(Credentials{
		APIKey: "key", SecretKey: "secret", AccountType: BinanceAccountTypeUnified,
	}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	account, positions, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.TotalEquity != "120" || account.Balance != "125" || account.AvailableBalance != "90" ||
		account.UnrealizedPnL != "1.5" || len(account.Assets) != 1 || account.Assets[0].ValueUSDT != "125" ||
		account.Assets[0].Borrowed != "2" || len(positions) != 1 || positions[0].Side != "short" {
		t.Fatalf("unexpected unified snapshot: %#v %#v", account, positions)
	}
}

func TestBinanceUnifiedHistoryUsesPortfolioMarginEndpoints(t *testing.T) {
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-MBX-APIKEY") != "key" || r.URL.Query().Get("signature") == "" {
			t.Fatal("missing Binance Portfolio Margin signature")
		}
		requested[r.URL.Path]++
		switch r.URL.Path {
		case "/papi/v1/um/income":
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","incomeType":"REALIZED_PNL","income":"2","asset":"USDT","time":1000,"tranId":10}]`))
		case "/papi/v1/um/allOrders":
			if r.URL.Query().Get("symbol") != "BTCUSDT" {
				t.Fatalf("missing unified order symbol: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","orderId":20,"clientOrderId":"manual","price":"100","avgPrice":"101","origQty":"1","executedQty":"1","cumQuote":"101","status":"FILLED","type":"MARKET","side":"BUY","positionSide":"BOTH","time":1000,"updateTime":1001}]`))
		case "/papi/v1/um/userTrades":
			_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","id":30,"orderId":20,"price":"101","qty":"1","quoteQty":"101","commission":"0.1","commissionAsset":"USDT","realizedPnl":"2","side":"BUY","positionSide":"BOTH","time":1001,"maker":false}]`))
		default:
			t.Fatalf("unexpected Binance Portfolio Margin history path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewBinance(Credentials{APIKey: "key", SecretKey: "secret", AccountType: BinanceAccountTypeUnified}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(2, 0) }
	batch, err := client.History(context.Background(), HistoryRequest{Since: time.Unix(0, 0), Until: time.Unix(2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Orders) != 1 || len(batch.Fills) != 1 || len(batch.IncomeEvents) != 1 ||
		batch.Orders[0].Market != BinanceAccountTypeUnified || batch.Fills[0].Market != BinanceAccountTypeUnified {
		t.Fatalf("unexpected unified history: %#v", batch)
	}
	for _, path := range []string{"/papi/v1/um/income", "/papi/v1/um/allOrders", "/papi/v1/um/userTrades"} {
		if requested[path] != 1 {
			t.Fatalf("endpoint %s requested %d times", path, requested[path])
		}
	}
}

func TestFactoryRejectsBinanceUnifiedTestnet(t *testing.T) {
	service := credential.NewService(make([]byte, 32), nil)
	apiKey, err := service.EncryptField("key", "user-1", "exchange-1", "api_key")
	if err != nil {
		t.Fatal(err)
	}
	secretKey, err := service.EncryptField("secret", "user-1", "exchange-1", "secret_key")
	if err != nil {
		t.Fatal(err)
	}
	factory := NewFactory(service)
	_, err = factory.Build(&schema.Exchange{
		ID: "exchange-1", OwnerUserID: "user-1", ExchangeType: "binance",
		AccountType: BinanceAccountTypeUnified, Testnet: true,
		APIKeyCiphertext: apiKey, SecretKeyCiphertext: secretKey,
	})
	if !errors.Is(err, ErrTestnetUnsupported) {
		t.Fatalf("expected testnet unsupported error, got %v", err)
	}
}

func TestBybitSnapshot(t *testing.T) {
	positionPages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("X-BAPI-SIGN") == "" {
			t.Fatal("Bybit request is not signed GET")
		}
		switch r.URL.Path {
		case "/v5/account/wallet-balance":
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[{"totalEquity":"100","totalWalletBalance":"90","totalAvailableBalance":"70","totalPerpUPL":"10"}]}}`))
		case "/v5/position/list":
			positionPages++
			if r.URL.Query().Get("limit") != "200" {
				t.Fatalf("unexpected Bybit page limit: %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("cursor") == "" {
				_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"nextPageCursor":"next","list":[{"symbol":"ETHUSDT","side":"Sell","size":"2","avgPrice":"20","markPrice":"19","liqPrice":"30","unrealisedPnl":"2","leverage":"3","positionIdx":2}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"nextPageCursor":"","list":[{"symbol":"BTCUSDT","side":"Buy","size":"1","avgPrice":"100","markPrice":"110","liqPrice":"50","unrealisedPnl":"10","leverage":"4","positionIdx":1}]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewBybit(Credentials{APIKey: "key", SecretKey: "secret"}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	account, positions, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.AvailableBalance != "70" || len(positions) != 2 || positions[0].Side != "short" || positionPages != 2 {
		t.Fatalf("unexpected snapshot: %#v %#v", account, positions)
	}
}

func TestOKXSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("OK-ACCESS-SIGN") == "" || r.Header.Get("OK-ACCESS-PASSPHRASE") != "pass" {
			t.Fatal("OKX request is not signed GET")
		}
		switch r.URL.Path {
		case "/api/v5/account/balance":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"totalEq":"100","adjEq":"90","availEq":"70","upl":"10"}]}`))
		case "/api/v5/account/positions":
			_, _ = w.Write([]byte(`{"code":"0","msg":"","data":[{"posId":"1","instId":"BTC-USDT-SWAP","posSide":"long","pos":"1","avgPx":"100","markPx":"110","liqPx":"50","upl":"10","lever":"4"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := NewOKX(Credentials{APIKey: "key", SecretKey: "secret", Passphrase: "pass"}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Unix(1, 0) }
	account, positions, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if account.TotalEquity != "100" || len(positions) != 1 || positions[0].Leverage != 4 {
		t.Fatalf("unexpected snapshot: %#v %#v", account, positions)
	}
}
