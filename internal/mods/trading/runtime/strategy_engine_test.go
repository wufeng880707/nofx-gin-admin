package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunStrategyCyclePersistsDryRunDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "No trading command service") && !strings.Contains(string(body), "dry-run strategy analysis") {
			t.Fatalf("runtime safety instruction missing: %s", body)
		}
		_, _ = io.WriteString(w, `{"id":"decision-request","choices":[{"finish_reason":"stop","message":{"content":"{\"decisions\":[{\"symbol\":\"BTCUSDT\",\"action\":\"HOLD\",\"confidence\":80,\"reason\":\"synthetic test\"}]}"}}],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}}`)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open("file:strategy-engine?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.AIModel), new(schema.AIInvocation), new(schema.Strategy),
		new(schema.StrategyVersion), new(schema.StrategyVersionSwitchEvent), new(schema.Trader), new(schema.AssetBalance), new(schema.Position), new(schema.Decision)); err != nil {
		t.Fatal(err)
	}
	key := sha256.Sum256([]byte("runtime-test-key"))
	credentials := credential.NewService(key[:], nil)
	model := &schema.AIModel{ID: "model-1", OwnerUserID: "user-1", Name: "Test", Provider: "custom", Enabled: true,
		CustomAPIURL: server.URL, CustomModelName: "test-model", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	model.APIKeyCiphertext, err = credentials.EncryptField("provider-key", model.OwnerUserID, model.ID, "api_key")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(model).Error; err != nil {
		t.Fatal(err)
	}
	config := schema.DefaultStrategyStudioConfig()
	configJSON, _ := json.Marshal(config)
	version := 1
	now := time.Now().UTC()
	strategy := &schema.Strategy{ID: "strategy-1", OwnerUserID: "user-1", Name: "Strategy", StrategyType: "ai_trading",
		IsActive: true, ConfigJSON: string(configJSON), CurrentVersion: version, PublishedVersion: &version, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(strategy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.StrategyVersion{ID: "version-1", StrategyID: strategy.ID, OwnerUserID: "user-1", Version: version,
		Name: strategy.Name, StrategyType: strategy.StrategyType, ConfigJSON: strategy.ConfigJSON, PublishedAt: &now, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	trader := &schema.Trader{ID: "trader-1", OwnerUserID: "user-1", Name: "Trader", AIModelID: model.ID,
		ExchangeID: "exchange-1", StrategyID: &strategy.ID, StrategyVersion: version, InitialBalance: "1000",
		ScanIntervalMinutes: 15, IsSimulation: true, ExecutionMode: schema.TraderExecutionNotifyOnly,
		AIConsentModelID: model.ID, AIConsentAt: &now, DesiredState: "running", RuntimeState: "running", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Exchange{ID: "exchange-1", OwnerUserID: "user-1", ExchangeType: "binance", AccountType: "spot", Enabled: true,
		AccountName: "main", Name: "Main", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	service := llm.NewService(db, credentials)
	service.AllowPrivateEndpoints = true
	registry := NewRegistry(db, nil, service)
	if err := registry.RunStrategyCycle(context.Background(), trader.ID); err != nil {
		t.Fatalf("run cycle: %v", err)
	}
	var decision schema.Decision
	if err := db.Where("trader_id = ?", trader.ID).First(&decision).Error; err != nil {
		t.Fatal(err)
	}
	if !decision.Success || !strings.Contains(decision.ExecutionLogJSON, "No trading command service was invoked") || decision.RawResponse == "" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
	if decision.CoTTrace != "" {
		t.Fatalf("hidden reasoning must not be persisted: %q", decision.CoTTrace)
	}
	if err := db.First(trader, "id = ?", trader.ID).Error; err != nil {
		t.Fatal(err)
	}
	if trader.CycleNumber != 1 || trader.StrategyState != "running" || !trader.SafeMode || trader.LastDecisionAt == nil {
		t.Fatalf("unexpected trader state: %#v", trader)
	}
}

func TestLoadRuntimeStrategyHotSwitchIsExplicitAndSimulationOnly(t *testing.T) {
	tests := []struct {
		name       string
		simulation bool
		enabled    bool
		want       int
		wantEvents int64
	}{
		{name: "explicit simulation switch", simulation: true, enabled: true, want: 2, wantEvents: 1},
		{name: "default disabled", simulation: true, enabled: false, want: 1},
		{name: "live remains pinned", simulation: false, enabled: true, want: 1},
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:strategy-hot-switch-"+strconv.Itoa(index)+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion), new(schema.StrategyVersionSwitchEvent), new(schema.Trader)); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			published := 2
			configV1 := schema.DefaultStrategyStudioConfig()
			configV2 := schema.DefaultStrategyStudioConfig()
			configV2.AIConfig.RiskControl.MinConfidence = 91
			encodedV1, _ := json.Marshal(configV1)
			encodedV2, _ := json.Marshal(configV2)
			strategy := &schema.Strategy{ID: "strategy", OwnerUserID: "owner", Name: "Hot", StrategyType: "ai_trading",
				IsActive: true, ConfigJSON: string(encodedV2), CurrentVersion: 2, PublishedVersion: &published, CreatedAt: now, UpdatedAt: now}
			if err := db.Create(strategy).Error; err != nil {
				t.Fatal(err)
			}
			for version, encoded := range map[int][]byte{1: encodedV1, 2: encodedV2} {
				item := &schema.StrategyVersion{ID: "version-" + strconv.Itoa(version), StrategyID: strategy.ID,
					OwnerUserID: strategy.OwnerUserID, Version: version, Name: strategy.Name, StrategyType: strategy.StrategyType,
					ConfigJSON: string(encoded), PublishedAt: &now, CreatedAt: now}
				if err := db.Create(item).Error; err != nil {
					t.Fatal(err)
				}
			}
			trader := &schema.Trader{ID: "trader", OwnerUserID: strategy.OwnerUserID, Name: "Trader",
				AIModelID: "model", ExchangeID: "exchange", StrategyID: &strategy.ID, StrategyVersion: 1,
				InitialBalance: "1000", IsSimulation: tc.simulation, AutoApplyStrategyUpdates: tc.enabled,
				CreatedAt: now, UpdatedAt: now}
			if err := db.Create(trader).Error; err != nil {
				t.Fatal(err)
			}
			if !tc.simulation {
				if err := db.Model(new(schema.Trader)).Where("id = ?", trader.ID).UpdateColumn("is_simulation", false).Error; err != nil {
					t.Fatal(err)
				}
				trader.IsSimulation = false
			}
			registry := NewRegistry(db, nil, nil)
			_, configJSON, err := registry.loadRuntimeStrategy(context.Background(), trader)
			if err != nil {
				t.Fatalf("load runtime strategy: %v", err)
			}
			loaded, err := schema.DecodeStrategyStudioConfig(json.RawMessage(configJSON))
			if err != nil {
				t.Fatal(err)
			}
			if trader.StrategyVersion != tc.want {
				t.Fatalf("expected version %d, got %d", tc.want, trader.StrategyVersion)
			}
			if tc.want == 2 && loaded.AIConfig.RiskControl.MinConfidence != 91 {
				t.Fatalf("published configuration was not loaded: %#v", loaded.AIConfig.RiskControl)
			}
			var count int64
			if err := db.Model(new(schema.StrategyVersionSwitchEvent)).Count(&count).Error; err != nil || count != tc.wantEvents {
				t.Fatalf("unexpected switch audit count=%d err=%v", count, err)
			}
		})
	}
}

func TestLoadRuntimeStrategyRejectsBrokenPublishedVersionWithoutSwitching(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:strategy-hot-switch-invalid?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Strategy), new(schema.StrategyVersion), new(schema.StrategyVersionSwitchEvent), new(schema.Trader)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	published := 2
	valid, _ := json.Marshal(schema.DefaultStrategyStudioConfig())
	strategy := &schema.Strategy{ID: "strategy-invalid", OwnerUserID: "owner", Name: "Invalid", StrategyType: "ai_trading",
		IsActive: true, ConfigJSON: `{}`, CurrentVersion: 2, PublishedVersion: &published, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(strategy).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range []*schema.StrategyVersion{
		{ID: "v1", StrategyID: strategy.ID, OwnerUserID: strategy.OwnerUserID, Version: 1, Name: strategy.Name,
			StrategyType: strategy.StrategyType, ConfigJSON: string(valid), PublishedAt: &now, CreatedAt: now},
		{ID: "v2", StrategyID: strategy.ID, OwnerUserID: strategy.OwnerUserID, Version: 2, Name: strategy.Name,
			StrategyType: strategy.StrategyType, ConfigJSON: `{broken`, PublishedAt: &now, CreatedAt: now},
	} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	trader := &schema.Trader{ID: "trader-invalid", OwnerUserID: strategy.OwnerUserID, Name: "Trader",
		AIModelID: "model", ExchangeID: "exchange", StrategyID: &strategy.ID, StrategyVersion: 1,
		InitialBalance: "1000", IsSimulation: true, AutoApplyStrategyUpdates: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(db, nil, nil)
	if _, _, err := registry.loadRuntimeStrategy(context.Background(), trader); err == nil || !strings.Contains(err.Error(), "validate published strategy version 2") {
		t.Fatalf("expected invalid published version error, got %v", err)
	}
	var persisted schema.Trader
	if err := db.First(&persisted, "id = ?", trader.ID).Error; err != nil {
		t.Fatal(err)
	}
	if trader.StrategyVersion != 1 || persisted.StrategyVersion != 1 {
		t.Fatalf("broken version must not switch: local=%d persisted=%d", trader.StrategyVersion, persisted.StrategyVersion)
	}
}

func TestValidateRuntimeDecisionsAppliesHardLimits(t *testing.T) {
	config := schema.DefaultStrategyStudioConfig()
	envelope := decisionEnvelope{Decisions: []runtimeDecision{{Symbol: "BTCUSDT", Action: "OPEN", Side: "long",
		Confidence: 90, Leverage: config.AIConfig.RiskControl.BTCETHMaxLeverage + 1,
		PositionSizeUSDT: 100, StopLoss: 90, TakeProfit: 120}}}
	validateRuntimeDecisions(&envelope, &config, 0)
	if envelope.Decisions[0].Valid || envelope.Decisions[0].ValidationError != "leverage exceeds strategy limit" {
		t.Fatalf("risk limit was not applied: %#v", envelope.Decisions[0])
	}
}

func TestMergeStrategyCandidatesAppliesPriorityExclusionAndLimit(t *testing.T) {
	result := mergeStrategyCandidates(
		[]string{"BTCUSDT", "ETH/USDT"},
		[]string{"ETHUSDT", "SOL-USDT", "DOGE"},
		[]string{"BTCUSDT", "XRPUSDT"},
		[]string{"SOLUSDT"},
		3,
	)
	want := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
	if len(result) != len(want) {
		t.Fatalf("unexpected candidates: %#v", result)
	}
	for index := range want {
		if result[index] != want[index] {
			t.Fatalf("unexpected candidate order: got=%#v want=%#v", result, want)
		}
	}
}

func TestSummarizeKlinesUsesConfiguredIndicatorsAndBoundsRawData(t *testing.T) {
	klines := make([]*schema.MarketKline, 0, 80)
	for index := 0; index < 80; index++ {
		price := strconv.Itoa(100 + index)
		klines = append(klines, &schema.MarketKline{Time: int64(index), Open: price, High: strconv.Itoa(102 + index),
			Low: strconv.Itoa(98 + index), Close: price, Volume: strconv.Itoa(1000 + index), Closed: true})
	}
	config := schema.DefaultStrategyStudioConfig().AIConfig.Indicators
	config.EnableBOLL = true
	config.EMAPeriods = []int{5, 21}
	config.RSIPeriods = []int{7}
	config.ATRPeriods = []int{10}
	config.BOLLPeriods = []int{20}
	result := summarizeKlines(klines, config, true)
	if result.EMA["5"] == 0 || result.EMA["21"] == 0 || result.RSI["7"] == 0 || result.ATR["10"] == 0 {
		t.Fatalf("configured indicators missing: %#v", result)
	}
	if result.MACD == nil || result.BOLL["20"].Middle == 0 || result.Volume != "1079" {
		t.Fatalf("MACD/BOLL/volume missing: %#v", result)
	}
	if len(result.RawKlines) != 60 || result.RawKlines[0].Time != 20 {
		t.Fatalf("raw K-line bound is invalid: count=%d first=%#v", len(result.RawKlines), result.RawKlines[0])
	}
	secondary := summarizeKlines(klines, config, false)
	if len(secondary.RawKlines) != 0 {
		t.Fatalf("raw K-lines must be limited to the primary timeframe")
	}
}
