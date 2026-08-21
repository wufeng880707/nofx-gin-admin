package api

import (
	"fmt"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
	"time"
)

type Analytics struct {
	AnalyticsBIZ *biz.Analytics
}

func (a *Analytics) QueryPerformanceReport(c *gin.Context) {
	var params schema.PerformanceReportQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.AnalyticsBIZ.QueryPerformanceReport(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Analytics) ExportPerformanceReport(c *gin.Context) {
	var params schema.PerformanceReportQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	content, err := a.AnalyticsBIZ.ExportPerformanceReport(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=xtrading-report-%s.csv", time.Now().UTC().Format("20060102-150405")))
	c.Data(200, "text/csv; charset=utf-8", content)
}

func (a *Analytics) QueryOverview(c *gin.Context) {
	var params schema.AnalyticsQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.AnalyticsBIZ.QueryOverview(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Analytics) QueryEquityCurve(c *gin.Context) {
	var params schema.AnalyticsQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.AnalyticsBIZ.QueryEquityCurve(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Analytics) QueryPerformance(c *gin.Context) {
	var params schema.AnalyticsQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.AnalyticsBIZ.QueryPerformance(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

func (a *Analytics) QueryClosedTrades(c *gin.Context) {
	var params schema.AnalyticsQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.AnalyticsBIZ.QueryClosedTrades(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}
