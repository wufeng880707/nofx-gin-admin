package provider

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func (a *MarketData) Context(ctx context.Context, exchange, market, symbol string, depth int) (*schema.MarketContext, error) {
	ticker, err := a.Ticker(ctx, exchange, market, symbol)
	if err != nil {
		return nil, err
	}
	book, err := a.orderBook(ctx, exchange, market, symbol, depth)
	if err != nil {
		return nil, err
	}
	metrics, err := a.marketMetrics(ctx, exchange, market, symbol)
	if err != nil {
		return nil, err
	}
	radar := buildMarketRiskRadar(book, metrics)
	return &schema.MarketContext{Exchange: exchange, Market: market, Symbol: symbol, Ticker: ticker,
		OrderBook: book, Derivatives: metrics, RiskRadar: radar, GeneratedAt: time.Now().UnixMilli()}, nil
}

func (a *MarketData) orderBook(ctx context.Context, exchange, market, symbol string, depth int) (*schema.MarketOrderBook, error) {
	book := &schema.MarketOrderBook{}
	switch exchange {
	case schema.MarketExchangeBinance:
		path := binancePath(market, "/api/v3/depth", "/fapi/v1/depth")
		request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+url.Values{
			"symbol": {strings.ReplaceAll(symbol, "-", "")}, "limit": {strconv.Itoa(depth)},
		}.Encode(), nil)
		var response struct {
			LastUpdateID int64      `json:"lastUpdateId"`
			Bids         [][]string `json:"bids"`
			Asks         [][]string `json:"asks"`
		}
		if err := doJSON(ctx, a.client, request, &response); err != nil {
			return nil, err
		}
		book.Bids, book.Asks, book.Timestamp = depthLevels(response.Bids), depthLevels(response.Asks), time.Now().UnixMilli()
	case schema.MarketExchangeOKX:
		request, _ := http.NewRequest(http.MethodGet, a.okxURL+"/api/v5/market/books?"+url.Values{
			"instId": {symbol}, "sz": {strconv.Itoa(depth)},
		}.Encode(), nil)
		var response struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data []struct {
				Bids [][]string `json:"bids"`
				Asks [][]string `json:"asks"`
				TS   string     `json:"ts"`
			} `json:"data"`
		}
		if err := doJSON(ctx, a.client, request, &response); err != nil {
			return nil, err
		}
		if response.Code != "0" || len(response.Data) == 0 {
			return nil, fmt.Errorf("okx order book failed: %s", response.Msg)
		}
		book.Bids, book.Asks = depthLevels(response.Data[0].Bids), depthLevels(response.Data[0].Asks)
		book.Timestamp, _ = strconv.ParseInt(response.Data[0].TS, 10, 64)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
	book.SpreadBPS, book.Imbalance = orderBookIndicators(book)
	return book, nil
}

func (a *MarketData) marketMetrics(ctx context.Context, exchange, market, symbol string) (*schema.MarketDerivatives, error) {
	metrics := &schema.MarketDerivatives{FlowSource: exchange + "_recent_public_trades", FlowIsProxy: true}
	buy, sell, err := a.recentTakerFlow(ctx, exchange, market, symbol)
	if err != nil {
		return nil, err
	}
	metrics.TakerBuyVolume, metrics.TakerSellVolume = compactMarketFloat(buy), compactMarketFloat(sell)
	if sell > 0 {
		metrics.TakerBuySellRatio = buy / sell
	}
	if market != schema.MarketTypeFutures {
		return metrics, nil
	}
	switch exchange {
	case schema.MarketExchangeBinance:
		base := a.binanceBaseURL(schema.MarketTypeFutures)
		query := url.Values{"symbol": {strings.ReplaceAll(symbol, "-", "")}}.Encode()
		var oi struct {
			OpenInterest string `json:"openInterest"`
		}
		request, _ := http.NewRequest(http.MethodGet, base+"/fapi/v1/openInterest?"+query, nil)
		if err := doJSON(ctx, a.client, request, &oi); err != nil {
			return nil, err
		}
		var premium struct {
			MarkPrice       string `json:"markPrice"`
			LastFundingRate string `json:"lastFundingRate"`
			NextFundingTime int64  `json:"nextFundingTime"`
		}
		request, _ = http.NewRequest(http.MethodGet, base+"/fapi/v1/premiumIndex?"+query, nil)
		if err := doJSON(ctx, a.client, request, &premium); err != nil {
			return nil, err
		}
		metrics.OpenInterest, metrics.FundingRate = oi.OpenInterest, premium.LastFundingRate
		metrics.MarkPrice, metrics.NextFundingTime = premium.MarkPrice, premium.NextFundingTime
		metrics.OpenInterestValue = multiplyMarketDecimals(oi.OpenInterest, premium.MarkPrice)
	case schema.MarketExchangeOKX:
		query := url.Values{"instType": {"SWAP"}, "instId": {symbol}}.Encode()
		var oi struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data []struct {
				OI    string `json:"oi"`
				OICcy string `json:"oiCcy"`
			} `json:"data"`
		}
		request, _ := http.NewRequest(http.MethodGet, a.okxURL+"/api/v5/public/open-interest?"+query, nil)
		if err := doJSON(ctx, a.client, request, &oi); err != nil {
			return nil, err
		}
		if oi.Code != "0" || len(oi.Data) == 0 {
			return nil, fmt.Errorf("okx open interest failed: %s", oi.Msg)
		}
		var funding struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data []struct {
				FundingRate     string `json:"fundingRate"`
				NextFundingTime string `json:"nextFundingTime"`
			} `json:"data"`
		}
		request, _ = http.NewRequest(http.MethodGet, a.okxURL+"/api/v5/public/funding-rate?"+url.Values{"instId": {symbol}}.Encode(), nil)
		if err := doJSON(ctx, a.client, request, &funding); err != nil {
			return nil, err
		}
		if funding.Code != "0" || len(funding.Data) == 0 {
			return nil, fmt.Errorf("okx funding rate failed: %s", funding.Msg)
		}
		metrics.OpenInterest, metrics.OpenInterestValue = oi.Data[0].OI, oi.Data[0].OICcy
		metrics.FundingRate = funding.Data[0].FundingRate
		metrics.NextFundingTime, _ = strconv.ParseInt(funding.Data[0].NextFundingTime, 10, 64)
	}
	return metrics, nil
}

