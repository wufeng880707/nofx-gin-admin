package trading

import (
	"context"
	"fmt"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/api"
	tradingruntime "github.com/LyricTian/gin-admin/v10/internal/mods/trading/runtime"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Trading owns the NOFX domain lifecycle inside the gin-admin process.
type Trading struct {
	DB              *gorm.DB
	CatalogAPI      *api.Catalog
	ObservationAPI  *api.Observation
	AnalyticsAPI    *api.Analytics
	ExecutionAPI    *api.Execution
	CompetitionAPI  *api.Competition
	AssistantAPI    *api.Assistant
	SpotStrategyAPI *api.SpotStrategy
	Runtime         *tradingruntime.Registry
}

func (a *Trading) RegisterV1Routers(_ context.Context, v1 *gin.RouterGroup) error {
	marketRouter := v1.Group("market")
	marketRouter.GET("instruments", a.CatalogAPI.QueryMarketInstruments)
	marketRouter.GET("ticker", a.CatalogAPI.GetMarketTicker)
	marketRouter.GET("klines", a.CatalogAPI.QueryMarketKlines)
	marketRouter.GET("context", a.CatalogAPI.GetMarketContext)
	marketRouter.GET("provider-capabilities", a.CatalogAPI.GetProviderCapabilities)
	marketRouter.GET("liquidations/heatmap", a.CatalogAPI.GetLiquidationHeatmap)

	competitionRouter := v1.Group("competition")
	competitionRouter.GET("leaderboard", a.CompetitionAPI.QueryLeaderboard)
	competitionRouter.GET("traders/:publicID/equity", a.CompetitionAPI.GetEquityHistory)

	router := v1.Group("trading")
	router.GET("exchanges", a.CatalogAPI.QueryExchanges)
	router.POST("exchanges", a.CatalogAPI.CreateExchange)
	router.GET("exchanges/:id", a.CatalogAPI.GetExchange)
	router.PUT("exchanges/:id", a.CatalogAPI.UpdateExchange)
	router.DELETE("exchanges/:id", a.CatalogAPI.DeleteExchange)
	router.POST("exchanges/:id/test", a.CatalogAPI.TestExchange)
	router.POST("exchanges/:id/sync", a.CatalogAPI.SyncExchange)
	router.POST("exchanges/:id/history/backfill", a.CatalogAPI.BackfillExchangeHistory)
	router.PUT("exchanges/:id/history/symbols", a.CatalogAPI.ReplaceExchangeHistorySymbols)
	router.GET("history-status", a.CatalogAPI.QueryHistorySyncStatuses)
	router.GET("scheduled-tasks", a.CatalogAPI.QueryScheduledTasks)
	router.POST("scheduled-tasks", a.CatalogAPI.CreateScheduledTask)
	router.PUT("scheduled-tasks/:id", a.CatalogAPI.UpdateScheduledTask)
	router.DELETE("scheduled-tasks/:id", a.CatalogAPI.DeleteScheduledTask)
	router.POST("scheduled-tasks/:id/trigger", a.CatalogAPI.TriggerScheduledTask)
	router.POST("scheduled-tasks/:id/pause", a.CatalogAPI.PauseScheduledTask)
	router.POST("scheduled-tasks/:id/resume", a.CatalogAPI.ResumeScheduledTask)
	router.POST("scheduled-tasks/:id/disable", a.CatalogAPI.DisableScheduledTask)
	router.GET("scheduled-task-runs", a.CatalogAPI.QueryScheduledTaskRuns)
	router.GET("scheduled-task-audits", a.CatalogAPI.QueryScheduledTaskAudits)
	router.GET("models", a.CatalogAPI.QueryAIModels)
	router.POST("models", a.CatalogAPI.CreateAIModel)
	router.GET("models/:id", a.CatalogAPI.GetAIModel)
	router.PUT("models/:id", a.CatalogAPI.UpdateAIModel)
	router.DELETE("models/:id", a.CatalogAPI.DeleteAIModel)
	router.POST("models/:id/test", a.CatalogAPI.TestAIModel)
	router.GET("strategies", a.CatalogAPI.QueryStrategies)
	router.POST("strategies", a.CatalogAPI.CreateStrategy)
	router.GET("strategies/default-config", a.CatalogAPI.GetDefaultStrategyConfig)
	router.POST("strategies/preview", a.CatalogAPI.PreviewStrategy)
	router.POST("strategies/estimate-tokens", a.CatalogAPI.EstimateStrategyTokens)
	router.POST("strategies/test", a.CatalogAPI.TestStrategy)
	router.GET("strategies/:id/versions", a.CatalogAPI.ListStrategyVersions)
	router.POST("strategies/:id/publish", a.CatalogAPI.PublishStrategy)
	router.POST("strategies/:id/rollback", a.CatalogAPI.RollbackStrategy)
	router.GET("strategies/:id", a.CatalogAPI.GetStrategy)
	router.PUT("strategies/:id", a.CatalogAPI.UpdateStrategy)
	router.DELETE("strategies/:id", a.CatalogAPI.DeleteStrategy)
	router.POST("strategies/:id/activate", a.CatalogAPI.ActivateStrategy)
	router.POST("strategies/:id/duplicate", a.CatalogAPI.DuplicateStrategy)
	router.GET("traders", a.CatalogAPI.QueryTraders)
	router.POST("traders", a.CatalogAPI.CreateTrader)
	router.GET("traders/:id", a.CatalogAPI.GetTrader)
	router.PUT("traders/:id", a.CatalogAPI.UpdateTrader)
	router.DELETE("traders/:id", a.CatalogAPI.DeleteTrader)
	router.POST("traders/:id/start", a.CatalogAPI.StartTrader)
	router.POST("traders/:id/stop", a.CatalogAPI.StopTrader)
	router.POST("traders/:id/sync", a.CatalogAPI.SyncTrader)
	router.GET("traders/:id/competition", a.CompetitionAPI.GetParticipation)
	router.PUT("traders/:id/competition", a.CompetitionAPI.SetParticipation)
	router.GET("notification-channels", a.CatalogAPI.QueryNotificationChannels)
	router.POST("notification-channels", a.CatalogAPI.CreateNotificationChannel)
	router.GET("notification-channels/:id", a.CatalogAPI.GetNotificationChannel)
	router.PUT("notification-channels/:id", a.CatalogAPI.UpdateNotificationChannel)
	router.DELETE("notification-channels/:id", a.CatalogAPI.DeleteNotificationChannel)
	router.POST("notification-channels/:id/test", a.CatalogAPI.TestNotificationChannel)
	router.POST("notification-channels/:id/telegram/binding-code", a.CatalogAPI.CreateTelegramBindingCode)
	router.GET("notification-channels/:id/telegram/session", a.CatalogAPI.GetTelegramSession)
	router.DELETE("notification-channels/:id/telegram/session", a.CatalogAPI.UnbindTelegramSession)
	router.GET("account-snapshots", a.ObservationAPI.QueryAccountSnapshots)
	router.GET("asset-summary", a.ObservationAPI.QueryAssetSummary)
	router.GET("ai-invocations/summary", a.ObservationAPI.QueryAIInvocationSummary)
	router.GET("asset-balances", a.ObservationAPI.QueryAssetBalances)
	router.GET("positions", a.ObservationAPI.QueryPositions)
	router.GET("orders", a.ObservationAPI.QueryOrders)
	router.GET("fills", a.ObservationAPI.QueryFills)
	router.GET("income-events", a.ObservationAPI.QueryIncomeEvents)
	router.GET("user-stream/states", a.ObservationAPI.QueryUserStreamStates)
	router.GET("user-stream/events", a.ObservationAPI.QueryUserStreamEvents)
	router.GET("decisions", a.ObservationAPI.QueryDecisions)
	router.GET("decisions/:id", a.ObservationAPI.GetDecision)
	router.GET("execution/safety", a.ExecutionAPI.GetSafetyState)
	router.POST("execution/safety/enable", a.ExecutionAPI.EnableLiveTrading)
	router.POST("execution/safety/disable", a.ExecutionAPI.DisableLiveTrading)
	router.GET("trade-commands", a.ExecutionAPI.QueryCommands)
	router.POST("trade-commands", a.ExecutionAPI.CreateCommand)
	router.POST("trade-commands/bulk-close", a.ExecutionAPI.BulkClosePositions)
	router.GET("trade-commands/:id/events", a.ExecutionAPI.QueryCommandEvents)
	router.GET("trade-approvals", a.ExecutionAPI.QueryApprovals)
	router.GET("trade-approvals/:id/events", a.ExecutionAPI.QueryApprovalEvents)
	router.POST("trade-approvals/:id/approve", a.ExecutionAPI.Approve)
	router.POST("trade-approvals/:id/reject", a.ExecutionAPI.RejectApproval)
	router.GET("dashboard/overview", a.AnalyticsAPI.QueryOverview)
	router.GET("analytics/equity-curve", a.AnalyticsAPI.QueryEquityCurve)
	router.GET("analytics/performance", a.AnalyticsAPI.QueryPerformance)
	router.GET("analytics/closed-trades", a.AnalyticsAPI.QueryClosedTrades)
	router.GET("analytics/reports", a.AnalyticsAPI.QueryPerformanceReport)
	router.GET("analytics/reports/export", a.AnalyticsAPI.ExportPerformanceReport)
	router.GET("spot-strategy/lots", a.SpotStrategyAPI.QueryLots)
	router.GET("spot-strategy/cycles/:cycleID/events", a.SpotStrategyAPI.QueryStateEvents)
	router.GET("backtests", a.SpotStrategyAPI.QueryBacktests)
	router.POST("backtests", a.SpotStrategyAPI.CreateBacktest)
	router.GET("backtests/:id", a.SpotStrategyAPI.GetBacktest)
	router.GET("backtests/:id/result", a.SpotStrategyAPI.GetBacktestResult)
	router.POST("backtests/:id/cancel", a.SpotStrategyAPI.CancelBacktest)
	router.POST("assistant/tasks", a.AssistantAPI.ExecuteTask)
	return nil
}

func (a *Trading) AutoMigrate(context.Context) error {
	return a.DB.AutoMigrate(
		new(schema.Exchange),
		new(schema.AIModel),
		new(schema.AIInvocation),
		new(schema.Strategy),
		new(schema.StrategyVersion),
		new(schema.StrategyVersionSwitchEvent),
		new(schema.Trader),
		new(schema.NotificationChannel),
		new(schema.TraderNotificationBinding),
		new(schema.TelegramBindingCode),
		new(schema.TelegramSession),
		new(schema.TelegramPollState),
		new(schema.AccountSnapshot),
		new(schema.AssetBalance),
		new(schema.Position),
		new(schema.ExchangeOrder),
		new(schema.Fill),
		new(schema.IncomeEvent),
		new(schema.ClosedTrade),
		new(schema.SyncCursor),
		new(schema.HistorySymbol),
		new(schema.Decision),
		new(schema.LegacyImportRun),
		new(schema.RuntimeLease),
		new(schema.ScheduledTask),
		new(schema.ScheduledTaskRun),
		new(schema.ScheduledTaskAudit),
		new(schema.TradingSafetyState),
		new(schema.TradingSafetyEvent),
		new(schema.TradeCommand),
		new(schema.TradeCommandEvent),
		new(schema.TradeApproval),
		new(schema.TradeApprovalEvent),
		new(schema.UserStreamState),
		new(schema.UserStreamEvent),
		new(schema.MarketLiquidationEvent),
		new(schema.SpotStrategyCycle),
		new(schema.SpotPositionLot),
		new(schema.SpotOrderIntent),
		new(schema.SpotSellAllocation),
		new(schema.SpotStateEvent),
		new(schema.SpotBacktestTask),
		new(schema.SpotBacktestResult),
	)
}

func (a *Trading) Init(ctx context.Context) error {
	a.Runtime.SetDecisionWorkflow(a.ExecutionAPI.ExecutionBIZ)
	if config.C.Storage.DB.AutoMigrate {
		if err := a.AutoMigrate(ctx); err != nil {
			return err
		}
		if err := a.CatalogAPI.CatalogBIZ.CatalogDAL.BackfillStrategyVersions(ctx); err != nil {
			return err
		}
	}
	if err := a.SpotStrategyAPI.SpotBIZ.Recover(ctx); err != nil {
		return err
	}
	if config.C.Trading.AccountSync.Enabled {
		if err := a.ensureDefaultScheduledTasks(ctx); err != nil {
			return err
		}
	}
	if err := a.Runtime.RequireRestartConfirmation(ctx); err != nil {
		return err
	}
	a.Runtime.StartPersistentTaskScheduler(5 * time.Second)
	if config.C.Trading.UserStream.Enabled {
		if err := a.Runtime.StartUserDataStreams(); err != nil {
			return err
		}
	}
	if config.C.Trading.MarketStream.Enabled {
		if err := a.Runtime.StartPublicMarketStream(); err != nil {
			return err
		}
	}
	if config.C.Trading.TelegramBot.Enabled {
		if err := a.Runtime.StartTelegramBot(); err != nil {
			return err
		}
	}
	return nil
}

func (a *Trading) Release(ctx context.Context) error {
	a.SpotStrategyAPI.SpotBIZ.Shutdown()
	if err := a.Runtime.StopTelegramBot(ctx); err != nil {
		return err
	}
	if err := a.Runtime.StopPublicMarketStream(ctx); err != nil {
		return err
	}
	if err := a.Runtime.StopUserDataStreams(ctx); err != nil {
		return err
	}
	if err := a.Runtime.StopPersistentTaskScheduler(ctx); err != nil {
		return err
	}
	return a.Runtime.StopAll(ctx)
}

func (a *Trading) ensureDefaultScheduledTasks(ctx context.Context) error {
	timeZone := config.C.Trading.AccountSync.SnapshotTimeZone
	if timeZone == "" {
		timeZone = "UTC"
	}
	timeout := config.C.Trading.AccountSync.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}
	syncInterval := time.Duration(config.C.Trading.AccountSync.IntervalSeconds) * time.Second
	if syncInterval < time.Minute {
		syncInterval = 5 * time.Minute
	}
	snapshotInterval := config.C.Trading.AccountSync.SnapshotIntervalHours
	if snapshotInterval <= 0 {
		snapshotInterval = 12
	}
	snapshotCron := fmt.Sprintf("@every %dh", snapshotInterval)
	if snapshotInterval == 12 {
		snapshotCron = "0 0,12 * * *"
	}

	now := time.Now().UTC()
	defaults := []*schema.ScheduledTask{
		{
			ID: util.MustNewUUID(), OwnerUserID: "root", Name: "账户数据同步",
			Handler: schema.TaskHandlerAccountSync, CronExpression: "@every " + syncInterval.String(), TimeZone: timeZone,
			Status: schema.TaskStatusActive, TimeoutSeconds: timeout, MaxRetries: 2, RetryIntervalSeconds: 5,
			MisfirePolicy: schema.TaskMisfireFireOnce, ConcurrencyPolicy: schema.TaskConcurrencyForbid, NextRunAt: &now,
		},
		{
			ID: util.MustNewUUID(), OwnerUserID: "root", Name: "账户快照",
			Handler: schema.TaskHandlerAccountSnapshot, CronExpression: snapshotCron, TimeZone: timeZone,
			Status: schema.TaskStatusActive, TimeoutSeconds: timeout, MaxRetries: 2, RetryIntervalSeconds: 5,
			MisfirePolicy: schema.TaskMisfireFireOnce, ConcurrencyPolicy: schema.TaskConcurrencyForbid,
		},
	}
	for _, task := range defaults {
		if task.NextRunAt == nil {
			next, err := schema.NextScheduledTaskRun(task.CronExpression, task.TimeZone, now)
			if err != nil {
				return err
			}
			task.NextRunAt = &next
		}
		if err := ensureScheduledTask(ctx, a.DB, task); err != nil {
			return err
		}
	}
	return nil
}

func ensureScheduledTask(ctx context.Context, db *gorm.DB, task *schema.ScheduledTask) error {
	var existing schema.ScheduledTask
	return db.WithContext(ctx).
		Where("owner_user_id = ? AND name = ?", task.OwnerUserID, task.Name).
		Attrs(task).
		FirstOrCreate(&existing).Error
}
