package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Execution struct{ ExecutionBIZ *biz.Execution }

func (a *Execution) GetSafetyState(c *gin.Context) {
	result, err := a.ExecutionBIZ.GetSafetyState(c.Request.Context(), c.Query("ownerUserID"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) EnableLiveTrading(c *gin.Context) {
	form := new(schema.TradingSafetyEnableForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.EnableLiveTrading(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) DisableLiveTrading(c *gin.Context) {
	form := new(schema.TradingSafetyDisableForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.DisableLiveTrading(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) QueryCommands(c *gin.Context) {
	var params schema.TradeCommandQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.QueryCommands(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Execution) CreateCommand(c *gin.Context) {
	form := new(schema.TradeCommandForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.CreateCommand(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) BulkClosePositions(c *gin.Context) {
	form := new(schema.BulkClosePositionsForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.Validate(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.BulkClosePositions(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) QueryCommandEvents(c *gin.Context) {
	result, err := a.ExecutionBIZ.QueryCommandEvents(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) QueryApprovals(c *gin.Context) {
	var params schema.TradeApprovalQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.QueryApprovals(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

func (a *Execution) QueryApprovalEvents(c *gin.Context) {
	result, err := a.ExecutionBIZ.QueryApprovalEvents(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) Approve(c *gin.Context) {
	form := new(schema.TradeApprovalActForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.ValidateApprove(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.Approve(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Execution) RejectApproval(c *gin.Context) {
	form := new(schema.TradeApprovalActForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := form.ValidateReject(); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.ExecutionBIZ.RejectApproval(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
