package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

const (
	SpotStrategyType = "spot_core_satellite"

	SpotStateWait         = "WAIT"
	SpotStateAccumulate   = "ACCUMULATE"
	SpotStateRangeTrading = "RANGE_TRADING"
	SpotStateBreakout     = "BREAKOUT"
	SpotStateDistribution = "DISTRIBUTION"
	SpotStateRiskOff      = "RISK_OFF"

	SpotBucketCore      = "core"
	SpotBucketSatellite = "satellite"

	SpotLotOpen          = "open"
	SpotLotReserved      = "reserved"
	SpotLotPartialClosed = "partial_closed"
	SpotLotClosed        = "closed"
	SpotLotReconciling   = "reconciling"

	SpotBacktestQueued      = "queued"
	SpotBacktestLoadingData = "loading_data"
	SpotBacktestRunning     = "running"
	SpotBacktestCalculating = "calculating"
	SpotBacktestCompleted   = "completed"
	SpotBacktestFailed      = "failed"
	SpotBacktestCancelled   = "cancelled"
)

// SpotCoreSatelliteConfig is stored inside StrategyStudioConfig and copied to
// every backtest task so historical results never depend on mutable settings.
type SpotCoreSatelliteConfig struct {
	Exchange                  string    `json:"exchange"`
	Symbol                    string    `json:"symbol"`
	Market                    string    `json:"market"`
	QuoteAsset                string    `json:"quote_asset"`
	ExecutionTimeframe        string    `json:"execution_timeframe"`
	StrategyBudgetUSDT        float64   `json:"strategy_budget_usdt"`
	CoreBudgetRatio           float64   `json:"core_budget_ratio"`
	SatelliteBudgetRatio      float64   `json:"satellite_budget_ratio"`
	MinimumCashRatio          float64   `json:"minimum_cash_ratio"`
	MaxSymbolExposureRatio    float64   `json:"max_symbol_exposure_ratio"`
	LowAreaLookbackBars       int       `json:"low_area_lookback_bars"`
	LowAreaDrawdownPct        float64   `json:"low_area_drawdown_pct"`
	TrendFilterPeriod         int       `json:"trend_filter_period"`
	StabilizationBars         int       `json:"stabilization_bars"`
	AccumulationDrawdowns     []float64 `json:"accumulation_drawdowns"`
	AccumulationWeights       []float64 `json:"accumulation_weights"`
	AccumulationCooldownBars  int       `json:"accumulation_cooldown_bars"`
	SatelliteEntryDrawdowns   []float64 `json:"satellite_entry_drawdowns"`
	SatelliteTakeProfitRatio  float64   `json:"satellite_take_profit_ratio"`
	SatelliteSellRatio        float64   `json:"satellite_sell_ratio"`
	ResistanceLookbackBars    int       `json:"resistance_lookback_bars"`
	BreakoutConfirmBars       int       `json:"breakout_confirm_bars"`
	BreakoutVolumeMultiplier  float64   `json:"breakout_volume_multiplier"`
	DistributionLookbackBars  int       `json:"distribution_lookback_bars"`
	DistributionStepGainPct   float64   `json:"distribution_step_gain_pct"`
	DistributionStepSellRatio float64   `json:"distribution_step_sell_ratio"`
	MaxCoreDistributionRatio  float64   `json:"max_core_distribution_ratio"`
	CoreMinimumRetentionRatio float64   `json:"core_minimum_retention_ratio"`
	EnableDrawdownRiskControl *bool     `json:"enable_drawdown_risk_control"`
	MaxStrategyDrawdownPct    float64   `json:"max_strategy_drawdown_pct"`
	MaxDailyBuyRatio          float64   `json:"max_daily_buy_ratio"`
	FeeRateBPS                float64   `json:"fee_rate_bps"`
	SlippageBPS               float64   `json:"slippage_bps"`
	StressSlippageBPS         float64   `json:"stress_slippage_bps"`
	AllowLiveExecution        bool      `json:"allow_live_execution"`
}

