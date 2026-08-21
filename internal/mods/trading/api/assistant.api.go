package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/assistant"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Assistant struct {
	Service *assistant.Service
}

// ExecuteTask runs an authenticated, owner-scoped, read-only task skill.
// @Tags TradingAssistantAPI
// @Security ApiKeyAuth
// @Summary Execute an owner-scoped read-only trading assistant task
// @Param body body schema.AssistantTaskForm true "Task request"
// @Success 200 {object} util.ResponseResult{data=schema.AssistantTaskView}
// @Router /api/v1/trading/assistant/tasks [post]
func (a *Assistant) ExecuteTask(c *gin.Context) {
	form := new(schema.AssistantTaskForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	form.Validate()
	owner := util.FromUserID(c.Request.Context())
	result, err := a.Service.Execute(c.Request.Context(), owner, form.Message)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, &schema.AssistantTaskView{Skill: result.Skill, Text: result.Text})
}
