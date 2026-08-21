package schema

import (
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
)

const (
	OrderOriginSystem   = "system"
	OrderOriginImported = "imported"
	OrderOriginExternal = "external"
)

// AssetBalance stores the latest normalized balance for one asset in one trading account.
type AssetBalance struct {
	ID            uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerUserID   string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_asset_owner"`
	TraderID      string    `json:"trader_id" gorm:"size:64;not null;uniqueIndex:uk_trading_asset_balance,priority:1;index"`
	ExchangeID    string    `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_asset_balance,priority:2;index"`
	AccountType   string    `json:"account_type" gorm:"size:16;not null;uniqueIndex:uk_trading_asset_balance,priority:3"`
	Asset         string    `json:"asset" gorm:"size:32;not null;uniqueIndex:uk_trading_asset_balance,priority:4;index"`
	Total         string    `json:"total" gorm:"type:decimal(36,18);not null;default:0"`
	Available     string    `json:"available" gorm:"type:decimal(36,18);not null;default:0"`
	Locked        string    `json:"locked" gorm:"type:decimal(36,18);not null;default:0"`
	Borrowed      string    `json:"borrowed" gorm:"type:decimal(36,18);not null;default:0"`
	Interest      string    `json:"interest" gorm:"type:decimal(36,18);not null;default:0"`
	PriceUSDT     string    `json:"price_usdt" gorm:"type:decimal(36,18);not null;default:0"`
	ValueUSDT     string    `json:"value_usdt" gorm:"type:decimal(36,18);not null;default:0;index"`
	UnrealizedPnL string    `json:"unrealized_pnl" gorm:"column:unrealized_pnl;type:decimal(36,18);not null;default:0"`
	CapturedAt    time.Time `json:"captured_at" gorm:"not null;index"`
	Source        string    `json:"source" gorm:"size:32;not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (*AssetBalance) TableName() string { return config.C.FormatTableName("trading_asset_balances") }

// ExchangeOrder is an exchange-normalized order. Imported orders are not attributed to a strategy by default.
type ExchangeOrder struct {
	ID                string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID       string     `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_order_owner"`
	TraderID          string     `json:"trader_id" gorm:"size:64;not null;index"`
	ExchangeID        string     `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_exchange_order,priority:1;index"`
	Market            string     `json:"market" gorm:"size:16;not null;uniqueIndex:uk_trading_exchange_order,priority:2;index"`
	ExchangeOrderID   string     `json:"exchange_order_id" gorm:"size:128;not null;uniqueIndex:uk_trading_exchange_order,priority:3"`
	ClientOrderID     string     `json:"client_order_id" gorm:"size:128;index"`
	SignalID          *string    `json:"signal_id" gorm:"size:36;index"`
	StrategyID        *string    `json:"strategy_id" gorm:"size:64;index"`
	Origin            string     `json:"origin" gorm:"size:16;not null;default:external;index"`
	Symbol            string     `json:"symbol" gorm:"size:64;not null;index"`
	Side              string     `json:"side" gorm:"size:16;not null;index"`
	PositionSide      string     `json:"position_side" gorm:"size:16"`
	OrderType         string     `json:"order_type" gorm:"size:32;not null"`
	Status            string     `json:"status" gorm:"size:32;not null;index"`
	Price             string     `json:"price" gorm:"type:decimal(36,18);not null;default:0"`
	AveragePrice      string     `json:"average_price" gorm:"type:decimal(36,18);not null;default:0"`
	Quantity          string     `json:"quantity" gorm:"type:decimal(36,18);not null;default:0"`
	ExecutedQuantity  string     `json:"executed_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	QuoteQuantity     string     `json:"quote_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	ReduceOnly        bool       `json:"reduce_only" gorm:"not null;default:false"`
	OrderedAt         time.Time  `json:"ordered_at" gorm:"not null;index"`
	ExchangeUpdatedAt time.Time  `json:"exchange_updated_at" gorm:"not null;index"`
	LastSyncedAt      time.Time  `json:"last_synced_at" gorm:"not null;index"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"-" gorm:"index"`
}

func (*ExchangeOrder) TableName() string { return config.C.FormatTableName("trading_orders") }

