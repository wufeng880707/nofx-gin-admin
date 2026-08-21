package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeProvider struct{ positions []*provider.Position }

func (*fakeProvider) Name() string                 { return "fake" }
func (*fakeProvider) Health(context.Context) error { return nil }
func (a *fakeProvider) Snapshot(context.Context) (*provider.Account, []*provider.Position, error) {
	return &provider.Account{TotalEquity: "100", Balance: "90", AvailableBalance: "70", UnrealizedPnL: "10",
		Assets: []*provider.Asset{{Asset: "USDT", Total: "100", Available: "70", PriceUSDT: "1", ValueUSDT: "100"}}}, a.positions, nil
}

type fakeHistoricalProvider struct{ fakeProvider }

func (*fakeHistoricalProvider) History(_ context.Context, request provider.HistoryRequest) (*provider.HistoryBatch, error) {
	now := request.Until
	return &provider.HistoryBatch{
		Orders: []*provider.Order{{Market: "futures", ExchangeOrderID: "order-1", ClientOrderID: "external-1",
			Symbol: "BTCUSDT", Side: "buy", OrderType: "market", Status: "filled", Quantity: "1",
			ExecutedQuantity: "1", QuoteQuantity: "100", AveragePrice: "100", OrderedAt: now, ExchangeUpdatedAt: now}},
		Fills: []*provider.Fill{{Market: "futures", ExchangeTradeID: "trade-1", ExchangeOrderID: "order-1",
			Symbol: "BTCUSDT", Side: "buy", Price: "100", Quantity: "1", QuoteQuantity: "100",
			FeeAsset: "USDT", FeeQuantity: "0.1", FeeValueUSDT: "0.1", RealizedPnL: "2", FilledAt: now}},
		IncomeEvents: []*provider.IncomeEvent{{Market: "futures", EventType: "funding_fee", ExchangeEventID: "income-1",
			Symbol: "BTCUSDT", Asset: "USDT", Amount: "1", PriceUSDT: "1", ValueUSDT: "1", OccurredAt: now}},
		SyncedUntil: now,
	}, nil
}

type capturingHistoricalProvider struct {
	fakeProvider
	request provider.HistoryRequest
	batch   *provider.HistoryBatch
}

func (a *capturingHistoricalProvider) History(_ context.Context, request provider.HistoryRequest) (*provider.HistoryBatch, error) {
	a.request = request
	return a.batch, nil
}

func TestRuntimeLeasePreventsDuplicateInstance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-lease-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.RuntimeLease)); err != nil {
		t.Fatal(err)
	}
	first := &Registry{DB: db, InstanceID: "instance-a", LeaseDuration: time.Minute}
	second := &Registry{DB: db, InstanceID: "instance-b", LeaseDuration: time.Minute}
	if err := first.acquireLease(context.Background(), "trader-1"); err != nil {
		t.Fatal(err)
	}
	if err := second.acquireLease(context.Background(), "trader-1"); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("expected duplicate lease rejection, got %v", err)
	}
	if err := db.Model(new(schema.RuntimeLease)).Where("trader_id = ?", "trader-1").Update("lease_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := second.acquireLease(context.Background(), "trader-1"); err != nil {
		t.Fatalf("expired lease was not recoverable: %v", err)
	}
}

func TestRecordRuntimeCycleTracksDurationAndConsecutiveFailures(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Trader)); err != nil {
		t.Fatal(err)
	}
	trader := &schema.Trader{ID: "runtime-metrics", OwnerUserID: "user-1", Name: "Metrics", AIModelID: "model-1",
		ExchangeID: "exchange-1", InitialBalance: "100", ScanIntervalMinutes: 15}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(db, nil, nil)
	registry.recordRuntimeCycle(context.Background(), trader.ID, time.Now().Add(-20*time.Millisecond), errors.New("failed"))
	registry.recordRuntimeCycle(context.Background(), trader.ID, time.Now().Add(-20*time.Millisecond), errors.New("failed again"))
	if err := db.First(trader, "id = ?", trader.ID).Error; err != nil {
		t.Fatal(err)
	}
	if trader.ConsecutiveFailures != 2 || trader.LastCycleDurationMS < 1 {
		t.Fatalf("unexpected failure metrics: %#v", trader)
	}
	registry.recordRuntimeCycle(context.Background(), trader.ID, time.Now().Add(-10*time.Millisecond), nil)
	if err := db.First(trader, "id = ?", trader.ID).Error; err != nil {
		t.Fatal(err)
	}
	if trader.ConsecutiveFailures != 0 || trader.LastCycleDurationMS < 1 {
		t.Fatalf("unexpected recovery metrics: %#v", trader)
	}
}

