package schema

import (
	"errors"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"gorm.io/gorm"
)

const (
	TradeCommandStatusValidated = "validated"
	TradeCommandStatusSimulated = "simulated"
	TradeCommandStatusSubmitted = "submitted"
	TradeCommandStatusAccepted  = "accepted"
	TradeCommandStatusPartial   = "partially_filled"
	TradeCommandStatusFilled    = "filled"
	TradeCommandStatusRejected  = "rejected"
	TradeCommandStatusCancelled = "cancelled"
)

// TradingSafetyState is the user-scoped live-trading gate. The process-wide
// configuration gate must also be enabled before a production request can run.
type TradingSafetyState struct {
	OwnerUserID string     `json:"owner_user_id" gorm:"size:20;primaryKey"`
	LiveEnabled bool       `json:"live_enabled" gorm:"not null;default:false;index"`
	Version     uint64     `json:"version" gorm:"not null;default:1"`
	EnabledBy   string     `json:"enabled_by" gorm:"size:20"`
	EnabledAt   *time.Time `json:"enabled_at"`
	DisabledBy  string     `json:"disabled_by" gorm:"size:20"`
	DisabledAt  *time.Time `json:"disabled_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (*TradingSafetyState) TableName() string {
	return config.C.FormatTableName("trading_safety_states")
}

// TradeCommand stores an immutable request envelope plus its mutable delivery
// state. Request fields are never updated after insertion.
type TradeCommand struct {
	ID                    string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID           string     `json:"owner_user_id" gorm:"size:20;not null;uniqueIndex:uk_trading_command_idempotency,priority:1;index"`
	TraderID              string     `json:"trader_id" gorm:"size:64;not null;index"`
	StrategyID            *string    `json:"strategy_id" gorm:"size:64;index"`
	StrategyVersion       int        `json:"strategy_version" gorm:"not null;default:0"`
	ExchangeID            string     `json:"exchange_id" gorm:"size:36;not null;index"`
	DecisionID            *uint64    `json:"decision_id" gorm:"index"`
	IdempotencyKey        string     `json:"idempotency_key" gorm:"size:128;not null;uniqueIndex:uk_trading_command_idempotency,priority:2"`
	Action                string     `json:"action" gorm:"size:16;not null;index"`
	Market                string     `json:"market" gorm:"size:16;not null"`
	Symbol                string     `json:"symbol" gorm:"size:64;not null;index"`
	Side                  string     `json:"side" gorm:"size:8;not null"`
	PositionSide          string     `json:"position_side" gorm:"size:8"`
	OrderType             string     `json:"order_type" gorm:"size:16;not null"`
	TimeInForce           string     `json:"time_in_force" gorm:"size:8"`
	Quantity              string     `json:"quantity" gorm:"type:decimal(36,18);not null;default:0"`
	QuoteQuantity         string     `json:"quote_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	Price                 string     `json:"price" gorm:"type:decimal(36,18);not null;default:0"`
	StopLoss              string     `json:"stop_loss" gorm:"type:decimal(36,18);not null;default:0"`
	TakeProfit            string     `json:"take_profit" gorm:"type:decimal(36,18);not null;default:0"`
	Leverage              int        `json:"leverage" gorm:"not null;default:1"`
	ReduceOnly            bool       `json:"reduce_only" gorm:"not null;default:false"`
	TargetExchangeOrderID string     `json:"target_exchange_order_id" gorm:"size:128"`
	Simulation            bool       `json:"simulation" gorm:"not null;default:true;index"`
	Testnet               bool       `json:"testnet" gorm:"not null;default:false;index"`
	RiskSnapshotJSON      string     `json:"risk_snapshot" gorm:"type:json;not null"`
	Status                string     `json:"status" gorm:"size:16;not null;index"`
	ExchangeOrderID       string     `json:"exchange_order_id" gorm:"size:128;index"`
	ExchangeClientOrderID string     `json:"exchange_client_order_id" gorm:"size:128"`
	ProviderStatus        string     `json:"provider_status" gorm:"size:64"`
	ExecutedQuantity      string     `json:"executed_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	AveragePrice          string     `json:"average_price" gorm:"type:decimal(36,18);not null;default:0"`
	ErrorCode             string     `json:"error_code" gorm:"size:64"`
	ErrorMessage          string     `json:"error_message" gorm:"size:1000"`
	SubmittedAt           *time.Time `json:"submitted_at"`
	FinishedAt            *time.Time `json:"finished_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (*TradeCommand) TableName() string {
	return config.C.FormatTableName("trading_trade_commands")
}

// TradeCommandEvent is append-only evidence for every validation, submission,
// provider acknowledgement, rejection, and cancellation transition.
type TradeCommandEvent struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	CommandID   string    `json:"command_id" gorm:"size:36;not null;index:idx_trading_command_event_time,priority:1"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	ActorUserID string    `json:"actor_user_id" gorm:"size:20;not null"`
	EventType   string    `json:"event_type" gorm:"size:32;not null;index"`
	FromStatus  string    `json:"from_status" gorm:"size:16"`
	ToStatus    string    `json:"to_status" gorm:"size:16;not null"`
	DetailJSON  string    `json:"detail" gorm:"type:json;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;index:idx_trading_command_event_time,priority:2"`
}

func (*TradeCommandEvent) TableName() string {
	return config.C.FormatTableName("trading_trade_command_events")
}

func (*TradeCommandEvent) BeforeUpdate(*gorm.DB) error {
	return errors.New("trade command events are immutable")
}
func (*TradeCommandEvent) BeforeDelete(*gorm.DB) error {
	return errors.New("trade command events are immutable")
}

// TradingSafetyEvent is append-only evidence for enabling and disabling the
// owner-scoped production trading gate.
type TradingSafetyEvent struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	ActorUserID string    `json:"actor_user_id" gorm:"size:20;not null"`
	Action      string    `json:"action" gorm:"size:16;not null;index"`
	Version     uint64    `json:"version" gorm:"not null"`
	Reason      string    `json:"reason" gorm:"size:500"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;index"`
}

func (*TradingSafetyEvent) TableName() string {
	return config.C.FormatTableName("trading_safety_events")
}

func (*TradingSafetyEvent) BeforeUpdate(*gorm.DB) error {
	return errors.New("trading safety events are immutable")
}
func (*TradingSafetyEvent) BeforeDelete(*gorm.DB) error {
	return errors.New("trading safety events are immutable")
}
