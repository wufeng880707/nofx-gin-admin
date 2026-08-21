package api

import (
	"strconv"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type SpotStrategy struct{ SpotBIZ *biz.SpotStrategy }

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Create a deterministic spot core-satellite backtest
// @Param body body schema.SpotBacktestCreateForm true "Backtest configuration"
// @Success 200 {object} util.ResponseResult{data=schema.SpotBacktestTask}
// @Router /api/v1/trading/backtests [post]
func (a *SpotStrategy) CreateBacktest(c *gin.Context) {
	form := new(schema.SpotBacktestCreateForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.SpotBIZ.CreateBacktest(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Query spot strategy backtest tasks
// @Success 200 {object} util.ResponseResult{data=[]schema.SpotBacktestTask}
// @Router /api/v1/trading/backtests [get]
func (a *SpotStrategy) QueryBacktests(c *gin.Context) {
	var params schema.SpotBacktestQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.SpotBIZ.QueryBacktests(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Get one spot strategy backtest task
// @Param id path string true "Task ID"
// @Success 200 {object} util.ResponseResult{data=schema.SpotBacktestTask}
// @Router /api/v1/trading/backtests/{id} [get]
func (a *SpotStrategy) GetBacktest(c *gin.Context) {
	result, err := a.SpotBIZ.GetBacktest(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Get metrics, benchmarks, lots and events for a completed backtest
// @Param id path string true "Task ID"
// @Success 200 {object} util.ResponseResult{data=schema.SpotBacktestResultView}
// @Router /api/v1/trading/backtests/{id}/result [get]
func (a *SpotStrategy) GetBacktestResult(c *gin.Context) {
	result, err := a.SpotBIZ.GetBacktestResult(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Cancel a running spot strategy backtest
// @Param id path string true "Task ID"
// @Success 200 {object} util.ResponseResult
// @Router /api/v1/trading/backtests/{id}/cancel [post]
func (a *SpotStrategy) CancelBacktest(c *gin.Context) {
	if err := a.SpotBIZ.CancelBacktest(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, nil)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Query explicitly attributed spot position lots
// @Success 200 {object} util.ResponseResult{data=[]schema.SpotPositionLot}
// @Router /api/v1/trading/spot-strategy/lots [get]
func (a *SpotStrategy) QueryLots(c *gin.Context) {
	var params schema.SpotLotQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.SpotBIZ.QueryLots(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags SpotStrategyAPI
// @Security ApiKeyAuth
// @Summary Query immutable state events for a spot strategy cycle
// @Param cycleID path string true "Cycle ID"
// @Success 200 {object} util.ResponseResult{data=[]schema.SpotStateEvent}
// @Router /api/v1/trading/spot-strategy/cycles/{cycleID}/events [get]
func (a *SpotStrategy) QueryStateEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	result, err := a.SpotBIZ.QueryStateEvents(c.Request.Context(), c.Param("cycleID"), limit)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
