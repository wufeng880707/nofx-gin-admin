package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (a roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return a(request) }

func TestBinanceKlinesRangePaginatesAndDeduplicates(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 4 * time.Hour
	requestCount := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount++
		cursor, _ := strconv.ParseInt(request.URL.Query().Get("startTime"), 10, 64)
		count := 1000
		if requestCount > 1 {
			count = 2
		}
		rows := make([][]interface{}, 0, count)
		for index := 0; index < count; index++ {
			open := cursor + int64(index)*step.Milliseconds()
			rows = append(rows, []interface{}{open, "100", "101", "99", "100", "10", open + step.Milliseconds() - 1, "1000"})
		}
		body, _ := json.Marshal(rows)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	market := NewMarketData(client)
	end := start.Add(1001 * step)
	rows, err := market.KlinesRange(context.Background(), schema.MarketExchangeBinance, schema.MarketTypeSpot, "BTCUSDT", "4h", start, end)
	if err != nil {
		t.Fatalf("load range: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected two pages, got %d", requestCount)
	}
	if len(rows) != 1001 {
		t.Fatalf("expected 1001 unique candles in the requested range, got %d", len(rows))
	}
	for index := 1; index < len(rows); index++ {
		if rows[index].Time <= rows[index-1].Time {
			t.Fatalf("candles are not chronological at %d", index)
		}
	}
}
