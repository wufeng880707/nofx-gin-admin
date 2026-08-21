package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBybitHistorySplitsWindowsAndPaginatesAllMarkets(t *testing.T) {
	calls := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		category := query.Get("category")
		if category != "spot" && category != "linear" {
			t.Fatalf("unexpected category: %s", category)
		}
		if category == "linear" && query.Get("settleCoin") != "USDT" {
			t.Fatalf("missing linear settle coin: %s", query.Encode())
		}
		start, _ := strconv.ParseInt(query.Get("startTime"), 10, 64)
		end, _ := strconv.ParseInt(query.Get("endTime"), 10, 64)
		if end-start > bybitHistoryWindow.Milliseconds() {
			t.Fatalf("history window is too large: %d", end-start)
		}
		key := request.URL.Path + ":" + category + ":" + query.Get("startTime")
		calls[key]++
		cursor := query.Get("cursor")
		next := ""
		if cursor == "" {
			next = "next-" + category
		} else if cursor != "next-"+category {
			t.Fatalf("unexpected cursor: %s", cursor)
		}
		if request.URL.Path == "/v5/order/history" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"retCode": 0,
				"result": map[string]interface{}{
					"nextPageCursor": next,
					"list": []map[string]interface{}{{
						"orderId": key + ":" + cursor, "symbol": "BTCUSDT", "side": "Buy", "positionIdx": 1,
						"orderStatus": "Filled", "orderType": "Limit", "price": "100", "avgPrice": "101",
						"qty": "2", "cumExecQty": "2", "cumExecValue": "202", "createdTime": "1000", "updatedTime": "2000",
					}},
				},
			})
			return
		}
		if request.URL.Path != "/v5/execution/list" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": 0,
			"result": map[string]interface{}{
				"nextPageCursor": next,
				"list": []map[string]interface{}{{
					"execId": key + ":" + cursor, "orderId": "order-1", "symbol": "BTCUSDT", "side": "Sell",
					"positionIdx": 2, "execPrice": "101", "execQty": "2", "execValue": "202", "execFee": "0.2",
					"feeCurrency": "USDT", "isMaker": true, "execTime": "3000",
				}},
			},
		})
	}))
	defer server.Close()

	client := NewBybit(Credentials{APIKey: "key", SecretKey: "secret"}, server.Client())
	client.baseURL = server.URL
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch, err := client.History(context.Background(), HistoryRequest{Since: since, Until: since.Add(8 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 8 {
		t.Fatalf("unexpected endpoint/window coverage: %#v", calls)
	}
	for key, count := range calls {
		if count != 2 {
			t.Fatalf("%s was called %d times", key, count)
		}
	}
	if len(batch.Orders) != 8 || len(batch.Fills) != 8 {
		t.Fatalf("unexpected batch sizes: orders=%d fills=%d", len(batch.Orders), len(batch.Fills))
	}
	if batch.Orders[0].PositionSide != "long" || batch.Fills[0].PositionSide != "short" || batch.Fills[0].Liquidity != "maker" {
		t.Fatalf("unexpected normalization: order=%#v fill=%#v", batch.Orders[0], batch.Fills[0])
	}
}

func TestBybitHistoryRejectsNonAdvancingCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		cursor := request.URL.Query().Get("cursor")
		if cursor == "" {
			cursor = "stuck"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"retCode": 0,
			"result":  map[string]interface{}{"nextPageCursor": cursor, "list": []interface{}{}},
		})
	}))
	defer server.Close()

	client := NewBybit(Credentials{APIKey: "key", SecretKey: "secret"}, server.Client())
	client.baseURL = server.URL
	now := time.Now().UTC()
	_, err := client.History(context.Background(), HistoryRequest{Since: now.Add(-time.Hour), Until: now})
	if err == nil || !strings.Contains(err.Error(), "cursor did not advance") {
		t.Fatalf("unexpected error: %v", err)
	}
}
