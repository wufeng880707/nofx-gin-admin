package biz

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDecisionViewDoesNotExposeSensitiveModelTrace(t *testing.T) {
	view := buildDecisionView(&schema.Decision{
		TraderID: "trader-1", ExchangeID: "exchange-1", SystemPrompt: "system-secret",
		InputPrompt: "input-secret", CoTTrace: "cot-secret", RawResponse: "raw-secret",
		CandidateCoinsJSON: `[]`, ExecutionLogJSON: `[]`, ActionsJSON: `[]`,
	})
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, secret := range []string{"system-secret", "input-secret", "cot-secret", "raw-secret"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("decision view leaked %q: %s", secret, encoded)
		}
	}
	if view.ExchangeID != "exchange-1" {
		t.Fatalf("decision view lost exchange ID: %#v", view)
	}
}

func TestDecisionDetailIsOwnerScopedAndOmitsInternalTrace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Decision)); err != nil {
		t.Fatal(err)
	}
	decision := &schema.Decision{
		OwnerUserID: "user-a", TraderID: "trader-a", ExchangeID: "exchange-a", CycleNumber: 7,
		DecidedAt: time.Now().UTC(), Success: true, SystemPrompt: "system-secret", InputPrompt: "auditable-input",
		CoTTrace: "private-cot", RawResponse: "auditable-output", DecisionJSON: `{"action":"hold"}`,
		CandidateCoinsJSON: `[]`, ExecutionLogJSON: `[{"status":"skipped"}]`, ActionsJSON: `[]`,
	}
	if err := db.Create(decision).Error; err != nil {
		t.Fatal(err)
	}
	service := &Observation{ObservationDAL: &dal.Observation{DB: db}}
	detail, err := service.GetDecision(util.NewUserID(context.Background(), "user-a"), "1")
	if err != nil {
		t.Fatalf("get decision detail: %v", err)
	}
	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	for _, expected := range []string{"auditable-input", "auditable-output", `"action":"hold"`} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("detail is missing %q: %s", expected, encoded)
		}
	}
	for _, secret := range []string{"system-secret", "private-cot"} {
		if strings.Contains(encoded, secret) {
			t.Fatalf("decision detail leaked %q: %s", secret, encoded)
		}
	}
	_, err = service.GetDecision(util.NewUserID(context.Background(), "user-b"), "1")
	parsed, ok := errors.As(err)
	if !ok || parsed.Code != 404 {
		t.Fatalf("expected owner-scoped 404, got %v", err)
	}
}

func TestAIInvocationSummaryIsOwnerScopedAndGrouped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.AIInvocation)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	traderID, strategyID := "trader-a", "strategy-a"
	items := []*schema.AIInvocation{
		{ID: "inv-a1", OwnerUserID: "user-a", AIModelID: "model-a", TraderID: &traderID, StrategyID: &strategyID,
			Purpose: "decision", Provider: "openai", ModelName: "gpt", InputTokens: 100, OutputTokens: 50,
			TotalTokens: 150, EstimatedCostUSD: "0.1", DurationMS: 100, Success: true, StartedAt: now, CreatedAt: now},
		{ID: "inv-a2", OwnerUserID: "user-a", AIModelID: "model-a", TraderID: &traderID, StrategyID: &strategyID,
			Purpose: "decision", Provider: "openai", ModelName: "gpt", InputTokens: 200, OutputTokens: 100,
			TotalTokens: 300, EstimatedCostUSD: "0.2", DurationMS: 300, Success: false, StartedAt: now, CreatedAt: now},
		{ID: "inv-b", OwnerUserID: "user-b", AIModelID: "model-b", Purpose: "decision", Provider: "openai",
			ModelName: "gpt", TotalTokens: 999, EstimatedCostUSD: "9", DurationMS: 999, Success: true, StartedAt: now, CreatedAt: now},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	service := &Observation{ObservationDAL: &dal.Observation{DB: db}}
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	result, err := service.QueryAIInvocationSummary(util.NewUserID(context.Background(), "user-a"), schema.AIInvocationSummaryQueryParam{
		StartAt: &start, EndAt: &end,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Overall.Calls != 2 || result.Overall.TotalTokens != 450 || result.Overall.EstimatedCostUSD != "0.3" ||
		result.Overall.SuccessRate != 50 || result.Overall.AverageLatencyMS != 200 {
		t.Fatalf("unexpected AI summary: %#v", result.Overall)
	}
	if len(result.ByModel) != 1 || result.ByModel[0].Key != "model-a" || len(result.ByTrader) != 1 || len(result.ByStrategy) != 1 {
		t.Fatalf("unexpected AI groups: %#v", result)
	}
}
