package schema

import (
	"fmt"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"gorm.io/gorm"
)

const (
	NotificationChannelTelegram = "telegram"
	NotificationChannelLark     = "lark"

	TraderExecutionNotifyOnly       = "notify_only"
	TraderExecutionApprovalRequired = "approval_required"
	TraderExecutionAuto             = "auto"

	TraderNotificationSignal          = "signal"
	TraderNotificationApprovalRequest = "approval_request"
	TraderNotificationExecutionResult = "execution_result"
	TraderNotificationSystemError     = "system_error"
)

type TraderNotificationEvent struct {
	EventType string
	Title     string
	Body      string
}

// NotificationChannel stores one outbound destination. Secrets are ciphertext only.
type NotificationChannel struct {
	ID                      string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID             string     `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_notification_owner"`
	Name                    string     `json:"name" gorm:"size:128;not null"`
	ChannelType             string     `json:"channel_type" gorm:"size:16;not null;index"`
	Enabled                 bool       `json:"enabled" gorm:"not null;default:true;index"`
	BotTokenCiphertext      string     `json:"-" gorm:"type:text"`
	ChatID                  string     `json:"chat_id" gorm:"size:128"`
	WebhookURLCiphertext    string     `json:"-" gorm:"type:text"`
	SigningSecretCiphertext string     `json:"-" gorm:"type:text"`
	CredentialVersion       int        `json:"-" gorm:"not null;default:1"`
	LastTestedAt            *time.Time `json:"last_tested_at" gorm:"index"`
	LastTestStatus          string     `json:"last_test_status" gorm:"size:16"`
	LastTestMessage         string     `json:"last_test_message" gorm:"size:500"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func (*NotificationChannel) TableName() string {
	return config.C.FormatTableName("trading_notification_channels")
}

func (a *NotificationChannel) BeforeSave(*gorm.DB) error {
	a.ChannelType = strings.ToLower(strings.TrimSpace(a.ChannelType))
	if a.ChannelType != NotificationChannelTelegram && a.ChannelType != NotificationChannelLark {
		return fmt.Errorf("unsupported notification channel type %q", a.ChannelType)
	}
	return requireEncryptedCredentials(a.BotTokenCiphertext, a.WebhookURLCiphertext, a.SigningSecretCiphertext)
}

// TraderNotificationBinding is the many-to-many relation between traders and destinations.
type TraderNotificationBinding struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TraderID  string    `json:"trader_id" gorm:"size:64;not null;uniqueIndex:uk_trading_trader_notification,priority:1;index"`
	ChannelID string    `json:"channel_id" gorm:"size:36;not null;uniqueIndex:uk_trading_trader_notification,priority:2;index"`
	CreatedAt time.Time `json:"created_at"`
}

func (*TraderNotificationBinding) TableName() string {
	return config.C.FormatTableName("trading_trader_notification_bindings")
}

type NotificationChannelView struct {
	ID               string     `json:"id"`
	OwnerUserID      string     `json:"owner_user_id"`
	Name             string     `json:"name"`
	ChannelType      string     `json:"channel_type"`
	Enabled          bool       `json:"enabled"`
	ChatID           string     `json:"chat_id"`
	HasBotToken      bool       `json:"has_bot_token"`
	HasWebhookURL    bool       `json:"has_webhook_url"`
	HasSigningSecret bool       `json:"has_signing_secret"`
	LastTestedAt     *time.Time `json:"last_tested_at"`
	LastTestStatus   string     `json:"last_test_status"`
	LastTestMessage  string     `json:"last_test_message"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type NotificationChannelTestView struct {
	ChannelID   string `json:"channel_id"`
	ChannelType string `json:"channel_type"`
	Delivered   bool   `json:"delivered"`
	LatencyMS   int64  `json:"latency_ms"`
	Message     string `json:"message"`
}
