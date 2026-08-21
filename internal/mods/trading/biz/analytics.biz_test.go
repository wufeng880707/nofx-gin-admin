package biz

import (
	"bytes"
	"context"
	"encoding/csv"
	"math"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAnalyticsPerformanceUsesClosedTradeReturnsAndSnapshotDrawdown(t *testing.T) {
	db := analyticsTestDB(t)
	start := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	trades := []*schema.ClosedTrade{
		closedTrade("win", "long", "BTCUSDT", "100", "1", "10", "1", "9", start, start.Add(time.Hour)),
		closedTrade("loss", "short", "ETHUSDT", "100", "1", "-5", "1", "-6", start.Add(2*time.Hour), start.Add(3*time.Hour)),
	}
	if err := db.Create(trades).Error; err != nil {
		t.Fatal(err)
	}
	snapshots := []*schema.AccountSnapshot{
		analyticsSnapshot("100", start), analyticsSnapshot("120", start.Add(12*time.Hour)), analyticsSnapshot("90", start.Add(24*time.Hour)),
	}
	if err := db.Create(snapshots).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.IncomeEvent{ID: "funding", OwnerUserID: "user-1", TraderID: "exchange:exchange-1",
		ExchangeID: "exchange-1", EventType: "funding_fee", ExchangeEventID: "funding", Market: "futures",
		Symbol: "BTCUSDT", Asset: "USDT", Amount: "2", PriceUSDT: "1", ValueUSDT: "2",
		OccurredAt: start.Add(4 * time.Hour), Source: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	service := &Analytics{AnalyticsDAL: &dal.Analytics{DB: db}}
	ctx := util.NewUserID(context.Background(), "user-1")
	result, err := service.QueryPerformance(ctx, schema.AnalyticsQueryParam{ExchangeID: "exchange-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalTrades != 2 || result.WinningTrades != 1 || result.LosingTrades != 1 || result.WinRate != 50 {
		t.Fatalf("unexpected trade counts: %#v", result)
	}
	if result.NetPnL != "5" || result.GrossPnL != "5" || result.TotalFee != "2" || result.Funding != "2" {
		t.Fatalf("unexpected pnl totals: %#v", result)
	}
	if math.Abs(result.ProfitFactor-1.5) > 0.000001 || math.Abs(result.ProfitLossRatio-1.5) > 0.000001 {
		t.Fatalf("unexpected ratios: %#v", result)
	}
	if math.Abs(result.MaxDrawdownPct-25) > 0.000001 || result.DrawdownSource != "account_snapshots" {
		t.Fatalf("unexpected drawdown: %#v", result)
	}
	if len(result.BySide) != 2 || len(result.BySymbol) != 2 {
		t.Fatalf("missing groups: %#v", result)
	}
}

func TestAnalyticsOverviewCombinesRealizedUnrealizedFundingAndFees(t *testing.T) {
	db := analyticsTestDB(t)
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	if err := db.Create(&schema.AssetBalance{OwnerUserID: "user-1", TraderID: "exchange:exchange-1", ExchangeID: "exchange-1",
		AccountType: "futures", Asset: "USDT", Total: "100", Available: "80", PriceUSDT: "1", ValueUSDT: "100",
		CapturedAt: now, Source: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Position{ID: "position", OwnerUserID: "user-1", TraderID: "trader-1", ExchangeID: "exchange-1",
		ExchangePositionID: "BTC:long", Symbol: "BTCUSDT", Side: "long", Status: schema.PositionStatusOpen,
		EntryQuantity: "1", Quantity: "1", EntryPrice: "100", MarkPrice: "104", UnrealizedPnL: "4", Leverage: 1,
		OpenedAt: now, LastSyncedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(closedTrade("trade", "long", "BTCUSDT", "100", "1", "5", "1", "4", now, now)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.IncomeEvent{ID: "funding", OwnerUserID: "user-1", TraderID: "exchange:exchange-1",
		ExchangeID: "exchange-1", EventType: "funding_fee", ExchangeEventID: "funding", Market: "futures",
		Symbol: "BTCUSDT", Asset: "USDT", Amount: "2", PriceUSDT: "1", ValueUSDT: "2", OccurredAt: now, Source: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	service := &Analytics{AnalyticsDAL: &dal.Analytics{DB: db}}
	result, err := service.QueryOverview(util.NewUserID(context.Background(), "user-1"), schema.AnalyticsQueryParam{ExchangeID: "exchange-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalEquity != "100" || result.AvailableBalance != "80" || result.PositionCount != 1 || result.ClosedTradeCount != 1 {
		t.Fatalf("unexpected overview: %#v", result)
	}
	if result.NetPnL != "10" {
		t.Fatalf("expected 5 realized + 4 unrealized + 2 funding - 1 fee, got %#v", result)
	}
}

func TestAggregateEquityPointsKeepsLatestSnapshotPerExchangeAndBucket(t *testing.T) {
	start := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	snapshots := []*schema.AccountSnapshot{
		analyticsSnapshotForExchange("exchange-1", "100", start.Add(time.Hour)),
		analyticsSnapshotForExchange("exchange-1", "110", start.Add(2*time.Hour)),
		analyticsSnapshotForExchange("exchange-2", "50", start.Add(3*time.Hour)),
		analyticsSnapshotForExchange("exchange-1", "120", start.Add(13*time.Hour)),
		analyticsSnapshotForExchange("exchange-2", "60", start.Add(14*time.Hour)),
	}

	result := aggregateEquityPoints(snapshots)
	if len(result) != 2 {
		t.Fatalf("expected two 12-hour buckets, got %#v", result)
	}
	if result[0].TotalEquity != "160" || result[1].TotalEquity != "180" {
		t.Fatalf("snapshots in the same bucket were not de-duplicated by exchange: %#v", result)
	}
}

func TestBuildPerformanceReportGroupsTradesAndFundingByDay(t *testing.T) {
	start := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	trades := []*schema.ClosedTrade{
		closedTrade("day-1", "long", "BTCUSDT", "100", "1", "10", "1", "9", start, start.Add(time.Hour)),
		closedTrade("day-2", "short", "ETHUSDT", "100", "1", "-5", "1", "-6", start.Add(24*time.Hour), start.Add(25*time.Hour)),
	}
	funding := []*schema.IncomeEvent{
		{OccurredAt: start.Add(2 * time.Hour), ValueUSDT: "2"},
		{OccurredAt: start.Add(26 * time.Hour), ValueUSDT: "-1"},
	}

	report := buildPerformanceReport(trades, funding, "day")
	if len(report) != 2 {
		t.Fatalf("expected two daily buckets, got %#v", report)
	}
	if report[0].Summary.NetPnL != "11" || report[0].Summary.Funding != "2" {
		t.Fatalf("unexpected first report bucket: %#v", report[0])
	}
	if report[1].Summary.NetPnL != "-7" || report[1].Summary.Funding != "-1" {
		t.Fatalf("unexpected second report bucket: %#v", report[1])
	}
}

func TestExportPerformanceReportProducesParseableCSV(t *testing.T) {
	db := analyticsTestDB(t)
	start := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	trade := closedTrade("csv", "long", "=DANGEROUS", "100", "1", "10", "1", "9", start, start.Add(time.Hour))
	if err := db.Create(trade).Error; err != nil {
		t.Fatal(err)
	}
	service := &Analytics{AnalyticsDAL: &dal.Analytics{DB: db}}
	ctx := util.NewUserID(context.Background(), "user-1")
	end := start.Add(24 * time.Hour)
	content, err := service.ExportPerformanceReport(ctx, schema.PerformanceReportQueryParam{
		AnalyticsQueryParam: schema.AnalyticsQueryParam{ExchangeID: "exchange-1", StartAt: &start, EndAt: &end},
		Period:              "day",
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(content)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "day" || rows[1][8] != "9" {
		t.Fatalf("unexpected CSV export: %#v", rows)
	}
	if got := safeCSVCell("=1+1"); got != "'=1+1" {
		t.Fatalf("CSV formula was not escaped: %q", got)
	}
}

func analyticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Trader), new(schema.AssetBalance), new(schema.Position), new(schema.AccountSnapshot),
		new(schema.Fill), new(schema.IncomeEvent), new(schema.ClosedTrade)); err != nil {
		t.Fatal(err)
	}
	return db
}

func closedTrade(id, side, symbol, entryPrice, quantity, gross, fee, net string, openedAt, closedAt time.Time) *schema.ClosedTrade {
	return &schema.ClosedTrade{ID: id, OwnerUserID: "user-1", TraderID: "exchange:exchange-1", ExchangeID: "exchange-1",
		Market: "futures", Symbol: symbol, Side: side, EntryQuantity: quantity, ExitQuantity: quantity,
		EntryPrice: entryPrice, ExitPrice: entryPrice, GrossPnL: gross, Fee: fee, Funding: "0", NetPnL: net,
		OpenedAt: openedAt, ClosedAt: closedAt, SourceFillCount: 2, RebuiltAt: closedAt}
}

func analyticsSnapshot(equity string, capturedAt time.Time) *schema.AccountSnapshot {
	return analyticsSnapshotForExchange("exchange-1", equity, capturedAt)
}

func analyticsSnapshotForExchange(exchangeID, equity string, capturedAt time.Time) *schema.AccountSnapshot {
	return &schema.AccountSnapshot{OwnerUserID: "user-1", TraderID: "exchange:" + exchangeID, ExchangeID: exchangeID,
		CapturedAt: capturedAt, TotalEquity: equity, Balance: equity, AvailableBalance: equity, Source: "test"}
}
