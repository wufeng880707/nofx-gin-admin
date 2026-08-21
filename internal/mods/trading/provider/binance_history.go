package provider

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const binanceHistoryLimit = 1000

type binanceOrderRow struct {
	Symbol             string `json:"symbol"`
	OrderID            int64  `json:"orderId"`
	ClientOrderID      string `json:"clientOrderId"`
	Price              string `json:"price"`
	AvgPrice           string `json:"avgPrice"`
	OrigQty            string `json:"origQty"`
	ExecutedQty        string `json:"executedQty"`
	CumQuote           string `json:"cumQuote"`
	CumulativeQuoteQty string `json:"cummulativeQuoteQty"`
	Status             string `json:"status"`
	OrderType          string `json:"type"`
	Side               string `json:"side"`
	PositionSide       string `json:"positionSide"`
	ReduceOnly         bool   `json:"reduceOnly"`
	Time               int64  `json:"time"`
	UpdateTime         int64  `json:"updateTime"`
}

type binanceTradeRow struct {
	Symbol          string `json:"symbol"`
	ID              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	RealizedPnL     string `json:"realizedPnl"`
	Side            string `json:"side"`
	PositionSide    string `json:"positionSide"`
	Time            int64  `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	Buyer           bool   `json:"buyer"`
	IsMaker         bool   `json:"isMaker"`
	Maker           bool   `json:"maker"`
}

type binanceIncomeRow struct {
	Symbol     string `json:"symbol"`
	IncomeType string `json:"incomeType"`
	Income     string `json:"income"`
	Asset      string `json:"asset"`
	Time       int64  `json:"time"`
	TranID     int64  `json:"tranId"`
	TradeID    string `json:"tradeId"`
}

func (a *Binance) History(ctx context.Context, request HistoryRequest) (*HistoryBatch, error) {
	until := request.Until.UTC()
	if until.IsZero() {
		until = a.now().UTC()
	}
	since := request.Since.UTC()
	if since.IsZero() || !since.Before(until) {
		since = until.Add(-7 * 24 * time.Hour)
	}
	batch := &HistoryBatch{SyncedUntil: until}
	for _, symbol := range request.Symbols {
		a.rememberSymbol(symbol)
	}
	if a.accountType == BinanceAccountTypeSpot {
		if err := a.loadSpotOpenOrders(ctx, batch); err != nil {
			return nil, err
		}
	} else if err := a.loadDerivativeIncome(ctx, since, until, batch); err != nil {
		return nil, err
	}

	symbols := make([]string, 0, len(a.symbols))
	for symbol := range a.symbols {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	if len(symbols) > 50 {
		symbols = symbols[:50]
	}
	for _, symbol := range symbols {
		if a.accountType == BinanceAccountTypeSpot {
			if err := a.loadSpotSymbolHistory(ctx, symbol, since, until, batch); err != nil {
				return nil, err
			}
			continue
		}
		if err := a.loadDerivativeSymbolHistory(ctx, symbol, since, until, batch); err != nil {
			return nil, err
		}
	}
	return batch, nil
}

func (a *Binance) loadSpotOpenOrders(ctx context.Context, batch *HistoryBatch) error {
	var rows []binanceOrderRow
	if err := a.signedGet(ctx, "/api/v3/openOrders", nil, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		a.rememberSymbol(row.Symbol)
		batch.Orders = append(batch.Orders, normalizeBinanceOrder(row, BinanceAccountTypeSpot))
	}
	return nil
}

func (a *Binance) loadSpotSymbolHistory(ctx context.Context, symbol string, since, until time.Time, batch *HistoryBatch) error {
	orders, err := loadBinanceTimeRange(ctx, since, until, 24*time.Hour, func(start, end time.Time) ([]binanceOrderRow, error) {
		query := url.Values{"symbol": {symbol}, "startTime": {millis(start)}, "endTime": {millis(end)}, "limit": {strconv.Itoa(binanceHistoryLimit)}}
		var orders []binanceOrderRow
		if err := a.signedGet(ctx, "/api/v3/allOrders", query, &orders); err != nil {
			return nil, fmt.Errorf("load Binance spot orders for %s: %w", symbol, err)
		}
		return orders, nil
	})
	if err != nil {
		return err
	}
	for _, row := range orders {
		batch.Orders = append(batch.Orders, normalizeBinanceOrder(row, BinanceAccountTypeSpot))
	}
	trades, err := loadBinanceTimeRange(ctx, since, until, 24*time.Hour, func(start, end time.Time) ([]binanceTradeRow, error) {
		query := url.Values{"symbol": {symbol}, "startTime": {millis(start)}, "endTime": {millis(end)}, "limit": {strconv.Itoa(binanceHistoryLimit)}}
		var trades []binanceTradeRow
		if err := a.signedGet(ctx, "/api/v3/myTrades", query, &trades); err != nil {
			return nil, fmt.Errorf("load Binance spot fills for %s: %w", symbol, err)
		}
		return trades, nil
	})
	if err != nil {
		return err
	}
	for _, row := range trades {
		batch.Fills = append(batch.Fills, normalizeBinanceFill(row, BinanceAccountTypeSpot))
	}
	return nil
}

func (a *Binance) loadDerivativeSymbolHistory(ctx context.Context, symbol string, since, until time.Time, batch *HistoryBatch) error {
	ordersPath, tradesPath, market := "/fapi/v1/allOrders", "/fapi/v1/userTrades", BinanceAccountTypeFutures
	if a.accountType == BinanceAccountTypeUnified {
		ordersPath, tradesPath, market = "/papi/v1/um/allOrders", "/papi/v1/um/userTrades", BinanceAccountTypeUnified
	}
	orders, err := loadBinanceTimeRange(ctx, since, until, 7*24*time.Hour, func(start, end time.Time) ([]binanceOrderRow, error) {
		query := url.Values{"symbol": {symbol}, "startTime": {millis(start)}, "endTime": {millis(end)}, "limit": {strconv.Itoa(binanceHistoryLimit)}}
		var orders []binanceOrderRow
		if err := a.signedGet(ctx, ordersPath, query, &orders); err != nil {
			return nil, fmt.Errorf("load Binance %s orders for %s: %w", market, symbol, err)
		}
		return orders, nil
	})
	if err != nil {
		return err
	}
	for _, row := range orders {
		batch.Orders = append(batch.Orders, normalizeBinanceOrder(row, market))
	}
	trades, err := loadBinanceTimeRange(ctx, since, until, 7*24*time.Hour, func(start, end time.Time) ([]binanceTradeRow, error) {
		query := url.Values{"symbol": {symbol}, "startTime": {millis(start)}, "endTime": {millis(end)}, "limit": {strconv.Itoa(binanceHistoryLimit)}}
		var trades []binanceTradeRow
		if err := a.signedGet(ctx, tradesPath, query, &trades); err != nil {
			return nil, fmt.Errorf("load Binance %s fills for %s: %w", market, symbol, err)
		}
		return trades, nil
	})
	if err != nil {
		return err
	}
	for _, row := range trades {
		batch.Fills = append(batch.Fills, normalizeBinanceFill(row, market))
	}
	return nil
}

func (a *Binance) loadDerivativeIncome(ctx context.Context, since, until time.Time, batch *HistoryBatch) error {
	path, market := "/fapi/v1/income", BinanceAccountTypeFutures
	if a.accountType == BinanceAccountTypeUnified {
		path, market = "/papi/v1/um/income", BinanceAccountTypeUnified
	}
	rows, err := loadBinanceTimeRange(ctx, since, until, 7*24*time.Hour, func(start, end time.Time) ([]binanceIncomeRow, error) {
		query := url.Values{"startTime": {millis(start)}, "endTime": {millis(end)}, "limit": {strconv.Itoa(binanceHistoryLimit)}}
		var rows []binanceIncomeRow
		if err := a.signedGet(ctx, path, query, &rows); err != nil {
			return nil, fmt.Errorf("load Binance %s income: %w", market, err)
		}
		return rows, nil
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		a.rememberSymbol(row.Symbol)
		value := "0"
		if isUSDStableAsset(row.Asset) {
			value = zeroDecimal(row.Income)
		}
		batch.IncomeEvents = append(batch.IncomeEvents, &IncomeEvent{Market: market,
			EventType: strings.ToLower(row.IncomeType), ExchangeEventID: strconv.FormatInt(row.TranID, 10),
			Symbol: strings.ToUpper(row.Symbol), Asset: strings.ToUpper(row.Asset), Amount: zeroDecimal(row.Income),
			PriceUSDT: stablePrice(row.Asset), ValueUSDT: value, OccurredAt: timeFromMillis(row.Time)})
	}
	return nil
}

func loadBinanceTimeRange[T any](ctx context.Context, since, until time.Time, window time.Duration,
	fetch func(time.Time, time.Time) ([]T, error)) ([]T, error) {
	if window <= 0 {
		return nil, fmt.Errorf("Binance history window must be positive")
	}
	result := make([]T, 0)
	for start := since; start.Before(until); {
		end := start.Add(window - time.Millisecond)
		if end.After(until) {
			end = until
		}
		rows, err := loadBinanceWindow(ctx, start, end, fetch)
		if err != nil {
			return nil, err
		}
		result = append(result, rows...)
		start = end.Add(time.Millisecond)
	}
	return result, nil
}

func loadBinanceWindow[T any](ctx context.Context, start, end time.Time,
	fetch func(time.Time, time.Time) ([]T, error)) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := fetch(start, end)
	if err != nil {
		return nil, err
	}
	if len(rows) < binanceHistoryLimit {
		return rows, nil
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("Binance history is saturated at millisecond %s; use ID-based recovery", millis(start))
	}
	durationMillis := end.Sub(start).Milliseconds()
	middle := start.Add(time.Duration(durationMillis/2) * time.Millisecond)
	if !middle.Before(end) {
		return nil, fmt.Errorf("Binance history range cannot be split further: %s-%s", millis(start), millis(end))
	}
	left, err := loadBinanceWindow(ctx, start, middle, fetch)
	if err != nil {
		return nil, err
	}
	right, err := loadBinanceWindow(ctx, middle.Add(time.Millisecond), end, fetch)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func normalizeBinanceOrder(row binanceOrderRow, market string) *Order {
	quote := row.CumQuote
	if strings.TrimSpace(quote) == "" {
		quote = row.CumulativeQuoteQty
	}
	average := row.AvgPrice
	if strings.TrimSpace(average) == "" || zeroDecimal(average) == "0" {
		average = decimalQuotient(quote, row.ExecutedQty)
	}
	updated := timeFromMillis(row.UpdateTime)
	ordered := timeFromMillis(row.Time)
	if updated.IsZero() {
		updated = ordered
	}
	return &Order{Market: market, ExchangeOrderID: strconv.FormatInt(row.OrderID, 10), ClientOrderID: row.ClientOrderID,
		Symbol: strings.ToUpper(row.Symbol), Side: strings.ToLower(row.Side), PositionSide: strings.ToLower(row.PositionSide),
		OrderType: strings.ToLower(row.OrderType), Status: strings.ToLower(row.Status), Price: zeroDecimal(row.Price),
		AveragePrice: zeroDecimal(average), Quantity: zeroDecimal(row.OrigQty), ExecutedQuantity: zeroDecimal(row.ExecutedQty),
		QuoteQuantity: zeroDecimal(quote), ReduceOnly: row.ReduceOnly, OrderedAt: ordered, ExchangeUpdatedAt: updated}
}

func normalizeBinanceFill(row binanceTradeRow, market string) *Fill {
	side := row.Side
	if strings.TrimSpace(side) == "" {
		if row.IsBuyer || row.Buyer {
			side = "buy"
		} else {
			side = "sell"
		}
	}
	feeValue := "0"
	if isUSDStableAsset(row.CommissionAsset) {
		feeValue = zeroDecimal(row.Commission)
	}
	liquidity := "taker"
	if row.IsMaker || row.Maker {
		liquidity = "maker"
	}
	return &Fill{Market: market, ExchangeTradeID: strconv.FormatInt(row.ID, 10), ExchangeOrderID: strconv.FormatInt(row.OrderID, 10),
		Symbol: strings.ToUpper(row.Symbol), Side: strings.ToLower(side), PositionSide: strings.ToLower(row.PositionSide),
		Price: zeroDecimal(row.Price), Quantity: zeroDecimal(row.Qty), QuoteQuantity: zeroDecimal(row.QuoteQty),
		FeeAsset: strings.ToUpper(row.CommissionAsset), FeeQuantity: zeroDecimal(row.Commission), FeeValueUSDT: feeValue,
		RealizedPnL: zeroDecimal(row.RealizedPnL), Liquidity: liquidity, FilledAt: timeFromMillis(row.Time)}
}

func millis(value time.Time) string { return strconv.FormatInt(value.UnixMilli(), 10) }

func timeFromMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func stablePrice(asset string) string {
	if isUSDStableAsset(asset) {
		return "1"
	}
	return "0"
}

func decimalQuotient(numerator, denominator string) string {
	n, nOK := new(big.Rat).SetString(strings.TrimSpace(numerator))
	d, dOK := new(big.Rat).SetString(strings.TrimSpace(denominator))
	if !nOK || !dOK || d.Sign() == 0 {
		return "0"
	}
	return formatDecimal(new(big.Rat).Quo(n, d))
}