func DefaultSpotCoreSatelliteConfig() SpotCoreSatelliteConfig {
	return SpotCoreSatelliteConfig{
		Exchange: "binance", Symbol: "BTCUSDT", Market: MarketTypeSpot, QuoteAsset: "USDT",
		ExecutionTimeframe: "1d", StrategyBudgetUSDT: 10000,
		CoreBudgetRatio: 0.35, SatelliteBudgetRatio: 0.25, MinimumCashRatio: 0.40,
		MaxSymbolExposureRatio: 0.60, LowAreaLookbackBars: 365, LowAreaDrawdownPct: 0.35,
		TrendFilterPeriod: 200, StabilizationBars: 5,
		AccumulationDrawdowns: []float64{0, 0.05, 0.10, 0.16, 0.24, 0.34},
		AccumulationWeights:   []float64{1, 1, 1.2, 1.4, 1.6, 1.8}, AccumulationCooldownBars: 2,
		SatelliteEntryDrawdowns:  []float64{0.03, 0.05, 0.08, 0.12},
		SatelliteTakeProfitRatio: 0.03, SatelliteSellRatio: 1,
		ResistanceLookbackBars: 30, BreakoutConfirmBars: 2, BreakoutVolumeMultiplier: 1.2,
		DistributionLookbackBars: 365, DistributionStepGainPct: 0.15,
		DistributionStepSellRatio: 0.10, MaxCoreDistributionRatio: 0.70,
		CoreMinimumRetentionRatio: 0.10, EnableDrawdownRiskControl: spotBool(true), MaxStrategyDrawdownPct: 0.25,
		MaxDailyBuyRatio: 0.10, FeeRateBPS: 10, SlippageBPS: 5, StressSlippageBPS: 20,
		AllowLiveExecution: false,
	}
}

func (a *SpotCoreSatelliteConfig) Normalize() {
	defaults := DefaultSpotCoreSatelliteConfig()
	a.Exchange = strings.ToLower(strings.TrimSpace(a.Exchange))
	a.Symbol = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(a.Symbol), "/", ""))
	a.Market = strings.ToLower(strings.TrimSpace(a.Market))
	a.QuoteAsset = strings.ToUpper(strings.TrimSpace(a.QuoteAsset))
	a.ExecutionTimeframe = strings.ToLower(strings.TrimSpace(a.ExecutionTimeframe))
	if a.Exchange == "" {
		a.Exchange = defaults.Exchange
	}
	if a.Symbol == "" {
		a.Symbol = defaults.Symbol
	}
	if a.Market == "" {
		a.Market = defaults.Market
	}
	if a.QuoteAsset == "" {
		a.QuoteAsset = defaults.QuoteAsset
	}
	if a.Exchange == MarketExchangeBinance {
		a.Symbol = strings.ReplaceAll(a.Symbol, "-", "")
	} else if a.Exchange == MarketExchangeOKX && !strings.Contains(a.Symbol, "-") && strings.HasSuffix(a.Symbol, a.QuoteAsset) {
		a.Symbol = strings.TrimSuffix(a.Symbol, a.QuoteAsset) + "-" + a.QuoteAsset
	}
	if a.ExecutionTimeframe == "" {
		a.ExecutionTimeframe = defaults.ExecutionTimeframe
	}
	if a.StrategyBudgetUSDT == 0 {
		a.StrategyBudgetUSDT = defaults.StrategyBudgetUSDT
	}
	if a.CoreBudgetRatio == 0 {
		a.CoreBudgetRatio = defaults.CoreBudgetRatio
	}
	if a.SatelliteBudgetRatio == 0 {
		a.SatelliteBudgetRatio = defaults.SatelliteBudgetRatio
	}
	if a.MinimumCashRatio == 0 {
		a.MinimumCashRatio = defaults.MinimumCashRatio
	}
	if a.MaxSymbolExposureRatio == 0 {
		a.MaxSymbolExposureRatio = defaults.MaxSymbolExposureRatio
	}
	if a.LowAreaLookbackBars == 0 {
		a.LowAreaLookbackBars = defaults.LowAreaLookbackBars
	}
	if a.LowAreaDrawdownPct == 0 {
		a.LowAreaDrawdownPct = defaults.LowAreaDrawdownPct
	}
	if a.TrendFilterPeriod == 0 {
		a.TrendFilterPeriod = defaults.TrendFilterPeriod
	}
	if a.StabilizationBars == 0 {
		a.StabilizationBars = defaults.StabilizationBars
	}
	if len(a.AccumulationDrawdowns) == 0 {
		a.AccumulationDrawdowns = append([]float64(nil), defaults.AccumulationDrawdowns...)
	}
	if len(a.AccumulationWeights) == 0 {
		a.AccumulationWeights = append([]float64(nil), defaults.AccumulationWeights...)
	}
	if a.AccumulationCooldownBars == 0 {
		a.AccumulationCooldownBars = defaults.AccumulationCooldownBars
	}
	if len(a.SatelliteEntryDrawdowns) == 0 {
		a.SatelliteEntryDrawdowns = append([]float64(nil), defaults.SatelliteEntryDrawdowns...)
	}
	if a.SatelliteTakeProfitRatio == 0 {
		a.SatelliteTakeProfitRatio = defaults.SatelliteTakeProfitRatio
	}
	if a.SatelliteSellRatio == 0 {
		a.SatelliteSellRatio = defaults.SatelliteSellRatio
	}
	if a.ResistanceLookbackBars == 0 {
		a.ResistanceLookbackBars = defaults.ResistanceLookbackBars
	}
	if a.BreakoutConfirmBars == 0 {
		a.BreakoutConfirmBars = defaults.BreakoutConfirmBars
	}
	if a.BreakoutVolumeMultiplier == 0 {
		a.BreakoutVolumeMultiplier = defaults.BreakoutVolumeMultiplier
	}
	if a.DistributionLookbackBars == 0 {
		a.DistributionLookbackBars = defaults.DistributionLookbackBars
	}
	if a.DistributionStepGainPct == 0 {
		a.DistributionStepGainPct = defaults.DistributionStepGainPct
	}
	if a.DistributionStepSellRatio == 0 {
		a.DistributionStepSellRatio = defaults.DistributionStepSellRatio
	}
	if a.MaxCoreDistributionRatio == 0 {
		a.MaxCoreDistributionRatio = defaults.MaxCoreDistributionRatio
	}
	if a.EnableDrawdownRiskControl == nil {
		a.EnableDrawdownRiskControl = defaults.EnableDrawdownRiskControl
	}
	if a.MaxStrategyDrawdownPct == 0 {
		a.MaxStrategyDrawdownPct = defaults.MaxStrategyDrawdownPct
	}
	if a.MaxDailyBuyRatio == 0 {
		a.MaxDailyBuyRatio = defaults.MaxDailyBuyRatio
	}
	if a.StressSlippageBPS == 0 {
		a.StressSlippageBPS = defaults.StressSlippageBPS
	}
	sort.Float64s(a.AccumulationDrawdowns)
	sort.Float64s(a.SatelliteEntryDrawdowns)
}