// Fill stores one immutable exchange trade execution.
type Fill struct {
	ID                string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID       string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_fill_owner"`
	TraderID          string    `json:"trader_id" gorm:"size:64;not null;index"`
	StrategyID        string    `json:"strategy_id" gorm:"size:64;index"`
	StrategyVersion   int       `json:"strategy_version" gorm:"not null;default:0"`
	AttributionSource string    `json:"attribution_source" gorm:"size:16;not null;default:external;index"`
	ExchangeID        string    `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_exchange_fill,priority:1;index"`
	Market            string    `json:"market" gorm:"size:16;not null;uniqueIndex:uk_trading_exchange_fill,priority:2;index"`
	ExchangeTradeID   string    `json:"exchange_trade_id" gorm:"size:128;not null;uniqueIndex:uk_trading_exchange_fill,priority:3"`
	ExchangeOrderID   string    `json:"exchange_order_id" gorm:"size:128;not null;index"`
	Symbol            string    `json:"symbol" gorm:"size:64;not null;index"`
	Side              string    `json:"side" gorm:"size:16;not null;index"`
	PositionSide      string    `json:"position_side" gorm:"size:16"`
	Price             string    `json:"price" gorm:"type:decimal(36,18);not null;default:0"`
	Quantity          string    `json:"quantity" gorm:"type:decimal(36,18);not null;default:0"`
	QuoteQuantity     string    `json:"quote_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	FeeAsset          string    `json:"fee_asset" gorm:"size:32"`
	FeeQuantity       string    `json:"fee_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	FeeValueUSDT      string    `json:"fee_value_usdt" gorm:"type:decimal(36,18);not null;default:0"`
	RealizedPnL       string    `json:"realized_pnl" gorm:"column:realized_pnl;type:decimal(36,18);not null;default:0"`
	Liquidity         string    `json:"liquidity" gorm:"size:16"`
	FilledAt          time.Time `json:"filled_at" gorm:"not null;index"`
	Source            string    `json:"source" gorm:"size:32;not null"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (*Fill) TableName() string { return config.C.FormatTableName("trading_fills") }

// IncomeEvent stores funding fees, realized PnL, commissions, rebates, and other account ledger events.
type IncomeEvent struct {
	ID              string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID     string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_income_owner"`
	TraderID        string    `json:"trader_id" gorm:"size:64;not null;index"`
	ExchangeID      string    `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_exchange_income,priority:1;index"`
	EventType       string    `json:"event_type" gorm:"size:32;not null;uniqueIndex:uk_trading_exchange_income,priority:2;index"`
	ExchangeEventID string    `json:"exchange_event_id" gorm:"size:128;not null;uniqueIndex:uk_trading_exchange_income,priority:3"`
	Market          string    `json:"market" gorm:"size:16;not null;index"`
	Symbol          string    `json:"symbol" gorm:"size:64;index"`
	Asset           string    `json:"asset" gorm:"size:32;not null"`
	Amount          string    `json:"amount" gorm:"type:decimal(36,18);not null;default:0"`
	PriceUSDT       string    `json:"price_usdt" gorm:"type:decimal(36,18);not null;default:0"`
	ValueUSDT       string    `json:"value_usdt" gorm:"type:decimal(36,18);not null;default:0"`
	OccurredAt      time.Time `json:"occurred_at" gorm:"not null;index"`
	Source          string    `json:"source" gorm:"size:32;not null"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (*IncomeEvent) TableName() string { return config.C.FormatTableName("trading_income_events") }

// SyncCursor records the incremental synchronization watermark and latest failure per dataset.
type SyncCursor struct {
	ID           uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TraderID     string     `json:"trader_id" gorm:"size:64;not null;uniqueIndex:uk_trading_sync_cursor,priority:1"`
	ExchangeID   string     `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_sync_cursor,priority:2;index"`
	Dataset      string     `json:"dataset" gorm:"size:32;not null;uniqueIndex:uk_trading_sync_cursor,priority:3"`
	Market       string     `json:"market" gorm:"size:16;not null;uniqueIndex:uk_trading_sync_cursor,priority:4"`
	Symbol       string     `json:"symbol" gorm:"size:64;not null;default:'';uniqueIndex:uk_trading_sync_cursor,priority:5"`
	Cursor       string     `json:"cursor" gorm:"size:256"`
	SyncedUntil  *time.Time `json:"synced_until" gorm:"index"`
	Status       string     `json:"status" gorm:"size:16;not null;default:pending;index"`
	ErrorMessage string     `json:"error_message" gorm:"size:1000"`
	LastRunAt    *time.Time `json:"last_run_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (*SyncCursor) TableName() string { return config.C.FormatTableName("trading_sync_cursors") }

// HistorySymbol stores an explicit instrument registry used by exchange APIs
// that cannot enumerate historical orders without a symbol.
type HistorySymbol struct {
	ID          string    `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_history_symbol_owner"`
	ExchangeID  string    `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_history_symbol,priority:1;index"`
	Symbol      string    `json:"symbol" gorm:"size:64;not null;uniqueIndex:uk_trading_history_symbol,priority:2"`
	Source      string    `json:"source" gorm:"size:16;not null;default:manual"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (*HistorySymbol) TableName() string {
	return config.C.FormatTableName("trading_history_symbols")
}
