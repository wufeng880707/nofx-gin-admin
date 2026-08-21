package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const liquidationFixture = `{"e":"forceOrder","E":1728972148778,"o":{"s":"BTCUSDT","S":"SELL","q":"0.2500","p":"65000.00","ap":"64000.00","T":1728972148777}}`

func TestParseBinanceLiquidationEventsSupportsDirectArrayAndCombinedPayload(t *testing.T) {
	cases := []string{
		liquidationFixture,
		`[` + liquidationFixture + `]`,
		`{"stream":"!forceOrder@arr","data":` + liquidationFixture + `}`,
	}
	for _, payload := range cases {
		events, err := parseBinanceLiquidationEvents([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("expected one event for %s, got %d", payload, len(events))
		}
		item := events[0]
		if item.Symbol != "BTCUSDT" || item.Side != "long" || item.Price != "64000.00" || item.NotionalUSDT != "16000" || len(item.PayloadHash) != 64 {
			t.Fatalf("unexpected parsed event: %#v", item)
		}
	}
}

func TestPersistMarketLiquidationIsIdempotentAndPrunesExpiredRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.MarketLiquidationEvent)); err != nil {
		t.Fatal(err)
	}
	events, err := parseBinanceLiquidationEvents([]byte(liquidationFixture))
	if err != nil {
		t.Fatal(err)
	}
	registry := &Registry{DB: db}
	for index := 0; index < 2; index++ {
		if err := registry.persistMarketLiquidation(context.Background(), events[0]); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(new(schema.MarketLiquidationEvent)).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("expected one idempotent row, count=%d err=%v", count, err)
	}
	oldRetention := config.C.Trading.MarketStream.RetentionHours
	config.C.Trading.MarketStream.RetentionHours = 1
	t.Cleanup(func() { config.C.Trading.MarketStream.RetentionHours = oldRetention })
	if err := registry.pruneMarketLiquidations(context.Background(), events[0].EventAt.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(new(schema.MarketLiquidationEvent)).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("expected expired row to be removed, count=%d err=%v", count, err)
	}
}

func TestParseBinanceLiquidationEventMapsBuyToShort(t *testing.T) {
	events, err := parseBinanceLiquidationEvents([]byte(`{"e":"forceOrder","E":1728972148778,"o":{"s":"ETHUSDT","S":"BUY","q":"2","p":"2500","ap":"0","T":1728972148777}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Side != "short" || events[0].NotionalUSDT != "5000" {
		t.Fatalf("unexpected short liquidation: %#v", events)
	}
}
