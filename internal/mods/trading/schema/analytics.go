package schema

import (
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

// ClosedTrade is a deterministic round trip rebuilt from immutable exchange fills.
// Raw fills remain the source of truth; this table is a query-optimized fact model.
type ClosedTrade struct {
	ID                string    `json:"id" gorm:"size:64;primaryKey"`
	OwnerUserID       string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_closed_trade_owner"`
	TraderID          string    `json:"trader_id" gorm:"size:64;not null;index"`
	StrategyID        string    `json:"strategy_id" gorm:"size:64;index"`
	StrategyVersion   int       `json:"strategy_version" gorm:"not null;default:0;index"`
	AttributionSource string    `json:"attribution_source" gorm:"size:16;not null;default:external;index"`
	ExchangeID        string    `json:"exchange_id" gorm:"size:36;not null;index:idx_trading_closed_trade_scope,priority:1"`
	Market            string    `json:"market" gorm:"size:16;not null;index:idx_trading_closed_trade_scope,priority:2"`
	Symbol            string    `json:"symbol" gorm:"size:64;not null;index:idx_trading_closed_trade_scope,priority:3"`
	Side              string    `json:"side" gorm:"size:16;not null;index"`
	EntryQuantity     string    `json:"entry_quantity" gorm:"type:decimal(36,18);not null"`
	ExitQuantity      string    `json:"exit_quantity" gorm:"type:decimal(36,18);not null"`
	EntryPrice        string    `json:"entry_price" gorm:"type:decimal(36,18);not null"`
	ExitPrice         string    `json:"exit_price" gorm:"type:decimal(36,18);not null"`
	GrossPnL          string    `json:"gross_pnl" gorm:"column:gross_pnl;type:decimal(36,18);not null"`
	Fee               string    `json:"fee" gorm:"type:decimal(36,18);not null"`
	Funding           string    `json:"funding" gorm:"type:decimal(36,18);not null;default:0"`
	NetPnL            string    `json:"net_pnl" gorm:"column:net_pnl;type:decimal(36,18);not null"`
	OpenedAt          time.Time `json:"opened_at" gorm:"not null;index"`
	ClosedAt          time.Time `json:"closed_at" gorm:"not null;index"`
	DurationSeconds   int64     `json:"duration_seconds" gorm:"not null;default:0"`
	SourceFillCount   int       `json:"source_fill_count" gorm:"not null;default:0"`
	RebuiltAt         time.Time `json:"rebuilt_at" gorm:"not null"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (*ClosedTrade) TableName() string {
	return config.C.FormatTableName("trading_closed_trades")
}

type AnalyticsQueryParam struct {
	util.PaginationParam
	OwnerUserID     string     `form:"ownerUserID"`
	TraderID        string     `form:"traderID"`
	StrategyID      string     `form:"strategyID"`
	StrategyVersion int        `form:"strategyVersion"`
	ExchangeID      string     `form:"exchangeID"`
	Market          string     `form:"market"`
	Symbol          string     `form:"symbol"`
	Side            string     `form:"side"`
	StartAt         *time.Time `form:"startAt" time_format:"2006-01-02T15:04:05Z07:00"`
	EndAt           *time.Time `form:"endAt" time_format:"2006-01-02T15:04:05Z07:00"`
}

type ClosedTradeQueryResult struct {
	Data       []*ClosedTrade
	PageResult *util.PaginationResult
}

type DashboardOverview struct {
	TotalEquity      string     `json:"total_equity"`
	AvailableBalance string     `json:"available_balance"`
	RealizedPnL      string     `json:"realized_pnl"`
	UnrealizedPnL    string     `json:"unrealized_pnl"`
	Funding          string     `json:"funding"`
	TotalFee         string     `json:"total_fee"`
	NetPnL           string     `json:"net_pnl"`
	PositionCount    int64      `json:"position_count"`
	ClosedTradeCount int64      `json:"closed_trade_count"`
	LastUpdatedAt    *time.Time `json:"last_updated_at"`
}

type EquityPoint struct {
	CapturedAt       time.Time `json:"captured_at"`
	TotalEquity      string    `json:"total_equity"`
	AvailableBalance string    `json:"available_balance"`
	UnrealizedPnL    string    `json:"unrealized_pnl"`
	PositionCount    int       `json:"position_count"`
}

type PerformanceSummary struct {
	TotalTrades     int64               `json:"total_trades"`
	WinningTrades   int64               `json:"winning_trades"`
	LosingTrades    int64               `json:"losing_trades"`
	BreakEvenTrades int64               `json:"break_even_trades"`
	WinRate         float64             `json:"win_rate"`
	GrossProfit     string              `json:"gross_profit"`
	GrossLoss       string              `json:"gross_loss"`
	GrossPnL        string              `json:"gross_pnl"`
	TotalFee        string              `json:"total_fee"`
	Funding         string              `json:"funding"`
	NetPnL          string              `json:"net_pnl"`
	AverageWin      string              `json:"average_win"`
	AverageLoss     string              `json:"average_loss"`
	ProfitFactor    float64             `json:"profit_factor"`
	ProfitLossRatio float64             `json:"profit_loss_ratio"`
	TradeSharpe     float64             `json:"trade_sharpe"`
	MaxDrawdownPct  float64             `json:"max_drawdown_pct"`
	SharpeBasis     string              `json:"sharpe_basis"`
	DrawdownSource  string              `json:"drawdown_source"`
	BySide          []*PerformanceGroup `json:"by_side"`
	BySymbol        []*PerformanceGroup `json:"by_symbol"`
	ByTrader        []*PerformanceGroup `json:"by_trader"`
	ByStrategy      []*PerformanceGroup `json:"by_strategy"`
	ByExchange      []*PerformanceGroup `json:"by_exchange"`
}

type PerformanceGroup struct {
	Key           string  `json:"key"`
	TotalTrades   int64   `json:"total_trades"`
	WinningTrades int64   `json:"winning_trades"`
	WinRate       float64 `json:"win_rate"`
	NetPnL        string  `json:"net_pnl"`
	AveragePnL    string  `json:"average_pnl"`
}

type PerformanceReportQueryParam struct {
	AnalyticsQueryParam
	Period string `form:"period" binding:"omitempty,oneof=day week month"`
}

type PerformanceReportBucket struct {
	Period  string              `json:"period"`
	StartAt time.Time           `json:"start_at"`
	EndAt   time.Time           `json:"end_at"`
	Summary *PerformanceSummary `json:"summary"`
}
