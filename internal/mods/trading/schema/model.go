package schema

import (
	"fmt"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"gorm.io/gorm"
)

const (
	TraderDesiredStateStopped = "stopped"
	TraderRuntimeUnavailable  = "unavailable"
)

// Exchange stores a user's exchange account. Credential fields contain ciphertext only.
type Exchange struct {
	ID                         string    `json:"id" gorm:"size:36;primaryKey"`
	LegacySource               *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_exchange_legacy,priority:1"`
	LegacyID                   *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_exchange_legacy,priority:2"`
	OwnerUserID                string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_exchange_owner;uniqueIndex:uk_trading_exchange_account,priority:1"`
	ExchangeType               string    `json:"exchange_type" gorm:"size:32;not null;index;uniqueIndex:uk_trading_exchange_account,priority:2"`
	AccountName                string    `json:"account_name" gorm:"size:128;not null;uniqueIndex:uk_trading_exchange_account,priority:3"`
	Name                       string    `json:"name" gorm:"size:128;not null"`
	AccountType                string    `json:"account_type" gorm:"size:16;not null"`
	Enabled                    bool      `json:"enabled" gorm:"not null;default:false;index"`
	Testnet                    bool      `json:"testnet" gorm:"not null;default:false"`
	APIKeyCiphertext           string    `json:"-" gorm:"type:text"`
	SecretKeyCiphertext        string    `json:"-" gorm:"type:text"`
	PassphraseCiphertext       string    `json:"-" gorm:"type:text"`
	PrivateKeyCiphertext       string    `json:"-" gorm:"type:text"`
	APIKeyPrivateKeyCiphertext string    `json:"-" gorm:"type:text"`
	WalletAddress              string    `json:"wallet_address" gorm:"size:128"`
	AccountUser                string    `json:"account_user" gorm:"size:128"`
	SignerAddress              string    `json:"signer_address" gorm:"size:128"`
	APIKeyIndex                int       `json:"api_key_index" gorm:"not null;default:0"`
	UnifiedAccount             bool      `json:"unified_account" gorm:"not null;default:false"`
	BuilderApproved            bool      `json:"builder_approved" gorm:"not null;default:false"`
	CredentialVersion          int       `json:"-" gorm:"not null;default:1"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

func (*Exchange) TableName() string {
	return config.C.FormatTableName("trading_exchanges")
}

func (a *Exchange) BeforeSave(*gorm.DB) error {
	return requireEncryptedCredentials(
		a.APIKeyCiphertext,
		a.SecretKeyCiphertext,
		a.PassphraseCiphertext,
		a.PrivateKeyCiphertext,
		a.APIKeyPrivateKeyCiphertext,
	)
}

// AIModel stores an LLM provider configuration without exposing its credential.
type AIModel struct {
	ID                    string    `json:"id" gorm:"size:64;primaryKey"`
	LegacySource          *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_model_legacy,priority:1"`
	LegacyID              *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_model_legacy,priority:2"`
	OwnerUserID           string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_model_owner"`
	Name                  string    `json:"name" gorm:"size:128;not null"`
	Provider              string    `json:"provider" gorm:"size:64;not null;index"`
	Enabled               bool      `json:"enabled" gorm:"not null;default:false;index"`
	APIKeyCiphertext      string    `json:"-" gorm:"type:text"`
	CustomAPIURL          string    `json:"custom_api_url" gorm:"size:512"`
	CustomModelName       string    `json:"custom_model_name" gorm:"size:128"`
	InputPricePerMTokens  string    `json:"input_price_per_m_tokens" gorm:"type:decimal(18,8);not null;default:0"`
	OutputPricePerMTokens string    `json:"output_price_per_m_tokens" gorm:"type:decimal(18,8);not null;default:0"`
	CredentialVersion     int       `json:"-" gorm:"not null;default:1"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (*AIModel) TableName() string {
	return config.C.FormatTableName("trading_ai_models")
}

func (a *AIModel) BeforeSave(*gorm.DB) error {
	if strings.TrimSpace(a.InputPricePerMTokens) == "" {
		a.InputPricePerMTokens = "0"
	}
	if strings.TrimSpace(a.OutputPricePerMTokens) == "" {
		a.OutputPricePerMTokens = "0"
	}
	return requireEncryptedCredentials(a.APIKeyCiphertext)
}

// AIInvocation records provider observability without persisting credentials or prompts.
type AIInvocation struct {
	ID               string     `json:"id" gorm:"size:64;primaryKey"`
	OwnerUserID      string     `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_ai_invocation_owner"`
	AIModelID        string     `json:"ai_model_id" gorm:"size:64;not null;index"`
	TraderID         *string    `json:"trader_id" gorm:"size:64;index"`
	StrategyID       *string    `json:"strategy_id" gorm:"size:64;index"`
	Purpose          string     `json:"purpose" gorm:"size:32;not null;index"`
	Provider         string     `json:"provider" gorm:"size:64;not null;index"`
	ModelName        string     `json:"model_name" gorm:"size:128;not null"`
	RequestID        string     `json:"request_id" gorm:"size:128"`
	InputTokens      int        `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens     int        `json:"output_tokens" gorm:"not null;default:0"`
	TotalTokens      int        `json:"total_tokens" gorm:"not null;default:0"`
	EstimatedCostUSD string     `json:"estimated_cost_usd" gorm:"type:decimal(18,8);not null;default:0"`
	DurationMS       int64      `json:"duration_ms" gorm:"not null;default:0"`
	HTTPStatus       int        `json:"http_status" gorm:"not null;default:0"`
	Success          bool       `json:"success" gorm:"not null;default:false;index"`
	ErrorCode        string     `json:"error_code" gorm:"size:64"`
	ErrorMessage     string     `json:"error_message" gorm:"size:500"`
	StartedAt        time.Time  `json:"started_at" gorm:"not null;index"`
	FinishedAt       *time.Time `json:"finished_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (*AIInvocation) TableName() string {
	return config.C.FormatTableName("trading_ai_invocations")
}