func TestSyncOnceDoesNotPersistSnapshotAndClosesMissingPositions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Trader), new(schema.AccountSnapshot), new(schema.AssetBalance), new(schema.Position), new(schema.RuntimeLease)); err != nil {
		t.Fatal(err)
	}
	trader := &schema.Trader{ID: "trader-1", OwnerUserID: "user-1", Name: "Trader", AIModelID: "model-1", ExchangeID: "exchange-1",
		InitialBalance: "100", ScanIntervalMinutes: 15, DesiredState: schema.TraderDesiredStateStopped, RuntimeState: schema.TraderRuntimeUnavailable}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange-1", OwnerUserID: "user-1"}
	remote := &fakeProvider{positions: []*provider.Position{{ExchangePositionID: "BTC:long", Symbol: "BTCUSDT", Side: "long", Quantity: "0.1", EntryPrice: "100", MarkPrice: "110", UnrealizedPnL: "1", Leverage: 5}}}
	registry := NewRegistry(db, nil, nil)
	if err := registry.SyncOnce(context.Background(), trader, exchange, remote); err != nil {
		t.Fatal(err)
	}
	var snapshotCount int64
	if err := db.Model(new(schema.AccountSnapshot)).Count(&snapshotCount).Error; err != nil || snapshotCount != 0 {
		t.Fatalf("snapshots=%d err=%v", snapshotCount, err)
	}
	var position schema.Position
	if err := db.First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Status != schema.PositionStatusOpen || position.MarkPrice != "110" {
		t.Fatalf("unexpected position: %#v", position)
	}
	var asset schema.AssetBalance
	if err := db.First(&asset).Error; err != nil || asset.Asset != "USDT" || asset.ValueUSDT != "100" {
		t.Fatalf("unexpected asset balance: %#v err=%v", asset, err)
	}
	if asset.TraderID != accountScopeID(exchange.ID) {
		t.Fatalf("asset is not exchange-scoped: trader_id=%q", asset.TraderID)
	}

	remote.positions = nil
	time.Sleep(time.Millisecond)
	if err := registry.SyncOnce(context.Background(), trader, exchange, remote); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&position, "id = ?", position.ID).Error; err != nil {
		t.Fatal(err)
	}
	if position.Status != schema.PositionStatusClosed || position.ClosedAt == nil {
		t.Fatalf("position was not closed: %#v", position)
	}
}

func TestSyncOncePersistsHistoryIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-history-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Trader), new(schema.AccountSnapshot), new(schema.AssetBalance), new(schema.Position),
		new(schema.ExchangeOrder), new(schema.Fill), new(schema.IncomeEvent), new(schema.ClosedTrade), new(schema.SyncCursor),
		new(schema.HistorySymbol), new(schema.TradeCommand), new(schema.TradeCommandEvent)); err != nil {
		t.Fatal(err)
	}
	trader := &schema.Trader{ID: "trader-history", OwnerUserID: "user-1", Name: "History Trader", AIModelID: "model-1",
		ExchangeID: "exchange-history", InitialBalance: "100", ScanIntervalMinutes: 15,
		DesiredState: schema.TraderDesiredStateStopped, RuntimeState: schema.TraderRuntimeUnavailable}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange-history", OwnerUserID: "user-1", ExchangeType: "binance", AccountType: "futures"}
	remote := &fakeHistoricalProvider{}
	registry := NewRegistry(db, nil, nil)
	for i := 0; i < 2; i++ {
		if err := registry.SyncOnce(context.Background(), trader, exchange, remote); err != nil {
			t.Fatal(err)
		}
	}
	for name, model := range map[string]interface{}{"orders": new(schema.ExchangeOrder), "fills": new(schema.Fill),
		"income": new(schema.IncomeEvent), "cursors": new(schema.SyncCursor)} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", name, count, err)
		}
	}
	var order schema.ExchangeOrder
	if err := db.First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.TraderID != accountScopeID(exchange.ID) {
		t.Fatalf("order is not exchange-scoped: trader_id=%q", order.TraderID)
	}
}

