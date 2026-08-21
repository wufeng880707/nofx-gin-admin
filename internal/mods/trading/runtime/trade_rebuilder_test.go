package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildClosedTradesAggregatesPartialCloses(t *testing.T) {
	start := time.Date(2026, 8, 19, 1, 0, 0, 0, time.UTC)
	fills := []*schema.Fill{
		tradeFill("open-1", "buy", "long", "100", "2", "0.2", "0", start),
		tradeFill("close-1", "sell", "long", "110", "1", "0.1", "10", start.Add(time.Hour)),
		tradeFill("close-2", "sell", "long", "120", "1", "0.1", "20", start.Add(2*time.Hour)),
	}
	trades, err := buildClosedTrades(fills, start.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected one round trip, got %d", len(trades))
	}
	trade := trades[0]
	if trade.EntryPrice != "100" || trade.ExitPrice != "115" || trade.GrossPnL != "30" {
		t.Fatalf("unexpected prices or pnl: %#v", trade)
	}
	if trade.Fee != "0.4" || trade.NetPnL != "29.6" || trade.SourceFillCount != 3 {
		t.Fatalf("unexpected costs: %#v", trade)
	}
	if trade.DurationSeconds != 7200 || trade.Side != "long" {
		t.Fatalf("unexpected lifecycle: %#v", trade)
	}
}

func TestBuildClosedTradesHandlesOneWayReversal(t *testing.T) {
	start := time.Date(2026, 8, 19, 1, 0, 0, 0, time.UTC)
	fills := []*schema.Fill{
		tradeFill("buy", "buy", "", "100", "1", "0.1", "0", start),
		tradeFill("reverse", "sell", "", "110", "2", "0.2", "10", start.Add(time.Hour)),
		tradeFill("cover", "buy", "", "90", "1", "0.1", "20", start.Add(2*time.Hour)),
	}
	trades, err := buildClosedTrades(fills, start.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected long and short round trips, got %d", len(trades))
	}
	if trades[0].Side != "long" || trades[0].NetPnL != "9.8" {
		t.Fatalf("unexpected long trade: %#v", trades[0])
	}
	if trades[1].Side != "short" || trades[1].EntryPrice != "110" || trades[1].ExitPrice != "90" || trades[1].NetPnL != "19.8" {
		t.Fatalf("unexpected short trade: %#v", trades[1])
	}
}

func TestRebuildClosedTradesIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:closed-trade-rebuild?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Fill), new(schema.ClosedTrade), new(schema.TradeCommand)); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 19, 1, 0, 0, 0, time.UTC)
	fills := []*schema.Fill{
		tradeFill("open", "buy", "long", "100", "1", "0.1", "0", start),
		tradeFill("close", "sell", "long", "120", "1", "0.1", "20", start.Add(time.Hour)),
	}
	if err := db.Create(fills).Error; err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(db, nil, nil)
	for i := 0; i < 2; i++ {
		if err := registry.RebuildClosedTrades(context.Background(), "user-1", "exchange-1"); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(new(schema.ClosedTrade)).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("closed trades=%d err=%v", count, err)
	}
}

func TestRebuildClosedTradesAttributesSystemCommandsToStrategyVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Fill), new(schema.ClosedTrade), new(schema.TradeCommand)); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 19, 1, 0, 0, 0, time.UTC)
	fills := []*schema.Fill{
		tradeFill("open", "buy", "long", "100", "1", "0.1", "0", start),
		tradeFill("close", "sell", "long", "120", "1", "0.1", "20", start.Add(time.Hour)),
	}
	if err := db.Create(fills).Error; err != nil {
		t.Fatal(err)
	}
	strategyID := "strategy-1"
	for index, orderID := range []string{"order-open", "order-close"} {
		command := &schema.TradeCommand{ID: string(rune('a' + index)), OwnerUserID: "user-1", TraderID: "trader-1",
			StrategyID: &strategyID, StrategyVersion: 3, ExchangeID: "exchange-1", IdempotencyKey: "key-" + orderID,
			Action: "open", Market: "futures", Symbol: "BTCUSDT", Side: "buy", OrderType: "market", Quantity: "1",
			RiskSnapshotJSON: `{}`, Status: schema.TradeCommandStatusFilled, ExchangeOrderID: orderID, CreatedAt: start, UpdatedAt: start}
		if err := db.Create(command).Error; err != nil {
			t.Fatal(err)
		}
	}
	registry := NewRegistry(db, nil, nil)
	if err := registry.RebuildClosedTrades(context.Background(), "user-1", "exchange-1"); err != nil {
		t.Fatal(err)
	}
	var trade schema.ClosedTrade
	if err := db.First(&trade).Error; err != nil {
		t.Fatal(err)
	}
	if trade.TraderID != "trader-1" || trade.StrategyID != strategyID || trade.StrategyVersion != 3 || trade.AttributionSource != "system" {
		t.Fatalf("unexpected attribution: %#v", trade)
	}
}

func tradeFill(id, side, positionSide, price, quantity, fee, pnl string, filledAt time.Time) *schema.Fill {
	return &schema.Fill{
		ID: id, OwnerUserID: "user-1", TraderID: "account::exchange-1", ExchangeID: "exchange-1",
		Market: "futures", ExchangeTradeID: id, ExchangeOrderID: "order-" + id, Symbol: "BTCUSDT",
		Side: side, PositionSide: positionSide, Price: price, Quantity: quantity, FeeValueUSDT: fee,
		RealizedPnL: pnl, FilledAt: filledAt, Source: "test", CreatedAt: filledAt, UpdatedAt: filledAt,
	}
}
