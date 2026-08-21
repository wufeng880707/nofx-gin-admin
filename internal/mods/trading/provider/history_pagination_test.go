package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBinanceHistorySplitsSaturatedTimeWindow(t *testing.T) {
	start := time.UnixMilli(1000)
	end := start.Add(9 * time.Millisecond)
	calls := 0
	rows, err := loadBinanceTimeRange(context.Background(), start, end, 10*time.Millisecond,
		func(pageStart, pageEnd time.Time) ([]int, error) {
			calls++
			if pageEnd.Sub(pageStart) >= 9*time.Millisecond {
				return make([]int, binanceHistoryLimit), nil
			}
			return []int{int(pageStart.UnixMilli())}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(rows) != 2 {
		t.Fatalf("unexpected Binance split result: calls=%d rows=%d", calls, len(rows))
	}
}

func TestBinanceHistoryRejectsSaturatedMillisecond(t *testing.T) {
	at := time.UnixMilli(1000)
	_, err := loadBinanceWindow(context.Background(), at, at, func(time.Time, time.Time) ([]int, error) {
		return make([]int, binanceHistoryLimit), nil
	})
	if err == nil {
		t.Fatal("expected saturated millisecond to fail instead of silently truncating history")
	}
}

func TestOKXHistoryPaginatesOrdersFillsAndBills(t *testing.T) {
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("OK-ACCESS-SIGN") == "" || r.URL.Query().Get("limit") != "100" {
			t.Fatalf("invalid OKX history request: %s", r.URL.RawQuery)
		}
		market := r.URL.Query().Get("instType")
		key := r.URL.Path + ":" + market
		requests[key]++
		after := r.URL.Query().Get("after")
		rows := make([]map[string]string, 0, okxHistoryLimit)
		switch r.URL.Path {
		case "/api/v5/trade/orders-history-archive":
			rows = okxOrderPage(t, market, after)
		case "/api/v5/trade/fills-history":
			rows = okxFillPage(t, market, after)
		case "/api/v5/account/bills-archive":
			rows = okxBillPage(t, after)
		default:
			t.Fatalf("unexpected OKX history path %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"code": "0", "msg": "", "data": rows}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := NewOKX(Credentials{APIKey: "key", SecretKey: "secret", Passphrase: "pass"}, server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }
	client.wait = func(context.Context, time.Duration) error { return nil }
	batch, err := client.History(context.Background(), HistoryRequest{
		Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Orders) != 202 || len(batch.Fills) != 202 || len(batch.IncomeEvents) != 101 {
		t.Fatalf("unexpected paged OKX batch: orders=%d fills=%d income=%d",
			len(batch.Orders), len(batch.Fills), len(batch.IncomeEvents))
	}
	for _, market := range []string{"SPOT", "SWAP"} {
		if requests["/api/v5/trade/orders-history-archive:"+market] != 2 ||
			requests["/api/v5/trade/fills-history:"+market] != 2 {
			t.Fatalf("OKX %s history did not fetch both pages: %#v", market, requests)
		}
	}
	if requests["/api/v5/account/bills-archive:"] != 2 {
		t.Fatalf("OKX bills did not fetch both pages: %#v", requests)
	}
}

func okxOrderPage(t *testing.T, market, after string) []map[string]string {
	t.Helper()
	if after != "" && after != "101" {
		t.Fatalf("unexpected OKX order cursor %q", after)
	}
	if after == "101" {
		return []map[string]string{{"instType": market, "instId": "BTC-USDT", "ordId": "100", "cTime": "1000", "uTime": "1001"}}
	}
	rows := make([]map[string]string, 0, okxHistoryLimit)
	for i := 0; i < okxHistoryLimit; i++ {
		rows = append(rows, map[string]string{"instType": market, "instId": "BTC-USDT", "ordId": fmt.Sprintf("%d", 200-i), "cTime": "1000", "uTime": "1001"})
	}
	return rows
}

func okxFillPage(t *testing.T, market, after string) []map[string]string {
	t.Helper()
	if after != "" && after != "101" {
		t.Fatalf("unexpected OKX fill cursor %q", after)
	}
	if after == "101" {
		return []map[string]string{{"instType": market, "instId": "BTC-USDT", "billId": "100", "tradeId": "trade-100", "fillTime": "1000"}}
	}
	rows := make([]map[string]string, 0, okxHistoryLimit)
	for i := 0; i < okxHistoryLimit; i++ {
		id := fmt.Sprintf("%d", 200-i)
		rows = append(rows, map[string]string{"instType": market, "instId": "BTC-USDT", "billId": id, "tradeId": "trade-" + id, "fillTime": "1000"})
	}
	return rows
}

func okxBillPage(t *testing.T, after string) []map[string]string {
	t.Helper()
	if after != "" && after != "101" {
		t.Fatalf("unexpected OKX bill cursor %q", after)
	}
	if after == "101" {
		return []map[string]string{{"billId": "100", "instType": "SPOT", "instId": "BTC-USDT", "ccy": "USDT", "ts": "1000"}}
	}
	rows := make([]map[string]string, 0, okxHistoryLimit)
	for i := 0; i < okxHistoryLimit; i++ {
		rows = append(rows, map[string]string{"billId": fmt.Sprintf("%d", 200-i), "instType": "SPOT", "instId": "BTC-USDT", "ccy": "USDT", "ts": "1000"})
	}
	return rows
}
