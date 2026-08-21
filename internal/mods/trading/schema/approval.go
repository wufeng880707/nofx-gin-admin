package schema

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"gorm.io/gorm"
)

const (
	TradeApprovalStatusPending   = "pending"
	TradeApprovalStatusApproved  = "approved"
	TradeApprovalStatusExecuting = "executing"
	TradeApprovalStatusSucceeded = "succeeded"
	TradeApprovalStatusFailed    = "failed"
	TradeApprovalStatusRejected  = "rejected"
	TradeApprovalStatusExpired   = "expired"
)

// TradeApproval is a user-visible approval request derived from one validated
// strategy action. Its command envelope is immutable after creation.
type TradeApproval struct {
	ID                    string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID           string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	TraderID              string     `json:"trader_id" gorm:"size:64;not null;index"`
	ExchangeID            string     `json:"exchange_id" gorm:"size:36;not null;index"`
	DecisionID            uint64     `json:"decision_id" gorm:"not null;uniqueIndex:uk_trading_approval_action,priority:1;index"`
	ActionIndex           int        `json:"action_index" gorm:"not null;uniqueIndex:uk_trading_approval_action,priority:2"`
	Action                string     `json:"action" gorm:"size:16;not null"`
	Market                string     `json:"market" gorm:"size:16;not null"`
	Symbol                string     `json:"symbol" gorm:"size:64;not null;index"`
	Side                  string     `json:"side" gorm:"size:8;not null"`
	PositionSide          string     `json:"position_side" gorm:"size:8"`
	OrderType             string     `json:"order_type" gorm:"size:16;not null"`
	Quantity              string     `json:"quantity" gorm:"type:decimal(36,18);not null;default:0"`
	QuoteQuantity         string     `json:"quote_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	Price                 string     `json:"price" gorm:"type:decimal(36,18);not null;default:0"`
	StopLoss              string     `json:"stop_loss" gorm:"type:decimal(36,18);not null;default:0"`
	TakeProfit            string     `json:"take_profit" gorm:"type:decimal(36,18);not null;default:0"`
	Leverage              int        `json:"leverage" gorm:"not null;default:1"`
	ReduceOnly            bool       `json:"reduce_only" gorm:"not null;default:false"`
	TargetExchangeOrderID string     `json:"target_exchange_order_id" gorm:"size:128"`
	Reason                string     `json:"reason" gorm:"size:2000"`
	Status                string     `json:"status" gorm:"size:16;not null;index"`
	CommandID             string     `json:"command_id" gorm:"size:36;index"`
	FailureCode           string     `json:"failure_code" gorm:"size:64"`
	FailureMessage        string     `json:"failure_message" gorm:"size:1000"`
	RequestedAt           time.Time  `json:"requested_at" gorm:"not null;index"`
	ExpiresAt             time.Time  `json:"expires_at" gorm:"not null;index"`
	ActedBy               string     `json:"acted_by" gorm:"size:20"`
	ActedAt               *time.Time `json:"acted_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (*TradeApproval) TableName() string {
	return config.C.FormatTableName("trading_trade_approvals")
}

func (a *TradeApproval) BeforeCreate(*gorm.DB) error {
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	a.Status = strings.ToLower(strings.TrimSpace(a.Status))
	if a.Status != TradeApprovalStatusPending {
		return fmt.Errorf("new trade approval must be pending")
	}
	return nil
}

// TradeApprovalEvent is append-only audit evidence for every approval state transition.
type TradeApprovalEvent struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	ApprovalID  string    `json:"approval_id" gorm:"size:36;not null;index:idx_trading_approval_event_time,priority:1"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	ActorUserID string    `json:"actor_user_id" gorm:"size:20;not null"`
	EventType   string    `json:"event_type" gorm:"size:32;not null;index"`
	FromStatus  string    `json:"from_status" gorm:"size:16"`
	ToStatus    string    `json:"to_status" gorm:"size:16;not null"`
	DetailJSON  string    `json:"detail" gorm:"type:json;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;index:idx_trading_approval_event_time,priority:2"`
}

func (*TradeApprovalEvent) TableName() string {
	return config.C.FormatTableName("trading_trade_approval_events")
}

func (*TradeApprovalEvent) BeforeUpdate(*gorm.DB) error {
	return errors.New("trade approval events are immutable")
}
func (*TradeApprovalEvent) BeforeDelete(*gorm.DB) error {
	return errors.New("trade approval events are immutable")
}
