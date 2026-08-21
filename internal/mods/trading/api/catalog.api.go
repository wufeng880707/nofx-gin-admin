package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Catalog struct {
	CatalogBIZ *biz.Catalog
}

// @Tags TradingCatalogAPI
// @Security ApiKeyAuth
// @Summary Query masked exchange configurations
// @Success 200 {object} util.ResponseResult{data=[]schema.ExchangeView}
// @Router /api/v1/trading/exchanges [get]
func (a *Catalog) QueryExchanges(c *gin.Context) {
	var params schema.ExchangeQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryExchanges(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags TradingCatalogAPI
// @Security ApiKeyAuth
// @Summary Query masked AI model configurations
// @Success 200 {object} util.ResponseResult{data=[]schema.AIModelView}
// @Router /api/v1/trading/models [get]
func (a *Catalog) QueryAIModels(c *gin.Context) {
	var params schema.AIModelQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryAIModels(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags TradingCatalogAPI
// @Security ApiKeyAuth
// @Summary Query strategy summaries
// @Success 200 {object} util.ResponseResult{data=[]schema.StrategyView}
// @Router /api/v1/trading/strategies [get]
func (a *Catalog) QueryStrategies(c *gin.Context) {
	var params schema.StrategyQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryStrategies(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags TradingCatalogAPI
// @Security ApiKeyAuth
// @Summary Query trader configurations and runtime snapshots
// @Success 200 {object} util.ResponseResult{data=[]schema.TraderView}
// @Router /api/v1/trading/traders [get]
func (a *Catalog) QueryTraders(c *gin.Context) {
	var params schema.TraderQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryTraders(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Catalog) GetExchange(c *gin.Context) {
	result, err := a.CatalogBIZ.GetExchange(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateExchange(c *gin.Context) {
	form := new(schema.ExchangeForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateExchange(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateExchange(c *gin.Context) {
	form := new(schema.ExchangeForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateExchange(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteExchange(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteExchange(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) TestExchange(c *gin.Context) {
	result, err := a.CatalogBIZ.TestExchange(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) SyncExchange(c *gin.Context) {
	result, err := a.CatalogBIZ.SyncExchange(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) BackfillExchangeHistory(c *gin.Context) {
	form := new(schema.HistoryBackfillForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.BackfillExchangeHistory(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) ReplaceExchangeHistorySymbols(c *gin.Context) {
	form := new(schema.HistorySymbolsForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.ReplaceExchangeHistorySymbols(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) QueryHistorySyncStatuses(c *gin.Context) {
	result, err := a.CatalogBIZ.QueryHistorySyncStatuses(c.Request.Context(), c.Query("ownerUserID"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) QueryScheduledTasks(c *gin.Context) {
	var params schema.ScheduledTaskQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryScheduledTasks(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Catalog) QueryScheduledTaskRuns(c *gin.Context) {
	var params schema.ScheduledTaskRunQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryScheduledTaskRuns(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Catalog) QueryScheduledTaskAudits(c *gin.Context) {
	var params schema.ScheduledTaskAuditQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryScheduledTaskAudits(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Catalog) CreateScheduledTask(c *gin.Context) {
	form := new(schema.ScheduledTaskForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateScheduledTask(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateScheduledTask(c *gin.Context) {
	form := new(schema.ScheduledTaskForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateScheduledTask(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteScheduledTask(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteScheduledTask(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) TriggerScheduledTask(c *gin.Context) {
	result, err := a.CatalogBIZ.TriggerScheduledTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) PauseScheduledTask(c *gin.Context) {
	if err := a.CatalogBIZ.PauseScheduledTask(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) ResumeScheduledTask(c *gin.Context) {
	if err := a.CatalogBIZ.ResumeScheduledTask(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DisableScheduledTask(c *gin.Context) {
	if err := a.CatalogBIZ.DisableScheduledTask(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) GetAIModel(c *gin.Context) {
	result, err := a.CatalogBIZ.GetAIModel(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateAIModel(c *gin.Context) {
	form := new(schema.AIModelForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateAIModel(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateAIModel(c *gin.Context) {
	form := new(schema.AIModelForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateAIModel(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteAIModel(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteAIModel(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) TestAIModel(c *gin.Context) {
	result, err := a.CatalogBIZ.TestAIModel(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) GetStrategy(c *gin.Context) {
	result, err := a.CatalogBIZ.GetStrategy(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) ListStrategyVersions(c *gin.Context) {
	result, err := a.CatalogBIZ.ListStrategyVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) PublishStrategy(c *gin.Context) {
	result, err := a.CatalogBIZ.PublishStrategy(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) RollbackStrategy(c *gin.Context) {
	var form struct {
		Version int `json:"version"`
	}
	if err := util.ParseJSON(c, &form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.RollbackStrategy(c.Request.Context(), c.Param("id"), form.Version)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) GetDefaultStrategyConfig(c *gin.Context) {
	util.ResSuccess(c, a.CatalogBIZ.DefaultStrategyConfig())
}

func (a *Catalog) PreviewStrategy(c *gin.Context) {
	form := new(schema.StrategyPreviewForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.PreviewStrategy(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) EstimateStrategyTokens(c *gin.Context) {
	form := new(schema.StrategyTokenEstimateForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.EstimateStrategyTokens(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) TestStrategy(c *gin.Context) {
	form := new(schema.StrategyTestForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.TestStrategy(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) ActivateStrategy(c *gin.Context) {
	if err := a.CatalogBIZ.ActivateStrategy(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DuplicateStrategy(c *gin.Context) {
	form := new(schema.StrategyDuplicateForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, errors.BadRequest("trading_strategy_duplicate_invalid", "Unable to duplicate strategy: %v", err))
		return
	}
	result, err := a.CatalogBIZ.DuplicateStrategy(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateStrategy(c *gin.Context) {
	form := new(schema.StrategyForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateStrategy(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateStrategy(c *gin.Context) {
	form := new(schema.StrategyForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateStrategy(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteStrategy(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteStrategy(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) GetTrader(c *gin.Context) {
	result, err := a.CatalogBIZ.GetTrader(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateTrader(c *gin.Context) {
	form := new(schema.TraderForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateTrader(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateTrader(c *gin.Context) {
	form := new(schema.TraderForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateTrader(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteTrader(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteTrader(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) StartTrader(c *gin.Context) {
	form := new(schema.TraderStartForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.StartTrader(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) StopTrader(c *gin.Context) {
	if err := a.CatalogBIZ.StopTrader(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) SyncTrader(c *gin.Context) {
	if err := a.CatalogBIZ.SyncTrader(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) QueryNotificationChannels(c *gin.Context) {
	var params schema.NotificationChannelQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryNotificationChannels(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Catalog) GetNotificationChannel(c *gin.Context) {
	result, err := a.CatalogBIZ.GetNotificationChannel(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateNotificationChannel(c *gin.Context) {
	form := new(schema.NotificationChannelForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.CreateNotificationChannel(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UpdateNotificationChannel(c *gin.Context) {
	form := new(schema.NotificationChannelForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.CatalogBIZ.UpdateNotificationChannel(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) DeleteNotificationChannel(c *gin.Context) {
	if err := a.CatalogBIZ.DeleteNotificationChannel(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *Catalog) TestNotificationChannel(c *gin.Context) {
	result, err := a.CatalogBIZ.TestNotificationChannel(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) CreateTelegramBindingCode(c *gin.Context) {
	result, err := a.CatalogBIZ.CreateTelegramBindingCode(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) GetTelegramSession(c *gin.Context) {
	result, err := a.CatalogBIZ.GetTelegramSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Catalog) UnbindTelegramSession(c *gin.Context) {
	if err := a.CatalogBIZ.UnbindTelegramSession(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}
