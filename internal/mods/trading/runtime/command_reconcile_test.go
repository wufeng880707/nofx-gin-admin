package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReconcileTradeCommandsAdvancesPartialAndFilledStatesIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.TradeCommand), new(schema.TradeCommandEvent), new(schema.ExchangeOrder)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	command := &schema.TradeCommand{ID: "command-a", OwnerUserID: "owner-a", TraderID: "trader-a",
		ExchangeID: "exchange-a", IdempotencyKey: "reconcile-a", Action: "open", Market: "spot",
		Symbol: "BTCUSDT", Side: "buy", OrderType: "market", Quantity: "1", Leverage: 1,
		RiskSnapshotJSON: `{}`, Status: schema.TradeCommandStatusAccepted, ExchangeOrderID: "order-a",
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(command).Error; err != nil {
		t.Fatal(err)
	}
	order := &schema.ExchangeOrder{ID: "history-a", OwnerUserID: "owner-a", TraderID: "exchange:exchange-a",
		ExchangeID: "exchange-a", Market: "spot", ExchangeOrderID: "order-a", Symbol: "BTCUSDT",
		Side: "buy", OrderType: "market", Status: "partially_filled", Quantity: "1", ExecutedQuantity: "0.4",
		AveragePrice: "2000", OrderedAt: now, ExchangeUpdatedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(order).Error; err != nil {
		t.Fatal(err)
	}
	registry := &Registry{DB: db}
	if err := registry.ReconcileTradeCommands(context.Background(), "owner-a", "exchange-a"); err != nil {
		t.Fatal(err)
	}
	if err := db.First(command, "id = ?", command.ID).Error; err != nil {
		t.Fatal(err)
	}
	if command.Status != schema.TradeCommandStatusPartial || command.ExecutedQuantity != "0.4" || command.AveragePrice != "2000" {
		t.Fatalf("unexpected partial reconciliation: %#v", command)
	}
	if err := db.Model(order).Updates(map[string]interface{}{"status": "filled", "executed_quantity": "1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := registry.ReconcileTradeCommands(context.Background(), "owner-a", "exchange-a"); err != nil {
		t.Fatal(err)
	}
	if err := db.First(command, "id = ?", command.ID).Error; err != nil {
		t.Fatal(err)
	}
	if command.Status != schema.TradeCommandStatusFilled || command.ExecutedQuantity != "1" || command.FinishedAt == nil {
		t.Fatalf("unexpected filled reconciliation: %#v", command)
	}
	if err := registry.ReconcileTradeCommands(context.Background(), "owner-a", "exchange-a"); err != nil {
		t.Fatal(err)
	}
	var eventCount int64
	if err := db.Model(new(schema.TradeCommandEvent)).Where("command_id = ?", command.ID).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 {
		t.Fatalf("expected exactly two reconciliation events, got %d", eventCount)
	}
}
