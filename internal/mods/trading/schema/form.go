package schema

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

type ExchangeForm struct {
	OwnerUserID      string `json:"owner_user_id"`
	ExchangeType     string `json:"exchange_type" binding:"required,oneof=binance bybit okx"`
	AccountName      string `json:"account_name" binding:"required,max=128"`
	Name             string `json:"name" binding:"required,max=128"`
	AccountType      string `json:"account_type" binding:"required,oneof=futures spot unified"`
	Enabled          bool   `json:"enabled"`
	Testnet          bool   `json:"testnet"`
	APIKey           string `json:"api_key"`
	SecretKey        string `json:"secret_key"`
	Passphrase       string `json:"passphrase"`
	WalletAddress    string `json:"wallet_address" binding:"max=128"`
	AccountUser      string `json:"account_user" binding:"max=128"`
	SignerAddress    string `json:"signer_address" binding:"max=128"`
	APIKeyIndex      int    `json:"api_key_index" binding:"min=0"`
	UnifiedAccount   bool   `json:"unified_account"`
	BuilderApproved  bool   `json:"builder_approved"`
	PrivateKey       string `json:"private_key"`
	APIKeyPrivateKey string `json:"api_key_private_key"`
}

type HistoryBackfillForm struct {
	StartAt time.Time  `json:"start_at" binding:"required"`
	EndAt   *time.Time `json:"end_at"`
	Symbols []string   `json:"symbols" binding:"max=20"`
}

type HistorySymbolsForm struct {
	Symbols []string `json:"symbols" binding:"max=50"`
}

func (a *HistoryBackfillForm) Validate() error {
	now := time.Now().UTC()
	if a.EndAt == nil {
		a.EndAt = &now
	}
	a.StartAt = a.StartAt.UTC()
	endAt := a.EndAt.UTC()
	a.EndAt = &endAt
	if a.StartAt.IsZero() || !a.StartAt.Before(endAt) {
		return errors.BadRequest("trading_history_backfill_range_invalid", "History backfill start time must be before end time")
	}
	if endAt.After(now.Add(5 * time.Minute)) {
		return errors.BadRequest("trading_history_backfill_future_unsupported", "History backfill end time cannot be in the future")
	}
	if endAt.Sub(a.StartAt) > 90*24*time.Hour {
		return errors.BadRequest("trading_history_backfill_range_too_large", "One history backfill request cannot exceed 90 days")
	}
	symbols := normalizeHistorySymbols(a.Symbols)
	if len(symbols) > 20 {
		return errors.BadRequest("trading_history_backfill_symbols_too_many", "One history backfill request supports at most 20 symbols")
	}
	a.Symbols = symbols
	return nil
}

func (a *HistorySymbolsForm) Validate() error {
	a.Symbols = normalizeHistorySymbols(a.Symbols)
	if len(a.Symbols) > 50 {
		return errors.BadRequest("trading_history_symbols_too_many", "One exchange supports at most 50 registered history symbols")
	}
	return nil
}

