package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	tradingruntime "github.com/LyricTian/gin-admin/v10/internal/mods/trading/runtime"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type Catalog struct {
	CatalogDAL      *dal.Catalog
	Credentials     *credential.Service
	ProviderFactory *provider.Factory
	LLM             *llm.Service
	Runtime         *tradingruntime.Registry
}

func (a *Catalog) QueryExchanges(ctx context.Context, params schema.ExchangeQueryParam) (*schema.ExchangeViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.CatalogDAL.QueryExchanges(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.ExchangeView, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, buildExchangeView(item))
	}
	return &schema.ExchangeViewResult{Data: data, PageResult: result.PageResult}, nil
}

func (a *Catalog) QueryAIModels(ctx context.Context, params schema.AIModelQueryParam) (*schema.AIModelViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.CatalogDAL.QueryAIModels(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.AIModelView, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, buildAIModelView(item))
	}
	return &schema.AIModelViewResult{Data: data, PageResult: result.PageResult}, nil
}

func (a *Catalog) QueryStrategies(ctx context.Context, params schema.StrategyQueryParam) (*schema.StrategyViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.CatalogDAL.QueryStrategies(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.StrategyView, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, buildStrategyView(item))
	}
	return &schema.StrategyViewResult{Data: data, PageResult: result.PageResult}, nil
}

func (a *Catalog) QueryTraders(ctx context.Context, params schema.TraderQueryParam) (*schema.TraderViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.CatalogDAL.QueryTraders(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.TraderView, 0, len(result.Data))
	for _, item := range result.Data {
		channelIDs, err := a.CatalogDAL.ListTraderNotificationChannelIDs(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		data = append(data, buildTraderView(item, channelIDs))
	}
	return &schema.TraderViewResult{Data: data, PageResult: result.PageResult}, nil
}

func (a *Catalog) GetExchange(ctx context.Context, id string) (*schema.ExchangeView, error) {
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildExchangeView(item), nil
}

func (a *Catalog) CreateExchange(ctx context.Context, form *schema.ExchangeForm) (*schema.ExchangeView, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if exists, err := a.CatalogDAL.ExchangeAccountExists(ctx, owner, form.ExchangeType, form.AccountName, ""); err != nil {
		return nil, err
	} else if exists {
		return nil, errors.Conflict("trading_exchange_account_exists", "Exchange account already exists")
	}
	item := &schema.Exchange{
		ID: util.MustNewUUID(), OwnerUserID: owner, ExchangeType: form.ExchangeType,
		AccountName: form.AccountName, Name: form.Name, AccountType: form.AccountType,
		Enabled: form.Enabled, Testnet: form.Testnet, WalletAddress: form.WalletAddress,
		AccountUser: form.AccountUser, SignerAddress: form.SignerAddress, APIKeyIndex: form.APIKeyIndex,
		UnifiedAccount: form.UnifiedAccount, BuilderApproved: form.BuilderApproved,
		CredentialVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := a.applyExchangeCredentials(item, form); err != nil {
		return nil, err
	}
	if err := validateEnabledExchange(item); err != nil {
		return nil, err
	}
	if err := a.CatalogDAL.CreateExchange(ctx, item); err != nil {
		return nil, err
	}
	return buildExchangeView(item), nil
}

func (a *Catalog) UpdateExchange(ctx context.Context, id string, form *schema.ExchangeForm) error {
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return err
	}
	if exists, err := a.CatalogDAL.ExchangeAccountExists(ctx, item.OwnerUserID, form.ExchangeType, form.AccountName, id); err != nil {
		return err
	} else if exists {
		return errors.Conflict("trading_exchange_account_exists", "Exchange account already exists")
	}
	item.ExchangeType, item.AccountName, item.Name, item.AccountType = form.ExchangeType, form.AccountName, form.Name, form.AccountType
	item.Enabled, item.Testnet = form.Enabled, form.Testnet
	item.WalletAddress, item.AccountUser, item.SignerAddress = form.WalletAddress, form.AccountUser, form.SignerAddress
	item.APIKeyIndex, item.UnifiedAccount, item.BuilderApproved = form.APIKeyIndex, form.UnifiedAccount, form.BuilderApproved
	item.UpdatedAt = time.Now()
	if err := a.applyExchangeCredentials(item, form); err != nil {
		return err
	}
	if err := validateEnabledExchange(item); err != nil {
		return err
	}
	return a.CatalogDAL.UpdateExchange(ctx, item)
}

func (a *Catalog) DeleteExchange(ctx context.Context, id string) error {
	if _, err := a.getExchange(ctx, id); err != nil {
		return err
	}
	return a.deleteUnreferenced(ctx, "exchange_id", id, "Exchange is used by a trader", a.CatalogDAL.DeleteExchange)
}

func (a *Catalog) TestExchange(ctx context.Context, id string) (*schema.ExchangeConnectionTestView, error) {
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return nil, err
	}
	client, err := a.ProviderFactory.Build(item)
	if err != nil {
		return nil, errors.BadRequest("trading_exchange_provider_invalid", "Exchange provider is unavailable: %v", err)
	}
	return testExchangeConnection(ctx, item, client)
}

func (a *Catalog) SyncExchange(ctx context.Context, id string) (*schema.ExchangeDataSyncView, error) {
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return nil, err
	}
	if !item.Enabled {
		return nil, errors.BadRequest("trading_exchange_disabled", "Enable the exchange account before synchronization")
	}
	client, err := a.ProviderFactory.Build(item)
	if err != nil {
		return nil, errors.BadRequest("trading_exchange_provider_invalid", "Exchange provider is unavailable: %v", err)
	}
	syncCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	result, err := a.Runtime.SyncExchangeOnce(syncCtx, item, client)
	if err != nil {
		return nil, errors.BadRequest("trading_exchange_sync_failed", "Exchange account synchronization failed: %v", err)
	}
	return result, nil
}

