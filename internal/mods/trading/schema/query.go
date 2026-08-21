package schema

import (
	"time"

	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type CatalogQueryParam struct {
	util.PaginationParam
	Keyword     string `form:"keyword"`
	OwnerUserID string `form:"ownerUserID"`
}

type ExchangeQueryParam struct {
	CatalogQueryParam
	ExchangeType string `form:"exchangeType"`
	Enabled      *bool  `form:"enabled"`
}

type AIModelQueryParam struct {
	CatalogQueryParam
	Provider string `form:"provider"`
	Enabled  *bool  `form:"enabled"`
}

type StrategyQueryParam struct {
	CatalogQueryParam
	StrategyType string `form:"strategyType"`
	Active       *bool  `form:"active"`
}

type TraderQueryParam struct {
	CatalogQueryParam
	RuntimeState string `form:"runtimeState"`
}

type NotificationChannelQueryParam struct {
	CatalogQueryParam
	ChannelType string `form:"channelType"`
	Enabled     *bool  `form:"enabled"`
}

type ExchangeQueryResult struct {
	Data       []*Exchange
	PageResult *util.PaginationResult
}

type AIModelQueryResult struct {
	Data       []*AIModel
	PageResult *util.PaginationResult
}

type StrategyQueryResult struct {
	Data       []*Strategy
	PageResult *util.PaginationResult
}

type TraderQueryResult struct {
	Data       []*Trader
	PageResult *util.PaginationResult
}

type NotificationChannelQueryResult struct {
	Data       []*NotificationChannel
	PageResult *util.PaginationResult
}

type ExchangeView struct {
	ID                  string    `json:"id"`
	OwnerUserID         string    `json:"owner_user_id"`
	ExchangeType        string    `json:"exchange_type"`
	AccountName         string    `json:"account_name"`
	Name                string    `json:"name"`
	AccountType         string    `json:"account_type"`
	Enabled             bool      `json:"enabled"`
	Testnet             bool      `json:"testnet"`
	WalletAddress       string    `json:"wallet_address"`
	AccountUser         string    `json:"account_user"`
	SignerAddress       string    `json:"signer_address"`
	APIKeyIndex         int       `json:"api_key_index"`
	UnifiedAccount      bool      `json:"unified_account"`
	BuilderApproved     bool      `json:"builder_approved"`
	HasAPIKey           bool      `json:"has_api_key"`
	HasSecretKey        bool      `json:"has_secret_key"`
	HasPassphrase       bool      `json:"has_passphrase"`
	HasPrivateKey       bool      `json:"has_private_key"`
	HasAPIKeyPrivateKey bool      `json:"has_api_key_private_key"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ExchangeConnectionTestView struct {
	ExchangeID              string `json:"exchange_id"`
	ExchangeType            string `json:"exchange_type"`
	AccountType             string `json:"account_type"`
	Testnet                 bool   `json:"testnet"`
	Environment             string `json:"environment"`
	NetworkOK               bool   `json:"network_ok"`
	AuthenticationOK        bool   `json:"authentication_ok"`
	NetworkLatencyMS        int64  `json:"network_latency_ms"`
	AuthenticationLatencyMS int64  `json:"authentication_latency_ms"`
	LatencyMS               int64  `json:"latency_ms"`
}

type ExchangeDataSyncView struct {
	ExchangeID       string    `json:"exchange_id"`
	ExchangeType     string    `json:"exchange_type"`
	AccountType      string    `json:"account_type"`
	AssetCount       int64     `json:"asset_count"`
	OrderCount       int64     `json:"order_count"`
	FillCount        int64     `json:"fill_count"`
	IncomeEventCount int64     `json:"income_event_count"`
	SyncedAt         time.Time `json:"synced_at"`
}

type HistoryBackfillView struct {
	ExchangeDataSyncView
	StartAt                 time.Time `json:"start_at"`
	EndAt                   time.Time `json:"end_at"`
	Symbols                 []string  `json:"symbols"`
	FetchedOrderCount       int       `json:"fetched_order_count"`
	FetchedFillCount        int       `json:"fetched_fill_count"`
	FetchedIncomeEventCount int       `json:"fetched_income_event_count"`
	ClosedTradeCount        int64     `json:"closed_trade_count"`
}

type HistorySyncStatusView struct {
	ExchangeID          string     `json:"exchange_id"`
	ExchangeName        string     `json:"exchange_name"`
	ExchangeType        string     `json:"exchange_type"`
	AccountType         string     `json:"account_type"`
	Enabled             bool       `json:"enabled"`
	Status              string     `json:"status"`
	CoverageStatus      string     `json:"coverage_status"`
	Warning             string     `json:"warning"`
	SyncedUntil         *time.Time `json:"synced_until"`
	LastRunAt           *time.Time `json:"last_run_at"`
	ErrorMessage        string     `json:"error_message"`
	RegisteredSymbols   []string   `json:"registered_symbols"`
	OrderCount          int64      `json:"order_count"`
	FillCount           int64      `json:"fill_count"`
	IncomeEventCount    int64      `json:"income_event_count"`
	ClosedTradeCount    int64      `json:"closed_trade_count"`
	OrphanFillCount     int64      `json:"orphan_fill_count"`
	UnmatchedOrderCount int64      `json:"unmatched_order_count"`
	GapCount            int64      `json:"gap_count"`
	GapDetails          []string   `json:"gap_details"`
}

type AIModelView struct {
	ID                    string    `json:"id"`
	OwnerUserID           string    `json:"owner_user_id"`
	Name                  string    `json:"name"`
	Provider              string    `json:"provider"`
	Enabled               bool      `json:"enabled"`
	HasAPIKey             bool      `json:"has_api_key"`
	CustomAPIURL          string    `json:"custom_api_url"`
	CustomModelName       string    `json:"custom_model_name"`
	InputPricePerMTokens  string    `json:"input_price_per_m_tokens"`
	OutputPricePerMTokens string    `json:"output_price_per_m_tokens"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type AIModelConnectionTestView struct {
	AIModelID    string `json:"ai_model_id"`
	Provider     string `json:"provider"`
	ModelName    string `json:"model_name"`
	RequestID    string `json:"request_id"`
	Response     string `json:"response"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	Success      bool   `json:"success"`
}

type StrategyView struct {
	ID               string    `json:"id"`
	OwnerUserID      string    `json:"owner_user_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	StrategyType     string    `json:"strategy_type"`
	IsActive         bool      `json:"is_active"`
	IsDefault        bool      `json:"is_default"`
	IsPublic         bool      `json:"is_public"`
	ConfigVisible    bool      `json:"config_visible"`
	CurrentVersion   int       `json:"current_version"`
	PublishedVersion *int      `json:"published_version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type StrategyVersionView struct {
	ID           string      `json:"id"`
	StrategyID   string      `json:"strategy_id"`
	Version      int         `json:"version"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	StrategyType string      `json:"strategy_type"`
	Config       interface{} `json:"config"`
	RollbackFrom *int        `json:"rollback_from"`
	PublishedAt  *time.Time  `json:"published_at"`
	CreatedAt    time.Time   `json:"created_at"`
}

type StrategyDetailView struct {
	StrategyView
	Config interface{} `json:"config"`
}

type TraderView struct {
	ID                       string     `json:"id"`
	OwnerUserID              string     `json:"owner_user_id"`
	Name                     string     `json:"name"`
	AIModelID                string     `json:"ai_model_id"`
	ExchangeID               string     `json:"exchange_id"`
	StrategyID               *string    `json:"strategy_id"`
	StrategyVersion          int        `json:"strategy_version"`
	InitialBalance           string     `json:"initial_balance"`
	ScanIntervalMinutes      int        `json:"scan_interval_minutes"`
	IsCrossMargin            bool       `json:"is_cross_margin"`
	IsSimulation             bool       `json:"is_simulation"`
	AutoApplyStrategyUpdates bool       `json:"auto_apply_strategy_updates"`
	ShowInCompetition        bool       `json:"show_in_competition"`
	CompetitionPublicID      string     `json:"competition_public_id"`
	CompetitionAlias         string     `json:"competition_alias"`
	CompetitionJoinedAt      *time.Time `json:"competition_joined_at"`
	LastCycleDurationMS      int64      `json:"last_cycle_duration_ms"`
	ConsecutiveFailures      int        `json:"consecutive_failures"`
	ExecutionMode            string     `json:"execution_mode"`
	NotificationChannelIDs   []string   `json:"notification_channel_ids"`
	DesiredState             string     `json:"desired_state"`
	RuntimeState             string     `json:"runtime_state"`
	SyncState                string     `json:"sync_state"`
	StrategyState            string     `json:"strategy_state"`
	CycleNumber              int        `json:"cycle_number"`
	LastDecisionAt           *time.Time `json:"last_decision_at"`
	SafeMode                 bool       `json:"safe_mode"`
	PauseReason              string     `json:"pause_reason"`
	AIConsentModelID         string     `json:"ai_consent_model_id"`
	AIConsentAt              *time.Time `json:"ai_consent_at"`
	LastHeartbeatAt          *time.Time `json:"last_heartbeat_at"`
	LastErrorCode            string     `json:"last_error_code"`
	LastErrorMessage         string     `json:"last_error_message"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type ExchangeViewResult struct {
	Data       []*ExchangeView
	PageResult *util.PaginationResult
}

type AIModelViewResult struct {
	Data       []*AIModelView
	PageResult *util.PaginationResult
}

type StrategyViewResult struct {
	Data       []*StrategyView
	PageResult *util.PaginationResult
}

type TraderViewResult struct {
	Data       []*TraderView
	PageResult *util.PaginationResult
}

type NotificationChannelViewResult struct {
	Data       []*NotificationChannelView
	PageResult *util.PaginationResult
}
