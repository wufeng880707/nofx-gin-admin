package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

type MarketData struct {
	client            *http.Client
	binanceSpotURL    string
	binanceFuturesURL string
	okxURL            string
}

func NewMarketData(client *http.Client) *MarketData {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &MarketData{client: client, binanceSpotURL: "https://api.binance.com",
		binanceFuturesURL: "https://fapi.binance.com", okxURL: "https://www.okx.com"}
}

func (a *Factory) MarketData() *MarketData {
	if a == nil {
		return NewMarketData(nil)
	}
	return NewMarketData(a.HTTPClient)
}

func (a *MarketData) Instruments(ctx context.Context, exchange, market string) ([]*schema.MarketInstrument, error) {
	switch exchange {
	case schema.MarketExchangeBinance:
		return a.binanceInstruments(ctx, market)
	case schema.MarketExchangeOKX:
		return a.okxInstruments(ctx, market)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
}

func (a *MarketData) Ticker(ctx context.Context, exchange, market, symbol string) (*schema.MarketTicker, error) {
	switch exchange {
	case schema.MarketExchangeBinance:
		return a.binanceTicker(ctx, market, symbol)
	case schema.MarketExchangeOKX:
		return a.okxTicker(ctx, market, symbol)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
}

func (a *MarketData) Klines(ctx context.Context, exchange, market, symbol, interval string, limit int) ([]*schema.MarketKline, error) {
	switch exchange {
	case schema.MarketExchangeBinance:
		return a.binanceKlines(ctx, market, symbol, interval, limit)
	case schema.MarketExchangeOKX:
		return a.okxKlines(ctx, market, symbol, interval, limit)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
}

// KlinesRange loads closed candles in chronological order. Public exchange
// endpoints are paginated and deduplicated so callers can run deterministic
// backtests over more than the recent-candle limit.
func (a *MarketData) KlinesRange(ctx context.Context, exchange, market, symbol, interval string, start, end time.Time) ([]*schema.MarketKline, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("invalid kline time range")
	}
	switch exchange {
	case schema.MarketExchangeBinance:
		return a.binanceKlinesRange(ctx, market, symbol, interval, start, end)
	case schema.MarketExchangeOKX:
		return a.okxKlinesRange(ctx, market, symbol, interval, start, end)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
}

func (a *MarketData) binanceKlinesRange(ctx context.Context, market, symbol, interval string, start, end time.Time) ([]*schema.MarketKline, error) {
	path := binancePath(market, "/api/v3/klines", "/fapi/v1/klines")
	cursor := start.UTC().UnixMilli()
	endMillis := end.UTC().UnixMilli()
	result := make([]*schema.MarketKline, 0, 2048)
	seen := make(map[int64]struct{})
	for cursor < endMillis {
		query := url.Values{"symbol": {strings.ReplaceAll(symbol, "-", "")}, "interval": {interval}, "limit": {"1000"},
			"startTime": {strconv.FormatInt(cursor, 10)}, "endTime": {strconv.FormatInt(endMillis, 10)}}
		request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+query.Encode(), nil)
		var rows [][]json.RawMessage
		if err := doJSON(ctx, a.client, request, &rows); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		lastOpen := cursor
		for _, row := range rows {
			if len(row) < 8 {
				continue
			}
			openMillis, err := rawInt64(row[0])
			if err != nil {
				return nil, fmt.Errorf("decode Binance kline time: %w", err)
			}
			if openMillis < start.UnixMilli() || openMillis > endMillis {
				continue
			}
			closeMillis, _ := rawInt64(row[6])
			timestamp := openMillis / 1000
			if _, ok := seen[timestamp]; ok {
				continue
			}
			seen[timestamp] = struct{}{}
			result = append(result, &schema.MarketKline{Time: timestamp, Open: rawString(row[1]), High: rawString(row[2]),
				Low: rawString(row[3]), Close: rawString(row[4]), Volume: rawString(row[5]), QuoteVolume: rawString(row[7]),
				Closed: closeMillis < time.Now().UnixMilli()})
			if openMillis > lastOpen {
				lastOpen = openMillis
			}
		}
		if lastOpen <= cursor || len(rows) < 1000 {
			break
		}
		cursor = lastOpen + 1
		if len(result) > 50000 {
			return nil, fmt.Errorf("kline range exceeds 50000 data points")
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result, nil
}

func (a *MarketData) okxKlinesRange(ctx context.Context, market, symbol, interval string, start, end time.Time) ([]*schema.MarketKline, error) {
	bar := map[string]string{"1m": "1m", "5m": "5m", "15m": "15m", "1h": "1H", "4h": "4H", "1d": "1D", "1w": "1W"}[interval]
	if bar == "" {
		return nil, fmt.Errorf("unsupported OKX interval %q", interval)
	}
	cursor := end.UTC().UnixMilli() + 1
	startMillis := start.UTC().UnixMilli()
	endMillis := end.UTC().UnixMilli()
	result := make([]*schema.MarketKline, 0, 2048)
	seen := make(map[int64]struct{})
	for cursor > startMillis {
		var rows [][]json.RawMessage
		query := url.Values{"instId": {symbol}, "bar": {bar}, "limit": {"100"}, "after": {strconv.FormatInt(cursor, 10)}}
		if err := a.okxGet(ctx, "/api/v5/market/history-candles", query, &rows); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		oldest := cursor
		for _, row := range rows {
			if len(row) < 9 {
				continue
			}
			openMillis, err := strconv.ParseInt(rawString(row[0]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("decode OKX kline time: %w", err)
			}
			if openMillis < oldest {
				oldest = openMillis
			}
			if openMillis < startMillis || openMillis > endMillis {
				continue
			}
			timestamp := openMillis / 1000
			if _, ok := seen[timestamp]; ok {
				continue
			}
			seen[timestamp] = struct{}{}
			result = append(result, &schema.MarketKline{Time: timestamp, Open: rawString(row[1]), High: rawString(row[2]),
				Low: rawString(row[3]), Close: rawString(row[4]), Volume: rawString(row[5]), QuoteVolume: rawString(row[7]),
				Closed: rawString(row[8]) == "1"})
		}
		if oldest >= cursor || oldest <= startMillis || len(rows) < 100 {
			break
		}
		cursor = oldest
		if len(result) > 50000 {
			return nil, fmt.Errorf("kline range exceeds 50000 data points")
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result, nil
}

// InstrumentRules loads public exchange filters used by the command risk
// validator. It has no credential and no trading side effects.
func (a *MarketData) InstrumentRules(ctx context.Context, exchange, market, symbol string) (*InstrumentRules, error) {
	switch exchange {
	case schema.MarketExchangeBinance:
		return a.binanceInstrumentRules(ctx, market, symbol)
	case schema.MarketExchangeOKX:
		return a.okxInstrumentRules(ctx, market, symbol)
	default:
		return nil, fmt.Errorf("unsupported market exchange %q", exchange)
	}
}

func (a *MarketData) binanceInstrumentRules(ctx context.Context, market, symbol string) (*InstrumentRules, error) {
	path := binancePath(market, "/api/v3/exchangeInfo", "/fapi/v1/exchangeInfo")
	request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+
		url.Values{"symbol": {strings.ReplaceAll(symbol, "-", "")}}.Encode(), nil)
	var response struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Filters []struct {
				Type        string `json:"filterType"`
				MinQty      string `json:"minQty"`
				StepSize    string `json:"stepSize"`
				TickSize    string `json:"tickSize"`
				MinNotional string `json:"minNotional"`
				Notional    string `json:"notional"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return nil, err
	}
	if len(response.Symbols) != 1 {
		return nil, fmt.Errorf("Binance instrument %s not found", symbol)
	}
	rules := &InstrumentRules{Symbol: response.Symbols[0].Symbol}
	for _, filter := range response.Symbols[0].Filters {
		switch filter.Type {
		case "LOT_SIZE":
			rules.MinQuantity, rules.StepSize = filter.MinQty, filter.StepSize
		case "PRICE_FILTER":
			rules.TickSize = filter.TickSize
		case "MIN_NOTIONAL", "NOTIONAL":
			rules.MinNotional = filter.MinNotional
			if rules.MinNotional == "" {
				rules.MinNotional = filter.Notional
			}
		}
	}
	return rules, nil
}

func (a *MarketData) okxInstrumentRules(ctx context.Context, market, symbol string) (*InstrumentRules, error) {
	instType := "SPOT"
	if market == schema.MarketTypeFutures {
		instType = "SWAP"
	}
	var rows []struct {
		InstID string `json:"instId"`
		MinSz  string `json:"minSz"`
		LotSz  string `json:"lotSz"`
		TickSz string `json:"tickSz"`
	}
	if err := a.okxGet(ctx, "/api/v5/public/instruments", url.Values{"instType": {instType}, "instId": {symbol}}, &rows); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("OKX instrument %s not found", symbol)
	}
	return &InstrumentRules{Symbol: rows[0].InstID, MinQuantity: rows[0].MinSz,
		StepSize: rows[0].LotSz, TickSize: rows[0].TickSz}, nil
}

func (a *MarketData) binanceBaseURL(market string) string {
	if market == schema.MarketTypeSpot {
		return a.binanceSpotURL
	}
	return a.binanceFuturesURL
}

func binancePath(market, spotPath, futuresPath string) string {
	if market == schema.MarketTypeSpot {
		return spotPath
	}
	return futuresPath
}

func (a *MarketData) binanceInstruments(ctx context.Context, market string) ([]*schema.MarketInstrument, error) {
	if market == schema.MarketTypeSpot {
		return a.binanceSpotInstruments(ctx)
	}

	path := binancePath(market, "/api/v3/exchangeInfo", "/fapi/v1/exchangeInfo")
	request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path, nil)
	var response struct {
		Symbols []struct {
			Symbol, Status, BaseAsset, QuoteAsset string
		} `json:"symbols"`
	}
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return nil, err
	}
	result := make([]*schema.MarketInstrument, 0, len(response.Symbols))
	for _, item := range response.Symbols {
		if item.Status != "TRADING" || !strings.EqualFold(item.QuoteAsset, "USDT") {
			continue
		}
		result = append(result, &schema.MarketInstrument{Exchange: schema.MarketExchangeBinance, Market: market,
			Symbol: item.Symbol, BaseAsset: item.BaseAsset, QuoteAsset: item.QuoteAsset,
			Display: item.BaseAsset + "/" + item.QuoteAsset})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result, nil
}

// The spot exchangeInfo payload contains every filter for every symbol and can
// exceed the limits of some local proxy paths. MINI tickers are much smaller
// and contain the currently listed symbols needed by the selector.
func (a *MarketData) binanceSpotInstruments(ctx context.Context) ([]*schema.MarketInstrument, error) {
	query := url.Values{"type": {"MINI"}}
	request, _ := http.NewRequest(http.MethodGet, a.binanceSpotURL+"/api/v3/ticker/24hr?"+query.Encode(), nil)
	var response []struct {
		Symbol string `json:"symbol"`
	}
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return nil, err
	}
	result := make([]*schema.MarketInstrument, 0, len(response))
	for _, item := range response {
		if !strings.HasSuffix(item.Symbol, "USDT") || len(item.Symbol) <= len("USDT") {
			continue
		}
		base := strings.TrimSuffix(item.Symbol, "USDT")
		result = append(result, &schema.MarketInstrument{
			Exchange:   schema.MarketExchangeBinance,
			Market:     schema.MarketTypeSpot,
			Symbol:     item.Symbol,
			BaseAsset:  base,
			QuoteAsset: "USDT",
			Display:    base + "/USDT",
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result, nil
}

func (a *MarketData) binanceTicker(ctx context.Context, market, symbol string) (*schema.MarketTicker, error) {
	path := binancePath(market, "/api/v3/ticker/24hr", "/fapi/v1/ticker/24hr")
	query := url.Values{"symbol": {strings.ReplaceAll(symbol, "-", "")}}
	request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+query.Encode(), nil)
	var response struct {
		Symbol, LastPrice, OpenPrice, HighPrice, LowPrice    string
		PriceChange, PriceChangePercent, Volume, QuoteVolume string
		CloseTime                                            int64 `json:"closeTime"`
	}
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return nil, err
	}
	return &schema.MarketTicker{Exchange: schema.MarketExchangeBinance, Market: market, Symbol: response.Symbol,
		LastPrice: response.LastPrice, OpenPrice: response.OpenPrice, HighPrice: response.HighPrice,
		LowPrice: response.LowPrice, PriceChange: response.PriceChange, PriceChangeRate: response.PriceChangePercent,
		BaseVolume: response.Volume, QuoteVolume: response.QuoteVolume, Timestamp: response.CloseTime}, nil
}

func (a *MarketData) binanceKlines(ctx context.Context, market, symbol, interval string, limit int) ([]*schema.MarketKline, error) {
	path := binancePath(market, "/api/v3/klines", "/fapi/v1/klines")
	query := url.Values{"symbol": {strings.ReplaceAll(symbol, "-", "")}, "interval": {interval}, "limit": {strconv.Itoa(limit)}}
	request, _ := http.NewRequest(http.MethodGet, a.binanceBaseURL(market)+path+"?"+query.Encode(), nil)
	var rows [][]json.RawMessage
	if err := doJSON(ctx, a.client, request, &rows); err != nil {
		return nil, err
	}
	result := make([]*schema.MarketKline, 0, len(rows))
	for _, row := range rows {
		if len(row) < 8 {
			continue
		}
		openTime, err := rawInt64(row[0])
		if err != nil {
			return nil, fmt.Errorf("decode Binance kline time: %w", err)
		}
		closeTime, _ := rawInt64(row[6])
		result = append(result, &schema.MarketKline{Time: openTime / 1000, Open: rawString(row[1]),
			High: rawString(row[2]), Low: rawString(row[3]), Close: rawString(row[4]), Volume: rawString(row[5]),
			QuoteVolume: rawString(row[7]), Closed: closeTime < time.Now().UnixMilli()})
	}
	return result, nil
}

type okxMarketResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (a *MarketData) okxGet(ctx context.Context, path string, query url.Values, output interface{}) error {
	request, _ := http.NewRequest(http.MethodGet, a.okxURL+path+"?"+query.Encode(), nil)
	var response okxMarketResponse
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return err
	}
	if response.Code != "0" {
		return fmt.Errorf("OKX market error %s: %s", response.Code, response.Msg)
	}
	return json.Unmarshal(response.Data, output)
}

func (a *MarketData) okxInstruments(ctx context.Context, market string) ([]*schema.MarketInstrument, error) {
	instType := "SPOT"
	if market == schema.MarketTypeFutures {
		instType = "SWAP"
	}
	var rows []struct {
		InstID, State, BaseCcy, QuoteCcy, SettleCcy string
	}
	if err := a.okxGet(ctx, "/api/v5/public/instruments", url.Values{"instType": {instType}}, &rows); err != nil {
		return nil, err
	}
	result := make([]*schema.MarketInstrument, 0, len(rows))
	for _, item := range rows {
		quote := item.QuoteCcy
		base := item.BaseCcy
		if market == schema.MarketTypeFutures {
			quote = item.SettleCcy
			base = strings.TrimSuffix(strings.TrimSuffix(item.InstID, "-SWAP"), "-"+quote)
		}
		if item.State != "live" || !strings.EqualFold(quote, "USDT") {
			continue
		}
		result = append(result, &schema.MarketInstrument{Exchange: schema.MarketExchangeOKX, Market: market,
			Symbol: item.InstID, BaseAsset: base, QuoteAsset: quote, Display: base + "/" + quote})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result, nil
}

func (a *MarketData) okxTicker(ctx context.Context, market, symbol string) (*schema.MarketTicker, error) {
	var rows []struct {
		InstID, Last, Open24h, High24h, Low24h string
		Vol24h, VolCcy24h, VolCcyQuote24h, Ts  string
	}
	if err := a.okxGet(ctx, "/api/v5/market/ticker", url.Values{"instId": {symbol}}, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("OKX ticker not found")
	}
	item := rows[0]
	change, changeRate := decimalChange(item.Last, item.Open24h)
	timestamp, _ := strconv.ParseInt(item.Ts, 10, 64)
	baseVolume, quoteVolume := item.Vol24h, item.VolCcy24h
	if market == schema.MarketTypeFutures {
		baseVolume = item.VolCcy24h
		quoteVolume = item.VolCcyQuote24h
	}
	return &schema.MarketTicker{Exchange: schema.MarketExchangeOKX, Market: market, Symbol: item.InstID,
		LastPrice: item.Last, OpenPrice: item.Open24h, HighPrice: item.High24h, LowPrice: item.Low24h,
		PriceChange: change, PriceChangeRate: changeRate, BaseVolume: baseVolume,
		QuoteVolume: quoteVolume, Timestamp: timestamp}, nil
}

func (a *MarketData) okxKlines(ctx context.Context, market, symbol, interval string, limit int) ([]*schema.MarketKline, error) {
	bar := map[string]string{"1m": "1m", "5m": "5m", "15m": "15m", "1h": "1H", "4h": "4H", "1d": "1D", "1w": "1W"}[interval]
	var rows [][]json.RawMessage
	if err := a.okxGet(ctx, "/api/v5/market/candles", url.Values{"instId": {symbol}, "bar": {bar}, "limit": {strconv.Itoa(limit)}}, &rows); err != nil {
		return nil, err
	}
	result := make([]*schema.MarketKline, 0, len(rows))
	for index := len(rows) - 1; index >= 0; index-- {
		row := rows[index]
		if len(row) < 9 {
			continue
		}
		timestamp, err := strconv.ParseInt(rawString(row[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("decode OKX kline time: %w", err)
		}
		result = append(result, &schema.MarketKline{Time: timestamp / 1000, Open: rawString(row[1]),
			High: rawString(row[2]), Low: rawString(row[3]), Close: rawString(row[4]), Volume: rawString(row[5]),
			QuoteVolume: rawString(row[7]), Closed: rawString(row[8]) == "1"})
	}
	return result, nil
}

func rawString(value json.RawMessage) string {
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}
	return strings.Trim(string(value), `"`)
}

func rawInt64(value json.RawMessage) (int64, error) {
	return strconv.ParseInt(rawString(value), 10, 64)
}

func decimalChange(last, open string) (string, string) {
	lastValue, lastOK := new(big.Rat).SetString(last)
	openValue, openOK := new(big.Rat).SetString(open)
	if !lastOK || !openOK || openValue.Sign() == 0 {
		return "0", "0"
	}
	change := new(big.Rat).Sub(lastValue, openValue)
	rate := new(big.Rat).Mul(new(big.Rat).Quo(change, openValue), big.NewRat(100, 1))
	rateText := strings.TrimRight(strings.TrimRight(rate.FloatString(6), "0"), ".")
	if rateText == "" || rateText == "-0" {
		rateText = "0"
	}
	return formatDecimal(change), rateText
}