func normalizeHistorySymbols(values []string) []string {
	seen := make(map[string]struct{})
	symbols := make([]string, 0, len(values))
	for _, raw := range values {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
		}) {
			symbol := strings.ToUpper(strings.TrimSpace(part))
			if symbol == "" {
				continue
			}
			if _, ok := seen[symbol]; ok {
				continue
			}
			seen[symbol] = struct{}{}
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

func (a *ExchangeForm) Normalize() {
	a.ExchangeType = strings.ToLower(strings.TrimSpace(a.ExchangeType))
	a.AccountName = strings.TrimSpace(a.AccountName)
	a.Name = strings.TrimSpace(a.Name)
	a.AccountType = strings.ToLower(strings.TrimSpace(a.AccountType))
	a.WalletAddress = strings.TrimSpace(a.WalletAddress)
	a.AccountUser = strings.TrimSpace(a.AccountUser)
	a.SignerAddress = strings.TrimSpace(a.SignerAddress)
}

func (a *ExchangeForm) Validate() error {
	a.Normalize()
	a.UnifiedAccount = a.AccountType == "unified"
	if a.ExchangeType == "binance" && a.AccountType == "unified" && a.Testnet {
		return errors.BadRequest(
			"trading_binance_unified_testnet_unsupported",
			"Binance Portfolio Margin does not support testnet",
		)
	}
	return nil
}

type AIModelForm struct {
	OwnerUserID           string `json:"owner_user_id"`
	Name                  string `json:"name" binding:"required,max=128"`
	Provider              string `json:"provider" binding:"required,max=64"`
	Enabled               bool   `json:"enabled"`
	APIKey                string `json:"api_key"`
	CustomAPIURL          string `json:"custom_api_url" binding:"max=512"`
	CustomModelName       string `json:"custom_model_name" binding:"max=128"`
	InputPricePerMTokens  string `json:"input_price_per_m_tokens"`
	OutputPricePerMTokens string `json:"output_price_per_m_tokens"`
}

func (a *AIModelForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.Provider = strings.ToLower(strings.TrimSpace(a.Provider))
	a.CustomAPIURL = strings.TrimSpace(a.CustomAPIURL)
	a.CustomModelName = strings.TrimSpace(a.CustomModelName)
	var err error
	if a.InputPricePerMTokens, err = normalizeNonNegativeDecimal(a.InputPricePerMTokens); err != nil {
		return errors.BadRequest("trading_model_input_price_invalid", "Input token price must be a non-negative decimal")
	}
	if a.OutputPricePerMTokens, err = normalizeNonNegativeDecimal(a.OutputPricePerMTokens); err != nil {
		return errors.BadRequest("trading_model_output_price_invalid", "Output token price must be a non-negative decimal")
	}
	return nil
}

func normalizeNonNegativeDecimal(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0", nil
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 {
		return "", fmt.Errorf("invalid non-negative decimal")
	}
	return strings.TrimRight(strings.TrimRight(parsed.FloatString(8), "0"), "."), nil
}

type StrategyForm struct {
	OwnerUserID   string          `json:"owner_user_id"`
	Name          string          `json:"name" binding:"required,max=128"`
	Description   string          `json:"description" binding:"max=1000"`
	StrategyType  string          `json:"strategy_type" binding:"required,max=32"`
	IsActive      bool            `json:"is_active"`
	IsDefault     bool            `json:"is_default"`
	IsPublic      bool            `json:"is_public"`
	ConfigVisible bool            `json:"config_visible"`
	Config        json.RawMessage `json:"config" binding:"required"`
}

func (a *StrategyForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.Description = strings.TrimSpace(a.Description)
	a.StrategyType = strings.ToLower(strings.TrimSpace(a.StrategyType))
	if len(a.Config) == 0 || !json.Valid(a.Config) {
		return errors.BadRequest("trading_strategy_config_invalid", "Strategy config must be valid JSON")
	}
	var object map[string]interface{}
	if err := json.Unmarshal(a.Config, &object); err != nil || object == nil {
		return errors.BadRequest("trading_strategy_config_object_required", "Strategy config must be a JSON object")
	}
	if _, isStudioConfig := object["ai_config"]; isStudioConfig {
		config, err := DecodeStrategyStudioConfig(a.Config)
		if err != nil {
			return errors.BadRequest("trading_strategy_studio_config_invalid", "Invalid strategy studio config: %s", err.Error())
		}
		if config.StrategyType != a.StrategyType {
			return errors.BadRequest("trading_strategy_type_mismatch", "Strategy type must match the versioned strategy config")
		}
		normalized, err := json.Marshal(config)
		if err != nil {
			return errors.WithStack(err)
		}
		a.Config = normalized
	} else if a.StrategyType == SpotStrategyType {
		var config SpotCoreSatelliteConfig
		if err := json.Unmarshal(a.Config, &config); err != nil {
			return errors.BadRequest("trading_spot_strategy_config_invalid", "Invalid spot core-satellite config")
		}
		config.Normalize()
		if err := config.Validate(); err != nil {
			return errors.BadRequest("trading_spot_strategy_config_invalid", "Invalid spot core-satellite config: %s", err.Error())
		}
		normalized, err := json.Marshal(config)
		if err != nil {
			return errors.WithStack(err)
		}
		a.Config = normalized
	}
	return nil
}

type TraderForm struct {
	OwnerUserID              string   `json:"owner_user_id"`
	Name                     string   `json:"name" binding:"required,max=128"`
	AIModelID                string   `json:"ai_model_id" binding:"required,max=64"`
	ExchangeID               string   `json:"exchange_id" binding:"required,max=36"`
	StrategyID               *string  `json:"strategy_id"`
	InitialBalance           string   `json:"initial_balance" binding:"required"`
	ScanIntervalMinutes      int      `json:"scan_interval_minutes" binding:"min=1,max=1440"`
	IsCrossMargin            bool     `json:"is_cross_margin"`
	IsSimulation             bool     `json:"is_simulation"`
	AutoApplyStrategyUpdates bool     `json:"auto_apply_strategy_updates"`
	ShowInCompetition        bool     `json:"show_in_competition"`
	ExecutionMode            string   `json:"execution_mode"`
	NotificationChannelIDs   []string `json:"notification_channel_ids" binding:"max=20,dive,max=36"`
}

type TraderStartForm struct {
	ConfirmExternalAI         bool   `json:"confirm_external_ai"`
	ConfirmAutomatedExecution bool   `json:"confirm_automated_execution"`
	AutomationPhrase          string `json:"automation_phrase"`
}

func (a *TraderForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.AIModelID = strings.TrimSpace(a.AIModelID)
	a.ExchangeID = strings.TrimSpace(a.ExchangeID)
	a.InitialBalance = strings.TrimSpace(a.InitialBalance)
	a.ExecutionMode = strings.ToLower(strings.TrimSpace(a.ExecutionMode))
	if a.ExecutionMode == "" {
		a.ExecutionMode = TraderExecutionNotifyOnly
	}
	switch a.ExecutionMode {
	case TraderExecutionNotifyOnly, TraderExecutionApprovalRequired, TraderExecutionAuto:
	default:
		return errors.BadRequest("trading_execution_mode_invalid", "Unsupported trader execution mode")
	}
	a.NotificationChannelIDs = normalizeStringIDs(a.NotificationChannelIDs)
	if a.StrategyID != nil {
		value := strings.TrimSpace(*a.StrategyID)
		if value == "" {
			a.StrategyID = nil
		} else {
			a.StrategyID = &value
		}
	}
	value, ok := new(big.Rat).SetString(a.InitialBalance)
	if !ok || value.Sign() < 0 {
		return errors.BadRequest("trading_initial_balance_invalid", "Initial balance must be a non-negative decimal")
	}
	return nil
}

type NotificationChannelForm struct {
	OwnerUserID   string `json:"owner_user_id"`
	Name          string `json:"name" binding:"required,max=128"`
	ChannelType   string `json:"channel_type" binding:"required,oneof=telegram lark"`
	Enabled       bool   `json:"enabled"`
	BotToken      string `json:"bot_token"`
	ChatID        string `json:"chat_id" binding:"max=128"`
	WebhookURL    string `json:"webhook_url"`
	SigningSecret string `json:"signing_secret"`
}

func (a *NotificationChannelForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.ChannelType = strings.ToLower(strings.TrimSpace(a.ChannelType))
	a.BotToken = strings.TrimSpace(a.BotToken)
	a.ChatID = strings.TrimSpace(a.ChatID)
	a.WebhookURL = strings.TrimSpace(a.WebhookURL)
	a.SigningSecret = strings.TrimSpace(a.SigningSecret)
	return nil
}

func normalizeStringIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
