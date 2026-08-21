package schema

import (
	"errors"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

const (
	UserStreamStatusConnecting   = "connecting"
	UserStreamStatusConnected    = "connected"
	UserStreamStatusReconnecting = "reconnecting"
	UserStreamStatusStopped      = "stopped"
	UserStreamStatusError        = "error"
)

type UserStreamState struct {
	ExchangeID       string     `json:"exchange_id" gorm:"size:36;primaryKey"`
	OwnerUserID      string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	ExchangeType     string     `json:"exchange_type" gorm:"size:16;not null"`
	Status           string     `json:"status" gorm:"size:20;not null;index"`
	ReconnectCount   int        `json:"reconnect_count" gorm:"not null;default:0"`
	ConnectedAt      *time.Time `json:"connected_at"`
	LastEventAt      *time.Time `json:"last_event_at"`
	LastReconciledAt *time.Time `json:"last_reconciled_at"`
	LastError        string     `json:"last_error" gorm:"size:1000"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"not null;index"`
}

func (*UserStreamState) TableName() string {
	return config.C.FormatTableName("trading_user_stream_states")
}

type UserStreamEvent struct {
	ID              uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerUserID     string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	ExchangeID      string     `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_user_stream_event,priority:1;index"`
	ExchangeType    string     `json:"exchange_type" gorm:"size:16;not null"`
	EventType       string     `json:"event_type" gorm:"size:64;not null;index"`
	PayloadHash     string     `json:"payload_hash" gorm:"size:64;not null;uniqueIndex:uk_trading_user_stream_event,priority:2"`
	EventTime       time.Time  `json:"event_time" gorm:"not null;index"`
	TransactionTime *time.Time `json:"transaction_time,omitempty"`
	ReceivedAt      time.Time  `json:"received_at" gorm:"not null;index"`
}

func (*UserStreamEvent) TableName() string {
	return config.C.FormatTableName("trading_user_stream_events")
}
func (*UserStreamEvent) BeforeUpdate(*gorm.DB) error {
	return errors.New("user stream events are immutable")
}
func (*UserStreamEvent) BeforeDelete(*gorm.DB) error {
	return errors.New("user stream events are immutable")
}

type UserStreamEventQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	ExchangeID  string `form:"exchangeID"`
	EventType   string `form:"eventType"`
}

type UserStreamEventQueryResult struct {
	Data       []*UserStreamEvent
	PageResult *util.PaginationResult
}
