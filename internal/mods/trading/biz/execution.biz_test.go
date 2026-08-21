package biz

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	apperrors "github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type executionRulesStub struct {
	rules  *provider.InstrumentRules
	ticker *schema.MarketTicker
}

func (a *executionRulesStub) InstrumentRules(context.Context, string, string, string) (*provider.InstrumentRules, error) {
	return a.rules, nil
}

func (a *executionRulesStub) Ticker(context.Context, string, string, string) (*schema.MarketTicker, error) {
	return a.ticker, nil
}

type executionTradingFactoryStub struct{ calls int }

func (a *executionTradingFactoryStub) BuildTrading(*schema.Exchange) (provider.Trading, error) {
	a.calls++
	return nil, provider.ErrTradingProviderDisabled
}

func TestSimulationCommandIsAuditedWithoutProviderCall(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	ctx := util.NewUserID(context.Background(), "owner-a")
	form := validSimulationCommand()
	command, err := service.CreateCommand(ctx, form)
	if err != nil {
		t.Fatal(err)
	}
	if command.Status != schema.TradeCommandStatusSimulated || !command.Simulation {
		t.Fatalf("unexpected simulation command: %#v", command)
	}
	if factory.calls != 0 {
		t.Fatalf("simulation must not build a trading provider, calls=%d", factory.calls)
	}
	var events []schema.TradeCommandEvent
	if err := db.Where("command_id = ?", command.ID).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ToStatus != schema.TradeCommandStatusSimulated {
		t.Fatalf("unexpected command audit events: %#v", events)
	}

	repeated, err := service.CreateCommand(ctx, validSimulationCommand())
	if err != nil || repeated.ID != command.ID {
		t.Fatalf("idempotent replay failed: command=%#v err=%v", repeated, err)
	}
	conflict := validSimulationCommand()
	conflict.Quantity = "0.02"
	if _, err := service.CreateCommand(ctx, conflict); errorID(err) != "trading_command_idempotency_conflict" {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestProductionCommandIsBlockedByDefault(t *testing.T) {
	service, db, factory := newExecutionTestService(t, false)
	previous := config.C.Trading.Execution.Enabled
	config.C.Trading.Execution.Enabled = false
	t.Cleanup(func() { config.C.Trading.Execution.Enabled = previous })
	form := validSimulationCommand()
	form.ConfirmExecution = true
	_, err := service.CreateCommand(util.NewUserID(context.Background(), "owner-a"), form)
	if errorID(err) != "trading_execution_global_disabled" {
		t.Fatalf("expected global gate rejection, got %v", err)
	}
	if factory.calls != 0 {
		t.Fatalf("disabled production command must not build provider, calls=%d", factory.calls)
	}
	var count int64
	if err := db.Model(new(schema.TradeCommand)).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("blocked command must not persist as executable: count=%d err=%v", count, err)
	}
}

func TestLiveTradingSafetyLifecycleIsAudited(t *testing.T) {
	service, db, _ := newExecutionTestService(t, false)
	previous := config.C.Trading.Execution.Enabled
	previousTestnetOnly := config.C.Trading.Execution.TestnetOnly
	config.C.Trading.Execution.Enabled = true
	config.C.Trading.Execution.TestnetOnly = false
	t.Cleanup(func() {
		config.C.Trading.Execution.Enabled = previous
		config.C.Trading.Execution.TestnetOnly = previousTestnetOnly
	})
	ctx := util.NewUserID(context.Background(), "owner-a")
	enable := &schema.TradingSafetyEnableForm{ConfirmationPhrase: "ENABLE LIVE TRADING", AcknowledgeRisk: true, Reason: "reviewed"}
	if err := enable.Validate(); err != nil {
		t.Fatal(err)
	}
	state, err := service.EnableLiveTrading(ctx, enable)
	if err != nil || !state.LiveEnabled {
		t.Fatalf("enable live trading: state=%#v err=%v", state, err)
	}
	disable := &schema.TradingSafetyDisableForm{Reason: "operator stop"}
	if err := disable.Validate(); err != nil {
		t.Fatal(err)
	}
	state, err = service.DisableLiveTrading(ctx, disable)
	if err != nil || state.LiveEnabled {
		t.Fatalf("disable live trading: state=%#v err=%v", state, err)
	}
	var events []schema.TradingSafetyEvent
	if err := db.Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Action != "enable" || events[1].Action != "disable" {
		t.Fatalf("unexpected safety events: %#v", events)
	}
}

func TestProductionCommandIsBlockedByTestnetOnlyPolicy(t *testing.T) {
	service, db, factory := newExecutionTestService(t, false)
	previousEnabled := config.C.Trading.Execution.Enabled
	previousTestnetOnly := config.C.Trading.Execution.TestnetOnly
	config.C.Trading.Execution.Enabled = true
	config.C.Trading.Execution.TestnetOnly = true
	t.Cleanup(func() {
		config.C.Trading.Execution.Enabled = previousEnabled
		config.C.Trading.Execution.TestnetOnly = previousTestnetOnly
	})
	form := validSimulationCommand()
	form.ConfirmExecution = true
	_, err := service.CreateCommand(util.NewUserID(context.Background(), "owner-a"), form)
	if errorID(err) != "trading_execution_testnet_only" {
		t.Fatalf("expected testnet-only rejection, got %v", err)
	}
	if factory.calls != 0 {
		t.Fatalf("testnet-only rejection must occur before provider construction, calls=%d", factory.calls)
	}
	var count int64
	if err := db.Model(new(schema.TradeCommand)).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("testnet-only command must not persist: count=%d err=%v", count, err)
	}
}