func (a SpotCoreSatelliteConfig) Validate() error {
	if a.Market != MarketTypeSpot {
		return fmt.Errorf("spot core-satellite strategy only supports spot market")
	}
	if a.Exchange != MarketExchangeBinance && a.Exchange != MarketExchangeOKX {
		return fmt.Errorf("unsupported exchange %q", a.Exchange)
	}
	if a.Symbol == "" || !strings.HasSuffix(a.Symbol, a.QuoteAsset) {
		return fmt.Errorf("symbol must use the configured quote asset")
	}
	if a.QuoteAsset != "USDT" {
		return fmt.Errorf("quote asset must be USDT in the first release")
	}
	if a.ExecutionTimeframe != "4h" && a.ExecutionTimeframe != "1d" {
		return fmt.Errorf("execution timeframe must be 4h or 1d")
	}
	if a.StrategyBudgetUSDT <= 0 {
		return fmt.Errorf("strategy budget must be positive")
	}
	if !nearOne(a.CoreBudgetRatio + a.SatelliteBudgetRatio + a.MinimumCashRatio) {
		return fmt.Errorf("core, satellite and cash ratios must total 1")
	}
	if a.CoreBudgetRatio <= 0 || a.SatelliteBudgetRatio <= 0 || a.MinimumCashRatio < 0.2 {
		return fmt.Errorf("capital ratios are outside the safe range")
	}
	if a.MaxSymbolExposureRatio <= 0 || a.MaxSymbolExposureRatio > 1-a.MinimumCashRatio+1e-9 {
		return fmt.Errorf("symbol exposure exceeds available strategy capital")
	}
	if a.LowAreaLookbackBars < 20 || a.TrendFilterPeriod < 20 || a.ResistanceLookbackBars < 5 {
		return fmt.Errorf("lookback periods are too short")
	}
	if !validRatio(a.LowAreaDrawdownPct) || !validRatio(a.MaxStrategyDrawdownPct) {
		return fmt.Errorf("drawdown ratios must be between 0 and 1")
	}
	if len(a.AccumulationDrawdowns) == 0 || len(a.AccumulationDrawdowns) > 8 || len(a.AccumulationDrawdowns) != len(a.AccumulationWeights) {
		return fmt.Errorf("accumulation ladders and weights must have the same length between 1 and 8")
	}
	for index, value := range a.AccumulationDrawdowns {
		if value < 0 || value >= 1 || (index > 0 && value <= a.AccumulationDrawdowns[index-1]) {
			return fmt.Errorf("accumulation drawdowns must be strictly increasing")
		}
		if a.AccumulationWeights[index] <= 0 {
			return fmt.Errorf("accumulation weights must be positive")
		}
		if index > 0 && a.AccumulationWeights[index] > a.AccumulationWeights[index-1]*1.75 {
			return fmt.Errorf("accumulation weight escalation is too aggressive")
		}
	}
	for index, value := range a.SatelliteEntryDrawdowns {
		if value <= 0 || value >= 1 || (index > 0 && value <= a.SatelliteEntryDrawdowns[index-1]) {
			return fmt.Errorf("satellite drawdowns must be strictly increasing")
		}
	}
	for _, value := range []float64{a.SatelliteTakeProfitRatio, a.SatelliteSellRatio, a.DistributionStepGainPct,
		a.DistributionStepSellRatio, a.MaxCoreDistributionRatio, a.MaxDailyBuyRatio} {
		if !validRatio(value) {
			return fmt.Errorf("strategy ratios must be between 0 and 1")
		}
	}
	if a.CoreMinimumRetentionRatio < 0 || a.CoreMinimumRetentionRatio > 1 {
		return fmt.Errorf("core minimum retention ratio must be between 0 and 1")
	}
	if a.FeeRateBPS < 0 || a.SlippageBPS < 0 || a.StressSlippageBPS < a.SlippageBPS {
		return fmt.Errorf("invalid cost assumptions")
	}
	return nil
}

