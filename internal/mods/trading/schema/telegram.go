package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
)

const (
	TelegramSessionActive   = "active"
	TelegramSessionUnbound  = "unbound"
	TelegramBindingMaxTries = 5
)

// TelegramBindingCode stores a short-lived one-time binding secret as a hash.
// PublicID is only a lookup handle and cannot be used to reconstruct the code.
type TelegramBindingCode struct {
	ID          string     `json:"id" gorm:"size:36;primaryKey"`
	PublicID    string     `json:"public_id" gorm:"size:16;not null;uniqueIndex"`
	OwnerUserID string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	ChannelID   string     `json:"channel_id" gorm:"size:36;not null;index"`
	CodeHash    string     `json:"-" gorm:"size:64;not null"`
	Attempts    int        `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts int        `json:"max_attempts" gorm:"not null;default:5"`
	ExpiresAt   time.Time  `json:"expires_at" gorm:"not null;index"`
	ConsumedAt  *time.Time `json:"consumed_at" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (*TelegramBindingCode) TableName() string {
	return config.C.FormatTableName("trading_telegram_binding_codes")
}

// TelegramSession is the owner-scoped authorization boundary for bot commands.
type TelegramSession struct {
	ID                 string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID        string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	ChannelID          string     `json:"channel_id" gorm:"size:36;not null;uniqueIndex"`
	ChatID             string     `json:"chat_id" gorm:"size:128;not null;index"`
	TelegramUserID     string     `json:"telegram_user_id" gorm:"size:64"`
	TelegramUsername   string     `json:"telegram_username" gorm:"size:128"`
	Status             string     `json:"status" gorm:"size:16;not null;default:active;index"`
	BoundAt            time.Time  `json:"bound_at" gorm:"not null"`
	UnboundAt          *time.Time `json:"unbound_at"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	LastCommand        string     `json:"last_command" gorm:"size:64"`
	LastTelegramUpdate int64      `json:"last_telegram_update" gorm:"not null;default:0"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (*TelegramSession) TableName() string {
	return config.C.FormatTableName("trading_telegram_sessions")
}

// TelegramPollState persists the Bot API offset independently from a binding
// session so unbound or invalid commands are not replayed after a restart.
type TelegramPollState struct {
	ChannelID    string    `json:"channel_id" gorm:"size:36;primaryKey"`
	LastUpdateID int64     `json:"last_update_id" gorm:"not null;default:0"`
	LastPolledAt time.Time `json:"last_polled_at" gorm:"not null"`
	LastError    string    `json:"last_error" gorm:"size:500"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (*TelegramPollState) TableName() string {
	return config.C.FormatTableName("trading_telegram_poll_states")
}

type TelegramBindingCodeView struct {
	ChannelID string    `json:"channel_id"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TelegramSessionView struct {
	ChannelID        string     `json:"channel_id"`
	Bound            bool       `json:"bound"`
	Status           string     `json:"status"`
	ChatID           string     `json:"chat_id"`
	TelegramUsername string     `json:"telegram_username"`
	BoundAt          *time.Time `json:"bound_at"`
	LastMessageAt    *time.Time `json:"last_message_at"`
}

func HashTelegramBindingCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}
