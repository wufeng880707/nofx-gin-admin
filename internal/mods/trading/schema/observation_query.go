package schema

import (
	"encoding/json"
	"time"

	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type ObservationQueryParam struct {
	util.PaginationParam
	OwnerUserID string     `form:"ownerUserID"`
	TraderID    string     `form:"traderID"`
	ExchangeID  string     `form:"exchangeID"`
	StartAt     *time.Time `form:"startAt" time_format:"2006-01-02T15:04:05Z07:00"`
	EndAt       *time.Time `form:"endAt" time_format:"2006-01-02T15:04:05Z07:00"`
}

type PositionQueryParam struct {
	ObservationQueryParam
	Symbol string `form:"symbol"`
	Side   string `form:"side"`
	Status string `form:"status"`
}

type AssetBalanceQueryParam struct {
	ObservationQueryParam
	Asset       string `form:"asset"`
	AccountType string `form:"accountType"`
}

type OrderQueryParam struct {
	ObservationQueryParam
	Symbol string `form:"symbol"`
	Market string `form:"market"`
	Side   string `form:"side"`
	Status string `form:"status"`
	Origin string `form:"origin"`
}

type FillQueryParam struct {
	ObservationQueryParam
	Symbol string `form:"symbol"`
	Market string `form:"market"`
	Side   string `form:"side"`
}

type IncomeEventQueryParam struct {
	ObservationQueryParam
	Symbol    string `form:"symbol"`
	Market    string `form:"market"`
	Asset     string `form:"asset"`
	EventType string `form:"eventType"`
}

type DecisionQueryParam struct {
	ObservationQueryParam
	Success *bool `form:"success"`
}

type AIInvocationSummaryQueryParam struct {
	OwnerUserID string     `form:"ownerUserID"`
	AIModelID   string     `form:"aiModelID"`
	TraderID    string     `form:"traderID"`
	StrategyID  string     `form:"strategyID"`
	Purpose     string     `form:"purpose"`
	StartAt     *time.Time `form:"startAt" time_format:"2006-01-02T15:04:05Z07:00"`
	EndAt       *time.Time `form:"endAt" time_format:"2006-01-02T15:04:05Z07:00"`
}

type AIInvocationMetric struct {
	Key              string  `json:"key"`
	Calls            int64   `json:"calls"`
	Successes        int64   `json:"successes"`
	Failures         int64   `json:"failures"`
	SuccessRate      float64 `json:"success_rate"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	EstimatedCostUSD string  `json:"estimated_cost_usd"`
	AverageLatencyMS float64 `json:"average_latency_ms"`
}

type AIInvocationSummary struct {
	StartAt    time.Time             `json:"start_at"`
	EndAt      time.Time             `json:"end_at"`
	Overall    *AIInvocationMetric   `json:"overall"`
	ByModel    []*AIInvocationMetric `json:"by_model"`
	ByTrader   []*AIInvocationMetric `json:"by_trader"`
	ByStrategy []*AIInvocationMetric `json:"by_strategy"`
}

type AccountSnapshotQueryResult struct {
	Data       []*AccountSnapshot
	PageResult *util.PaginationResult
}

type PositionQueryResult struct {
	Data       []*Position
	PageResult *util.PaginationResult
}

type AssetBalanceQueryResult struct {
	Data       []*AssetBalance
	PageResult *util.PaginationResult
}

type OrderQueryResult struct {
	Data       []*ExchangeOrder
	PageResult *util.PaginationResult
}

type FillQueryResult struct {
	Data       []*Fill
	PageResult *util.PaginationResult
}

type IncomeEventQueryResult struct {
	Data       []*IncomeEvent
	PageResult *util.PaginationResult
}

type AssetSummaryExchange struct {
	ExchangeID    string     `json:"exchange_id"`
	AccountType   string     `json:"account_type"`
	AssetCount    int64      `json:"asset_count"`
	ValueUSDT     string     `json:"value_usdt"`
	LastUpdatedAt *time.Time `json:"last_updated_at"`
}

type AssetSummary struct {
	TotalValueUSDT string                  `json:"total_value_usdt"`
	AssetCount     int64                   `json:"asset_count"`
	Exchanges      []*AssetSummaryExchange `json:"exchanges"`
}

type DecisionQueryResult struct {
	Data       []*Decision
	PageResult *util.PaginationResult
}

type DecisionView struct {
	ID                  uint64          `json:"id"`
	OwnerUserID         string          `json:"owner_user_id"`
	TraderID            string          `json:"trader_id"`
	ExchangeID          string          `json:"exchange_id"`
	CycleNumber         int             `json:"cycle_number"`
	DecidedAt           time.Time       `json:"decided_at"`
	Success             bool            `json:"success"`
	ErrorMessage        string          `json:"error_message"`
	AIRequestDurationMs int64           `json:"ai_request_duration_ms"`
	CandidateCoins      json.RawMessage `json:"candidate_coins"`
	ExecutionLog        json.RawMessage `json:"execution_log"`
	Actions             json.RawMessage `json:"actions"`
	CreatedAt           time.Time       `json:"created_at"`
}

// DecisionDetailView exposes auditable decision inputs and outputs on demand.
// System prompts and chain-of-thought traces are intentionally excluded.
type DecisionDetailView struct {
	*DecisionView
	InputPrompt string          `json:"input_prompt"`
	RawResponse string          `json:"raw_response"`
	Decision    json.RawMessage `json:"decision"`
}

type DecisionViewResult struct {
	Data       []*DecisionView
	PageResult *util.PaginationResult
}
