package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Query Binance or OKX instruments
// @Success 200 {object} util.ResponseResult{data=[]schema.MarketInstrument}
// @Router /api/v1/market/instruments [get]
func (a *Catalog) QueryMarketInstruments(c *gin.Context) {
	params := new(schema.MarketInstrumentsQuery)
	if err := util.ParseQuery(c, params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryMarketInstruments(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Get a Binance or OKX 24-hour ticker
// @Success 200 {object} util.ResponseResult{data=schema.MarketTicker}
// @Router /api/v1/market/ticker [get]
func (a *Catalog) GetMarketTicker(c *gin.Context) {
	params := new(schema.MarketSymbolQuery)
	if err := util.ParseQuery(c, params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.GetMarketTicker(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Query Binance or OKX candlesticks
// @Success 200 {object} util.ResponseResult{data=[]schema.MarketKline}
// @Router /api/v1/market/klines [get]
func (a *Catalog) QueryMarketKlines(c *gin.Context) {
	params := new(schema.MarketKlinesQuery)
	if err := util.ParseQuery(c, params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.QueryMarketKlines(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Get public order book, derivatives metrics, taker-flow proxy and risk radar
// @Success 200 {object} util.ResponseResult{data=schema.MarketContext}
// @Router /api/v1/market/context [get]
func (a *Catalog) GetMarketContext(c *gin.Context) {
	params := new(schema.MarketContextQuery)
	if err := util.ParseQuery(c, params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.GetMarketContext(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Get the explicit provider and public data-source capability matrix
// @Success 200 {object} util.ResponseResult{data=schema.ProviderCapabilityMatrix}
// @Router /api/v1/market/provider-capabilities [get]
func (a *Catalog) GetProviderCapabilities(c *gin.Context) {
	util.ResSuccess(c, a.CatalogBIZ.GetProviderCapabilities())
}

// @Tags MarketAPI
// @Security ApiKeyAuth
// @Summary Query persisted Binance USD-M liquidation heatmap buckets
// @Success 200 {object} util.ResponseResult{data=schema.LiquidationHeatmap}
// @Router /api/v1/market/liquidations/heatmap [get]
func (a *Catalog) GetLiquidationHeatmap(c *gin.Context) {
	params := new(schema.LiquidationHeatmapQuery)
	if err := util.ParseQuery(c, params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.CatalogBIZ.GetLiquidationHeatmap(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
