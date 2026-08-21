package biz

import (
	"bytes"
	"context"
	"encoding/csv"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

type Analytics struct {
	AnalyticsDAL *dal.Analytics
}

func (a *Analytics) QueryOverview(ctx context.Context, params schema.AnalyticsQueryParam) (*schema.DashboardOverview, error) {
	if err := a.resolveScope(ctx, &params); err != nil {
		return nil, err
	}
	result, err := a.AnalyticsDAL.QueryOverview(ctx, params)
	if err != nil {
		return nil, err
	}
	result.NetPnL = sumDecimal(result.RealizedPnL, result.UnrealizedPnL, result.Funding, negateDecimal(result.TotalFee))
	return result, nil
}

func (a *Analytics) QueryClosedTrades(ctx context.Context, params schema.AnalyticsQueryParam) (*schema.ClosedTradeQueryResult, error) {
	if err := a.resolveScope(ctx, &params); err != nil {
		return nil, err
	}
	params.Pagination = true
	return a.AnalyticsDAL.QueryClosedTrades(ctx, params)
}

func (a *Analytics) QueryEquityCurve(ctx context.Context, params schema.AnalyticsQueryParam) ([]*schema.EquityPoint, error) {
	if err := a.resolveScope(ctx, &params); err != nil {
		return nil, err
	}
	snapshots, err := a.AnalyticsDAL.ListEquityPoints(ctx, params)
	if err != nil {
		return nil, err
	}
	return aggregateEquityPoints(snapshots), nil
}

func (a *Analytics) QueryPerformance(ctx context.Context, params schema.AnalyticsQueryParam) (*schema.PerformanceSummary, error) {
	if err := a.resolveScope(ctx, &params); err != nil {
		return nil, err
	}
	trades, err := a.AnalyticsDAL.ListClosedTrades(ctx, params)
	if err != nil {
		return nil, err
	}
	funding, err := a.AnalyticsDAL.QueryFunding(ctx, params)
	if err != nil {
		return nil, err
	}
	snapshots, err := a.AnalyticsDAL.ListEquityPoints(ctx, params)
	if err != nil {
		return nil, err
	}
	return calculatePerformance(trades, funding, aggregateEquityPoints(snapshots)), nil
}

func (a *Analytics) QueryPerformanceReport(ctx context.Context, params schema.PerformanceReportQueryParam) ([]*schema.PerformanceReportBucket, error) {
	if params.Period == "" {
		params.Period = "day"
	}
	if params.Period != "day" && params.Period != "week" && params.Period != "month" {
		return nil, errors.BadRequest("trading_report_period_invalid", "Report period must be day, week, or month")
	}
	if params.EndAt == nil {
		now := time.Now().UTC()
		params.EndAt = &now
	}
	if params.StartAt == nil {
		start := params.EndAt.Add(-90 * 24 * time.Hour)
		params.StartAt = &start
	}
	if params.StartAt.After(*params.EndAt) || params.EndAt.Sub(*params.StartAt) > 2*366*24*time.Hour {
		return nil, errors.BadRequest("trading_report_range_invalid", "Report range must be ordered and no longer than two years")
	}
	if err := a.resolveScope(ctx, &params.AnalyticsQueryParam); err != nil {
		return nil, err
	}
	trades, err := a.AnalyticsDAL.ListClosedTrades(ctx, params.AnalyticsQueryParam)
	if err != nil {
		return nil, err
	}
	funding, err := a.AnalyticsDAL.ListFundingEvents(ctx, params.AnalyticsQueryParam)
	if err != nil {
		return nil, err
	}
	return buildPerformanceReport(trades, funding, params.Period), nil
}

func (a *Analytics) ExportPerformanceReport(ctx context.Context, params schema.PerformanceReportQueryParam) ([]byte, error) {
	report, err := a.QueryPerformanceReport(ctx, params)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write([]string{"period", "start_at", "end_at", "trades", "win_rate", "gross_pnl", "fees", "funding", "net_pnl", "max_drawdown_pct"}); err != nil {
		return nil, err
	}
	for _, bucket := range report {
		row := []string{bucket.Period, bucket.StartAt.Format(time.RFC3339), bucket.EndAt.Format(time.RFC3339),
			strconv.FormatInt(bucket.Summary.TotalTrades, 10), strconv.FormatFloat(bucket.Summary.WinRate, 'f', 4, 64),
			bucket.Summary.GrossPnL, bucket.Summary.TotalFee, bucket.Summary.Funding, bucket.Summary.NetPnL,
			strconv.FormatFloat(bucket.Summary.MaxDrawdownPct, 'f', 4, 64)}
		for index := range row {
			row[index] = safeCSVCell(row[index])
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func buildPerformanceReport(trades []*schema.ClosedTrade, funding []*schema.IncomeEvent, period string) []*schema.PerformanceReportBucket {
	type bucketData struct {
		start   time.Time
		trades  []*schema.ClosedTrade
		funding *big.Rat
	}
	buckets := make(map[time.Time]*bucketData)
	get := func(at time.Time) *bucketData {
		start := reportBucketStart(at, period)
		if buckets[start] == nil {
			buckets[start] = &bucketData{start: start, funding: new(big.Rat)}
		}
		return buckets[start]
	}
	for _, trade := range trades {
		bucket := get(trade.ClosedAt)
		bucket.trades = append(bucket.trades, trade)
	}
	for _, item := range funding {
		bucket := get(item.OccurredAt)
		bucket.funding.Add(bucket.funding, ratOrZero(item.ValueUSDT))
	}
	keys := make([]time.Time, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	result := make([]*schema.PerformanceReportBucket, 0, len(keys))
	for _, key := range keys {
		item := buckets[key]
		result = append(result, &schema.PerformanceReportBucket{Period: period, StartAt: item.start,
			EndAt: reportBucketEnd(item.start, period), Summary: calculatePerformance(item.trades, ratString(item.funding), nil)})
	}
	return result
}

func reportBucketStart(at time.Time, period string) time.Time {
	at = at.UTC()
	switch period {
	case "month":
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "week":
		day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	default:
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	}
}

func reportBucketEnd(start time.Time, period string) time.Time {
	switch period {
	case "month":
		return start.AddDate(0, 1, 0)
	case "week":
		return start.AddDate(0, 0, 7)
	default:
		return start.AddDate(0, 0, 1)
	}
}

func safeCSVCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@", rune(value[0])) {
		return "'" + value
	}
	return value
}

func (a *Analytics) resolveScope(ctx context.Context, params *schema.AnalyticsQueryParam) error {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return err
	}
	params.OwnerUserID = owner
	if strings.TrimSpace(params.TraderID) == "" {
		return nil
	}
	exchangeID, err := a.AnalyticsDAL.ResolveTraderExchange(ctx, owner, params.TraderID)
	if err != nil {
		return errors.BadRequest("trading_analytics_trader_not_found", "Trader not found")
	}
	if params.ExchangeID != "" && params.ExchangeID != exchangeID {
		return errors.BadRequest("trading_analytics_scope_conflict", "Trader and exchange filters do not match")
	}
	params.ExchangeID = exchangeID
	return nil
}

type performanceGroupAccumulator struct {
	total int64
	wins  int64
	net   *big.Rat
}

func calculatePerformance(trades []*schema.ClosedTrade, funding string, equity []*schema.EquityPoint) *schema.PerformanceSummary {
	result := &schema.PerformanceSummary{SharpeBasis: "closed_trade_return", DrawdownSource: "account_snapshots"}
	grossPnL, fees, tradeNet := new(big.Rat), new(big.Rat), new(big.Rat)
	grossProfit, grossLoss := new(big.Rat), new(big.Rat)
	returns := make([]float64, 0, len(trades))
	bySide := make(map[string]*performanceGroupAccumulator)
	bySymbol := make(map[string]*performanceGroupAccumulator)
	byTrader := make(map[string]*performanceGroupAccumulator)
	byStrategy := make(map[string]*performanceGroupAccumulator)
	byExchange := make(map[string]*performanceGroupAccumulator)
	for _, trade := range trades {
		gross := ratOrZero(trade.GrossPnL)
		fee := ratOrZero(trade.Fee)
		net := ratOrZero(trade.NetPnL)
		grossPnL.Add(grossPnL, gross)
		fees.Add(fees, fee)
		tradeNet.Add(tradeNet, net)
		result.TotalTrades++
		switch net.Sign() {
		case 1:
			result.WinningTrades++
			grossProfit.Add(grossProfit, net)
		case -1:
			result.LosingTrades++
			grossLoss.Add(grossLoss, new(big.Rat).Neg(net))
		default:
			result.BreakEvenTrades++
		}
		entryValue := new(big.Rat).Mul(ratOrZero(trade.EntryPrice), ratOrZero(trade.EntryQuantity))
		if entryValue.Sign() > 0 {
			value, _ := new(big.Rat).Quo(net, entryValue).Float64()
			returns = append(returns, value)
		}
		addPerformanceGroup(bySide, strings.ToLower(trade.Side), net)
		addPerformanceGroup(bySymbol, strings.ToUpper(trade.Symbol), net)
		addPerformanceGroup(byTrader, fallbackKey(trade.TraderID), net)
		strategyKey := fallbackKey(trade.StrategyID)
		if trade.StrategyVersion > 0 {
			strategyKey += ":v" + strconv.Itoa(trade.StrategyVersion)
		}
		addPerformanceGroup(byStrategy, strategyKey, net)
		addPerformanceGroup(byExchange, fallbackKey(trade.ExchangeID), net)
	}
	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades) * 100
	}
	if result.WinningTrades > 0 {
		result.AverageWin = ratString(new(big.Rat).Quo(grossProfit, big.NewRat(result.WinningTrades, 1)))
	} else {
		result.AverageWin = "0"
	}
	if result.LosingTrades > 0 {
		result.AverageLoss = ratString(new(big.Rat).Quo(grossLoss, big.NewRat(result.LosingTrades, 1)))
	} else {
		result.AverageLoss = "0"
	}
	if grossLoss.Sign() > 0 {
		result.ProfitFactor, _ = new(big.Rat).Quo(grossProfit, grossLoss).Float64()
	}
	avgLoss := floatDecimal(result.AverageLoss)
	if avgLoss > 0 {
		result.ProfitLossRatio = floatDecimal(result.AverageWin) / avgLoss
	}
	result.GrossProfit = ratString(grossProfit)
	result.GrossLoss = ratString(grossLoss)
	result.GrossPnL = ratString(grossPnL)
	result.TotalFee = ratString(fees)
	result.Funding = ratString(ratOrZero(funding))
	result.NetPnL = ratString(new(big.Rat).Add(tradeNet, ratOrZero(funding)))
	result.TradeSharpe = sampleSharpe(returns)
	result.MaxDrawdownPct = equityMaxDrawdown(equity)
	result.BySide = finishPerformanceGroups(bySide)
	result.BySymbol = finishPerformanceGroups(bySymbol)
	result.ByTrader = finishPerformanceGroups(byTrader)
	result.ByStrategy = finishPerformanceGroups(byStrategy)
	result.ByExchange = finishPerformanceGroups(byExchange)
	return result
}

func fallbackKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unassigned"
	}
	return value
}

func addPerformanceGroup(groups map[string]*performanceGroupAccumulator, key string, net *big.Rat) {
	group := groups[key]
	if group == nil {
		group = &performanceGroupAccumulator{net: new(big.Rat)}
		groups[key] = group
	}
	group.total++
	if net.Sign() > 0 {
		group.wins++
	}
	group.net.Add(group.net, net)
}

func finishPerformanceGroups(groups map[string]*performanceGroupAccumulator) []*schema.PerformanceGroup {
	result := make([]*schema.PerformanceGroup, 0, len(groups))
	for key, group := range groups {
		item := &schema.PerformanceGroup{Key: key, TotalTrades: group.total, WinningTrades: group.wins, NetPnL: ratString(group.net)}
		if group.total > 0 {
			item.WinRate = float64(group.wins) / float64(group.total) * 100
			item.AveragePnL = ratString(new(big.Rat).Quo(group.net, big.NewRat(group.total, 1)))
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return floatDecimal(result[i].NetPnL) > floatDecimal(result[j].NetPnL) })
	return result
}

func aggregateEquityPoints(snapshots []*schema.AccountSnapshot) []*schema.EquityPoint {
	type snapshotKey struct {
		at         time.Time
		exchangeID string
	}
	latest := make(map[snapshotKey]*schema.AccountSnapshot)
	for _, item := range snapshots {
		key := snapshotKey{at: item.CapturedAt.UTC().Truncate(12 * time.Hour), exchangeID: item.ExchangeID}
		if current := latest[key]; current == nil || item.CapturedAt.After(current.CapturedAt) {
			latest[key] = item
		}
	}

	type bucket struct {
		at                           time.Time
		total, available, unrealized *big.Rat
		positions                    int
	}
	buckets := make(map[time.Time]*bucket)
	for key, item := range latest {
		at := key.at
		current := buckets[at]
		if current == nil {
			current = &bucket{at: at, total: new(big.Rat), available: new(big.Rat), unrealized: new(big.Rat)}
			buckets[at] = current
		}
		current.total.Add(current.total, ratOrZero(item.TotalEquity))
		current.available.Add(current.available, ratOrZero(item.AvailableBalance))
		current.unrealized.Add(current.unrealized, ratOrZero(item.UnrealizedPnL))
		current.positions += item.PositionCount
	}
	keys := make([]time.Time, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	result := make([]*schema.EquityPoint, 0, len(keys))
	for _, key := range keys {
		item := buckets[key]
		result = append(result, &schema.EquityPoint{CapturedAt: item.at, TotalEquity: ratString(item.total),
			AvailableBalance: ratString(item.available), UnrealizedPnL: ratString(item.unrealized), PositionCount: item.positions})
	}
	return result
}

func sampleSharpe(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	var variance float64
	for _, value := range values {
		variance += math.Pow(value-mean, 2)
	}
	deviation := math.Sqrt(variance / float64(len(values)-1))
	if deviation == 0 {
		return 0
	}
	return mean / deviation
}

func equityMaxDrawdown(points []*schema.EquityPoint) float64 {
	peak, maximum := 0.0, 0.0
	for _, point := range points {
		equity := floatDecimal(point.TotalEquity)
		if equity <= 0 {
			continue
		}
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			maximum = math.Max(maximum, (peak-equity)/peak*100)
		}
	}
	return maximum
}

func ratOrZero(value string) *big.Rat {
	result, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return new(big.Rat)
	}
	return result
}

func ratString(value *big.Rat) string {
	if value == nil || value.Sign() == 0 {
		return "0"
	}
	result := strings.TrimRight(strings.TrimRight(value.FloatString(18), "0"), ".")
	if result == "-0" {
		return "0"
	}
	return result
}

func floatDecimal(value string) float64 {
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

func sumDecimal(values ...string) string {
	total := new(big.Rat)
	for _, value := range values {
		total.Add(total, ratOrZero(value))
	}
	return ratString(total)
}

func negateDecimal(value string) string {
	return ratString(new(big.Rat).Neg(ratOrZero(value)))
}