func validRatio(value float64) bool { return value > 0 && value <= 1 }
func nearOne(value float64) bool    { return math.Abs(value-1) < 1e-8 }
func spotBool(value bool) *bool     { return &value }

func (a SpotCoreSatelliteConfig) DrawdownRiskControlEnabled() bool {
	return a.EnableDrawdownRiskControl == nil || *a.EnableDrawdownRiskControl
}

type SpotStrategyCycle struct {
	ID            string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID   string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	StrategyID    string     `json:"strategy_id" gorm:"size:64;not null;index"`
	TraderID      string     `json:"trader_id" gorm:"size:64;index"`
	ConfigVersion int        `json:"config_version" gorm:"not null"`
	State         string     `json:"state" gorm:"size:24;not null;index"`
	InitialBudget string     `json:"initial_budget" gorm:"type:decimal(36,18);not null"`
	AvailableCash string     `json:"available_cash" gorm:"type:decimal(36,18);not null"`
	ReservedCash  string     `json:"reserved_cash" gorm:"type:decimal(36,18);not null;default:0"`
	RealizedPnL   string     `json:"realized_pnl" gorm:"column:realized_pnl;type:decimal(36,18);not null;default:0"`
	UnrealizedPnL string     `json:"unrealized_pnl" gorm:"column:unrealized_pnl;type:decimal(36,18);not null;default:0"`
	StartedAt     time.Time  `json:"started_at" gorm:"not null;index"`
	EndedAt       *time.Time `json:"ended_at"`
	Version       uint64     `json:"version" gorm:"not null;default:1"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (*SpotStrategyCycle) TableName() string {
	return config.C.FormatTableName("trading_spot_strategy_cycles")
}

type SpotPositionLot struct {
	ID                  string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID         string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	StrategyID          string     `json:"strategy_id" gorm:"size:64;not null;index"`
	CycleID             string     `json:"cycle_id" gorm:"size:36;not null;index"`
	TraderID            string     `json:"trader_id" gorm:"size:64;index"`
	ExchangeID          string     `json:"exchange_id" gorm:"size:36;not null;index"`
	Market              string     `json:"market" gorm:"size:16;not null"`
	Symbol              string     `json:"symbol" gorm:"size:64;not null;index"`
	Bucket              string     `json:"bucket" gorm:"size:16;not null;index"`
	LayerNo             int        `json:"layer_no" gorm:"not null"`
	SourceOrderIntentID string     `json:"source_order_intent_id" gorm:"size:36;not null;index"`
	SourceBuyFillID     string     `json:"source_buy_fill_id" gorm:"size:36;not null;uniqueIndex"`
	AcquiredQty         string     `json:"acquired_qty" gorm:"type:decimal(36,18);not null"`
	RemainingQty        string     `json:"remaining_qty" gorm:"type:decimal(36,18);not null"`
	ReservedQty         string     `json:"reserved_qty" gorm:"type:decimal(36,18);not null;default:0"`
	TotalCost           string     `json:"total_cost" gorm:"type:decimal(36,18);not null"`
	UnitCost            string     `json:"unit_cost" gorm:"type:decimal(36,18);not null"`
	TargetNetProfitRate string     `json:"target_net_profit_rate" gorm:"type:decimal(18,8);not null"`
	TargetPrice         string     `json:"target_price" gorm:"type:decimal(36,18);not null"`
	ConfigVersion       int        `json:"config_version" gorm:"not null"`
	Status              string     `json:"status" gorm:"size:24;not null;index"`
	OpenedAt            time.Time  `json:"opened_at" gorm:"not null;index"`
	ClosedAt            *time.Time `json:"closed_at"`
	Version             uint64     `json:"version" gorm:"not null;default:1"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (*SpotPositionLot) TableName() string {
	return config.C.FormatTableName("trading_spot_position_lots")
}

type SpotOrderIntent struct {
	ID                 string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID        string    `json:"owner_user_id" gorm:"size:20;not null;uniqueIndex:uk_spot_intent_idempotency,priority:1;index"`
	IdempotencyKey     string    `json:"idempotency_key" gorm:"size:128;not null;uniqueIndex:uk_spot_intent_idempotency,priority:2"`
	StrategyID         string    `json:"strategy_id" gorm:"size:64;not null;index"`
	CycleID            string    `json:"cycle_id" gorm:"size:36;not null;index"`
	StateEventID       string    `json:"state_event_id" gorm:"size:36;index"`
	TradeCommandID     string    `json:"trade_command_id" gorm:"size:36;index"`
	Side               string    `json:"side" gorm:"size:8;not null"`
	Bucket             string    `json:"bucket" gorm:"size:16;not null"`
	LayerNo            *int      `json:"layer_no"`
	SourceLotsJSON     string    `json:"source_lots" gorm:"type:json;not null"`
	OrderType          string    `json:"order_type" gorm:"size:16;not null"`
	Price              string    `json:"price" gorm:"type:decimal(36,18);not null"`
	Quantity           string    `json:"quantity" gorm:"type:decimal(36,18);not null"`
	ReasonCode         string    `json:"reason_code" gorm:"size:64;not null;index"`
	SignalSnapshotJSON string    `json:"signal_snapshot" gorm:"type:json;not null"`
	Status             string    `json:"status" gorm:"size:24;not null;index"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (*SpotOrderIntent) TableName() string {
	return config.C.FormatTableName("trading_spot_order_intents")
}

type SpotSellAllocation struct {
	ID               string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID      string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	SellFillID       string    `json:"sell_fill_id" gorm:"size:36;not null;uniqueIndex:uk_spot_sell_allocation,priority:1"`
	LotID            string    `json:"lot_id" gorm:"size:36;not null;uniqueIndex:uk_spot_sell_allocation,priority:2;index"`
	AllocatedQty     string    `json:"allocated_qty" gorm:"type:decimal(36,18);not null"`
	AllocatedCost    string    `json:"allocated_cost" gorm:"type:decimal(36,18);not null"`
	AllocatedBuyFee  string    `json:"allocated_buy_fee" gorm:"type:decimal(36,18);not null;default:0"`
	AllocatedSellFee string    `json:"allocated_sell_fee" gorm:"type:decimal(36,18);not null;default:0"`
	NetProceeds      string    `json:"net_proceeds" gorm:"type:decimal(36,18);not null"`
	RealizedPnL      string    `json:"realized_pnl" gorm:"column:realized_pnl;type:decimal(36,18);not null"`
	RealizedPnLRate  string    `json:"realized_pnl_rate" gorm:"column:realized_pnl_rate;type:decimal(18,8);not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"not null;index"`
}

func (*SpotSellAllocation) TableName() string {
	return config.C.FormatTableName("trading_spot_sell_allocations")
}
func (*SpotSellAllocation) BeforeUpdate(*gorm.DB) error {
	return errors.New("spot sell allocations are immutable")
}
func (*SpotSellAllocation) BeforeDelete(*gorm.DB) error {
	return errors.New("spot sell allocations are immutable")
}

type SpotStateEvent struct {
	ID                 string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID        string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	StrategyID         string    `json:"strategy_id" gorm:"size:64;not null;index"`
	CycleID            string    `json:"cycle_id" gorm:"size:36;not null;index"`
	EventType          string    `json:"event_type" gorm:"size:40;not null;index"`
	FromState          string    `json:"from_state" gorm:"size:24"`
	ToState            string    `json:"to_state" gorm:"size:24;not null"`
	ReasonCode         string    `json:"reason_code" gorm:"size:64;not null;index"`
	MarketTime         time.Time `json:"market_time" gorm:"not null;index"`
	ProcessedAt        time.Time `json:"processed_at" gorm:"not null"`
	ConfigVersion      int       `json:"config_version" gorm:"not null"`
	SignalSnapshotJSON string    `json:"signal_snapshot" gorm:"type:json;not null"`
	RiskSnapshotJSON   string    `json:"risk_snapshot" gorm:"type:json;not null"`
	CorrelationID      string    `json:"correlation_id" gorm:"size:64;not null;index"`
}

func (*SpotStateEvent) TableName() string {
	return config.C.FormatTableName("trading_spot_state_events")
}
func (*SpotStateEvent) BeforeUpdate(*gorm.DB) error {
	return errors.New("spot state events are immutable")
}
func (*SpotStateEvent) BeforeDelete(*gorm.DB) error {
	return errors.New("spot state events are immutable")
}

type SpotBacktestTask struct {
	ID                string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID       string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	StrategyID        string     `json:"strategy_id" gorm:"size:64;index"`
	Status            string     `json:"status" gorm:"size:24;not null;index"`
	Exchange          string     `json:"exchange" gorm:"size:16;not null"`
	Symbol            string     `json:"symbol" gorm:"size:64;not null;index"`
	Interval          string     `json:"interval" gorm:"size:8;not null"`
	StartTime         time.Time  `json:"start_time" gorm:"not null"`
	EndTime           time.Time  `json:"end_time" gorm:"not null"`
	InitialCapital    string     `json:"initial_capital" gorm:"type:decimal(36,18);not null"`
	ConfigJSON        string     `json:"config" gorm:"type:json;not null"`
	CostScenarioJSON  string     `json:"cost_scenario" gorm:"type:json;not null"`
	BenchmarksJSON    string     `json:"benchmarks" gorm:"type:json;not null"`
	Progress          int        `json:"progress" gorm:"not null;default:0"`
	ProcessedMarketAt *time.Time `json:"processed_market_at"`
	ErrorCode         string     `json:"error_code" gorm:"size:64"`
	ErrorMessage      string     `json:"error_message" gorm:"size:1000"`
	CancelRequested   bool       `json:"cancel_requested" gorm:"not null;default:false"`
	CodeVersion       string     `json:"code_version" gorm:"size:64"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (*SpotBacktestTask) TableName() string {
	return config.C.FormatTableName("trading_spot_backtest_tasks")
}

type SpotBacktestResult struct {
	TaskID          string    `json:"task_id" gorm:"size:36;primaryKey"`
	MetricsJSON     string    `json:"metrics" gorm:"type:json;not null"`
	BenchmarksJSON  string    `json:"benchmarks" gorm:"type:json;not null"`
	EquityCurveJSON string    `json:"equity_curve" gorm:"type:longtext;not null"`
	LotsJSON        string    `json:"lots" gorm:"type:longtext;not null"`
	EventsJSON      string    `json:"events" gorm:"type:longtext;not null"`
	AcceptanceJSON  string    `json:"acceptance" gorm:"type:json;not null"`
	DataPoints      int       `json:"data_points" gorm:"not null"`
	CreatedAt       time.Time `json:"created_at"`
}

func (*SpotBacktestResult) TableName() string {
	return config.C.FormatTableName("trading_spot_backtest_results")
}

type SpotBacktestCostScenario struct {
	FeeRateBPS  float64 `json:"fee_rate_bps"`
	SlippageBPS float64 `json:"slippage_bps"`
}

type SpotBacktestCreateForm struct {
	StrategyID     string                   `json:"strategy_id"`
	Exchange       string                   `json:"exchange" binding:"required"`
	Symbol         string                   `json:"symbol" binding:"required"`
	Interval       string                   `json:"interval" binding:"required"`
	StartTime      time.Time                `json:"start_time" binding:"required"`
	EndTime        time.Time                `json:"end_time" binding:"required"`
	InitialCapital string                   `json:"initial_capital" binding:"required"`
	Config         SpotCoreSatelliteConfig  `json:"config"`
	CostScenario   SpotBacktestCostScenario `json:"cost_scenario"`
	Benchmarks     []string                 `json:"benchmarks"`
}

func (a *SpotBacktestCreateForm) Validate() error {
	a.Exchange = strings.ToLower(strings.TrimSpace(a.Exchange))
	a.Symbol = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(a.Symbol), "/", ""))
	a.Interval = strings.ToLower(strings.TrimSpace(a.Interval))
	a.InitialCapital = strings.TrimSpace(a.InitialCapital)
	if a.StartTime.IsZero() || a.EndTime.IsZero() || !a.StartTime.Before(a.EndTime) {
		return fmt.Errorf("backtest time range is invalid")
	}
	if a.EndTime.After(time.Now().UTC().Add(5 * time.Minute)) {
		return fmt.Errorf("backtest end time cannot be in the future")
	}
	if a.Interval != "4h" && a.Interval != "1d" {
		return fmt.Errorf("backtest interval must be 4h or 1d")
	}
	if a.InitialCapital == "" {
		return fmt.Errorf("initial capital is required")
	}
	a.Config.Exchange, a.Config.Symbol, a.Config.ExecutionTimeframe = a.Exchange, a.Symbol, a.Interval
	a.Config.Normalize()
	a.Exchange, a.Symbol, a.Interval = a.Config.Exchange, a.Config.Symbol, a.Config.ExecutionTimeframe
	if a.CostScenario.FeeRateBPS > 0 {
		a.Config.FeeRateBPS = a.CostScenario.FeeRateBPS
	}
	if a.CostScenario.SlippageBPS > 0 {
		a.Config.SlippageBPS = a.CostScenario.SlippageBPS
	}
	if err := a.Config.Validate(); err != nil {
		return err
	}
	if len(a.Benchmarks) == 0 {
		a.Benchmarks = []string{"buy_and_hold", "fixed_dca", "core_only"}
	}
	return nil
}

type SpotBacktestQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	Status      string `form:"status"`
	Symbol      string `form:"symbol"`
}

type SpotLotQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	StrategyID  string `form:"strategyID"`
	CycleID     string `form:"cycleID"`
	Bucket      string `form:"bucket"`
	Status      string `form:"status"`
}

type SpotBacktestQueryResult struct {
	Data       []*SpotBacktestTask
	PageResult *util.PaginationResult
}
type SpotLotQueryResult struct {
	Data       []*SpotPositionLot
	PageResult *util.PaginationResult
}

type SpotBacktestMetrics struct {
	InitialCapital        float64 `json:"initial_capital"`
	FinalEquity           float64 `json:"final_equity"`
	TotalReturn           float64 `json:"total_return"`
	AnnualizedReturn      float64 `json:"annualized_return"`
	MaxDrawdown           float64 `json:"max_drawdown"`
	Calmar                float64 `json:"calmar"`
	ProfitFactor          float64 `json:"profit_factor"`
	ProfitFactorAvailable bool    `json:"profit_factor_available"`
	RealizedPnL           float64 `json:"realized_pnl"`
	Fees                  float64 `json:"fees"`
	Turnover              float64 `json:"turnover"`
	AverageCashRatio      float64 `json:"average_cash_ratio"`
	MinimumCashRatio      float64 `json:"minimum_cash_ratio"`
	MaximumExposureRatio  float64 `json:"maximum_exposure_ratio"`
	MaximumDailyBuyRatio  float64 `json:"maximum_daily_buy_ratio"`
	TradeCount            int     `json:"trade_count"`
	WinningTrades         int     `json:"winning_trades"`
	LosingTrades          int     `json:"losing_trades"`
	CoreRemainingQuantity float64 `json:"core_remaining_quantity"`
	SatelliteRemainingQty float64 `json:"satellite_remaining_quantity"`
}

type SpotBacktestBenchmark struct {
	Name        string  `json:"name"`
	FinalEquity float64 `json:"final_equity"`
	TotalReturn float64 `json:"total_return"`
	MaxDrawdown float64 `json:"max_drawdown"`
	Calmar      float64 `json:"calmar"`
}

type SpotBacktestPoint struct {
	Time     int64   `json:"time"`
	Equity   float64 `json:"equity"`
	Cash     float64 `json:"cash"`
	Exposure float64 `json:"exposure"`
	State    string  `json:"state"`
}
type SpotBacktestLot struct {
	ID           string  `json:"id"`
	Bucket       string  `json:"bucket"`
	LayerNo      int     `json:"layer_no"`
	OpenTime     int64   `json:"open_time"`
	OpenPrice    float64 `json:"open_price"`
	AcquiredQty  float64 `json:"acquired_qty"`
	RemainingQty float64 `json:"remaining_qty"`
	TotalCost    float64 `json:"total_cost"`
	TargetPrice  float64 `json:"target_price"`
	RealizedPnL  float64 `json:"realized_pnl"`
	ClosedAt     *int64  `json:"closed_at,omitempty"`
}
type SpotBacktestEvent struct {
	Time      int64   `json:"time"`
	Type      string  `json:"type"`
	FromState string  `json:"from_state,omitempty"`
	ToState   string  `json:"to_state,omitempty"`
	Reason    string  `json:"reason"`
	LotID     string  `json:"lot_id,omitempty"`
	Quantity  float64 `json:"quantity,omitempty"`
	Price     float64 `json:"price,omitempty"`
}
type SpotAcceptanceItem struct {
	Key           string `json:"key"`
	Passed        bool   `json:"passed"`
	NotApplicable bool   `json:"not_applicable,omitempty"`
	Actual        string `json:"actual"`
	Expected      string `json:"expected"`
}
type SpotBacktestOutput struct {
	Metrics     SpotBacktestMetrics     `json:"metrics"`
	Benchmarks  []SpotBacktestBenchmark `json:"benchmarks"`
	EquityCurve []SpotBacktestPoint     `json:"equity_curve"`
	Lots        []SpotBacktestLot       `json:"lots"`
	Events      []SpotBacktestEvent     `json:"events"`
	Acceptance  []SpotAcceptanceItem    `json:"acceptance"`
}

type SpotBacktestResultView struct {
	Task        *SpotBacktestTask       `json:"task"`
	Metrics     SpotBacktestMetrics     `json:"metrics"`
	Benchmarks  []SpotBacktestBenchmark `json:"benchmarks"`
	EquityCurve []SpotBacktestPoint     `json:"equity_curve"`
	Lots        []SpotBacktestLot       `json:"lots"`
	Events      []SpotBacktestEvent     `json:"events"`
	Acceptance  []SpotAcceptanceItem    `json:"acceptance"`
}

func DecodeSpotBacktestResult(task *SpotBacktestTask, result *SpotBacktestResult) (*SpotBacktestResultView, error) {
	view := &SpotBacktestResultView{Task: task}
	for raw, target := range map[string]interface{}{result.MetricsJSON: &view.Metrics, result.BenchmarksJSON: &view.Benchmarks,
		result.EquityCurveJSON: &view.EquityCurve, result.LotsJSON: &view.Lots, result.EventsJSON: &view.Events,
		result.AcceptanceJSON: &view.Acceptance} {
		if err := json.Unmarshal([]byte(raw), target); err != nil {
			return nil, err
		}
	}
	return view, nil
}
