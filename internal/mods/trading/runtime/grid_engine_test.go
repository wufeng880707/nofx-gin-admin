package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildGridPlanAppliesOrderAndCapitalLimits(t *testing.T) {
	config := schema.DefaultStrategyStudioConfig().GridConfig
	config.LowerPrice = 80
	config.UpperPrice = 120
	config.GridCount = 8
	config.MaxOpenOrders = 4
	config.InvestmentUSDT = 1000
	config.MaxPositionUSDT = 400
	config.ReserveRatio = 0.2
	plan, err := BuildGridPlan(config, 100)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RiskTriggered || len(plan.Levels) != 9 || len(plan.Orders) != 4 {
		t.Fatalf("unexpected grid plan: %#v", plan)
	}
	total := 0.0
	for _, order := range plan.Orders {
		total += order.NotionalUSDT
	}
	if total > config.MaxPositionUSDT || plan.ReserveUSDT != 200 {
		t.Fatalf("capital limits were not applied: total=%v plan=%#v", total, plan)
	}
}

func TestRunGridStrategyCyclePersistsSimulationPlanWithoutTradeCommand(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:grid-runtime?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.Strategy), new(schema.StrategyVersion),
		new(schema.StrategyVersionSwitchEvent), new(schema.Trader), new(schema.Decision), new(schema.TradeCommand)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	config := schema.DefaultStrategyStudioConfig()
	config.StrategyType = "grid"
	config.GridConfig.LowerPrice = 50000
	config.GridConfig.UpperPrice = 70000
	encoded, _ := json.Marshal(config)
	version := 1
	strategy := &schema.Strategy{ID: "grid-strategy", OwnerUserID: "owner", Name: "Grid", StrategyType: "grid",
		IsActive: true, ConfigJSON: string(encoded), CurrentVersion: version, PublishedVersion: &version, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(strategy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.StrategyVersion{ID: "grid-v1", StrategyID: strategy.ID, OwnerUserID: strategy.OwnerUserID,
		Version: version, Name: strategy.Name, StrategyType: strategy.StrategyType, ConfigJSON: strategy.ConfigJSON,
		PublishedAt: &now, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange", OwnerUserID: "owner", ExchangeType: schema.MarketExchangeBinance,
		AccountType: schema.MarketTypeSpot, AccountName: "main", Name: "Main", Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(exchange).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(new(schema.Exchange)).Where("id = ?", exchange.ID).UpdateColumn("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	trader := &schema.Trader{ID: "grid-trader", OwnerUserID: "owner", Name: "Grid Trader", AIModelID: "unused",
		ExchangeID: exchange.ID, StrategyID: &strategy.ID, StrategyVersion: version, InitialBalance: "1000",
		IsSimulation: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	factory := &provider.Factory{HTTPClient: &http.Client{Transport: gridRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"symbol":"BTCUSDT","lastPrice":"60000","openPrice":"59000","highPrice":"61000","lowPrice":"58000","priceChange":"1000","priceChangePercent":"1.69","volume":"10","quoteVolume":"600000","closeTime":1}`))}, nil
	})}}
	registry := NewRegistry(db, factory, nil)
	if err := registry.RunStrategyCycle(context.Background(), trader.ID); err != nil {
		t.Fatalf("run grid cycle: %v", err)
	}
	var decision schema.Decision
	if err := db.Where("trader_id = ?", trader.ID).First(&decision).Error; err != nil {
		t.Fatal(err)
	}
	if !decision.Success || !strings.Contains(decision.ExecutionLogJSON, "Simulation plan only") {
		t.Fatalf("unexpected grid decision: %#v", decision)
	}
	var commandCount int64
	if err := db.Model(new(schema.TradeCommand)).Count(&commandCount).Error; err != nil || commandCount != 0 {
		t.Fatalf("grid simulation must not create commands: count=%d err=%v", commandCount, err)
	}
}

type gridRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn gridRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestBuildGridPlanPausesOutsideRangeWithoutOrders(t *testing.T) {
	config := schema.DefaultStrategyStudioConfig().GridConfig
	plan, err := BuildGridPlan(config, config.UpperPrice+1)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RiskTriggered || len(plan.Orders) != 0 || plan.RiskReason == "" {
		t.Fatalf("outside-range plan must pause: %#v", plan)
	}
}