func TestHistorySymbolDiscoveryUsesAllPersistedDatasets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:history-symbol-discovery?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.ExchangeOrder), new(schema.Fill), new(schema.IncomeEvent), new(schema.HistorySymbol)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&schema.ExchangeOrder{ID: "order-symbol", OwnerUserID: "user-1", TraderID: "scope", ExchangeID: "exchange-symbols",
		Market: "spot", ExchangeOrderID: "order-1", Symbol: "btcusdt", Side: "buy", OrderType: "market", Status: "filled",
		Price: "0", AveragePrice: "1", Quantity: "1", ExecutedQuantity: "1", QuoteQuantity: "1", OrderedAt: now,
		ExchangeUpdatedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Fill{ID: "fill-symbol", OwnerUserID: "user-1", TraderID: "scope", ExchangeID: "exchange-symbols",
		Market: "spot", ExchangeTradeID: "trade-1", Symbol: "ethusdt", Side: "buy", Price: "1", Quantity: "1",
		QuoteQuantity: "1", FeeQuantity: "0", FeeValueUSDT: "0", RealizedPnL: "0", FilledAt: now,
		CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.IncomeEvent{ID: "income-symbol", OwnerUserID: "user-1", TraderID: "scope", ExchangeID: "exchange-symbols",
		EventType: "funding_fee", ExchangeEventID: "income-1", Market: "futures", Symbol: "solusdt", Asset: "USDT",
		Amount: "1", PriceUSDT: "1", ValueUSDT: "1", OccurredAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.HistorySymbol{ID: "registered-symbol", OwnerUserID: "user-1", ExchangeID: "exchange-symbols",
		Symbol: "adausdt", Source: "manual", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry(db, nil, nil)
	symbols, err := registry.discoverHistorySymbols(context.Background(), "exchange-symbols", []*provider.Position{{Symbol: "dogeusdt"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ADAUSDT", "BTCUSDT", "DOGEUSDT", "ETHUSDT", "SOLUSDT"}
	if !reflect.DeepEqual(symbols, want) {
		t.Fatalf("unexpected discovered symbols: got=%v want=%v", symbols, want)
	}
}

func TestSyncExchangeOnceDoesNotRequireTrader(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:exchange-sync-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.AccountSnapshot), new(schema.AssetBalance), new(schema.ExchangeOrder), new(schema.Fill),
		new(schema.IncomeEvent), new(schema.ClosedTrade), new(schema.SyncCursor), new(schema.HistorySymbol),
		new(schema.TradeCommand), new(schema.TradeCommandEvent)); err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange-direct", OwnerUserID: "user-1", ExchangeType: "binance", AccountType: "futures"}
	registry := NewRegistry(db, nil, nil)
	result, err := registry.SyncExchangeOnce(context.Background(), exchange, new(fakeHistoricalProvider))
	if err != nil {
		t.Fatal(err)
	}
	if result.AssetCount != 1 || result.OrderCount != 1 || result.FillCount != 1 || result.IncomeEventCount != 1 {
		t.Fatalf("unexpected sync result: %#v", result)
	}
	var asset schema.AssetBalance
	if err := db.First(&asset).Error; err != nil {
		t.Fatal(err)
	}
	if asset.TraderID != accountScopeID(exchange.ID) {
		t.Fatalf("asset is not exchange-scoped: trader_id=%q", asset.TraderID)
	}
	var snapshotCount int64
	if err := db.Model(new(schema.AccountSnapshot)).Count(&snapshotCount).Error; err != nil || snapshotCount != 0 {
		t.Fatalf("regular synchronization persisted snapshots=%d err=%v", snapshotCount, err)
	}
	if _, err := registry.CaptureExchangeSnapshotOnce(context.Background(), exchange, new(fakeHistoricalProvider)); err != nil {
		t.Fatal(err)
	}
	var snapshot schema.AccountSnapshot
	if err := db.First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot.TraderID != accountScopeID(exchange.ID) || snapshot.TotalEquity != "100" || snapshot.AvailableBalance != "70" {
		t.Fatalf("unexpected exchange-scoped snapshot: %#v", snapshot)
	}
}

func TestBackfillExchangeHistoryPreservesIncrementalCursorAndRebuildsTrades(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:exchange-backfill-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.AssetBalance), new(schema.ExchangeOrder), new(schema.Fill),
		new(schema.IncomeEvent), new(schema.ClosedTrade), new(schema.SyncCursor), new(schema.TradeCommand),
		new(schema.TradeCommandEvent)); err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange-backfill", OwnerUserID: "user-1", ExchangeType: "binance", AccountType: "spot"}
	startAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	endAt := startAt.Add(24 * time.Hour)
	cursorAt := endAt.Add(30 * 24 * time.Hour)
	cursor := &schema.SyncCursor{TraderID: accountScopeID(exchange.ID), ExchangeID: exchange.ID, Dataset: "history",
		Market: "", Symbol: "", Cursor: cursorAt.Format(time.RFC3339Nano), SyncedUntil: &cursorAt, Status: "success"}
	if err := db.Create(cursor).Error; err != nil {
		t.Fatal(err)
	}
	remote := &capturingHistoricalProvider{batch: &provider.HistoryBatch{
		Orders: []*provider.Order{
			{Market: "spot", ExchangeOrderID: "order-buy", Symbol: "BTCUSDT", Side: "buy", OrderType: "market",
				Status: "filled", AveragePrice: "100", Quantity: "1", ExecutedQuantity: "1", QuoteQuantity: "100", OrderedAt: startAt, ExchangeUpdatedAt: startAt},
			{Market: "spot", ExchangeOrderID: "order-sell", Symbol: "BTCUSDT", Side: "sell", OrderType: "market",
				Status: "filled", AveragePrice: "120", Quantity: "1", ExecutedQuantity: "1", QuoteQuantity: "120", OrderedAt: endAt, ExchangeUpdatedAt: endAt},
		},
		Fills: []*provider.Fill{
			{Market: "spot", ExchangeTradeID: "trade-buy", ExchangeOrderID: "order-buy", Symbol: "BTCUSDT", Side: "buy",
				Price: "100", Quantity: "1", QuoteQuantity: "100", FeeAsset: "USDT", FeeQuantity: "0.1", FeeValueUSDT: "0.1", FilledAt: startAt},
			{Market: "spot", ExchangeTradeID: "trade-sell", ExchangeOrderID: "order-sell", Symbol: "BTCUSDT", Side: "sell",
				Price: "120", Quantity: "1", QuoteQuantity: "120", FeeAsset: "USDT", FeeQuantity: "0.1", FeeValueUSDT: "0.1", FilledAt: endAt},
		},
		SyncedUntil: endAt,
	}}
	registry := NewRegistry(db, nil, nil)
	result, err := registry.BackfillExchangeHistory(context.Background(), exchange, remote, provider.HistoryRequest{
		Since: startAt, Until: endAt, Symbols: []string{"BTCUSDT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(remote.request.Symbols) != 1 || remote.request.Symbols[0] != "BTCUSDT" {
		t.Fatalf("unexpected history request: %#v", remote.request)
	}
	if result.FetchedOrderCount != 2 || result.FetchedFillCount != 2 || result.ClosedTradeCount != 1 {
		t.Fatalf("unexpected backfill result: %#v", result)
	}
	var storedCursor schema.SyncCursor
	if err := db.First(&storedCursor, cursor.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedCursor.SyncedUntil == nil || !storedCursor.SyncedUntil.Equal(cursorAt) {
		t.Fatalf("incremental cursor moved during backfill: %#v", storedCursor)
	}
	var trade schema.ClosedTrade
	if err := db.First(&trade).Error; err != nil {
		t.Fatal(err)
	}
	if trade.Symbol != "BTCUSDT" || trade.NetPnL != "19.8" {
		t.Fatalf("unexpected rebuilt trade: %#v", trade)
	}
}