func (a *MarketData) recentTakerFlow(ctx context.Context, exchange, market, symbol string) (float64, float64, error) {
	buy, sell := 0.0, 0.0
	switch exchange {
	case schema.MarketExchangeBinance:
		path := binancePath(market, "/api/v3/trades", "/fapi/v1/trades")
		request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+url.Values{
			"symbol": {strings.ReplaceAll(symbol, "-", "")}, "limit": {"100"},
		}.Encode(), nil)
		var trades []struct {
			Qty          string `json:"qty"`
			IsBuyerMaker bool   `json:"isBuyerMaker"`
		}
		if err := doJSON(ctx, a.client, request, &trades); err != nil {
			return 0, 0, err
		}
		for _, trade := range trades {
			quantity, _ := strconv.ParseFloat(trade.Qty, 64)
			if trade.IsBuyerMaker {
				sell += quantity
			} else {
				buy += quantity
			}
		}
	case schema.MarketExchangeOKX:
		request, _ := http.NewRequest(http.MethodGet, a.okxURL+"/api/v5/market/trades?"+url.Values{
			"instId": {symbol}, "limit": {"100"},
		}.Encode(), nil)
		var response struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data []struct {
				Side string `json:"side"`
				Sz   string `json:"sz"`
			} `json:"data"`
		}
		if err := doJSON(ctx, a.client, request, &response); err != nil {
			return 0, 0, err
		}
		if response.Code != "0" {
			return 0, 0, fmt.Errorf("okx recent trades failed: %s", response.Msg)
		}
		for _, trade := range response.Data {
			quantity, _ := strconv.ParseFloat(trade.Sz, 64)
			if trade.Side == "buy" {
				buy += quantity
			} else if trade.Side == "sell" {
				sell += quantity
			}
		}
	default:
		return 0, 0, fmt.Errorf("unsupported market exchange %q", exchange)
	}
	return buy, sell, nil
}

func depthLevels(rows [][]string) []schema.MarketDepthLevel {
	result := make([]schema.MarketDepthLevel, 0, len(rows))
	for _, row := range rows {
		if len(row) >= 2 {
			result = append(result, schema.MarketDepthLevel{Price: row[0], Quantity: row[1]})
		}
	}
	return result
}

func orderBookIndicators(book *schema.MarketOrderBook) (float64, float64) {
	if book == nil || len(book.Bids) == 0 || len(book.Asks) == 0 {
		return 0, 0
	}
	bestBid, _ := strconv.ParseFloat(book.Bids[0].Price, 64)
	bestAsk, _ := strconv.ParseFloat(book.Asks[0].Price, 64)
	mid := (bestBid + bestAsk) / 2
	spread := 0.0
	if mid > 0 {
		spread = (bestAsk - bestBid) / mid * 10000
	}
	bidQuantity, askQuantity := 0.0, 0.0
	for _, level := range book.Bids {
		quantity, _ := strconv.ParseFloat(level.Quantity, 64)
		bidQuantity += quantity
	}
	for _, level := range book.Asks {
		quantity, _ := strconv.ParseFloat(level.Quantity, 64)
		askQuantity += quantity
	}
	imbalance := 0.0
	if total := bidQuantity + askQuantity; total > 0 {
		imbalance = (bidQuantity - askQuantity) / total
	}
	return math.Max(0, spread), imbalance
}

func buildMarketRiskRadar(book *schema.MarketOrderBook, metrics *schema.MarketDerivatives) *schema.MarketRiskRadar {
	result := &schema.MarketRiskRadar{LiquidityRisk: "low", CrowdingRisk: "low"}
	if book != nil {
		result.SpreadBPS, result.BookImbalance = book.SpreadBPS, book.Imbalance
		if book.SpreadBPS > 10 {
			result.LiquidityRisk, result.RiskScore = "high", result.RiskScore+40
		} else if book.SpreadBPS > 3 {
			result.LiquidityRisk, result.RiskScore = "medium", result.RiskScore+20
		}
		if math.Abs(book.Imbalance) > 0.6 {
			result.RiskScore += 20
		}
	}
	if metrics != nil {
		funding, _ := strconv.ParseFloat(metrics.FundingRate, 64)
		result.FundingRatePct = funding * 100
		if math.Abs(result.FundingRatePct) > 0.1 {
			result.CrowdingRisk, result.RiskScore = "high", result.RiskScore+40
		} else if math.Abs(result.FundingRatePct) > 0.03 {
			result.CrowdingRisk, result.RiskScore = "medium", result.RiskScore+20
		}
	}
	if result.RiskScore > 100 {
		result.RiskScore = 100
	}
	return result
}

func compactMarketFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 8, 64), "0"), ".")
}

func multiplyMarketDecimals(left, right string) string {
	l, _ := strconv.ParseFloat(left, 64)
	r, _ := strconv.ParseFloat(right, 64)
	return compactMarketFloat(l * r)
}
