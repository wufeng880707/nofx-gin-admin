package api

import (
	"strconv"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Competition struct {
	CompetitionBIZ *biz.Competition
}

func (a *Competition) QueryLeaderboard(c *gin.Context) {
	var params schema.CompetitionQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CompetitionBIZ.QueryLeaderboard(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Competition) GetEquityHistory(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	result, err := a.CompetitionBIZ.GetEquityHistory(c.Request.Context(), c.Param("publicID"), days)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Competition) GetParticipation(c *gin.Context) {
	result, err := a.CompetitionBIZ.GetParticipation(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Competition) SetParticipation(c *gin.Context) {
	form := new(schema.CompetitionParticipationForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CompetitionBIZ.SetParticipation(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
