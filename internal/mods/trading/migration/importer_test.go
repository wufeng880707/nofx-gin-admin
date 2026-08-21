package migration

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestImporterDryRunAndIdempotency(t *testing.T) {
	source, err := gorm.Open(sqlite.Open("file:legacy-import-source?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	target, err := gorm.Open(sqlite.Open("file:legacy-import-target?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	legacyTables := []struct {
		name  string
		model interface{}
	}{
		{"exchanges", new(legacyExchange)}, {"traders", new(legacyTrader)},
		{"trader_equity_snapshots", new(legacyAccountSnapshot)}, {"decision_records", new(legacyDecision)},
	}
	for _, table := range legacyTables {
		if err := source.Table(table.name).AutoMigrate(table.model); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := source.Table("exchanges").Create(&legacyExchange{ID: "exchange-1", ExchangeType: "binance", AccountName: "Main", UserID: "legacy-user", Name: "Binance", Type: "cex", Enabled: true, APIKey: "plain-api", SecretKey: "plain-secret", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Table("traders").Create(&legacyTrader{ID: "trader-1", UserID: "legacy-user", Name: "Trader", AIModelID: "model-1", ExchangeID: "exchange-1", InitialBalance: 100, ScanIntervalMinutes: 15, IsCrossMargin: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Table("trader_equity_snapshots").Create(&legacyAccountSnapshot{ID: 1, TraderID: "trader-1", Timestamp: now, TotalEquity: 110, Balance: 100, AvailableBalance: 80, UnrealizedPnL: 10, PositionCount: 1, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Table("decision_records").Create(&legacyDecision{ID: 1, TraderID: "trader-1", CycleNumber: 1, Timestamp: now, DecisionJSON: `{}`, CandidateCoins: `[]`, ExecutionLog: `[]`, Decisions: `[]`, Success: true, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := target.AutoMigrate(new(schema.Exchange), new(schema.Trader), new(schema.AccountSnapshot), new(schema.Decision), new(schema.LegacyImportRun)); err != nil {
		t.Fatal(err)
	}
	importer := &Importer{Source: source, Target: target, Credentials: credential.NewService([]byte(strings.Repeat("k", 32)), nil)}
	options := Options{SourceName: "legacy", DefaultOwnerUserID: "user-1", DryRun: true}
	report, err := importer.Run(options)
	if err != nil {
		t.Fatal(err)
	}
	if report.Domains["exchanges"].Imported != 1 {
		t.Fatalf("unexpected dry-run report: %#v", report.Domains)
	}
	var count int64
	target.Model(new(schema.Exchange)).Count(&count)
	if count != 0 {
		t.Fatalf("dry-run wrote %d exchanges", count)
	}

	options.DryRun = false
	if _, err := importer.Run(options); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Run(options); err != nil {
		t.Fatal(err)
	}
	target.Model(new(schema.Exchange)).Count(&count)
	if count != 1 {
		t.Fatalf("idempotent import wrote %d exchanges", count)
	}
	var exchange schema.Exchange
	if err := target.First(&exchange).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(exchange.APIKeyCiphertext, "ENC:v1:") {
		t.Fatalf("credential was not encrypted: %q", exchange.APIKeyCiphertext)
	}
	var snapshot schema.AccountSnapshot
	if err := target.First(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if snapshot.ExchangeID != "exchange-1" || snapshot.OwnerUserID != "user-1" {
		t.Fatalf("snapshot mapping failed: %#v", snapshot)
	}
	var decision schema.Decision
	if err := target.First(&decision).Error; err != nil {
		t.Fatal(err)
	}
	if decision.ExchangeID != "exchange-1" {
		t.Fatalf("decision exchange mapping failed: %#v", decision)
	}
}

func TestImporterRotatesLegacyCiphertext(t *testing.T) {
	oldKey := []byte(strings.Repeat("o", 32))
	newKey := []byte(strings.Repeat("n", 32))
	block, err := aes.NewCipher(oldKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	sealed := gcm.Seal(nil, nonce, []byte("legacy-secret"), nil)
	legacy := "ENC:v1:" + base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(sealed)
	service := credential.NewService(newKey, oldKey)
	importer := &Importer{Credentials: service}

	rotated, err := importer.normalizeCredential(legacy, "user-1", "exchange-1", "api_key")
	if err != nil {
		t.Fatal(err)
	}
	if rotated == legacy || len(strings.Split(strings.TrimPrefix(rotated, "ENC:v1:"), ":")) != 3 {
		t.Fatalf("legacy ciphertext was not rotated: %q", rotated)
	}
	plaintext, err := service.DecryptField(rotated, "user-1", "exchange-1", "api_key")
	if err != nil || plaintext != "legacy-secret" {
		t.Fatalf("rotated decrypt = %q, %v", plaintext, err)
	}
}

func TestReportOwnerUsesMappedOwnerDeterministically(t *testing.T) {
	owner := reportOwner(Options{OwnerMap: map[string]string{"legacy-b": "user-b", "legacy-a": "user-a"}})
	if owner != "user-a" {
		t.Fatalf("unexpected report owner: %q", owner)
	}
}
