package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

type rankedMarketSymbol struct {
	Symbol string
	Score  float64
}

// LiquidityCandidates is the free public-data replacement for a proprietary
// AI500 pool. It ranks active USDT markets by public 24-hour quote volume.
func (a *MarketData) LiquidityCandidates(ctx context.Context, exchange, market string, limit int) ([]string, error) {
	if limit < 1 {
		return []string{}, nil
	}
	var ranked []rankedMarketSymbol
	var err error
	switch exchange {
	case schema.MarketExchangeBinance:
		ranked, err = a.binanceLiquidityCandidates(ctx, market)
	case schema.MarketExchangeOKX:
		ranked, err = a.okxLiquidityCandidates(ctx, market)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
	if err != nil {
		return nil, err
	}
	return topRankedSymbols(ranked, limit), nil
}

func (a *MarketData) OpenInterestCandidates(ctx context.Context, exchange string, limit int) ([]string, error) {
	if limit < 1 {
		return []string{}, nil
	}
	var ranked []rankedMarketSymbol
	var err error
	switch exchange {
	case schema.MarketExchangeBinance:
		ranked, err = a.binanceOpenInterestCandidates(ctx, limit)
	case schema.MarketExchangeOKX:
		ranked, err = a.okxOpenInterestCandidates(ctx)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
	if err != nil {
		return nil, err
	}
	return topRankedSymbols(ranked, limit), nil
}

func (a *MarketData) binanceLiquidityCandidates(ctx context.Context, market string) ([]rankedMarketSymbol, error) {
	path := binancePath(market, "/api/v3/ticker/24hr", "/fapi/v1/ticker/24hr")
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.binanceBaseURL(market)+path, nil)
	var rows []struct {
		Symbol      string `json:"symbol"`
		QuoteVolume string `json:"quoteVolume"`
	}
	if err := doJSON(ctx, a.client, request, &rows); err != nil {
		return nil, err
	}
	result := make([]rankedMarketSymbol, 0, len(rows))
	for _, row := range rows {
		if !strings.HasSuffix(row.Symbol, "USDT") || strings.Contains(row.Symbol, "_") {
			continue
		}
		score, _ := strconv.ParseFloat(row.QuoteVolume, 64)
		if score > 0 {
			result = append(result, rankedMarketSymbol{Symbol: row.Symbol, Score: score})
		}
	}
	return result, nil
}

func (a *MarketData) okxLiquidityCandidates(ctx context.Context, market string) ([]rankedMarketSymbol, error) {
	instType := "SPOT"
	if market == schema.MarketTypeFutures {
		instType = "SWAP"
	}
	var rows []struct {
		InstID    string `json:"instId"`
		VolumeCcy string `json:"volCcy24h"`
		Volume    string `json:"vol24h"`
	}
	if err := a.okxGet(ctx, "/api/v5/market/tickers", url.Values{"instType": {instType}}, &rows); err != nil {
		return nil, err
	}
	result := make([]rankedMarketSymbol, 0, len(rows))
	for _, row := range rows {
		if market == schema.MarketTypeSpot && !strings.HasSuffix(row.InstID, "-USDT") {
			continue
		}
		if market == schema.MarketTypeFutures && !strings.HasSuffix(row.InstID, "-USDT-SWAP") {
			continue
		}
		score, _ := strconv.ParseFloat(row.VolumeCcy, 64)
		if score <= 0 {
			score, _ = strconv.ParseFloat(row.Volume, 64)
		}
		if score > 0 {
			result = append(result, rankedMarketSymbol{Symbol: runtimeCanonicalSymbol(row.InstID), Score: score})
		}
	}
	return result, nil
}

func (a *MarketData) binanceOpenInterestCandidates(ctx context.Context, limit int) ([]rankedMarketSymbol, error) {
	seedLimit := limit * 3
	if seedLimit < 12 {
		seedLimit = 12
	}
	if seedLimit > 30 {
		seedLimit = 30
	}
	liquid, err := a.binanceLiquidityCandidates(ctx, schema.MarketTypeFutures)
	if err != nil {
		return nil, err
	}
	seeds := topRankedSymbols(liquid, seedLimit)
	result := make([]rankedMarketSymbol, 0, len(seeds))
	for _, symbol := range seeds {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, a.binanceFuturesURL+"/fapi/v1/openInterest?"+
			url.Values{"symbol": {symbol}}.Encode(), nil)
		var response struct {
			Symbol       string `json:"symbol"`
			OpenInterest string `json:"openInterest"`
		}
		if err := doJSON(ctx, a.client, request, &response); err != nil {
			return nil, err
		}
		score, _ := strconv.ParseFloat(response.OpenInterest, 64)
		if score > 0 {
			result = append(result, rankedMarketSymbol{Symbol: response.Symbol, Score: score})
		}
	}
	return result, nil
}

func (a *MarketData) okxOpenInterestCandidates(ctx context.Context) ([]rankedMarketSymbol, error) {
	var rows []struct {
		InstID string `json:"instId"`
		OI     string `json:"oi"`
		OICcy  string `json:"oiCcy"`
	}
	if err := a.okxGet(ctx, "/api/v5/public/open-interest", url.Values{"instType": {"SWAP"}}, &rows); err != nil {
		return nil, err
	}
	result := make([]rankedMarketSymbol, 0, len(rows))
	for _, row := range rows {
		if !strings.HasSuffix(row.InstID, "-USDT-SWAP") {
			continue
		}
		score, _ := strconv.ParseFloat(row.OICcy, 64)
		if score <= 0 {
			score, _ = strconv.ParseFloat(row.OI, 64)
		}
		if score > 0 {
			result = append(result, rankedMarketSymbol{Symbol: runtimeCanonicalSymbol(row.InstID), Score: score})
		}
	}
	return result, nil
}

func topRankedSymbols(rows []rankedMarketSymbol, limit int) []string {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score == rows[j].Score {
			return rows[i].Symbol < rows[j].Symbol
		}
		return rows[i].Score > rows[j].Score
	})
	seen := make(map[string]struct{}, limit)
	result := make([]string, 0, limit)
	for _, row := range rows {
		symbol := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if symbol == "" {
			continue
		}
		if _, exists := seen[symbol]; exists {
			continue
		}
		seen[symbol] = struct{}{}
		result = append(result, symbol)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func runtimeCanonicalSymbol(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "-SWAP")
	return strings.ReplaceAll(value, "-", "")
}
