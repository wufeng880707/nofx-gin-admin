package biz

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type exchangeConnectionStub struct {
	healthErr           error
	authenticationErr   error
	healthCalls         int
	authenticationCalls int
	snapshotCalls       int
}

func (*exchangeConnectionStub) Name() string { return "stub" }

func (a *exchangeConnectionStub) Health(context.Context) error {
	a.healthCalls++
	return a.healthErr
}

func (a *exchangeConnectionStub) ValidateCredentials(context.Context) error {
	a.authenticationCalls++
	return a.authenticationErr
}

func (a *exchangeConnectionStub) Snapshot(context.Context) (*provider.Account, []*provider.Position, error) {
	a.snapshotCalls++
	return &provider.Account{}, nil, nil
}

func TestResolveOwnerScopeUsesCurrentUser(t *testing.T) {
	ctx := util.NewUserID(context.Background(), "user-a")
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		t.Fatalf("resolve owner scope: %v", err)
	}
	if owner != "user-a" {
		t.Fatalf("unexpected owner: %q", owner)
	}
}

func TestResolveOwnerScopeRejectsCrossUserAccess(t *testing.T) {
	ctx := util.NewUserID(context.Background(), "user-a")
	_, err := resolveOwnerScope(ctx, "user-b")
	if err == nil {
		t.Fatal("expected cross-user access to be rejected")
	}
	parsed, ok := errors.As(err)
	if !ok || parsed.Code != 403 {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestResolveOwnerScopeAllowsRootFilter(t *testing.T) {
	ctx := util.NewIsRootUser(util.NewUserID(context.Background(), "root"))
	owner, err := resolveOwnerScope(ctx, "user-b")
	if err != nil {
		t.Fatalf("resolve root owner scope: %v", err)
	}
	if owner != "user-b" {
		t.Fatalf("unexpected owner: %q", owner)
	}
}

func TestResolveOwnerScopeDefaultsRootToCurrentUser(t *testing.T) {
	ctx := util.NewIsRootUser(util.NewUserID(context.Background(), "root"))
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		t.Fatalf("resolve root owner scope: %v", err)
	}
	if owner != "root" {
		t.Fatalf("unexpected root default owner: %q", owner)
	}
}

func TestSanitizeCustomAPIURL(t *testing.T) {
	got := sanitizeCustomAPIURL("https://user:secret@example.com/v1?token=secret#fragment")
	if got != "https://example.com/v1" {
		t.Fatalf("unexpected sanitized URL: %q", got)
	}
}

func TestCatalogConfigurationLifecycleEncryptsCredentials(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:catalog-lifecycle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.AIModel), new(schema.Strategy), new(schema.StrategyVersion), new(schema.Trader),
		new(schema.NotificationChannel), new(schema.TraderNotificationBinding)); err != nil {
		t.Fatal(err)
	}
	service := credential.NewService(make([]byte, 32), nil)
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}, Credentials: service}
	ctx := util.NewUserID(context.Background(), "user-a")

	exchangeForm := &schema.ExchangeForm{ExchangeType: "binance", AccountName: "main", Name: "Main",
		AccountType: "futures", Enabled: true, Testnet: true, APIKey: "api-key", SecretKey: "secret-key"}
	if err := exchangeForm.Validate(); err != nil {
		t.Fatal(err)
	}
	exchange, err := catalog.CreateExchange(ctx, exchangeForm)
	if err != nil {
		t.Fatal(err)
	}
	var stored schema.Exchange
	if err := db.First(&stored, "id = ?", exchange.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.APIKeyCiphertext, "ENC:v1:") || stored.APIKeyCiphertext == "api-key" {
		t.Fatalf("credential was not encrypted: %q", stored.APIKeyCiphertext)
	}
	if !exchange.HasAPIKey || !exchange.HasSecretKey {
		t.Fatal("masked credential flags are missing")
	}

	model, err := catalog.CreateAIModel(ctx, &schema.AIModelForm{Name: "DeepSeek", Provider: "deepseek", Enabled: true, APIKey: "model-key"})
	if err != nil {
		t.Fatal(err)
	}
	var storedModel schema.AIModel
	if err := db.First(&storedModel, "id = ?", model.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(storedModel.APIKeyCiphertext, "ENC:v1:") || storedModel.APIKeyCiphertext == "model-key" || !model.HasAPIKey {
		t.Fatalf("model credential was not encrypted or masked: stored=%q view=%#v", storedModel.APIKeyCiphertext, model)
	}
	originalModelCredential := storedModel.APIKeyCiphertext
	if err := catalog.UpdateAIModel(ctx, model.ID, &schema.AIModelForm{
		Name: "DeepSeek", Provider: "deepseek", Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&storedModel, "id = ?", model.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedModel.Enabled || storedModel.APIKeyCiphertext != originalModelCredential {
		t.Fatalf("model disable should retain credential: %#v", storedModel)
	}
	if err := catalog.UpdateAIModel(ctx, model.ID, &schema.AIModelForm{
		Name: "DeepSeek", Provider: "deepseek", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	unusedModel, err := catalog.CreateAIModel(ctx, &schema.AIModelForm{
		Name: "Unused", Provider: "openai", APIKey: "unused-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteAIModel(ctx, unusedModel.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(new(schema.AIModel), "id = ?", unusedModel.ID).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("unused model was not deleted: %v", err)
	}
	strategy, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{Name: "Default", StrategyType: "ai_trading",
		IsActive: true, ConfigVisible: true, Config: json.RawMessage(`{"risk":{"max_positions":3}}`)})
	if err != nil {
		t.Fatal(err)
	}
	strategyID := strategy.ID
	channel, err := catalog.CreateNotificationChannel(ctx, &schema.NotificationChannelForm{
		Name: "Ops Lark", ChannelType: schema.NotificationChannelLark, Enabled: true,
		WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/test-hook-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	var storedChannel schema.NotificationChannel
	if err := db.First(&storedChannel, "id = ?", channel.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(storedChannel.WebhookURLCiphertext, "ENC:v1:") || storedChannel.WebhookURLCiphertext == "https://open.feishu.cn/open-apis/bot/v2/hook/test-hook-id" {
		t.Fatalf("notification credential was not encrypted: %q", storedChannel.WebhookURLCiphertext)
	}
	trader, err := catalog.CreateTrader(ctx, &schema.TraderForm{Name: "Observer", ExchangeID: exchange.ID,
		AIModelID: model.ID, StrategyID: &strategyID, InitialBalance: "1000", ScanIntervalMinutes: 15,
		IsSimulation: true, ExecutionMode: schema.TraderExecutionApprovalRequired,
		NotificationChannelIDs: []string{channel.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if trader.OwnerUserID != "user-a" || trader.DesiredState != schema.TraderDesiredStateStopped {
		t.Fatalf("unexpected trader: %#v", trader)
	}
	if trader.ExecutionMode != schema.TraderExecutionApprovalRequired || len(trader.NotificationChannelIDs) != 1 || trader.NotificationChannelIDs[0] != channel.ID {
		t.Fatalf("unexpected trader notification binding: %#v", trader)
	}
}

func TestScheduledTaskLifecycleIsOwnerScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:scheduled-task-lifecycle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.ScheduledTask), new(schema.ScheduledTaskRun), new(schema.ScheduledTaskAudit)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ownerCtx := util.NewUserID(context.Background(), "user-a")
	form := &schema.ScheduledTaskForm{
		Name: "Account Sync", Handler: schema.TaskHandlerAccountSync,
		CronExpression: "*/5 * * * *", TimeZone: "UTC", TimeoutSeconds: 60,
		MaxRetries: 2, RetryIntervalSeconds: 5, MisfirePolicy: schema.TaskMisfireFireOnce,
		ConcurrencyPolicy: schema.TaskConcurrencyForbid,
	}
	if err := form.Validate(); err != nil {
		t.Fatal(err)
	}
	task, err := catalog.CreateScheduledTask(ownerCtx, form)
	if err != nil {
		t.Fatal(err)
	}
	if task.OwnerUserID != "user-a" || task.Status != schema.TaskStatusActive || task.NextRunAt == nil {
		t.Fatalf("unexpected created task: %#v", task)
	}
	if err := catalog.PauseScheduledTask(ownerCtx, task.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := catalog.CatalogDAL.GetScheduledTask(ownerCtx, task.ID, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != schema.TaskStatusPaused || stored.NextRunAt != nil {
		t.Fatalf("task was not paused: %#v", stored)
	}
	if err := catalog.ResumeScheduledTask(ownerCtx, task.ID); err != nil {
		t.Fatal(err)
	}
	stored, err = catalog.CatalogDAL.GetScheduledTask(ownerCtx, task.ID, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != schema.TaskStatusActive || stored.NextRunAt == nil {
		t.Fatalf("task was not resumed: %#v", stored)
	}
	otherCtx := util.NewUserID(context.Background(), "user-b")
	if err := catalog.DisableScheduledTask(otherCtx, task.ID); err == nil {
		t.Fatal("expected cross-owner task mutation to be rejected")
	}
	var audits []*schema.ScheduledTaskAudit
	if err := db.Order("created_at ASC").Find(&audits, "task_id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 3 {
		t.Fatalf("unexpected audit count: %d", len(audits))
	}
	wantActions := []string{schema.TaskAuditCreate, schema.TaskAuditPause, schema.TaskAuditResume}
	for index, audit := range audits {
		if audit.Action != wantActions[index] || audit.ActorUserID != "user-a" || audit.OwnerUserID != "user-a" {
			t.Fatalf("unexpected audit at %d: %#v", index, audit)
		}
	}
}

func TestExchangeConnectionChecksNetworkBeforeAuthentication(t *testing.T) {
	client := new(exchangeConnectionStub)
	result, err := testExchangeConnection(context.Background(), &schema.Exchange{
		ID: "exchange-1", ExchangeType: "binance", AccountType: "futures",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	if client.healthCalls != 1 || client.authenticationCalls != 1 || client.snapshotCalls != 0 {
		t.Fatalf("unexpected call counts: health=%d authentication=%d snapshot=%d", client.healthCalls, client.authenticationCalls, client.snapshotCalls)
	}
	if result.Environment != "production" || !result.NetworkOK || !result.AuthenticationOK {
		t.Fatalf("unexpected connection result: %#v", result)
	}
}

func TestExchangeConnectionStopsWhenNetworkIsUnavailable(t *testing.T) {
	client := &exchangeConnectionStub{healthErr: stderrors.New("network unavailable")}
	_, err := testExchangeConnection(context.Background(), &schema.Exchange{Testnet: true}, client)
	parsed, ok := errors.As(err)
	if !ok || parsed.ID != "trading_exchange_network_failed" {
		t.Fatalf("expected network error, got %v", err)
	}
	if client.authenticationCalls != 0 || client.snapshotCalls != 0 {
		t.Fatalf("authentication should not run after network failure, authentication=%d snapshot=%d", client.authenticationCalls, client.snapshotCalls)
	}
}

func TestExchangeConnectionReportsAuthenticationFailureAfterNetworkSuccess(t *testing.T) {
	client := &exchangeConnectionStub{authenticationErr: stderrors.New("invalid credentials")}
	_, err := testExchangeConnection(context.Background(), &schema.Exchange{}, client)
	parsed, ok := errors.As(err)
	if !ok || parsed.ID != "trading_exchange_authentication_failed" {
		t.Fatalf("expected authentication error, got %v", err)
	}
	if client.healthCalls != 1 || client.authenticationCalls != 1 || client.snapshotCalls != 0 {
		t.Fatalf("unexpected call counts: health=%d authentication=%d snapshot=%d", client.healthCalls, client.authenticationCalls, client.snapshotCalls)
	}
}

func TestHistorySymbolsAndSyncStatusLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:history-status-lifecycle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.HistorySymbol), new(schema.SyncCursor),
		new(schema.ExchangeOrder), new(schema.Fill), new(schema.IncomeEvent), new(schema.ClosedTrade)); err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "history-exchange", OwnerUserID: "history-owner", ExchangeType: "binance",
		AccountName: "main", Name: "Binance Spot", AccountType: "spot", Enabled: true}
	if err := db.Create(exchange).Error; err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ctx := util.NewUserID(context.Background(), "history-owner")
	symbols, err := catalog.ReplaceExchangeHistorySymbols(ctx, exchange.ID,
		&schema.HistorySymbolsForm{Symbols: []string{"BTCUSDT", "ethusdt", "BTCUSDT"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 2 || symbols[0] != "BTCUSDT" || symbols[1] != "ETHUSDT" {
		t.Fatalf("unexpected registered symbols: %#v", symbols)
	}
	now := time.Now().UTC()
	if err := db.Create(&schema.SyncCursor{TraderID: "exchange:" + exchange.ID, ExchangeID: exchange.ID,
		Dataset: "history", Market: "", Symbol: "", Status: "success", SyncedUntil: &now, LastRunAt: &now}).Error; err != nil {
		t.Fatal(err)
	}
	statuses, err := catalog.QueryHistorySyncStatuses(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].CoverageStatus != "healthy" || len(statuses[0].RegisteredSymbols) != 2 {
		t.Fatalf("unexpected history status: %#v", statuses)
	}
	if err := db.Create(&schema.Fill{ID: "orphan-fill", OwnerUserID: exchange.OwnerUserID,
		TraderID: "exchange:" + exchange.ID, ExchangeID: exchange.ID, Market: "spot",
		ExchangeTradeID: "trade-1", ExchangeOrderID: "missing-order", Symbol: "BTCUSDT", Side: "buy",
		Price: "100", Quantity: "1", QuoteQuantity: "100", Source: "test", FilledAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	statuses, err = catalog.QueryHistorySyncStatuses(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].CoverageStatus != "incomplete" || statuses[0].GapCount != 1 || statuses[0].OrphanFillCount != 1 {
		t.Fatalf("expected detected history gap: %#v", statuses[0])
	}
}

func TestStrategyLifecycleKeepsOneActiveStrategyPerOwner(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:strategy-active-lifecycle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ctx := util.NewUserID(context.Background(), "user-strategy-active")

	first, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{
		Name: "First", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"risk":{"max_positions":1}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{
		Name: "Second", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"risk":{"max_positions":2}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertOnlyActiveStrategy(t, db, "user-strategy-active", second.ID)

	if err := catalog.ActivateStrategy(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	assertOnlyActiveStrategy(t, db, "user-strategy-active", first.ID)

	if err := catalog.UpdateStrategy(ctx, second.ID, &schema.StrategyForm{
		Name: "Second", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"risk":{"max_positions":2}}`),
	}); err != nil {
		t.Fatal(err)
	}
	assertOnlyActiveStrategy(t, db, "user-strategy-active", second.ID)
}

func TestDuplicateStrategyCopiesConfigWithoutRuntimeFlags(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:strategy-duplicate?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ctx := util.NewUserID(context.Background(), "user-strategy-duplicate")
	source, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{
		Name: "Source", Description: "Source description", StrategyType: "ai_trading",
		IsActive: true, IsDefault: true, IsPublic: true, ConfigVisible: true,
		Config: json.RawMessage(`{"risk":{"max_positions":3},"custom":"keep-me"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := catalog.DuplicateStrategy(ctx, source.ID, &schema.StrategyDuplicateForm{Name: "Source Copy"})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID == source.ID || duplicate.Name != "Source Copy" {
		t.Fatalf("unexpected duplicate identity: %#v", duplicate)
	}
	if duplicate.IsActive || duplicate.IsDefault || duplicate.IsPublic {
		t.Fatalf("duplicate inherited runtime/public flags: %#v", duplicate.StrategyView)
	}
	config, ok := duplicate.Config.(map[string]interface{})
	if !ok || config["custom"] != "keep-me" {
		t.Fatalf("duplicate did not preserve config: %#v", duplicate.Config)
	}
	assertOnlyActiveStrategy(t, db, "user-strategy-duplicate", source.ID)
}

func TestStrategyVersionsPublishAndRollbackWithoutMutatingHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:strategy-version-lifecycle?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ctx := util.NewUserID(context.Background(), "user-strategy-version")

	created, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{
		Name: "Versioned", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"revision":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CurrentVersion != 1 || created.PublishedVersion != nil {
		t.Fatalf("unexpected initial version state: %#v", created.StrategyView)
	}
	published, err := catalog.PublishStrategy(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.PublishedVersion == nil || *published.PublishedVersion != 1 {
		t.Fatalf("expected v1 to be published: %#v", published.StrategyView)
	}
	if err := catalog.UpdateStrategy(ctx, created.ID, &schema.StrategyForm{
		Name: "Versioned", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"revision":2}`),
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := catalog.GetStrategy(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentVersion != 2 || updated.PublishedVersion == nil || *updated.PublishedVersion != 1 {
		t.Fatalf("draft update should not replace published version: %#v", updated.StrategyView)
	}
	rolledBack, err := catalog.RollbackStrategy(ctx, created.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.CurrentVersion != 3 || rolledBack.PublishedVersion == nil || *rolledBack.PublishedVersion != 1 {
		t.Fatalf("rollback should create v3 and preserve published v1: %#v", rolledBack.StrategyView)
	}
	config, ok := rolledBack.Config.(map[string]interface{})
	if !ok || config["revision"] != float64(1) {
		t.Fatalf("rollback did not restore v1 config: %#v", rolledBack.Config)
	}
	versions, err := catalog.ListStrategyVersions(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 || versions[0].Version != 3 || versions[0].RollbackFrom == nil || *versions[0].RollbackFrom != 1 {
		t.Fatalf("unexpected immutable version history: %#v", versions)
	}
}

func TestBackfillStrategyVersionsPublishesExistingActiveStrategy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:strategy-version-backfill?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion)); err != nil {
		t.Fatal(err)
	}
	legacy := &schema.Strategy{
		ID: "legacy-strategy", OwnerUserID: "legacy-owner", Name: "Legacy", StrategyType: "ai_trading",
		IsActive: true, ConfigVisible: true, ConfigJSON: `{"revision":"legacy"}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&dal.Catalog{DB: db}).BackfillStrategyVersions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.First(legacy, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.CurrentVersion != 1 || legacy.PublishedVersion == nil || *legacy.PublishedVersion != 1 {
		t.Fatalf("active legacy strategy was not published as v1: %#v", legacy)
	}
	var versions []schema.StrategyVersion
	if err := db.Where("strategy_id = ?", legacy.ID).Find(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].PublishedAt == nil {
		t.Fatalf("unexpected backfilled versions: %#v", versions)
	}
}

func TestTraderStartValidationRequiresPublishedStrategy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:trader-published-strategy?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.AIModel), new(schema.Strategy), new(schema.StrategyVersion)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}}
	ctx := util.NewUserID(context.Background(), "runtime-owner")
	if err := db.Create(&schema.Exchange{ID: "exchange", OwnerUserID: "runtime-owner", ExchangeType: "binance", AccountName: "main", Name: "Main", AccountType: "spot", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.AIModel{ID: "model", OwnerUserID: "runtime-owner", Name: "Model", Provider: "deepseek", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	strategy, err := catalog.CreateStrategy(ctx, &schema.StrategyForm{
		Name: "Runtime", StrategyType: "ai_trading", IsActive: true, ConfigVisible: true,
		Config: json.RawMessage(`{"runtime":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	form := &schema.TraderForm{ExchangeID: "exchange", AIModelID: "model", StrategyID: &strategy.ID}
	_, _, _, err = catalog.validateTraderReferences(ctx, "runtime-owner", form, true)
	parsed, ok := errors.As(err)
	if !ok || parsed.ID != "trading_strategy_unpublished" {
		t.Fatalf("expected unpublished strategy error, got %v", err)
	}
	if _, err := catalog.PublishStrategy(ctx, strategy.ID); err != nil {
		t.Fatal(err)
	}
	_, _, published, err := catalog.validateTraderReferences(ctx, "runtime-owner", form, true)
	if err != nil {
		t.Fatal(err)
	}
	if published.PublishedVersion == nil || *published.PublishedVersion != 1 {
		t.Fatalf("expected trader to resolve published v1: %#v", published)
	}
}

func assertOnlyActiveStrategy(t *testing.T, db *gorm.DB, ownerUserID, expectedID string) {
	t.Helper()
	var active []schema.Strategy
	if err := db.Where("owner_user_id = ? AND is_active = ?", ownerUserID, true).Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != expectedID {
		t.Fatalf("expected only strategy %q to be active, got %#v", expectedID, active)
	}
}