func TestCommandRejectsExchangeStepAndCrossUserQuery(t *testing.T) {
	service, _, _ := newExecutionTestService(t, true)
	form := validSimulationCommand()
	form.Quantity = "0.0105"
	_, err := service.CreateCommand(util.NewUserID(context.Background(), "owner-a"), form)
	if errorID(err) != "trading_command_quantity_step" {
		t.Fatalf("expected quantity step rejection, got %v", err)
	}
	_, err = service.QueryCommands(util.NewUserID(context.Background(), "owner-b"), schema.TradeCommandQueryParam{OwnerUserID: "owner-a"})
	if parsed, ok := apperrors.As(err); !ok || parsed.Code != 403 {
		t.Fatalf("expected cross-user query rejection, got %v", err)
	}
}

func TestCommandEnforcesProtectionDirectionAndStrategyRiskReward(t *testing.T) {
	service, db, _ := newExecutionTestService(t, true)
	ctx := util.NewUserID(context.Background(), "owner-a")
	wrongDirection := validSimulationCommand()
	wrongDirection.StopLoss = "2100"
	if _, err := service.CreateCommand(ctx, wrongDirection); errorID(err) != "trading_command_protection_direction" {
		t.Fatalf("expected protection direction rejection, got %v", err)
	}
	attachExecutionStrategy(t, db, 0.6, 2)
	lowReward := validSimulationCommand()
	lowReward.IdempotencyKey = "low-risk-reward"
	lowReward.TakeProfit = "2050"
	if _, err := service.CreateCommand(ctx, lowReward); errorID(err) != "trading_command_risk_reward_limit" {
		t.Fatalf("expected risk/reward rejection, got %v", err)
	}
}