func (a *Catalog) BackfillExchangeHistory(ctx context.Context, id string,
	form *schema.HistoryBackfillForm) (*schema.HistoryBackfillView, error) {
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return nil, err
	}
	if !item.Enabled {
		return nil, errors.BadRequest("trading_exchange_disabled", "Enable the exchange account before history backfill")
	}
	if item.ExchangeType == "binance" && len(form.Symbols) == 0 {
		return nil, errors.BadRequest("trading_binance_history_symbols_required",
			"Binance history endpoints require at least one explicit symbol, for example BTCUSDT")
	}
	client, err := a.ProviderFactory.Build(item)
	if err != nil {
		return nil, errors.BadRequest("trading_exchange_provider_invalid", "Exchange provider is unavailable: %v", err)
	}
	historical, ok := client.(provider.HistoricalReadOnly)
	if !ok {
		return nil, errors.BadRequest("trading_exchange_history_unsupported",
			"The configured exchange provider does not support historical order backfill")
	}
	backfillCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result, err := a.Runtime.BackfillExchangeHistory(backfillCtx, item, historical, provider.HistoryRequest{
		Since: form.StartAt, Until: *form.EndAt, Symbols: form.Symbols,
	})
	if err != nil {
		return nil, errors.BadRequest("trading_exchange_history_backfill_failed", "Exchange history backfill failed: %v", err)
	}
	if len(form.Symbols) > 0 {
		if err := a.CatalogDAL.AddHistorySymbols(ctx, item.ID, item.OwnerUserID, "backfill", form.Symbols); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *Catalog) ReplaceExchangeHistorySymbols(ctx context.Context, id string,
	form *schema.HistorySymbolsForm) ([]string, error) {
	if err := form.Validate(); err != nil {
		return nil, err
	}
	item, err := a.getExchange(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.ExchangeType != "binance" {
		return nil, errors.BadRequest("trading_history_symbols_exchange_unsupported",
			"Explicit history symbols are only required for Binance accounts")
	}
	if err := a.CatalogDAL.ReplaceHistorySymbols(ctx, item.ID, item.OwnerUserID, form.Symbols); err != nil {
		return nil, err
	}
	return a.CatalogDAL.ListHistorySymbols(ctx, item.ID, item.OwnerUserID)
}

func (a *Catalog) QueryHistorySyncStatuses(ctx context.Context, ownerUserID string) ([]*schema.HistorySyncStatusView, error) {
	owner, err := resolveOwnerScope(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	items, err := a.CatalogDAL.QueryHistorySyncStatuses(ctx, owner)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, item := range items {
		item.GapDetails = []string{}
		if item.OrphanFillCount > 0 {
			item.GapDetails = append(item.GapDetails, fmt.Sprintf("%d 条成交缺少对应订单", item.OrphanFillCount))
		}
		if item.UnmatchedOrderCount > 0 {
			item.GapDetails = append(item.GapDetails, fmt.Sprintf("%d 条已成交订单缺少成交明细", item.UnmatchedOrderCount))
		}
		switch {
		case !item.Enabled:
			item.CoverageStatus, item.Warning = "disabled", "账户已停用，不会执行自动同步"
		case item.Status == "error":
			item.CoverageStatus, item.Warning = "error", "最近一次历史同步失败，请查看错误并重新同步"
		case item.LastRunAt == nil || item.SyncedUntil == nil:
			item.CoverageStatus, item.Warning = "pending", "尚未完成首次历史同步"
		case now.Sub(item.SyncedUntil.UTC()) > 15*time.Minute:
			item.CoverageStatus, item.Warning = "stale", "历史同步进度已超过 15 分钟未更新"
		case item.ExchangeType == "binance" && len(item.RegisteredSymbols) == 0:
			item.CoverageStatus, item.Warning = "incomplete", "未登记历史交易对，已清仓币种可能无法自动发现"
		case item.GapCount > 0:
			item.CoverageStatus, item.Warning = "incomplete", "检测到订单与成交关联缺口："+strings.Join(item.GapDetails, "；")
		default:
			item.CoverageStatus = "healthy"
		}
	}
	return items, nil
}

func testExchangeConnection(ctx context.Context, item *schema.Exchange, client provider.ReadOnly) (*schema.ExchangeConnectionTestView, error) {
	testCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	startedAt := time.Now()
	healthStartedAt := time.Now()
	if err := client.Health(testCtx); err != nil {
		return nil, errors.BadRequest(
			"trading_exchange_network_failed",
			"Unable to reach the configured exchange environment: %v",
			err,
		)
	}
	networkLatency := time.Since(healthStartedAt).Milliseconds()

	authStartedAt := time.Now()
	var authenticationErr error
	if validator, ok := client.(provider.CredentialValidator); ok {
		authenticationErr = validator.ValidateCredentials(testCtx)
	} else {
		_, _, authenticationErr = client.Snapshot(testCtx)
	}
	if authenticationErr != nil {
		return nil, errors.BadRequest(
			"trading_exchange_authentication_failed",
			"Exchange network is reachable, but account authentication failed: %v",
			authenticationErr,
		)
	}
	authenticationLatency := time.Since(authStartedAt).Milliseconds()
	environment := "production"
	if item.Testnet {
		environment = "testnet"
	}

	return &schema.ExchangeConnectionTestView{
		ExchangeID:              item.ID,
		ExchangeType:            item.ExchangeType,
		AccountType:             item.AccountType,
		Testnet:                 item.Testnet,
		Environment:             environment,
		NetworkOK:               true,
		AuthenticationOK:        true,
		NetworkLatencyMS:        networkLatency,
		AuthenticationLatencyMS: authenticationLatency,
		LatencyMS:               time.Since(startedAt).Milliseconds(),
	}, nil
}

func (a *Catalog) GetAIModel(ctx context.Context, id string) (*schema.AIModelView, error) {
	item, err := a.getAIModel(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildAIModelView(item), nil
}

func (a *Catalog) CreateAIModel(ctx context.Context, form *schema.AIModelForm) (*schema.AIModelView, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	item := &schema.AIModel{ID: util.NewXID(), OwnerUserID: owner, Name: form.Name, Provider: form.Provider,
		Enabled: form.Enabled, CustomAPIURL: form.CustomAPIURL, CustomModelName: form.CustomModelName,
		InputPricePerMTokens: form.InputPricePerMTokens, OutputPricePerMTokens: form.OutputPricePerMTokens,
		CredentialVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if form.APIKey != "" {
		item.APIKeyCiphertext, err = a.Credentials.EncryptField(form.APIKey, owner, item.ID, "api_key")
		if err != nil {
			return nil, errors.BadRequest("trading_credential_encryption_failed", "Unable to encrypt model credential: %v", err)
		}
	}
	if err := a.CatalogDAL.CreateAIModel(ctx, item); err != nil {
		return nil, err
	}
	return buildAIModelView(item), nil
}

func (a *Catalog) UpdateAIModel(ctx context.Context, id string, form *schema.AIModelForm) error {
	item, err := a.getAIModel(ctx, id)
	if err != nil {
		return err
	}
	item.Name, item.Provider, item.Enabled = form.Name, form.Provider, form.Enabled
	item.CustomAPIURL, item.CustomModelName, item.UpdatedAt = form.CustomAPIURL, form.CustomModelName, time.Now()
	item.InputPricePerMTokens, item.OutputPricePerMTokens = form.InputPricePerMTokens, form.OutputPricePerMTokens
	if form.APIKey != "" {
		item.APIKeyCiphertext, err = a.Credentials.EncryptField(form.APIKey, item.OwnerUserID, item.ID, "api_key")
		if err != nil {
			return errors.BadRequest("trading_credential_encryption_failed", "Unable to encrypt model credential: %v", err)
		}
	}
	return a.CatalogDAL.UpdateAIModel(ctx, item)
}

func (a *Catalog) DeleteAIModel(ctx context.Context, id string) error {
	if _, err := a.getAIModel(ctx, id); err != nil {
		return err
	}
	return a.deleteUnreferenced(ctx, "ai_model_id", id, "AI model is used by a trader", a.CatalogDAL.DeleteAIModel)
}

func (a *Catalog) TestAIModel(ctx context.Context, id string) (*schema.AIModelConnectionTestView, error) {
	item, err := a.getAIModel(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.APIKeyCiphertext == "" {
		return nil, errors.BadRequest("trading_model_api_key_missing", "AI model API key is missing")
	}
	result, err := a.LLM.Invoke(ctx, item, llm.InvocationRequest{
		Purpose: "connection_test", SystemPrompt: "You are a connection test endpoint.",
		UserPrompt: "Reply with OK only.", MaxTokens: 16,
	})
	if err != nil {
		return nil, errors.BadRequest("trading_model_connection_failed", "AI model connection test failed: %v", err)
	}
	return &schema.AIModelConnectionTestView{
		AIModelID: item.ID, Provider: result.Provider, ModelName: result.ModelName,
		RequestID: result.RequestID, Response: result.Content, InputTokens: result.InputTokens,
		OutputTokens: result.OutputTokens, TotalTokens: result.TotalTokens,
		LatencyMS: result.DurationMS, Success: true,
	}, nil
}

func (a *Catalog) GetStrategy(ctx context.Context, id string) (*schema.StrategyDetailView, error) {
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return nil, err
	}
	var config interface{}
	if err := json.Unmarshal([]byte(item.ConfigJSON), &config); err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.StrategyDetailView{StrategyView: *buildStrategyView(item), Config: config}, nil
}

func (a *Catalog) ListStrategyVersions(ctx context.Context, id string) ([]*schema.StrategyVersionView, error) {
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return nil, err
	}
	versions, err := a.CatalogDAL.ListStrategyVersions(ctx, item.ID, item.OwnerUserID)
	if err != nil {
		return nil, err
	}
	result := make([]*schema.StrategyVersionView, 0, len(versions))
	for _, version := range versions {
		var config interface{}
		if err := json.Unmarshal([]byte(version.ConfigJSON), &config); err != nil {
			return nil, errors.WithStack(err)
		}
		result = append(result, &schema.StrategyVersionView{
			ID: version.ID, StrategyID: version.StrategyID, Version: version.Version,
			Name: version.Name, Description: version.Description, StrategyType: version.StrategyType,
			Config: config, RollbackFrom: version.RollbackFrom, PublishedAt: version.PublishedAt,
			CreatedAt: version.CreatedAt,
		})
	}
	return result, nil
}

func (a *Catalog) CreateStrategy(ctx context.Context, form *schema.StrategyForm) (*schema.StrategyDetailView, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	item := &schema.Strategy{ID: util.NewXID(), OwnerUserID: owner, Name: form.Name, Description: form.Description,
		StrategyType: form.StrategyType, IsActive: form.IsActive, IsDefault: form.IsDefault,
		IsPublic: form.IsPublic, ConfigVisible: form.ConfigVisible, ConfigJSON: string(form.Config),
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := a.CatalogDAL.CreateStrategy(ctx, item); err != nil {
		return nil, err
	}
	return a.GetStrategy(ctx, item.ID)
}

func (a *Catalog) UpdateStrategy(ctx context.Context, id string, form *schema.StrategyForm) error {
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return err
	}
	createVersion := item.Name != form.Name || item.Description != form.Description ||
		item.StrategyType != form.StrategyType || item.ConfigJSON != string(form.Config)
	item.Name, item.Description, item.StrategyType = form.Name, form.Description, form.StrategyType
	item.IsActive, item.IsDefault, item.IsPublic, item.ConfigVisible = form.IsActive, form.IsDefault, form.IsPublic, form.ConfigVisible
	item.ConfigJSON, item.UpdatedAt = string(form.Config), time.Now()
	return a.CatalogDAL.UpdateStrategy(ctx, item, createVersion)
}

func (a *Catalog) PublishStrategy(ctx context.Context, id string) (*schema.StrategyDetailView, error) {
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.CatalogDAL.PublishStrategy(ctx, item.ID, item.OwnerUserID, time.Now()); err != nil {
		return nil, err
	}
	return a.GetStrategy(ctx, item.ID)
}

func (a *Catalog) RollbackStrategy(ctx context.Context, id string, sourceVersion int) (*schema.StrategyDetailView, error) {
	if sourceVersion <= 0 {
		return nil, errors.BadRequest("trading_strategy_version_invalid", "Strategy version must be greater than zero")
	}
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return nil, err
	}
	versions, err := a.CatalogDAL.ListStrategyVersions(ctx, item.ID, item.OwnerUserID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, version := range versions {
		if version.Version == sourceVersion {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.NotFound("trading_strategy_version_not_found", "Strategy version not found")
	}
	if err := a.CatalogDAL.RollbackStrategy(ctx, item.ID, item.OwnerUserID, sourceVersion, time.Now()); err != nil {
		return nil, err
	}
	return a.GetStrategy(ctx, item.ID)
}

func (a *Catalog) ActivateStrategy(ctx context.Context, id string) error {
	item, err := a.getStrategy(ctx, id)
	if err != nil {
		return err
	}
	return a.CatalogDAL.ActivateStrategy(ctx, item.OwnerUserID, item.ID, time.Now())
}

func (a *Catalog) DuplicateStrategy(ctx context.Context, id string, form *schema.StrategyDuplicateForm) (*schema.StrategyDetailView, error) {
	source, err := a.getStrategy(ctx, id)
	if err != nil {
		return nil, err
	}
	item := &schema.Strategy{
		ID: util.NewXID(), OwnerUserID: source.OwnerUserID, Name: form.Name,
		Description: "基于策略 [" + source.Name + "] 创建", StrategyType: source.StrategyType,
		IsActive: false, IsDefault: false, IsPublic: false, ConfigVisible: source.ConfigVisible,
		ConfigJSON: source.ConfigJSON, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := a.CatalogDAL.CreateStrategy(ctx, item); err != nil {
		return nil, err
	}
	return a.GetStrategy(ctx, item.ID)
}

func (a *Catalog) DeleteStrategy(ctx context.Context, id string) error {
	if _, err := a.getStrategy(ctx, id); err != nil {
		return err
	}
	return a.deleteUnreferenced(ctx, "strategy_id", id, "Strategy is used by a trader", a.CatalogDAL.DeleteStrategy)
}

func (a *Catalog) GetTrader(ctx context.Context, id string) (*schema.TraderView, error) {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return nil, err
	}
	channelIDs, err := a.CatalogDAL.ListTraderNotificationChannelIDs(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return buildTraderView(item, channelIDs), nil
}

func (a *Catalog) CreateTrader(ctx context.Context, form *schema.TraderForm) (*schema.TraderView, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := a.validateTraderReferences(ctx, owner, form, false); err != nil {
		return nil, err
	}
	if err := a.validateTraderNotificationChannels(ctx, owner, form.NotificationChannelIDs); err != nil {
		return nil, err
	}
	item := &schema.Trader{ID: util.NewXID(), OwnerUserID: owner, Name: form.Name, AIModelID: form.AIModelID,
		ExchangeID: form.ExchangeID, StrategyID: form.StrategyID, StrategyVersion: 0, InitialBalance: form.InitialBalance,
		ScanIntervalMinutes: form.ScanIntervalMinutes, IsCrossMargin: form.IsCrossMargin,
		IsSimulation: form.IsSimulation, ShowInCompetition: false,
		AutoApplyStrategyUpdates: form.AutoApplyStrategyUpdates && form.IsSimulation,
		ExecutionMode:            form.ExecutionMode,
		DesiredState:             schema.TraderDesiredStateStopped, RuntimeState: schema.TraderRuntimeUnavailable,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := a.CatalogDAL.CreateTraderWithBindings(ctx, item, form.NotificationChannelIDs); err != nil {
		return nil, err
	}
	return buildTraderView(item, form.NotificationChannelIDs), nil
}

func (a *Catalog) UpdateTrader(ctx context.Context, id string, form *schema.TraderForm) error {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return err
	}
	if a.Runtime.Status(item.ID) || item.RuntimeState == "running" || item.RuntimeState == "starting" {
		return errors.Conflict("trading_trader_running", "Stop trader before editing")
	}
	if _, _, _, err := a.validateTraderReferences(ctx, item.OwnerUserID, form, false); err != nil {
		return err
	}
	if err := a.validateTraderNotificationChannels(ctx, item.OwnerUserID, form.NotificationChannelIDs); err != nil {
		return err
	}
	item.Name, item.AIModelID, item.ExchangeID, item.StrategyID = form.Name, form.AIModelID, form.ExchangeID, form.StrategyID
	item.StrategyVersion = 0
	item.InitialBalance, item.ScanIntervalMinutes = form.InitialBalance, form.ScanIntervalMinutes
	item.IsCrossMargin, item.IsSimulation = form.IsCrossMargin, form.IsSimulation
	item.AutoApplyStrategyUpdates = form.AutoApplyStrategyUpdates && form.IsSimulation
	item.ExecutionMode = form.ExecutionMode
	item.AIConsentModelID, item.AIConsentAt = "", nil
	item.UpdatedAt = time.Now()
	return a.CatalogDAL.UpdateTraderWithBindings(ctx, item, form.NotificationChannelIDs)
}

func (a *Catalog) DeleteTrader(ctx context.Context, id string) error {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return err
	}
	if a.Runtime.Status(item.ID) || item.DesiredState == "running" || item.RuntimeState == "running" || item.RuntimeState == "starting" {
		return errors.Conflict("trading_trader_running", "Stop trader before deleting")
	}
	return a.CatalogDAL.DeleteTrader(ctx, id)
}

func (a *Catalog) StartTrader(ctx context.Context, id string, startForm *schema.TraderStartForm) error {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return err
	}
	form := &schema.TraderForm{AIModelID: item.AIModelID, ExchangeID: item.ExchangeID, StrategyID: item.StrategyID}
	exchange, model, strategy, err := a.validateTraderReferences(ctx, item.OwnerUserID, form, true)
	if err != nil {
		return err
	}
	isDeterministicStrategy := false
	if strategy != nil && strategy.PublishedVersion != nil {
		versions, err := a.CatalogDAL.ListStrategyVersions(ctx, strategy.ID, item.OwnerUserID)
		if err != nil {
			return err
		}
		for _, version := range versions {
			if version.Version != *strategy.PublishedVersion {
				continue
			}
			config, err := schema.DecodeStrategyStudioConfig(json.RawMessage(version.ConfigJSON))
			if err != nil {
				return errors.Conflict("trading_strategy_published_config_invalid", "Published strategy configuration is invalid")
			}
			if config.StrategyType == "grid" && !item.IsSimulation {
				return errors.Conflict("trading_grid_live_disabled", "Grid runtime is simulation-only; enable simulation before starting")
			}
			if config.StrategyType == schema.SpotStrategyType && !item.IsSimulation {
				return errors.Conflict("trading_spot_live_disabled", "Spot core-satellite runtime is simulation-only until live execution is explicitly enabled and audited")
			}
			isDeterministicStrategy = config.StrategyType == "grid" || config.StrategyType == schema.SpotStrategyType
			break
		}
	}
	if !isDeterministicStrategy && (startForm == nil || !startForm.ConfirmExternalAI) {
		return errors.Conflict("trading_external_ai_consent_required", "Confirm sending strategy, account, position, public order-book, K-line and configured indicator context to %s before starting", model.Provider)
	}
	if item.ExecutionMode == schema.TraderExecutionAuto && !item.IsSimulation {
		if !startForm.ConfirmAutomatedExecution || strings.TrimSpace(startForm.AutomationPhrase) != "START AUTOMATED TRADING" {
			return errors.Conflict("trading_auto_execution_consent_required",
				"Automated exchange execution requires explicit acknowledgement and the exact phrase START AUTOMATED TRADING")
		}
	}
	readOnlyProvider, err := a.ProviderFactory.Build(exchange)
	if err != nil {
		return errors.BadRequest("trading_exchange_provider_invalid", "Exchange provider is unavailable: %v", err)
	}
	preflightCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, _, err := readOnlyProvider.Snapshot(preflightCtx); err != nil {
		return errors.BadRequest("trading_trader_preflight_failed", "Trader start preflight failed: %v", err)
	}
	item.StrategyVersion = 0
	if strategy != nil && strategy.PublishedVersion != nil {
		item.StrategyVersion = *strategy.PublishedVersion
	}
	consentAt := time.Now().UTC()
	consentModelID := model.ID
	var aiConsentAt *time.Time = &consentAt
	if isDeterministicStrategy {
		consentModelID, aiConsentAt = "", nil
	}
	var autoConsentAt *time.Time
	if item.ExecutionMode == schema.TraderExecutionAuto && !item.IsSimulation {
		autoConsentAt = &consentAt
	}
	if err := a.CatalogDAL.UpdateTraderFields(ctx, item.ID, map[string]interface{}{
		"desired_state": "running", "strategy_version": item.StrategyVersion,
		"ai_consent_model_id": consentModelID, "ai_consent_at": aiConsentAt,
		"auto_execution_consent_at": autoConsentAt, "updated_at": consentAt,
	}); err != nil {
		return err
	}
	item.AIConsentModelID, item.AIConsentAt, item.AutoExecutionConsentAt = consentModelID, aiConsentAt, autoConsentAt
	if err := a.Runtime.Start(item, exchange); err != nil {
		_ = a.CatalogDAL.UpdateTraderFields(context.Background(), item.ID, map[string]interface{}{
			"desired_state": schema.TraderDesiredStateStopped, "ai_consent_model_id": "", "ai_consent_at": nil,
			"auto_execution_consent_at": nil,
			"updated_at":                time.Now().UTC(),
		})
		return errors.Conflict("trading_trader_start_failed", "Unable to start trader: %v", err)
	}
	return nil
}

func (a *Catalog) StopTrader(ctx context.Context, id string) error {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return err
	}
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := a.Runtime.Stop(stopCtx, item.ID); err != nil {
		return errors.Conflict("trading_trader_stop_failed", "Unable to stop trader: %v", err)
	}
	return a.CatalogDAL.UpdateTraderFields(ctx, item.ID, map[string]interface{}{
		"desired_state": schema.TraderDesiredStateStopped, "strategy_state": "stopped",
		"ai_consent_model_id": "", "ai_consent_at": nil, "auto_execution_consent_at": nil, "updated_at": time.Now(),
	})
}

func (a *Catalog) SyncTrader(ctx context.Context, id string) error {
	item, err := a.getTrader(ctx, id)
	if err != nil {
		return err
	}
	if a.Runtime.Status(item.ID) {
		return errors.Conflict("trading_trader_running", "Running trader already synchronizes automatically")
	}
	exchange, err := a.CatalogDAL.GetExchange(ctx, item.ExchangeID, item.OwnerUserID)
	if err != nil || exchange == nil {
		return errors.NotFound("trading_exchange_not_found", "Exchange not found")
	}
	readOnlyProvider, err := a.ProviderFactory.Build(exchange)
	if err != nil {
		return errors.BadRequest("trading_exchange_provider_invalid", "Exchange provider is unavailable: %v", err)
	}
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := a.Runtime.SyncOnce(syncCtx, item, exchange, readOnlyProvider); err != nil {
		return errors.BadRequest("trading_trader_sync_failed", "Trader synchronization failed: %v", err)
	}
	return nil
}

func (a *Catalog) QueryScheduledTasks(ctx context.Context, params schema.ScheduledTaskQueryParam) (*schema.ScheduledTaskQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.CatalogDAL.QueryScheduledTasks(ctx, params)
}

func (a *Catalog) QueryScheduledTaskRuns(ctx context.Context, params schema.ScheduledTaskRunQueryParam) (*schema.ScheduledTaskRunQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.CatalogDAL.QueryScheduledTaskRuns(ctx, params)
}

func (a *Catalog) QueryScheduledTaskAudits(ctx context.Context, params schema.ScheduledTaskAuditQueryParam) (*schema.ScheduledTaskAuditQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.CatalogDAL.QueryScheduledTaskAudits(ctx, params)
}

func (a *Catalog) CreateScheduledTask(ctx context.Context, form *schema.ScheduledTaskForm) (*schema.ScheduledTask, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	nextRun, err := schema.NextScheduledTaskRun(form.CronExpression, form.TimeZone, now)
	if err != nil {
		return nil, errors.BadRequest("trading_task_schedule_invalid", "Invalid scheduled task configuration: %v", err)
	}
	item := &schema.ScheduledTask{ID: util.MustNewUUID(), OwnerUserID: owner, Name: form.Name,
		Handler: form.Handler, CronExpression: form.CronExpression, TimeZone: form.TimeZone,
		Status: schema.TaskStatusActive, TimeoutSeconds: form.TimeoutSeconds, MaxRetries: form.MaxRetries,
		RetryIntervalSeconds: form.RetryIntervalSeconds, MisfirePolicy: form.MisfirePolicy,
		ConcurrencyPolicy: form.ConcurrencyPolicy, NextRunAt: &nextRun, CreatedAt: now, UpdatedAt: now}
	trans := util.Trans{DB: a.CatalogDAL.DB}
	if err := trans.Exec(ctx, func(txCtx context.Context) error {
		if exists, err := a.CatalogDAL.ScheduledTaskNameExists(txCtx, owner, form.Name, ""); err != nil {
			return err
		} else if exists {
			return errors.Conflict("trading_task_name_exists", "Scheduled task name already exists")
		}
		if err := a.CatalogDAL.CreateScheduledTask(txCtx, item); err != nil {
			return err
		}
		return a.createScheduledTaskAudit(txCtx, item, schema.TaskAuditCreate, "", scheduledTaskAuditJSON(item))
	}); err != nil {
		return nil, err
	}
	return item, nil
}

func (a *Catalog) UpdateScheduledTask(ctx context.Context, id string, form *schema.ScheduledTaskForm) error {
	trans := util.Trans{DB: a.CatalogDAL.DB}
	return trans.Exec(ctx, func(txCtx context.Context) error {
		item, err := a.getScheduledTask(util.NewRowLock(txCtx), id)
		if err != nil {
			return err
		}
		if exists, err := a.CatalogDAL.ScheduledTaskNameExists(txCtx, item.OwnerUserID, form.Name, id); err != nil {
			return err
		} else if exists {
			return errors.Conflict("trading_task_name_exists", "Scheduled task name already exists")
		}
		before := scheduledTaskAuditJSON(item)
		item.Name, item.Handler = form.Name, form.Handler
		item.CronExpression, item.TimeZone = form.CronExpression, form.TimeZone
		item.TimeoutSeconds, item.MaxRetries = form.TimeoutSeconds, form.MaxRetries
		item.RetryIntervalSeconds = form.RetryIntervalSeconds
		item.MisfirePolicy, item.ConcurrencyPolicy = form.MisfirePolicy, form.ConcurrencyPolicy
		item.UpdatedAt = time.Now().UTC()
		if item.Status == schema.TaskStatusActive {
			nextRun, err := schema.NextScheduledTaskRun(item.CronExpression, item.TimeZone, item.UpdatedAt)
			if err != nil {
				return errors.BadRequest("trading_task_schedule_invalid", "Invalid scheduled task configuration: %v", err)
			}
			item.NextRunAt = &nextRun
		}
		if err := a.CatalogDAL.UpdateScheduledTask(txCtx, item); err != nil {
			return err
		}
		return a.createScheduledTaskAudit(txCtx, item, schema.TaskAuditUpdate, before, scheduledTaskAuditJSON(item))
	})
}

func (a *Catalog) DeleteScheduledTask(ctx context.Context, id string) error {
	trans := util.Trans{DB: a.CatalogDAL.DB}
	return trans.Exec(ctx, func(txCtx context.Context) error {
		item, err := a.getScheduledTask(util.NewRowLock(txCtx), id)
		if err != nil {
			return err
		}
		if item.LeaseUntil != nil && item.LeaseUntil.After(time.Now().UTC()) {
			return errors.Conflict("trading_task_running", "A running scheduled task cannot be deleted")
		}
		if err := a.CatalogDAL.DeleteScheduledTask(txCtx, id); err != nil {
			return err
		}
		return a.createScheduledTaskAudit(txCtx, item, schema.TaskAuditDelete, scheduledTaskAuditJSON(item), "")
	})
}

func (a *Catalog) PauseScheduledTask(ctx context.Context, id string) error {
	return a.setScheduledTaskStatus(ctx, id, schema.TaskStatusPaused, schema.TaskAuditPause)
}

func (a *Catalog) ResumeScheduledTask(ctx context.Context, id string) error {
	return a.setScheduledTaskStatus(ctx, id, schema.TaskStatusActive, schema.TaskAuditResume)
}

func (a *Catalog) DisableScheduledTask(ctx context.Context, id string) error {
	return a.setScheduledTaskStatus(ctx, id, schema.TaskStatusDisabled, schema.TaskAuditDisable)
}

func (a *Catalog) TriggerScheduledTask(ctx context.Context, id string) (*schema.ScheduledTaskRun, error) {
	item, err := a.getScheduledTask(ctx, id)
	if err != nil {
		return nil, err
	}
	run, err := a.Runtime.TriggerScheduledTask(ctx, item.ID, item.OwnerUserID, util.FromUserID(ctx))
	if err != nil {
		return nil, errors.Conflict("trading_task_trigger_failed", "Scheduled task could not be triggered: %v", err)
	}
	return run, nil
}

func (a *Catalog) setScheduledTaskStatus(ctx context.Context, id, status, action string) error {
	trans := util.Trans{DB: a.CatalogDAL.DB}
	return trans.Exec(ctx, func(txCtx context.Context) error {
		item, err := a.getScheduledTask(util.NewRowLock(txCtx), id)
		if err != nil {
			return err
		}
		before := scheduledTaskAuditJSON(item)
		item.Status, item.UpdatedAt = status, time.Now().UTC()
		if status == schema.TaskStatusActive {
			nextRun, err := schema.NextScheduledTaskRun(item.CronExpression, item.TimeZone, item.UpdatedAt)
			if err != nil {
				return errors.BadRequest("trading_task_schedule_invalid", "Invalid scheduled task configuration: %v", err)
			}
			item.NextRunAt = &nextRun
		} else {
			item.NextRunAt = nil
		}
		if err := a.CatalogDAL.UpdateScheduledTask(txCtx, item); err != nil {
			return err
		}
		return a.createScheduledTaskAudit(txCtx, item, action, before, scheduledTaskAuditJSON(item))
	})
}

func (a *Catalog) createScheduledTaskAudit(ctx context.Context, task *schema.ScheduledTask, action, beforeJSON, afterJSON string) error {
	return a.CatalogDAL.CreateScheduledTaskAudit(ctx, &schema.ScheduledTaskAudit{
		ID: util.MustNewUUID(), TaskID: task.ID, OwnerUserID: task.OwnerUserID,
		ActorUserID: util.FromUserID(ctx), TaskName: task.Name, Action: action,
		BeforeJSON: beforeJSON, AfterJSON: afterJSON, CreatedAt: time.Now().UTC(),
	})
}

func scheduledTaskAuditJSON(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func (a *Catalog) getScheduledTask(ctx context.Context, id string) (*schema.ScheduledTask, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetScheduledTask(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_task_not_found", "Scheduled task not found")
	}
	return item, nil
}

func (a *Catalog) getExchange(ctx context.Context, id string) (*schema.Exchange, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetExchange(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_exchange_not_found", "Exchange not found")
	}
	return item, nil
}

func (a *Catalog) getAIModel(ctx context.Context, id string) (*schema.AIModel, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetAIModel(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_model_not_found", "AI model not found")
	}
	return item, nil
}

func (a *Catalog) getStrategy(ctx context.Context, id string) (*schema.Strategy, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetStrategy(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_strategy_not_found", "Strategy not found")
	}
	return item, nil
}

func (a *Catalog) getTrader(ctx context.Context, id string) (*schema.Trader, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetTrader(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_trader_not_found", "Trader not found")
	}
	return item, nil
}

func resolveMutationOwner(ctx context.Context, requested string) (string, error) {
	current := strings.TrimSpace(util.FromUserID(ctx))
	if current == "" {
		return "", errors.Unauthorized("trading_user_missing", "Current user is unavailable")
	}
	requested = strings.TrimSpace(requested)
	if util.FromIsRootUser(ctx) && requested != "" {
		return requested, nil
	}
	if requested != "" && requested != current {
		return "", errors.Forbidden("trading_data_scope_denied", "Trading data scope denied")
	}
	return current, nil
}

func mutationOwnerFilter(ctx context.Context) (string, error) {
	if util.FromIsRootUser(ctx) {
		return "", nil
	}
	return resolveMutationOwner(ctx, "")
}

func (a *Catalog) applyExchangeCredentials(item *schema.Exchange, form *schema.ExchangeForm) error {
	fields := []struct {
		plain string
		name  string
		set   func(string)
	}{
		{form.APIKey, "api_key", func(value string) { item.APIKeyCiphertext = value }},
		{form.SecretKey, "secret_key", func(value string) { item.SecretKeyCiphertext = value }},
		{form.Passphrase, "passphrase", func(value string) { item.PassphraseCiphertext = value }},
		{form.PrivateKey, "private_key", func(value string) { item.PrivateKeyCiphertext = value }},
		{form.APIKeyPrivateKey, "api_key_private_key", func(value string) { item.APIKeyPrivateKeyCiphertext = value }},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.plain) == "" {
			continue
		}
		value, err := a.Credentials.EncryptField(field.plain, item.OwnerUserID, item.ID, field.name)
		if err != nil {
			return errors.BadRequest("trading_credential_encryption_failed", "Unable to encrypt exchange credential: %v", err)
		}
		field.set(value)
	}
	return nil
}

func validateEnabledExchange(item *schema.Exchange) error {
	if !item.Enabled {
		return nil
	}
	if item.APIKeyCiphertext == "" || item.SecretKeyCiphertext == "" {
		return errors.BadRequest("trading_exchange_credentials_required", "Enabled exchange requires API key and secret key")
	}
	if item.ExchangeType == "okx" && item.PassphraseCiphertext == "" {
		return errors.BadRequest("trading_exchange_passphrase_required", "Enabled OKX exchange requires passphrase")
	}
	return nil
}

func (a *Catalog) deleteUnreferenced(ctx context.Context, field, id, message string, remove func(context.Context, string) error) error {
	count, err := a.CatalogDAL.CountTradersByReference(ctx, field, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.Conflict("trading_resource_in_use", "%s", message)
	}
	return remove(ctx, id)
}

func (a *Catalog) validateTraderReferences(ctx context.Context, owner string, form *schema.TraderForm, requireEnabled bool) (*schema.Exchange, *schema.AIModel, *schema.Strategy, error) {
	exchange, err := a.CatalogDAL.GetExchange(ctx, form.ExchangeID, owner)
	if err != nil {
		return nil, nil, nil, err
	}
	if exchange == nil {
		return nil, nil, nil, errors.BadRequest("trading_exchange_not_found", "Exchange not found")
	}
	model, err := a.CatalogDAL.GetAIModel(ctx, form.AIModelID, owner)
	if err != nil {
		return nil, nil, nil, err
	}
	if model == nil {
		return nil, nil, nil, errors.BadRequest("trading_model_not_found", "AI model not found")
	}
	var strategy *schema.Strategy
	if form.StrategyID != nil {
		strategy, err = a.CatalogDAL.GetStrategy(ctx, *form.StrategyID, owner)
		if err != nil {
			return nil, nil, nil, err
		}
		if strategy == nil {
			return nil, nil, nil, errors.BadRequest("trading_strategy_not_found", "Strategy not found")
		}
	}
	if requireEnabled {
		if !exchange.Enabled {
			return nil, nil, nil, errors.Conflict("trading_exchange_disabled", "Exchange is disabled")
		}
		if !model.Enabled {
			return nil, nil, nil, errors.Conflict("trading_model_disabled", "AI model is disabled")
		}
		if strategy != nil && !strategy.IsActive {
			return nil, nil, nil, errors.Conflict("trading_strategy_inactive", "Strategy is inactive")
		}
		if strategy != nil && strategy.PublishedVersion == nil {
			return nil, nil, nil, errors.Conflict("trading_strategy_unpublished", "Publish the strategy before starting the trader")
		}
	}
	return exchange, model, strategy, nil
}

func resolveOwnerScope(ctx context.Context, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	current := strings.TrimSpace(util.FromUserID(ctx))
	if current == "" {
		return "", errors.Unauthorized("trading_user_missing", "Current user is unavailable")
	}
	if util.FromIsRootUser(ctx) {
		if requested == "" {
			return current, nil
		}
		return requested, nil
	}
	if requested != "" && requested != current {
		return "", errors.Forbidden("trading_data_scope_denied", "Trading data scope denied")
	}
	return current, nil
}

func buildExchangeView(item *schema.Exchange) *schema.ExchangeView {
	return &schema.ExchangeView{
		ID: item.ID, OwnerUserID: item.OwnerUserID, ExchangeType: item.ExchangeType,
		AccountName: item.AccountName, Name: item.Name, AccountType: item.AccountType,
		Enabled: item.Enabled, Testnet: item.Testnet, WalletAddress: item.WalletAddress,
		AccountUser: item.AccountUser, SignerAddress: item.SignerAddress, APIKeyIndex: item.APIKeyIndex,
		UnifiedAccount: item.UnifiedAccount, BuilderApproved: item.BuilderApproved,
		HasAPIKey: item.APIKeyCiphertext != "", HasSecretKey: item.SecretKeyCiphertext != "",
		HasPassphrase: item.PassphraseCiphertext != "", HasPrivateKey: item.PrivateKeyCiphertext != "",
		HasAPIKeyPrivateKey: item.APIKeyPrivateKeyCiphertext != "", CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func buildAIModelView(item *schema.AIModel) *schema.AIModelView {
	return &schema.AIModelView{
		ID: item.ID, OwnerUserID: item.OwnerUserID, Name: item.Name, Provider: item.Provider,
		Enabled: item.Enabled, HasAPIKey: item.APIKeyCiphertext != "", CustomAPIURL: sanitizeCustomAPIURL(item.CustomAPIURL),
		CustomModelName: item.CustomModelName, InputPricePerMTokens: item.InputPricePerMTokens,
		OutputPricePerMTokens: item.OutputPricePerMTokens, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func sanitizeCustomAPIURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func buildStrategyView(item *schema.Strategy) *schema.StrategyView {
	return &schema.StrategyView{
		ID: item.ID, OwnerUserID: item.OwnerUserID, Name: item.Name, Description: item.Description,
		StrategyType: item.StrategyType, IsActive: item.IsActive, IsDefault: item.IsDefault,
		IsPublic: item.IsPublic, ConfigVisible: item.ConfigVisible, CurrentVersion: item.CurrentVersion,
		PublishedVersion: item.PublishedVersion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func buildTraderView(item *schema.Trader, channelIDs []string) *schema.TraderView {
	return &schema.TraderView{
		ID: item.ID, OwnerUserID: item.OwnerUserID, Name: item.Name, AIModelID: item.AIModelID,
		ExchangeID: item.ExchangeID, StrategyID: item.StrategyID, StrategyVersion: item.StrategyVersion,
		InitialBalance:      item.InitialBalance,
		ScanIntervalMinutes: item.ScanIntervalMinutes, IsCrossMargin: item.IsCrossMargin,
		IsSimulation: item.IsSimulation, ShowInCompetition: item.ShowInCompetition,
		AutoApplyStrategyUpdates: item.AutoApplyStrategyUpdates,
		CompetitionPublicID:      item.CompetitionPublicID, CompetitionAlias: item.CompetitionAlias,
		CompetitionJoinedAt: item.CompetitionJoinedAt,
		LastCycleDurationMS: item.LastCycleDurationMS, ConsecutiveFailures: item.ConsecutiveFailures,
		ExecutionMode: item.ExecutionMode, NotificationChannelIDs: channelIDs,
		DesiredState: item.DesiredState, RuntimeState: item.RuntimeState,
		SyncState: item.SyncState, StrategyState: item.StrategyState, CycleNumber: item.CycleNumber,
		LastDecisionAt: item.LastDecisionAt, SafeMode: item.SafeMode, PauseReason: item.PauseReason,
		AIConsentModelID: item.AIConsentModelID, AIConsentAt: item.AIConsentAt,
		LastHeartbeatAt: item.LastHeartbeatAt, LastErrorCode: item.LastErrorCode,
		LastErrorMessage: item.LastErrorMessage, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (a *Catalog) validateTraderNotificationChannels(ctx context.Context, owner string, channelIDs []string) error {
	if len(channelIDs) == 0 {
		return nil
	}
	count, err := a.CatalogDAL.CountEnabledNotificationChannels(ctx, owner, channelIDs)
	if err != nil {
		return err
	}
	if count != int64(len(channelIDs)) {
		return errors.BadRequest("trading_notification_channel_invalid", "Every selected notification channel must exist, belong to the trader owner, and be enabled")
	}
	return nil
}
