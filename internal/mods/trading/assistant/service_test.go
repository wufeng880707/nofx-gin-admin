package assistant

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTaskSkillsAreOwnerScopedAndReadOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:assistant-skills?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.AssetBalance), new(schema.Position), new(schema.Trader), new(schema.Strategy),
		new(schema.Exchange), new(schema.AIModel), new(schema.ExchangeOrder)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, row := range []*schema.AssetBalance{
		{OwnerUserID: "owner-a", TraderID: "account-a", ExchangeID: "exchange-a", AccountType: "spot", Asset: "BTC", Total: "0.5", ValueUSDT: "30000", CapturedAt: now, Source: "test"},
		{OwnerUserID: "owner-b", TraderID: "account-b", ExchangeID: "exchange-b", AccountType: "spot", Asset: "USDT", Total: "999999", ValueUSDT: "999999", CapturedAt: now, Source: "test"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(db)
	response, err := service.Execute(context.Background(), "owner-a", "/assets")
	if err != nil {
		t.Fatal(err)
	}
	if response.Skill != "assets" || !strings.Contains(response.Text, "30000.00 USDT") || strings.Contains(response.Text, "999999") {
		t.Fatalf("unexpected owner-scoped response: %#v", response)
	}
	unknown, err := service.Execute(context.Background(), "owner-a", "/delete_everything")
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Skill != "help" || !strings.Contains(unknown.Text, "不会创建订单") {
		t.Fatalf("write-like command must not be executed: %#v", unknown)
	}
}

func TestDiagnosticSkillsReturnActionableState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:assistant-diagnostics?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.AIModel), new(schema.Trader), new(schema.Strategy), new(schema.ExchangeOrder)); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schema.Exchange{ID: "exchange-a", OwnerUserID: "owner-a", ExchangeType: "okx", AccountName: "main", Name: "OKX", AccountType: "spot"}).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	response, err := service.Execute(context.Background(), "owner-a", "/diagnose_exchange")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Text, "未启用") || !strings.Contains(response.Text, "API 凭证不完整") || !strings.Contains(response.Text, "Passphrase") {
		t.Fatalf("unexpected diagnosis: %s", response.Text)
	}
}
