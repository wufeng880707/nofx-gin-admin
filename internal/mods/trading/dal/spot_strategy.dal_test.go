package dal

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAllocateSellFillConsumesOnlyExplicitLotsAndIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Fill), new(schema.SpotStrategyCycle), new(schema.SpotPositionLot), new(schema.SpotSellAllocation)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	cycle := &schema.SpotStrategyCycle{ID: "cycle-1", OwnerUserID: "user-1", StrategyID: "strategy-1", State: schema.SpotStateRangeTrading,
		ConfigVersion: 1, InitialBudget: "1000", AvailableCash: "100", ReservedCash: "0", RealizedPnL: "0", UnrealizedPnL: "0", StartedAt: now}
	if err := db.Create(cycle).Error; err != nil {
		t.Fatal(err)
	}
	dal := &SpotStrategy{DB: db}
	for index, price := range []string{"100", "80"} {
		fill := &schema.Fill{ID: "buy-" + string(rune('1'+index)), OwnerUserID: "user-1", TraderID: "trader-1", StrategyID: "strategy-1",
			ExchangeID: "exchange-1", Market: schema.MarketTypeSpot, ExchangeTradeID: "trade-buy-" + string(rune('1'+index)),
			ExchangeOrderID: "order-buy", Symbol: "BTCUSDT", Side: "buy", Price: price, Quantity: "1", QuoteQuantity: price,
			FeeAsset: "USDT", FeeQuantity: "0.1", FeeValueUSDT: "0.1", FilledAt: now, Source: "test"}
		if err := db.Create(fill).Error; err != nil {
			t.Fatal(err)
		}
		lot := &schema.SpotPositionLot{ID: "lot-" + string(rune('1'+index)), OwnerUserID: "user-1", StrategyID: "strategy-1",
			CycleID: "cycle-1", TraderID: "trader-1", ExchangeID: "exchange-1", Market: schema.MarketTypeSpot,
			Symbol: "BTCUSDT", Bucket: schema.SpotBucketSatellite, LayerNo: index, SourceOrderIntentID: "intent", TargetNetProfitRate: "0.03", ConfigVersion: 1}
		if err := dal.CreateLotFromBuyFill(context.Background(), lot, fill, 10); err != nil {
			t.Fatal(err)
		}
	}
	sell := &schema.Fill{ID: "sell-1", OwnerUserID: "user-1", TraderID: "trader-1", StrategyID: "strategy-1",
		ExchangeID: "exchange-1", Market: schema.MarketTypeSpot, ExchangeTradeID: "trade-sell", ExchangeOrderID: "order-sell",
		Symbol: "BTCUSDT", Side: "sell", Price: "110", Quantity: "0.5", QuoteQuantity: "55", FeeAsset: "USDT",
		FeeQuantity: "0.05", FeeValueUSDT: "0.05", FilledAt: now.Add(time.Hour), Source: "test"}
	if err := db.Create(sell).Error; err != nil {
		t.Fatal(err)
	}
	requests := []SpotLotAllocationRequest{{LotID: "lot-2", Quantity: "0.5"}}
	allocations, err := dal.AllocateSellFill(context.Background(), "user-1", schema.SpotBucketSatellite, sell, requests)
	if err != nil || len(allocations) != 1 || allocations[0].LotID != "lot-2" {
		t.Fatalf("unexpected allocation: %+v, %v", allocations, err)
	}
	if _, err := dal.AllocateSellFill(context.Background(), "user-1", schema.SpotBucketSatellite, sell, requests); err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	var untouched schema.SpotPositionLot
	if err := db.Where("id = ?", "lot-1").First(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	if untouched.RemainingQty != "1" {
		t.Fatalf("unselected lot changed: %s", untouched.RemainingQty)
	}
}