// Strategy stores the editable strategy head and pointers to immutable versions.
type Strategy struct {
	ID               string    `json:"id" gorm:"size:64;primaryKey"`
	LegacySource     *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_strategy_legacy,priority:1"`
	LegacyID         *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_strategy_legacy,priority:2"`
	OwnerUserID      string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_strategy_owner"`
	Name             string    `json:"name" gorm:"size:128;not null"`
	Description      string    `json:"description" gorm:"size:1000"`
	StrategyType     string    `json:"strategy_type" gorm:"size:32;not null;default:ai_trading;index"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:false;index"`
	IsDefault        bool      `json:"is_default" gorm:"not null;default:false"`
	IsPublic         bool      `json:"is_public" gorm:"not null;default:false;index"`
	ConfigVisible    bool      `json:"config_visible" gorm:"not null;default:true"`
	ConfigJSON       string    `json:"-" gorm:"type:json;not null"`
	CurrentVersion   int       `json:"current_version" gorm:"not null;default:0"`
	PublishedVersion *int      `json:"published_version" gorm:"index"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (*Strategy) TableName() string {
	return config.C.FormatTableName("trading_strategies")
}

// StrategyVersion is an immutable snapshot used for publishing and runtime traceability.
type StrategyVersion struct {
	ID           string     `json:"id" gorm:"size:64;primaryKey"`
	StrategyID   string     `json:"strategy_id" gorm:"size:64;not null;uniqueIndex:uk_trading_strategy_version,priority:1;index"`
	OwnerUserID  string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	Version      int        `json:"version" gorm:"not null;uniqueIndex:uk_trading_strategy_version,priority:2"`
	Name         string     `json:"name" gorm:"size:128;not null"`
	Description  string     `json:"description" gorm:"size:1000"`
	StrategyType string     `json:"strategy_type" gorm:"size:32;not null"`
	ConfigJSON   string     `json:"-" gorm:"type:json;not null"`
	RollbackFrom *int       `json:"rollback_from"`
	PublishedAt  *time.Time `json:"published_at" gorm:"index"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (*StrategyVersion) TableName() string {
	return config.C.FormatTableName("trading_strategy_versions")
}

// StrategyVersionSwitchEvent records an immutable runtime strategy version transition.
type StrategyVersionSwitchEvent struct {
	ID          string    `json:"id" gorm:"size:64;primaryKey"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	TraderID    string    `json:"trader_id" gorm:"size:64;not null;index"`
	StrategyID  string    `json:"strategy_id" gorm:"size:64;not null;index"`
	FromVersion int       `json:"from_version" gorm:"not null"`
	ToVersion   int       `json:"to_version" gorm:"not null"`
	Reason      string    `json:"reason" gorm:"size:128;not null"`
	OccurredAt  time.Time `json:"occurred_at" gorm:"not null;index"`
}

func (*StrategyVersionSwitchEvent) TableName() string {
	return config.C.FormatTableName("trading_strategy_version_switch_events")
}

// Trader binds a user, exchange, model, and strategy to a managed runtime instance.
type Trader struct {
	ID                       string     `json:"id" gorm:"size:64;primaryKey"`
	LegacySource             *string    `json:"-" gorm:"size:64;uniqueIndex:uk_trading_trader_legacy,priority:1"`
	LegacyID                 *string    `json:"-" gorm:"size:64;uniqueIndex:uk_trading_trader_legacy,priority:2"`
	OwnerUserID              string     `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_trader_owner"`
	Name                     string     `json:"name" gorm:"size:128;not null"`
	AIModelID                string     `json:"ai_model_id" gorm:"size:64;not null;index"`
	ExchangeID               string     `json:"exchange_id" gorm:"size:36;not null;index"`
	StrategyID               *string    `json:"strategy_id" gorm:"size:64;index"`
	StrategyVersion          int        `json:"strategy_version" gorm:"not null;default:0"`
	InitialBalance           string     `json:"initial_balance" gorm:"type:decimal(36,18);not null;default:0"`
	ScanIntervalMinutes      int        `json:"scan_interval_minutes" gorm:"not null;default:15"`
	IsCrossMargin            bool       `json:"is_cross_margin" gorm:"not null;default:true"`
	IsSimulation             bool       `json:"is_simulation" gorm:"not null;default:true;index"`
	AutoApplyStrategyUpdates bool       `json:"auto_apply_strategy_updates" gorm:"not null;default:false"`
	ShowInCompetition        bool       `json:"show_in_competition" gorm:"not null;default:false"`
	CompetitionPublicID      string     `json:"-" gorm:"size:36;uniqueIndex"`
	CompetitionAlias         string     `json:"competition_alias" gorm:"size:32"`
	CompetitionJoinedAt      *time.Time `json:"competition_joined_at" gorm:"index"`
	ExecutionMode            string     `json:"execution_mode" gorm:"size:32;not null;default:notify_only;index"`
	DesiredState             string     `json:"desired_state" gorm:"size:32;not null;default:stopped;index"`
	RuntimeState             string     `json:"runtime_state" gorm:"size:32;not null;default:unavailable;index"`
	SyncState                string     `json:"sync_state" gorm:"size:32;not null;default:idle;index"`
	StrategyState            string     `json:"strategy_state" gorm:"size:32;not null;default:idle;index"`
	CycleNumber              int        `json:"cycle_number" gorm:"not null;default:0"`
	LastDecisionAt           *time.Time `json:"last_decision_at" gorm:"index"`
	SafeMode                 bool       `json:"safe_mode" gorm:"not null;default:true;index"`
	PauseReason              string     `json:"pause_reason" gorm:"size:500"`
	AIConsentModelID         string     `json:"ai_consent_model_id" gorm:"size:64"`
	AIConsentAt              *time.Time `json:"ai_consent_at"`
	AutoExecutionConsentAt   *time.Time `json:"auto_execution_consent_at" gorm:"index"`
	LastHeartbeatAt          *time.Time `json:"last_heartbeat_at" gorm:"index"`
	LastErrorCode            string     `json:"last_error_code" gorm:"size:64"`
	LastErrorMessage         string     `json:"last_error_message" gorm:"size:1000"`
	LastCycleDurationMS      int64      `json:"last_cycle_duration_ms" gorm:"not null;default:0"`
	ConsecutiveFailures      int        `json:"consecutive_failures" gorm:"not null;default:0"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

func (*Trader) TableName() string {
	return config.C.FormatTableName("trading_traders")
}

func requireEncryptedCredentials(values ...string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !strings.HasPrefix(value, "ENC:v1:") {
			return fmt.Errorf("trading credential must be encrypted before persistence")
		}
	}
	return nil
}
