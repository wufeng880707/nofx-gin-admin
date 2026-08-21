package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"gorm.io/gorm"
)

const (
	PositionStatusOpen   = "open"
	PositionStatusClosed = "closed"
)

// AccountSnapshot is an immutable account observation captured from a provider.
type AccountSnapshot struct {
	ID               uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	LegacySource     *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_account_snapshot_legacy,priority:1"`
	LegacyID         *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_account_snapshot_legacy,priority:2"`
	OwnerUserID      string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_account_snapshot_owner"`
	TraderID         string    `json:"trader_id" gorm:"size:64;not null;index:idx_trading_account_snapshot_trader_time,priority:1"`
	ExchangeID       string    `json:"exchange_id" gorm:"size:36;not null;index"`
	CapturedAt       time.Time `json:"captured_at" gorm:"not null;index:idx_trading_account_snapshot_trader_time,priority:2,sort:desc;index"`
	TotalEquity      string    `json:"total_equity" gorm:"type:decimal(36,18);not null;default:0"`
	Balance          string    `json:"balance" gorm:"type:decimal(36,18);not null;default:0"`
	AvailableBalance string    `json:"available_balance" gorm:"type:decimal(36,18);not null;default:0"`
	UnrealizedPnL    string    `json:"unrealized_pnl" gorm:"column:unrealized_pnl;type:decimal(36,18);not null;default:0"`
	PositionCount    int       `json:"position_count" gorm:"not null;default:0"`
	MarginUsedPct    string    `json:"margin_used_pct" gorm:"type:decimal(18,8);not null;default:0"`
	Source           string    `json:"source" gorm:"size:32;not null;default:provider"`
	CreatedAt        time.Time `json:"created_at"`
}

func (*AccountSnapshot) TableName() string {
	return config.C.FormatTableName("trading_account_snapshots")
}

// Position stores the normalized lifecycle of one exchange position.
type Position struct {
	ID                 string     `json:"id" gorm:"size:36;primaryKey"`
	LegacySource       *string    `json:"-" gorm:"size:64;uniqueIndex:uk_trading_position_legacy,priority:1"`
	LegacyID           *string    `json:"-" gorm:"size:64;uniqueIndex:uk_trading_position_legacy,priority:2"`
	OwnerUserID        string     `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_position_owner"`
	TraderID           string     `json:"trader_id" gorm:"size:64;not null;index"`
	ExchangeID         string     `json:"exchange_id" gorm:"size:36;not null;uniqueIndex:uk_trading_exchange_position,priority:1"`
	ExchangePositionID string     `json:"exchange_position_id" gorm:"size:128;not null;uniqueIndex:uk_trading_exchange_position,priority:2"`
	Symbol             string     `json:"symbol" gorm:"size:64;not null;index"`
	Side               string     `json:"side" gorm:"size:16;not null;index"`
	Status             string     `json:"status" gorm:"size:16;not null;default:open;index"`
	EntryQuantity      string     `json:"entry_quantity" gorm:"type:decimal(36,18);not null;default:0"`
	Quantity           string     `json:"quantity" gorm:"type:decimal(36,18);not null;default:0"`
	EntryPrice         string     `json:"entry_price" gorm:"type:decimal(36,18);not null;default:0"`
	MarkPrice          string     `json:"mark_price" gorm:"type:decimal(36,18);not null;default:0"`
	LiquidationPrice   string     `json:"liquidation_price" gorm:"type:decimal(36,18);not null;default:0"`
	UnrealizedPnL      string     `json:"unrealized_pnl" gorm:"column:unrealized_pnl;type:decimal(36,18);not null;default:0"`
	RealizedPnL        string     `json:"realized_pnl" gorm:"column:realized_pnl;type:decimal(36,18);not null;default:0"`
	Fee                string     `json:"fee" gorm:"type:decimal(36,18);not null;default:0"`
	Leverage           int        `json:"leverage" gorm:"not null;default:1"`
	EntryOrderID       string     `json:"entry_order_id" gorm:"size:128"`
	ExitOrderID        string     `json:"exit_order_id" gorm:"size:128"`
	CloseReason        string     `json:"close_reason" gorm:"size:64"`
	Source             string     `json:"source" gorm:"size:32;not null;default:provider"`
	OpenedAt           time.Time  `json:"opened_at" gorm:"not null;index"`
	ClosedAt           *time.Time `json:"closed_at" gorm:"index"`
	LastSyncedAt       time.Time  `json:"last_synced_at" gorm:"not null;index"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (*Position) TableName() string {
	return config.C.FormatTableName("trading_positions")
}

func (a *Position) BeforeSave(*gorm.DB) error {
	if strings.TrimSpace(a.Status) == "" && strings.TrimSpace(a.Side) == "" {
		// GORM invokes hooks with an empty model for scoped bulk updates.
		return nil
	}
	a.Status = strings.ToLower(strings.TrimSpace(a.Status))
	if a.Status != PositionStatusOpen && a.Status != PositionStatusClosed {
		return fmt.Errorf("invalid trading position status %q", a.Status)
	}
	a.Side = strings.ToLower(strings.TrimSpace(a.Side))
	if a.Side != "long" && a.Side != "short" {
		return fmt.Errorf("invalid trading position side %q", a.Side)
	}
	return nil
}

// Decision stores one AI decision cycle. Prompt and response fields are never exposed by list APIs.
type Decision struct {
	ID                  uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	LegacySource        *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_decision_legacy,priority:1"`
	LegacyID            *string   `json:"-" gorm:"size:64;uniqueIndex:uk_trading_decision_legacy,priority:2"`
	OwnerUserID         string    `json:"owner_user_id" gorm:"size:20;not null;index:idx_trading_decision_owner"`
	TraderID            string    `json:"trader_id" gorm:"size:64;not null;index:idx_trading_decision_trader_time,priority:1"`
	ExchangeID          string    `json:"exchange_id" gorm:"size:36;not null;index"`
	CycleNumber         int       `json:"cycle_number" gorm:"not null"`
	DecidedAt           time.Time `json:"decided_at" gorm:"not null;index:idx_trading_decision_trader_time,priority:2,sort:desc;index"`
	Success             bool      `json:"success" gorm:"not null;default:false;index"`
	ErrorMessage        string    `json:"error_message" gorm:"size:2000"`
	AIRequestDurationMs int64     `json:"ai_request_duration_ms" gorm:"not null;default:0"`
	SystemPrompt        string    `json:"-" gorm:"type:longtext"`
	InputPrompt         string    `json:"-" gorm:"type:longtext"`
	CoTTrace            string    `json:"-" gorm:"type:longtext"`
	DecisionJSON        string    `json:"-" gorm:"type:json;not null"`
	RawResponse         string    `json:"-" gorm:"type:longtext"`
	CandidateCoinsJSON  string    `json:"-" gorm:"type:json;not null"`
	ExecutionLogJSON    string    `json:"-" gorm:"type:json;not null"`
	ActionsJSON         string    `json:"-" gorm:"type:json;not null"`
	CreatedAt           time.Time `json:"created_at"`
}