func TestCommandEnforcesAggregateMarginUsage(t *testing.T) {
	service, db, _ := newExecutionTestService(t, true)
	attachExecutionStrategy(t, db, 0.6, 2)
	now := time.Now().UTC()
	position := &schema.Position{ID: "position-margin", OwnerUserID: "owner-a", TraderID: "trader-a",
		ExchangeID: "exchange-a", ExchangePositionID: "BTCUSDT:long", Symbol: "BTCUSDT", Side: "long",
		Status: schema.PositionStatusOpen, EntryQuantity: "0.29", Quantity: "0.29", EntryPrice: "2000",
		MarkPrice: "2000", Leverage: 1, OpenedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(position).Error; err != nil {
		t.Fatal(err)
	}
	form := validSimulationCommand()
	form.IdempotencyKey = "margin-limit"
	if _, err := service.CreateCommand(util.NewUserID(context.Background(), "owner-a"), form); errorID(err) != "trading_command_margin_usage_limit" {
		t.Fatalf("expected aggregate margin rejection, got %v", err)
	}
}

func TestReductionRequiresMatchingPositionAndCannotExceedQuantity(t *testing.T) {
	service, db, _ := newExecutionTestService(t, true)
	now := time.Now().UTC()
	position := &schema.Position{ID: "position-reduce", OwnerUserID: "owner-a", TraderID: "trader-a",
		ExchangeID: "exchange-a", ExchangePositionID: "ETHUSDT:long", Symbol: "ETHUSDT", Side: "long",
		Status: schema.PositionStatusOpen, EntryQuantity: "0.02", Quantity: "0.02", EntryPrice: "2000",
		MarkPrice: "2000", Leverage: 1, OpenedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(position).Error; err != nil {
		t.Fatal(err)
	}
	ctx := util.NewUserID(context.Background(), "owner-a")
	form := &schema.TradeCommandForm{TraderID: "trader-a", IdempotencyKey: "reduce-too-much",
		Action: "reduce", Market: "spot", Symbol: "ETHUSDT", Side: "sell", PositionSide: "long",
		OrderType: "market", Quantity: "0.03", Leverage: 1}
	if _, err := service.CreateCommand(ctx, form); errorID(err) != "trading_command_reduce_quantity_invalid" {
		t.Fatalf("expected reduce quantity rejection, got %v", err)
	}
	form.IdempotencyKey, form.Quantity = "reduce-valid", "0.01"
	command, err := service.CreateCommand(ctx, form)
	if err != nil {
		t.Fatal(err)
	}
	if !command.ReduceOnly || command.Status != schema.TradeCommandStatusSimulated {
		t.Fatalf("reduction was not forced to audited reduce-only simulation: %#v", command)
	}
}

func TestBulkClosePositionsRequiresPhraseAndCreatesIdempotentSimulationCommands(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	ctx := util.NewUserID(context.Background(), "owner-a")
	form := &schema.BulkClosePositionsForm{OperationID: "operation-a", AcknowledgeRisk: true,
		ConfirmationPhrase: "wrong", ConfirmExecution: true}
	if errorID(form.Validate()) != "trading_bulk_close_confirmation_required" {
		t.Fatalf("expected exact confirmation phrase")
	}
	form.ConfirmationPhrase = "CLOSE ALL POSITIONS"
	if err := form.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(new(schema.Exchange)).Where("id = ?", "exchange-a").Update("account_type", "futures").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, position := range []*schema.Position{
		{ID: "position-bulk-long", OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a",
			ExchangePositionID: "BTCUSDT:long", Symbol: "BTCUSDT", Side: "long", Status: schema.PositionStatusOpen,
			EntryQuantity: "0.01", Quantity: "0.01", EntryPrice: "2000", MarkPrice: "2000", Leverage: 1,
			OpenedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "position-bulk-short", OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a",
			ExchangePositionID: "BTCUSDT:short", Symbol: "BTCUSDT", Side: "short", Status: schema.PositionStatusOpen,
			EntryQuantity: "0.02", Quantity: "0.02", EntryPrice: "2000", MarkPrice: "2000", Leverage: 1,
			OpenedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(position).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.BulkClosePositions(ctx, form)
	if err != nil {
		t.Fatal(err)
	}
	if result.Requested != 2 || result.Succeeded != 2 || result.Failed != 0 || factory.calls != 0 {
		t.Fatalf("unexpected bulk close result: %#v provider_calls=%d", result, factory.calls)
	}
	if result.Results[0].Command.Status != schema.TradeCommandStatusSimulated || !result.Results[0].Command.ReduceOnly {
		t.Fatalf("bulk close did not create an audited simulation command: %#v", result.Results[0])
	}
	repeated, err := service.BulkClosePositions(ctx, form)
	if err != nil || repeated.Succeeded != 2 {
		t.Fatalf("bulk close idempotent replay failed: %#v err=%v", repeated, err)
	}
	var count int64
	if err := db.Model(new(schema.TradeCommand)).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("expected exactly two idempotent commands, count=%d err=%v", count, err)
	}
}

func attachExecutionStrategy(t *testing.T, db *gorm.DB, maxMargin, minRiskReward float64) {
	t.Helper()
	config := schema.DefaultStrategyStudioConfig()
	config.AIConfig.RiskControl.MaxMarginUsage = maxMargin
	config.AIConfig.RiskControl.MinRiskRewardRatio = minRiskReward
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	strategyID := "strategy-risk"
	version := &schema.StrategyVersion{ID: "strategy-risk-v1", StrategyID: strategyID, OwnerUserID: "owner-a",
		Version: 1, Name: "Risk", StrategyType: config.StrategyType, ConfigJSON: string(encoded),
		PublishedAt: &now, CreatedAt: now}
	if err := db.Create(version).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(new(schema.Trader)).Where("id = ?", "trader-a").Updates(map[string]interface{}{
		"strategy_id": strategyID, "strategy_version": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func newExecutionTestService(t *testing.T, simulation bool) (*Execution, *gorm.DB, *executionTradingFactoryStub) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.Trader), new(schema.Position), new(schema.AccountSnapshot),
		new(schema.StrategyVersion), new(schema.TradingSafetyState), new(schema.TradingSafetyEvent),
		new(schema.TradeCommand), new(schema.TradeCommandEvent), new(schema.TradeApproval),
		new(schema.TradeApprovalEvent), new(schema.Decision)); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Exchange{ID: "exchange-a", OwnerUserID: "owner-a", ExchangeType: "binance",
		AccountName: "main", Name: "Main", AccountType: "spot", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Trader{ID: "trader-a", OwnerUserID: "owner-a", ExchangeID: "exchange-a",
		AIModelID: "model-a", Name: "Trader", IsSimulation: simulation, ExecutionMode: schema.TraderExecutionApprovalRequired,
		DesiredState: schema.TraderDesiredStateStopped, RuntimeState: schema.TraderRuntimeUnavailable,
		InitialBalance: "1000", ScanIntervalMinutes: 15, CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if !simulation {
		if err := db.Model(new(schema.Trader)).Where("id = ?", "trader-a").UpdateColumn("is_simulation", false).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&schema.AccountSnapshot{OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a",
		CapturedAt: time.Now(), TotalEquity: "1000", AvailableBalance: "500", Balance: "1000", PositionCount: 0}).Error; err != nil {
		t.Fatal(err)
	}
	executionDAL := &dal.Execution{DB: db}
	factory := new(executionTradingFactoryStub)
	return &Execution{ExecutionDAL: executionDAL, CatalogDAL: &dal.Catalog{DB: db},
		MarketRules: &executionRulesStub{rules: &provider.InstrumentRules{Symbol: "BTCUSDT", MinQuantity: "0.001", StepSize: "0.001", TickSize: "0.1", MinNotional: "10"},
			ticker: &schema.MarketTicker{LastPrice: "2000"}}, TradingFactory: factory}, db, factory
}

func validSimulationCommand() *schema.TradeCommandForm {
	return &schema.TradeCommandForm{TraderID: "trader-a", IdempotencyKey: "decision-1-open-btc",
		Action: "open", Market: "spot", Symbol: "BTCUSDT", Side: "buy", OrderType: "market",
		Quantity: "0.01", QuoteQuantity: "0", Price: "0", StopLoss: "1900", TakeProfit: "2200", Leverage: 1}
}

func errorID(err error) string {
	parsed, ok := apperrors.As(err)
	if !ok {
		return ""
	}
	return parsed.ID
}
