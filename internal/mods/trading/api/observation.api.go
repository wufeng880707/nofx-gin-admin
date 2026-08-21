package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Observation struct {
	ObservationBIZ *biz.Observation
}

func (a *Observation) QueryUserStreamStates(c *gin.Context) {
	result, err := a.ObservationBIZ.QueryUserStreamStates(c.Request.Context())
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Observation) QueryUserStreamEvents(c *gin.Context) {
	var params schema.UserStreamEventQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryUserStreamEvents(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryAssetSummary(c *gin.Context) {
	result, err := a.ObservationBIZ.QueryAssetSummary(c.Request.Context(), c.Query("ownerUserID"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags TradingObservationAPI
// @Security ApiKeyAuth
// @Summary Aggregate owner-scoped AI invocation usage and reliability
// @Success 200 {object} util.ResponseResult{data=schema.AIInvocationSummary}
// @Router /api/v1/trading/ai-invocations/summary [get]
func (a *Observation) QueryAIInvocationSummary(c *gin.Context) {
	var params schema.AIInvocationSummaryQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryAIInvocationSummary(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Observation) QueryAccountSnapshots(c *gin.Context) {
	var params schema.ObservationQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryAccountSnapshots(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryPositions(c *gin.Context) {
	var params schema.PositionQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryPositions(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryAssetBalances(c *gin.Context) {
	var params schema.AssetBalanceQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryAssetBalances(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryOrders(c *gin.Context) {
	var params schema.OrderQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryOrders(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryFills(c *gin.Context) {
	var params schema.FillQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryFills(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryIncomeEvents(c *gin.Context) {
	var params schema.IncomeEventQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryIncomeEvents(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Observation) QueryDecisions(c *gin.Context) {
	var params schema.DecisionQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ObservationBIZ.QueryDecisions(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags TradingObservationAPI
// @Security ApiKeyAuth
// @Summary Get one owner-scoped trading decision with auditable input and output
// @Param id path string true "Decision ID"
// @Success 200 {object} util.ResponseResult{data=schema.DecisionDetailView}
// @Router /api/v1/trading/decisions/{id} [get]
func (a *Observation) GetDecision(c *gin.Context) {
	result, err := a.ObservationBIZ.GetDecision(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