func (*Decision) TableName() string {
	return config.C.FormatTableName("trading_decisions")
}

func (a *Decision) BeforeSave(*gorm.DB) error {
	for name, value := range map[string]string{
		"decision_json": a.DecisionJSON, "candidate_coins_json": a.CandidateCoinsJSON,
		"execution_log_json": a.ExecutionLogJSON, "actions_json": a.ActionsJSON,
	} {
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("%s must contain valid JSON", name)
		}
	}
	return nil
}

// LegacyImportRun is the auditable summary of one NOFX legacy import.
type LegacyImportRun struct {
	ID              string     `json:"id" gorm:"size:36;primaryKey"`
	SourceName      string     `json:"source_name" gorm:"size:64;not null;index"`
	OwnerUserID     string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	DryRun          bool       `json:"dry_run" gorm:"not null;default:true"`
	Status          string     `json:"status" gorm:"size:16;not null;index"`
	DomainStatsJSON string     `json:"domain_stats" gorm:"type:json;not null"`
	ErrorsJSON      string     `json:"errors" gorm:"type:json;not null"`
	StartedAt       time.Time  `json:"started_at" gorm:"not null;index"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (*LegacyImportRun) TableName() string {
	return config.C.FormatTableName("trading_legacy_import_runs")
}

// RuntimeLease prevents the same trader from running in multiple application instances.
type RuntimeLease struct {
	TraderID        string    `json:"trader_id" gorm:"size:64;primaryKey"`
	OwnerInstanceID string    `json:"owner_instance_id" gorm:"size:64;not null;index"`
	LeaseUntil      time.Time `json:"lease_until" gorm:"not null;index"`
	HeartbeatAt     time.Time `json:"heartbeat_at" gorm:"not null"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (*RuntimeLease) TableName() string {
	return config.C.FormatTableName("trading_runtime_leases")
}
