package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Options struct {
	SourceName         string
	DefaultOwnerUserID string
	OwnerMap           map[string]string
	DryRun             bool
}

type DomainStats struct {
	Scanned  int `json:"scanned"`
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`
}

type Report struct {
	RunID      string                  `json:"run_id"`
	SourceName string                  `json:"source_name"`
	DryRun     bool                    `json:"dry_run"`
	StartedAt  time.Time               `json:"started_at"`
	FinishedAt time.Time               `json:"finished_at"`
	Domains    map[string]*DomainStats `json:"domains"`
	Errors     []string                `json:"errors"`
}

type Importer struct {
	Source      *gorm.DB
	Target      *gorm.DB
	Credentials *credential.Service
}

func (a *Importer) Run(options Options) (*Report, error) {
	if a.Source == nil || a.Target == nil {
		return nil, errors.New("legacy importer requires source and target databases")
	}
	if strings.TrimSpace(options.SourceName) == "" {
		options.SourceName = "nofx"
	}
	report := &Report{
		RunID: uuid.NewString(), SourceName: options.SourceName, DryRun: options.DryRun,
		StartedAt: time.Now().UTC(), Domains: make(map[string]*DomainStats), Errors: make([]string, 0),
	}

	steps := []struct {
		name string
		run  func(Options, *Report) error
	}{
		{"exchanges", a.importExchanges}, {"ai_models", a.importAIModels},
		{"strategies", a.importStrategies}, {"traders", a.importTraders},
		{"account_snapshots", a.importAccountSnapshots}, {"positions", a.importPositions},
		{"decisions", a.importDecisions},
	}
	for _, step := range steps {
		if err := step.run(options, report); err != nil {
			report.Errors = append(report.Errors, step.name+": "+err.Error())
		}
	}
	report.FinishedAt = time.Now().UTC()
	if !options.DryRun {
		if err := a.persistReport(options, report); err != nil {
			return report, err
		}
	}
	if len(report.Errors) > 0 {
		return report, fmt.Errorf("legacy import completed with %d errors", len(report.Errors))
	}
	return report, nil
}

func (a *Importer) importExchanges(options Options, report *Report) error {
	var rows []legacyExchange
	if err := readLegacyTable(a.Source, "exchanges", &rows); err != nil {
		return err
	}
	stats := report.stats("exchanges")
	for _, row := range rows {
		stats.Scanned++
		owner, err := resolveOwner(options, row.UserID)
		if err != nil {
			report.fail(stats, "exchange "+row.ID, err)
			continue
		}
		item := &schema.Exchange{
			ID: row.ID, LegacySource: ptr(options.SourceName), LegacyID: ptr(row.ID), OwnerUserID: owner,
			ExchangeType: row.ExchangeType, AccountName: defaultString(row.AccountName, "Default"),
			Name: row.Name, AccountType: defaultString(row.Type, "cex"), Enabled: row.Enabled, Testnet: row.Testnet,
			WalletAddress: firstNonEmpty(row.HyperliquidWalletAddr, row.LighterWalletAddr),
			AccountUser:   row.AsterUser, SignerAddress: row.AsterSigner,
			APIKeyIndex: row.LighterAPIKeyIndex, UnifiedAccount: row.HyperliquidUnifiedAcct,
			BuilderApproved: row.HyperliquidBuilderApproved, CredentialVersion: 1,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}
		if item.ExchangeType == "" {
			item.ExchangeType = row.ID
		}
		credentials := []struct {
			value *string
			raw   string
			field string
		}{
			{&item.APIKeyCiphertext, row.APIKey, "api_key"}, {&item.SecretKeyCiphertext, row.SecretKey, "secret_key"},
			{&item.PassphraseCiphertext, row.Passphrase, "passphrase"}, {&item.PrivateKeyCiphertext, firstNonEmpty(row.AsterPrivateKey, row.LighterPrivateKey), "private_key"},
			{&item.APIKeyPrivateKeyCiphertext, row.LighterAPIKeyPrivateKey, "api_key_private_key"},
		}
		valid := true
		for _, field := range credentials {
			*field.value, err = a.normalizeCredential(field.raw, owner, item.ID, field.field)
			if err != nil {
				report.fail(stats, "exchange "+row.ID+" "+field.field, err)
				valid = false
				break
			}
		}
		if valid {
			a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
		}
	}
	return nil
}

func (a *Importer) importAIModels(options Options, report *Report) error {
	var rows []legacyAIModel
	if err := readLegacyTable(a.Source, "ai_models", &rows); err != nil {
		return err
	}
	stats := report.stats("ai_models")
	for _, row := range rows {
		stats.Scanned++
		owner, err := resolveOwner(options, row.UserID)
		if err != nil {
			report.fail(stats, "ai_model "+row.ID, err)
			continue
		}
		ciphertext, err := a.normalizeCredential(row.APIKey, owner, row.ID, "api_key")
		if err != nil {
			report.fail(stats, "ai_model "+row.ID, err)
			continue
		}
		item := &schema.AIModel{ID: row.ID, LegacySource: ptr(options.SourceName), LegacyID: ptr(row.ID),
			OwnerUserID: owner, Name: row.Name, Provider: row.Provider, Enabled: row.Enabled,
			APIKeyCiphertext: ciphertext, CustomAPIURL: row.CustomAPIURL, CustomModelName: row.CustomModelName,
			CredentialVersion: 1, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importStrategies(options Options, report *Report) error {
	var rows []legacyStrategy
	if err := readLegacyTable(a.Source, "strategies", &rows); err != nil {
		return err
	}
	stats := report.stats("strategies")
	for _, row := range rows {
		stats.Scanned++
		owner, err := resolveOwner(options, row.UserID)
		if err != nil {
			report.fail(stats, "strategy "+row.ID, err)
			continue
		}
		configJSON := defaultJSON(row.Config, "{}")
		strategyType := "ai_trading"
		var configValue map[string]interface{}
		if json.Unmarshal([]byte(configJSON), &configValue) == nil {
			if value, ok := configValue["strategy_type"].(string); ok && value != "" {
				strategyType = value
			}
		}
		item := &schema.Strategy{ID: row.ID, LegacySource: ptr(options.SourceName), LegacyID: ptr(row.ID),
			OwnerUserID: owner, Name: row.Name, Description: row.Description, StrategyType: strategyType,
			IsActive: row.IsActive, IsDefault: row.IsDefault, IsPublic: row.IsPublic,
			ConfigVisible: row.ConfigVisible, ConfigJSON: configJSON, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importTraders(options Options, report *Report) error {
	var rows []legacyTrader
	if err := readLegacyTable(a.Source, "traders", &rows); err != nil {
		return err
	}
	stats := report.stats("traders")
	for _, row := range rows {
		stats.Scanned++
		owner, err := resolveOwner(options, row.UserID)
		if err != nil {
			report.fail(stats, "trader "+row.ID, err)
			continue
		}
		var strategyID *string
		if row.StrategyID != "" {
			strategyID = ptr(row.StrategyID)
		}
		item := &schema.Trader{ID: row.ID, LegacySource: ptr(options.SourceName), LegacyID: ptr(row.ID),
			OwnerUserID: owner, Name: row.Name, AIModelID: row.AIModelID, ExchangeID: row.ExchangeID,
			StrategyID: strategyID, InitialBalance: decimal(row.InitialBalance), ScanIntervalMinutes: row.ScanIntervalMinutes,
			IsCrossMargin: row.IsCrossMargin, IsSimulation: true, ShowInCompetition: row.ShowInCompetition,
			DesiredState: schema.TraderDesiredStateStopped, RuntimeState: schema.TraderRuntimeUnavailable,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importAccountSnapshots(options Options, report *Report) error {
	var rows []legacyAccountSnapshot
	if err := readLegacyTable(a.Source, "trader_equity_snapshots", &rows); err != nil {
		return err
	}
	stats := report.stats("account_snapshots")
	for _, row := range rows {
		stats.Scanned++
		legacyTrader, owner, err := a.legacyTraderFor(options, row.TraderID)
		if err != nil {
			report.fail(stats, "account_snapshot "+strconv.FormatInt(row.ID, 10), err)
			continue
		}
		item := &schema.AccountSnapshot{LegacySource: ptr(options.SourceName), LegacyID: ptr(strconv.FormatInt(row.ID, 10)),
			OwnerUserID: owner, TraderID: row.TraderID, ExchangeID: legacyTrader.ExchangeID, CapturedAt: row.Timestamp.UTC(),
			TotalEquity: decimal(row.TotalEquity), Balance: decimal(row.Balance), AvailableBalance: decimal(row.AvailableBalance),
			UnrealizedPnL: decimal(row.UnrealizedPnL), PositionCount: row.PositionCount,
			MarginUsedPct: decimal(row.MarginUsedPct), Source: "legacy", CreatedAt: row.CreatedAt}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importPositions(options Options, report *Report) error {
	var rows []legacyPosition
	if err := readLegacyTable(a.Source, "trader_positions", &rows); err != nil {
		return err
	}
	stats := report.stats("positions")
	for _, row := range rows {
		stats.Scanned++
		legacyTrader, owner, err := a.legacyTraderFor(options, row.TraderID)
		if err != nil {
			report.fail(stats, "position "+strconv.FormatInt(row.ID, 10), err)
			continue
		}
		openedAt := unixMilli(row.EntryTime)
		lastSyncedAt := unixMilli(row.UpdatedAt)
		if lastSyncedAt.IsZero() {
			lastSyncedAt = openedAt
		}
		var closedAt *time.Time
		status := strings.ToLower(row.Status)
		if status == schema.PositionStatusClosed && row.ExitTime > 0 {
			value := unixMilli(row.ExitTime)
			closedAt = &value
		}
		exchangeID := row.ExchangeID
		if exchangeID == "" {
			exchangeID = legacyTrader.ExchangeID
		}
		item := &schema.Position{ID: uuid.NewString(), LegacySource: ptr(options.SourceName), LegacyID: ptr(strconv.FormatInt(row.ID, 10)),
			OwnerUserID: owner, TraderID: row.TraderID, ExchangeID: exchangeID,
			ExchangePositionID: row.ExchangePositionID, Symbol: strings.ToUpper(row.Symbol), Side: strings.ToLower(row.Side), Status: status,
			EntryQuantity: decimal(row.EntryQuantity), Quantity: decimal(row.Quantity), EntryPrice: decimal(row.EntryPrice),
			MarkPrice: "0", LiquidationPrice: "0", UnrealizedPnL: "0", RealizedPnL: decimal(row.RealizedPnL),
			Fee: decimal(row.Fee), Leverage: row.Leverage, EntryOrderID: row.EntryOrderID, ExitOrderID: row.ExitOrderID,
			CloseReason: row.CloseReason, Source: defaultString(row.Source, "legacy"), OpenedAt: openedAt,
			ClosedAt: closedAt, LastSyncedAt: lastSyncedAt, CreatedAt: unixMilli(row.CreatedAt), UpdatedAt: lastSyncedAt}
		if item.ExchangePositionID == "" {
			item.ExchangePositionID = "legacy:" + strconv.FormatInt(row.ID, 10)
		}
		if err := item.BeforeSave(nil); err != nil {
			report.fail(stats, "position "+strconv.FormatInt(row.ID, 10), err)
			continue
		}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importDecisions(options Options, report *Report) error {
	var rows []legacyDecision
	if err := readLegacyTable(a.Source, "decision_records", &rows); err != nil {
		return err
	}
	stats := report.stats("decisions")
	for _, row := range rows {
		stats.Scanned++
		legacyTrader, owner, err := a.legacyTraderFor(options, row.TraderID)
		if err != nil {
			report.fail(stats, "decision "+strconv.FormatInt(row.ID, 10), err)
			continue
		}
		item := &schema.Decision{LegacySource: ptr(options.SourceName), LegacyID: ptr(strconv.FormatInt(row.ID, 10)),
			OwnerUserID: owner, TraderID: row.TraderID, ExchangeID: legacyTrader.ExchangeID,
			CycleNumber: row.CycleNumber, DecidedAt: row.Timestamp.UTC(),
			Success: row.Success, ErrorMessage: row.ErrorMessage, AIRequestDurationMs: row.AIRequestDurationMs,
			SystemPrompt: row.SystemPrompt, InputPrompt: row.InputPrompt, CoTTrace: row.CoTTrace,
			DecisionJSON: defaultJSON(row.DecisionJSON, "{}"), RawResponse: row.RawResponse,
			CandidateCoinsJSON: defaultJSON(row.CandidateCoins, "[]"), ExecutionLogJSON: defaultJSON(row.ExecutionLog, "[]"),
			ActionsJSON: defaultJSON(row.Decisions, "[]"), CreatedAt: row.CreatedAt}
		if err := item.BeforeSave(nil); err != nil {
			report.fail(stats, "decision "+strconv.FormatInt(row.ID, 10), err)
			continue
		}
		a.importModel(options, report, stats, item, item.LegacySource, item.LegacyID)
	}
	return nil
}

func (a *Importer) importModel(options Options, report *Report, stats *DomainStats, model interface{}, source, legacyID *string) {
	var count int64
	if source != nil && legacyID != nil {
		if err := a.Target.Model(model).Where("legacy_source = ? AND legacy_id = ?", *source, *legacyID).Count(&count).Error; err != nil {
			report.fail(stats, fmt.Sprintf("legacy %s/%s", *source, *legacyID), err)
			return
		}
	}
	if count > 0 {
		stats.Skipped++
		return
	}
	if options.DryRun {
		stats.Imported++
		return
	}
	if err := a.Target.Create(model).Error; err != nil {
		report.fail(stats, fmt.Sprintf("legacy %s/%s", deref(source), deref(legacyID)), err)
		return
	}
	stats.Imported++
}

func (a *Importer) normalizeCredential(value, ownerUserID, entityID, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return value, nil
	}
	if a.Credentials == nil {
		return "", credential.ErrKeyNotConfigured
	}
	if strings.HasPrefix(value, "ENC:v1:") {
		parts := strings.Split(strings.TrimPrefix(value, "ENC:v1:"), ":")
		if len(parts) == 3 {
			return value, nil
		}
		plaintext, err := a.Credentials.DecryptField(value, ownerUserID, entityID, field)
		if err != nil {
			return "", err
		}
		return a.Credentials.EncryptField(plaintext, ownerUserID, entityID, field)
	}
	return a.Credentials.EncryptField(value, ownerUserID, entityID, field)
}

func (a *Importer) legacyTraderFor(options Options, traderID string) (*legacyTrader, string, error) {
	var trader legacyTrader
	if err := a.Source.Table("traders").Select("user_id", "exchange_id").Where("id = ?", traderID).First(&trader).Error; err != nil {
		return nil, "", fmt.Errorf("resolve trader owner: %w", err)
	}
	owner, err := resolveOwner(options, trader.UserID)
	return &trader, owner, err
}

func (a *Importer) persistReport(options Options, report *Report) error {
	domains, _ := json.Marshal(report.Domains)
	errorsJSON, _ := json.Marshal(report.Errors)
	status := "completed"
	if len(report.Errors) > 0 {
		status = "partial"
	}
	owner := reportOwner(options)
	if owner == "" {
		return errors.New("legacy import report requires a target owner")
	}
	finished := report.FinishedAt
	return a.Target.Create(&schema.LegacyImportRun{ID: report.RunID, SourceName: report.SourceName,
		OwnerUserID: owner, DryRun: false, Status: status, DomainStatsJSON: string(domains),
		ErrorsJSON: string(errorsJSON), StartedAt: report.StartedAt, FinishedAt: &finished}).Error
}

func reportOwner(options Options) string {
	if owner := strings.TrimSpace(options.DefaultOwnerUserID); owner != "" {
		return owner
	}
	legacyIDs := make([]string, 0, len(options.OwnerMap))
	for legacyID := range options.OwnerMap {
		legacyIDs = append(legacyIDs, legacyID)
	}
	sort.Strings(legacyIDs)
	for _, legacyID := range legacyIDs {
		if owner := strings.TrimSpace(options.OwnerMap[legacyID]); owner != "" {
			return owner
		}
	}
	return ""
}

func (r *Report) stats(domain string) *DomainStats {
	if r.Domains[domain] == nil {
		r.Domains[domain] = new(DomainStats)
	}
	return r.Domains[domain]
}

func (r *Report) fail(stats *DomainStats, ref string, err error) {
	stats.Failed++
	if len(r.Errors) < 100 {
		r.Errors = append(r.Errors, ref+": "+err.Error())
	}
}

func readLegacyTable(db *gorm.DB, table string, dest interface{}) error {
	if !db.Migrator().HasTable(table) {
		return nil
	}
	return db.Table(table).Find(dest).Error
}

func resolveOwner(options Options, legacyUserID string) (string, error) {
	if owner := strings.TrimSpace(options.OwnerMap[legacyUserID]); owner != "" {
		return owner, nil
	}
	if owner := strings.TrimSpace(options.DefaultOwnerUserID); owner != "" {
		return owner, nil
	}
	return "", fmt.Errorf("no owner mapping for legacy user %q", legacyUserID)
}

func decimal(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
func ptr(value string) *string     { return &value }
func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func defaultJSON(value, fallback string) string {
	if json.Valid([]byte(value)) {
		return value
	}
	return fallback
}
func unixMilli(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

type legacyExchange struct {
	ID, ExchangeType, AccountName, UserID, Name, Type             string
	Enabled                                                       bool
	Testnet                                                       bool
	HyperliquidUnifiedAcct                                        bool `gorm:"column:hyperliquid_unified_account"`
	HyperliquidBuilderApproved                                    bool
	APIKey, SecretKey, Passphrase, HyperliquidWalletAddr          string
	AsterUser, AsterSigner, AsterPrivateKey                       string
	LighterWalletAddr, LighterPrivateKey, LighterAPIKeyPrivateKey string
	LighterAPIKeyIndex                                            int
	CreatedAt, UpdatedAt                                          time.Time
}
type legacyAIModel struct {
	ID, UserID, Name, Provider, APIKey, CustomAPIURL, CustomModelName string
	Enabled                                                           bool
	CreatedAt, UpdatedAt                                              time.Time
}
type legacyStrategy struct {
	ID, UserID, Name, Description, Config        string
	IsActive, IsDefault, IsPublic, ConfigVisible bool
	CreatedAt, UpdatedAt                         time.Time
}
type legacyTrader struct {
	ID, UserID, Name, AIModelID, ExchangeID, StrategyID string
	InitialBalance                                      float64
	ScanIntervalMinutes                                 int
	IsRunning, IsCrossMargin, ShowInCompetition         bool
	CreatedAt, UpdatedAt                                time.Time
}
type legacyAccountSnapshot struct {
	ID                                                                   int64
	TraderID                                                             string
	Timestamp                                                            time.Time
	TotalEquity, Balance, AvailableBalance, UnrealizedPnL, MarginUsedPct float64
	PositionCount                                                        int
	CreatedAt                                                            time.Time
}
type legacyPosition struct {
	ID                                                                   int64
	TraderID, ExchangeID, ExchangeType, ExchangePositionID, Symbol, Side string
	EntryQuantity, Quantity, EntryPrice, ExitPrice, RealizedPnL, Fee     float64
	EntryOrderID, ExitOrderID                                            string
	EntryTime, ExitTime                                                  int64
	Leverage                                                             int
	Status, CloseReason, Source                                          string
	CreatedAt, UpdatedAt                                                 int64
}
type legacyDecision struct {
	ID                                                             int64
	TraderID                                                       string
	CycleNumber                                                    int
	Timestamp                                                      time.Time
	SystemPrompt, InputPrompt, CoTTrace, DecisionJSON, RawResponse string
	CandidateCoins, ExecutionLog, Decisions                        string
	Success                                                        bool
	ErrorMessage                                                   string
	AIRequestDurationMs                                            int64
	CreatedAt                                                      time.Time
}
