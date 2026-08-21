package biz

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildLiquidationHeatmapAggregatesLongAndShortBuckets(t *testing.T) {
	from := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Hour)
	params := &schema.LiquidationHeatmapQuery{Symbol: "BTCUSDT", Hours: 4, TimeBuckets: 4, PriceBuckets: 5}
	events := []*schema.MarketLiquidationEvent{
		{Symbol: "BTCUSDT", Side: "long", Price: "100", NotionalUSDT: "1000", EventAt: from.Add(30 * time.Minute)},
		{Symbol: "BTCUSDT", Side: "short", Price: "110", NotionalUSDT: "500", EventAt: from.Add(90 * time.Minute)},
	}
	result := buildLiquidationHeatmap(params, events, from, to)
	if !result.DataAvailable || result.EventCount != 2 || result.TotalNotional != 1500 || len(result.Cells) != 2 {
		t.Fatalf("unexpected heatmap: %#v", result)
	}
	if result.Cells[0].LongNotional != 1000 || result.Cells[1].ShortNotional != 500 {
		t.Fatalf("unexpected cell aggregation: %#v", result.Cells)
	}
}

func TestGetLiquidationHeatmapReturnsOnlyRequestedSymbolAndWindow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.MarketLiquidationEvent)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	items := []*schema.MarketLiquidationEvent{
		{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT", Side: "long", Price: "100", Quantity: "1", NotionalUSDT: "100", EventAt: now.Add(-time.Hour), PayloadHash: "a", Source: "test"},
		{Exchange: "binance", Market: "futures", Symbol: "ETHUSDT", Side: "short", Price: "2000", Quantity: "1", NotionalUSDT: "2000", EventAt: now.Add(-time.Hour), PayloadHash: "b", Source: "test"},
		{Exchange: "binance", Market: "futures", Symbol: "BTCUSDT", Side: "short", Price: "90", Quantity: "1", NotionalUSDT: "90", EventAt: now.Add(-48 * time.Hour), PayloadHash: "c", Source: "test"},
	}
	if err := db.Create(items).Error; err != nil {
		t.Fatal(err)
	}
	service := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	result, err := service.GetLiquidationHeatmap(context.Background(), &schema.LiquidationHeatmapQuery{Symbol: " btcusdt ", Hours: 24, TimeBuckets: 24, PriceBuckets: 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.EventCount != 1 || result.Symbol != "BTCUSDT" || result.TotalNotional != 100 {
		t.Fatalf("unexpected scoped heatmap: %#v", result)
	}
}
