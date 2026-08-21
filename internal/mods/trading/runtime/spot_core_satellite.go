package runtime

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

type spotPendingAction struct {
	side      string
	bucket    string
	layer     int
	quote     float64
	lotIDs    []string
	sellRatio float64
	maxQty    float64
	reason    string
}

type spotEngineLot struct {
	schema.SpotBacktestLot
	unitCost float64
}

// RunSpotCoreSatelliteBacktest runs a deterministic, next-bar-open simulation.
// It never creates a TradeCommand and is safe to use with production market data.
func RunSpotCoreSatelliteBacktest(config schema.SpotCoreSatelliteConfig, klines []*schema.MarketKline, initialCapital float64) (*schema.SpotBacktestOutput, error) {
	config.Normalize()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if initialCapital <= 0 {
		return nil, fmt.Errorf("initial capital must be positive")
	}
	bars, err := normalizeSpotBacktestBars(klines)
	if err != nil {
		return nil, err
	}
	minimumBars := maxInt(config.LowAreaLookbackBars, config.TrendFilterPeriod) + 2
	if len(bars) < minimumBars {
		return nil, fmt.Errorf("at least %d closed klines are required", minimumBars)
	}

	feeRate := config.FeeRateBPS / 10000
	slippage := config.SlippageBPS / 10000
	cash := initialCapital
	state := schema.SpotStateWait
	peakEquity := initialCapital
	realizedPnL, fees, turnover := 0.0, 0.0, 0.0
	maximumDailyBuyRatio := 0.0
	grossProfit, grossLoss := 0.0, 0.0
	wins, losses := 0, 0
	lastBuyIndex := -1 << 20
	accumulationAnchor := 0.0
	breakoutAnchor := 0.0
	distributionAnchor := 0.0
	distributionStep := 0
	coreDistributedQty := 0.0
	coreAcquiredQty := 0.0
	lotSequence := 0
	accumulationBought := make(map[int]bool)
	satelliteBought := make(map[int]bool)
	lots := make([]*spotEngineLot, 0, 32)
	events := make([]schema.SpotBacktestEvent, 0, 128)
	curve := make([]schema.SpotBacktestPoint, 0, len(bars))
	var pending []spotPendingAction
	dailyBuyQuote := make(map[string]float64)

	weightsTotal := 0.0
	for _, weight := range config.AccumulationWeights {
		weightsTotal += weight
	}
	coreBudget := initialCapital * config.CoreBudgetRatio
	satelliteBudget := initialCapital * config.SatelliteBudgetRatio
	minimumCash := initialCapital * config.MinimumCashRatio
	maxDailyBuy := initialCapital * config.MaxDailyBuyRatio

	transition := func(at int64, next, reason string) {
		if state == next {
			return
		}
		events = append(events, schema.SpotBacktestEvent{Time: at, Type: "STATE_CHANGED", FromState: state, ToState: next, Reason: reason})
		state = next
	}

	for index, bar := range bars {
		open := mustSpotFloat(bar.Open)
		closePrice := mustSpotFloat(bar.Close)
		marketTime := bar.Time
		dayKey := spotUnixTime(marketTime).Format("2006-01-02")

		for _, action := range pending {
			switch action.side {
			case "buy":
				available := math.Max(0, cash-minimumCash)
				exposureCapacity := math.Max(0, initialCapital*config.MaxSymbolExposureRatio-spotLotsRemainingCost(lots))
				quote := math.Min(action.quote, math.Min(available, math.Min(exposureCapacity, math.Max(0, maxDailyBuy-dailyBuyQuote[dayKey]))))
				if quote <= 0 {
					continue
				}
				price := open * (1 + slippage)
				fee := quote * feeRate
				quantity := (quote - fee) / price
				if quantity <= 0 {
					continue
				}
				cash -= quote
				dailyBuyQuote[dayKey] += quote
				maximumDailyBuyRatio = math.Max(maximumDailyBuyRatio, dailyBuyQuote[dayKey]/initialCapital)
				fees += fee
				turnover += quote
				lotSequence++
				lotID := fmt.Sprintf("LOT-%06d", lotSequence)
				unitCost := quote / quantity
				target := unitCost * (1 + config.SatelliteTakeProfitRatio) / math.Max(1e-9, 1-feeRate)
				if action.bucket == schema.SpotBucketCore {
					target = 0
					coreAcquiredQty += quantity
					accumulationBought[action.layer] = true
				} else {
					satelliteBought[action.layer] = true
				}
				lots = append(lots, &spotEngineLot{SpotBacktestLot: schema.SpotBacktestLot{ID: lotID, Bucket: action.bucket,
					LayerNo: action.layer, OpenTime: marketTime, OpenPrice: price, AcquiredQty: quantity,
					RemainingQty: quantity, TotalCost: quote, TargetPrice: target}, unitCost: unitCost})
				events = append(events, schema.SpotBacktestEvent{Time: marketTime, Type: "LOT_OPENED", Reason: action.reason,
					LotID: lotID, Quantity: quantity, Price: price})
				lastBuyIndex = index
			case "sell":
				price := open * (1 - slippage)
				remainingToSell := action.maxQty
				for _, lotID := range action.lotIDs {
					lot := findSpotEngineLot(lots, lotID)
					if lot == nil || lot.RemainingQty <= 0 || lot.Bucket != action.bucket {
						continue
					}
					quantity := lot.RemainingQty * action.sellRatio
					if action.maxQty > 0 {
						quantity = math.Min(quantity, remainingToSell)
					}
					if quantity <= 0 {
						break
					}
					gross := quantity * price
					fee := gross * feeRate
					proceeds := gross - fee
					cost := lot.unitCost * quantity
					pnl := proceeds - cost
					cash += proceeds
					fees += fee
					turnover += gross
					realizedPnL += pnl
					lot.RealizedPnL += pnl
					lot.RemainingQty = math.Max(0, lot.RemainingQty-quantity)
					if action.maxQty > 0 {
						remainingToSell -= quantity
					}
					if pnl >= 0 {
						grossProfit += pnl
						wins++
					} else {
						grossLoss += -pnl
						losses++
					}
					if lot.Bucket == schema.SpotBucketCore {
						coreDistributedQty += quantity
					}
					if lot.RemainingQty < 1e-12 {
						closed := marketTime
						lot.ClosedAt = &closed
					}
					events = append(events, schema.SpotBacktestEvent{Time: marketTime, Type: "LOT_ALLOCATED", Reason: action.reason,
						LotID: lot.ID, Quantity: quantity, Price: price})
				}
			}
		}
		pending = nil

		equity := cash + spotLotsMarketValue(lots, closePrice)
		if equity > peakEquity {
			peakEquity = equity
		}
		drawdown := 0.0
		if peakEquity > 0 {
			drawdown = (peakEquity - equity) / peakEquity
		}
		if config.DrawdownRiskControlEnabled() && drawdown >= config.MaxStrategyDrawdownPct && state != schema.SpotStateRiskOff {
			transition(marketTime, schema.SpotStateRiskOff, "MAX_DRAWDOWN")
			pending = nil
		}
		curve = append(curve, schema.SpotBacktestPoint{Time: marketTime, Equity: equity, Cash: cash,
			Exposure: spotLotsRemainingCost(lots), State: state})

		if index < minimumBars-1 || state == schema.SpotStateRiskOff {
			continue
		}
		rollingHigh := spotRollingHigh(bars, index, config.LowAreaLookbackBars)
		resistance := spotRollingHigh(bars, index, config.ResistanceLookbackBars)
		distributionHigh := spotRollingHigh(bars, index, config.DistributionLookbackBars)
		lowAreaDrawdown := 0.0
		if rollingHigh > 0 {
			lowAreaDrawdown = (rollingHigh - closePrice) / rollingHigh
		}
		stabilized := spotStabilized(bars, index, config.StabilizationBars)

		if state == schema.SpotStateWait && lowAreaDrawdown >= config.LowAreaDrawdownPct && stabilized {
			accumulationAnchor = closePrice
			transition(marketTime, schema.SpotStateAccumulate, "LOW_AREA_CONFIRMED")
		}

		if state == schema.SpotStateAccumulate || state == schema.SpotStateRangeTrading {
			if accumulationAnchor == 0 {
				accumulationAnchor = closePrice
			}
			for layer, threshold := range config.AccumulationDrawdowns {
				if accumulationBought[layer] || index-lastBuyIndex < config.AccumulationCooldownBars {
					continue
				}
				if closePrice > accumulationAnchor*(1-threshold) {
					continue
				}
				quote := coreBudget * config.AccumulationWeights[layer] / weightsTotal
				pending = append(pending, spotPendingAction{side: "buy", bucket: schema.SpotBucketCore, layer: layer,
					quote: quote, reason: "CORE_ACCUMULATION_LAYER"})
				break
			}
			if state == schema.SpotStateAccumulate && len(lots) > 0 && stabilized {
				transition(marketTime, schema.SpotStateRangeTrading, "CORE_POSITION_STABILIZED")
			}
		}

		if state == schema.SpotStateRangeTrading {
			for layer, threshold := range config.SatelliteEntryDrawdowns {
				if satelliteBought[layer] || index-lastBuyIndex < config.AccumulationCooldownBars {
					continue
				}
				if resistance <= 0 || (resistance-closePrice)/resistance < threshold {
					continue
				}
				pending = append(pending, spotPendingAction{side: "buy", bucket: schema.SpotBucketSatellite, layer: layer,
					quote: satelliteBudget / float64(len(config.SatelliteEntryDrawdowns)), reason: "SATELLITE_PULLBACK_LAYER"})
				break
			}
			if spotBreakoutConfirmed(bars, index, config.ResistanceLookbackBars, config.BreakoutConfirmBars, config.BreakoutVolumeMultiplier) {
				breakoutAnchor = resistance
				transition(marketTime, schema.SpotStateBreakout, "RESISTANCE_BREAKOUT_CONFIRMED")
				eligible := eligibleSpotLots(lots, schema.SpotBucketSatellite, closePrice)
				if len(eligible) > 0 {
					pending = append(pending, spotPendingAction{side: "sell", bucket: schema.SpotBucketSatellite,
						lotIDs: eligible, sellRatio: config.SatelliteSellRatio, reason: "SATELLITE_BREAKOUT_TAKE_PROFIT"})
				}
			}
		}

		if state == schema.SpotStateBreakout {
			longMA := spotSMA(bars, index, config.TrendFilterPeriod)
			previousMA := spotSMA(bars, index-1, config.TrendFilterPeriod)
			if closePrice >= distributionHigh && closePrice > longMA && longMA > previousMA {
				distributionAnchor = closePrice
				distributionStep = 0
				transition(marketTime, schema.SpotStateDistribution, "LONG_TREND_BREAKOUT")
			} else if breakoutAnchor > 0 && closePrice < breakoutAnchor {
				for key := range satelliteBought {
					delete(satelliteBought, key)
				}
				transition(marketTime, schema.SpotStateRangeTrading, "BREAKOUT_FAILED")
			}
		}

		if state == schema.SpotStateDistribution && distributionAnchor > 0 {
			nextTarget := distributionAnchor * (1 + config.DistributionStepGainPct*float64(distributionStep+1))
			maximumDistribution := coreAcquiredQty * config.MaxCoreDistributionRatio
			minimumRetention := coreAcquiredQty * config.CoreMinimumRetentionRatio
			coreRemaining := spotBucketQuantity(lots, schema.SpotBucketCore)
			if closePrice >= nextTarget && coreDistributedQty < maximumDistribution && coreRemaining > minimumRetention {
				sellRatio := config.DistributionStepSellRatio
				if coreRemaining*(1-sellRatio) < minimumRetention {
					sellRatio = (coreRemaining - minimumRetention) / coreRemaining
				}
				if sellRatio > 0 {
					maximumQuantity := math.Min(maximumDistribution-coreDistributedQty, coreRemaining-minimumRetention)
					pending = append(pending, spotPendingAction{side: "sell", bucket: schema.SpotBucketCore,
						lotIDs: openSpotLotIDs(lots, schema.SpotBucketCore), sellRatio: sellRatio, maxQty: maximumQuantity,
						reason: "CORE_DISTRIBUTION_STEP"})
					distributionStep++
				}
			}
		}
	}

	lastClose := mustSpotFloat(bars[len(bars)-1].Close)
	finalEquity := cash + spotLotsMarketValue(lots, lastClose)
	metrics := buildSpotBacktestMetrics(initialCapital, finalEquity, cash, realizedPnL, fees, turnover,
		grossProfit, grossLoss, maximumDailyBuyRatio, wins, losses, curve, lots, bars)
	benchmarks := buildSpotBenchmarks(config, bars, initialCapital)
	outputLots := make([]schema.SpotBacktestLot, 0, len(lots))
	for _, lot := range lots {
		outputLots = append(outputLots, lot.SpotBacktestLot)
	}
	acceptance := buildSpotAcceptance(config, metrics, benchmarks, outputLots)
	return &schema.SpotBacktestOutput{Metrics: metrics, Benchmarks: benchmarks, EquityCurve: curve,
		Lots: outputLots, Events: events, Acceptance: acceptance}, nil
}

func normalizeSpotBacktestBars(input []*schema.MarketKline) ([]*schema.MarketKline, error) {
	bars := make([]*schema.MarketKline, 0, len(input))
	seen := make(map[int64]struct{}, len(input))
	for _, bar := range input {
		if bar == nil || !bar.Closed {
			continue
		}
		if _, ok := seen[bar.Time]; ok {
			continue
		}
		for _, value := range []string{bar.Open, bar.High, bar.Low, bar.Close} {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("invalid kline decimal %q", value)
			}
		}
		volume, err := strconv.ParseFloat(bar.Volume, 64)
		if err != nil || volume < 0 {
			return nil, fmt.Errorf("invalid kline volume %q", bar.Volume)
		}
		seen[bar.Time] = struct{}{}
		bars = append(bars, bar)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Time < bars[j].Time })
	return bars, nil
}

func buildSpotBacktestMetrics(initial, final, cash, realized, fees, turnover, grossProfit, grossLoss, maximumDailyBuyRatio float64,
	wins, losses int, curve []schema.SpotBacktestPoint, lots []*spotEngineLot, bars []*schema.MarketKline) schema.SpotBacktestMetrics {
	totalReturn := (final - initial) / initial
	years := float64(bars[len(bars)-1].Time-bars[0].Time) / float64(365*24*60*60)
	annualized := totalReturn
	if years > 0 && final > 0 {
		annualized = math.Pow(final/initial, 1/years) - 1
	}
	maxDrawdown := spotCurveMaxDrawdown(curve)
	calmar := 0.0
	if maxDrawdown > 0 {
		calmar = annualized / maxDrawdown
	}
	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		// JSON has no Infinity value. A bounded sentinel still expresses that
		// the sample contains gains and no realized losses.
		profitFactor = 999
	}
	averageCash := 0.0
	minimumCashRatio := 1.0
	maximumExposureRatio := 0.0
	for _, point := range curve {
		if point.Equity > 0 {
			averageCash += point.Cash / point.Equity
		}
		minimumCashRatio = math.Min(minimumCashRatio, point.Cash/initial)
		maximumExposureRatio = math.Max(maximumExposureRatio, point.Exposure/initial)
	}
	if len(curve) > 0 {
		averageCash /= float64(len(curve))
	}
	return schema.SpotBacktestMetrics{InitialCapital: initial, FinalEquity: final, TotalReturn: totalReturn,
		AnnualizedReturn: annualized, MaxDrawdown: maxDrawdown, Calmar: calmar, ProfitFactor: profitFactor,
		ProfitFactorAvailable: wins+losses > 0,
		RealizedPnL:           realized, Fees: fees, Turnover: turnover, AverageCashRatio: averageCash,
		MinimumCashRatio: minimumCashRatio, MaximumExposureRatio: maximumExposureRatio,
		MaximumDailyBuyRatio: maximumDailyBuyRatio,
		TradeCount:           wins + losses, WinningTrades: wins, LosingTrades: losses,
		CoreRemainingQuantity: spotBucketQuantity(lots, schema.SpotBucketCore),
		SatelliteRemainingQty: spotBucketQuantity(lots, schema.SpotBucketSatellite)}
}

func buildSpotBenchmarks(config schema.SpotCoreSatelliteConfig, bars []*schema.MarketKline, initial float64) []schema.SpotBacktestBenchmark {
	feeRate, slippage := config.FeeRateBPS/10000, config.SlippageBPS/10000
	first := mustSpotFloat(bars[0].Open) * (1 + slippage)
	quantity := initial * (1 - feeRate) / first
	buyHoldCurve := make([]schema.SpotBacktestPoint, 0, len(bars))
	for _, bar := range bars {
		buyHoldCurve = append(buyHoldCurve, schema.SpotBacktestPoint{Time: bar.Time, Equity: quantity * mustSpotFloat(bar.Close)})
	}
	buyHold := spotBenchmark("buy_and_hold", initial, buyHoldCurve, bars)

	dcaCash := initial
	dcaQty := 0.0
	dcaCurve := make([]schema.SpotBacktestPoint, 0, len(bars))
	installments := math.Max(1, math.Ceil(float64(len(bars))/30))
	installment := initial / installments
	for index, bar := range bars {
		if index%30 == 0 && dcaCash > 0 {
			quote := math.Min(installment, dcaCash)
			price := mustSpotFloat(bar.Open) * (1 + slippage)
			dcaQty += quote * (1 - feeRate) / price
			dcaCash -= quote
		}
		dcaCurve = append(dcaCurve, schema.SpotBacktestPoint{Time: bar.Time, Equity: dcaCash + dcaQty*mustSpotFloat(bar.Close)})
	}
	dca := spotBenchmark("fixed_dca", initial, dcaCurve, bars)

	coreCash := initial
	coreQty := 0.0
	coreQuote := initial * config.CoreBudgetRatio
	if coreQuote > 0 {
		coreQty = coreQuote * (1 - feeRate) / first
		coreCash -= coreQuote
	}
	coreCurve := make([]schema.SpotBacktestPoint, 0, len(bars))
	for _, bar := range bars {
		coreCurve = append(coreCurve, schema.SpotBacktestPoint{Time: bar.Time,
			Equity: coreCash + coreQty*mustSpotFloat(bar.Close), Cash: coreCash})
	}
	coreOnly := spotBenchmark("core_only", initial, coreCurve, bars)
	return []schema.SpotBacktestBenchmark{buyHold, dca, coreOnly}
}

func spotBenchmark(name string, initial float64, curve []schema.SpotBacktestPoint, bars []*schema.MarketKline) schema.SpotBacktestBenchmark {
	final := curve[len(curve)-1].Equity
	ret := (final - initial) / initial
	years := float64(bars[len(bars)-1].Time-bars[0].Time) / float64(365*24*60*60)
	annualized := ret
	if years > 0 && final > 0 {
		annualized = math.Pow(final/initial, 1/years) - 1
	}
	dd := spotCurveMaxDrawdown(curve)
	calmar := 0.0
	if dd > 0 {
		calmar = annualized / dd
	}
	return schema.SpotBacktestBenchmark{Name: name, FinalEquity: final, TotalReturn: ret, MaxDrawdown: dd, Calmar: calmar}
}

func buildSpotAcceptance(config schema.SpotCoreSatelliteConfig, metrics schema.SpotBacktestMetrics, benchmarks []schema.SpotBacktestBenchmark, lots []schema.SpotBacktestLot) []schema.SpotAcceptanceItem {
	benchmark := schema.SpotBacktestBenchmark{}
	for _, item := range benchmarks {
		if item.Name == "buy_and_hold" {
			benchmark = item
		}
	}
	negativeLot := false
	for _, lot := range lots {
		if lot.RemainingQty < -1e-9 {
			negativeLot = true
		}
	}
	items := []schema.SpotAcceptanceItem{
		{Key: "lot_inventory_non_negative", Passed: !negativeLot, Actual: fmt.Sprintf("%t", !negativeLot), Expected: "true"},
		{Key: "minimum_cash_ratio", Passed: metrics.MinimumCashRatio+1e-9 >= config.MinimumCashRatio,
			Actual: fmt.Sprintf("%.6f", metrics.MinimumCashRatio), Expected: fmt.Sprintf(">=%.6f", config.MinimumCashRatio)},
		{Key: "maximum_symbol_exposure", Passed: metrics.MaximumExposureRatio <= config.MaxSymbolExposureRatio+1e-9,
			Actual: fmt.Sprintf("%.6f", metrics.MaximumExposureRatio), Expected: fmt.Sprintf("<=%.6f", config.MaxSymbolExposureRatio)},
		{Key: "maximum_daily_buy", Passed: metrics.MaximumDailyBuyRatio <= config.MaxDailyBuyRatio+1e-9,
			Actual: fmt.Sprintf("%.6f", metrics.MaximumDailyBuyRatio), Expected: fmt.Sprintf("<=%.6f", config.MaxDailyBuyRatio)},
		{Key: "max_drawdown", Passed: metrics.MaxDrawdown <= 0.30, Actual: fmt.Sprintf("%.6f", metrics.MaxDrawdown), Expected: "<=0.30"},
		{Key: "net_return_positive", Passed: metrics.TotalReturn > 0, Actual: fmt.Sprintf("%.6f", metrics.TotalReturn), Expected: ">0"},
		{Key: "profit_factor", Passed: metrics.ProfitFactorAvailable && metrics.ProfitFactor >= 1.20,
			NotApplicable: !metrics.ProfitFactorAvailable, Actual: spotProfitFactorActual(metrics), Expected: ">=1.20"},
	}
	if benchmark.MaxDrawdown > 0 {
		items = append(items, schema.SpotAcceptanceItem{Key: "drawdown_vs_buy_hold", Passed: metrics.MaxDrawdown <= benchmark.MaxDrawdown*0.8,
			Actual: fmt.Sprintf("%.6f", metrics.MaxDrawdown), Expected: fmt.Sprintf("<=%.6f", benchmark.MaxDrawdown*0.8)})
	}
	return items
}

func spotProfitFactorActual(metrics schema.SpotBacktestMetrics) string {
	if !metrics.ProfitFactorAvailable {
		return "N/A (no realized sell allocations)"
	}
	return fmt.Sprintf("%.6f", metrics.ProfitFactor)
}

func eligibleSpotLots(lots []*spotEngineLot, bucket string, price float64) []string {
	selected := make([]*spotEngineLot, 0)
	for _, lot := range lots {
		if lot.Bucket == bucket && lot.RemainingQty > 0 && lot.TargetPrice > 0 && price >= lot.TargetPrice {
			selected = append(selected, lot)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].TargetPrice == selected[j].TargetPrice {
			return selected[i].OpenTime > selected[j].OpenTime
		}
		return selected[i].TargetPrice < selected[j].TargetPrice
	})
	ids := make([]string, 0, len(selected))
	for _, lot := range selected {
		ids = append(ids, lot.ID)
	}
	return ids
}

func openSpotLotIDs(lots []*spotEngineLot, bucket string) []string {
	ids := make([]string, 0)
	for _, lot := range lots {
		if lot.Bucket == bucket && lot.RemainingQty > 0 {
			ids = append(ids, lot.ID)
		}
	}
	return ids
}

func findSpotEngineLot(lots []*spotEngineLot, id string) *spotEngineLot {
	for _, lot := range lots {
		if lot.ID == id {
			return lot
		}
	}
	return nil
}
func spotLotsMarketValue(lots []*spotEngineLot, price float64) float64 {
	total := 0.0
	for _, lot := range lots {
		total += lot.RemainingQty * price
	}
	return total
}
func spotLotsRemainingCost(lots []*spotEngineLot) float64 {
	total := 0.0
	for _, lot := range lots {
		if lot.AcquiredQty <= 0 {
			continue
		}
		total += lot.TotalCost * lot.RemainingQty / lot.AcquiredQty
	}
	return total
}
func spotBucketQuantity(lots []*spotEngineLot, bucket string) float64 {
	total := 0.0
	for _, lot := range lots {
		if lot.Bucket == bucket {
			total += lot.RemainingQty
		}
	}
	return total
}
func mustSpotFloat(value string) float64 { parsed, _ := strconv.ParseFloat(value, 64); return parsed }

func spotRollingHigh(bars []*schema.MarketKline, index, lookback int) float64 {
	start := index - lookback
	if start < 0 {
		start = 0
	}
	high := 0.0
	for i := start; i < index; i++ {
		value := mustSpotFloat(bars[i].High)
		if value > high {
			high = value
		}
	}
	return high
}

func spotSMA(bars []*schema.MarketKline, index, period int) float64 {
	if index < 0 {
		return 0
	}
	start := index - period + 1
	if start < 0 {
		start = 0
	}
	total := 0.0
	for i := start; i <= index; i++ {
		total += mustSpotFloat(bars[i].Close)
	}
	return total / float64(index-start+1)
}

func spotStabilized(bars []*schema.MarketKline, index, count int) bool {
	if index < count {
		return false
	}
	current := mustSpotFloat(bars[index].Close)
	old := mustSpotFloat(bars[index-count].Close)
	minimum := current
	for i := index - count + 1; i <= index; i++ {
		value := mustSpotFloat(bars[i].Close)
		if value < minimum {
			minimum = value
		}
	}
	return current > minimum && current >= old
}

func spotBreakoutConfirmed(bars []*schema.MarketKline, index, lookback, confirm int, volumeMultiplier float64) bool {
	if index < lookback+confirm {
		return false
	}
	resistance := spotRollingHigh(bars, index-confirm+1, lookback)
	if resistance <= 0 {
		return false
	}
	for i := index - confirm + 1; i <= index; i++ {
		if mustSpotFloat(bars[i].Close) <= resistance {
			return false
		}
	}
	averageVolume := 0.0
	for i := index - lookback; i < index; i++ {
		averageVolume += mustSpotFloat(bars[i].Volume)
	}
	averageVolume /= float64(lookback)
	return averageVolume <= 0 || mustSpotFloat(bars[index].Volume) >= averageVolume*volumeMultiplier
}

func spotCurveMaxDrawdown(curve []schema.SpotBacktestPoint) float64 {
	peak, maximum := 0.0, 0.0
	for _, point := range curve {
		if point.Equity > peak {
			peak = point.Equity
		}
		if peak > 0 {
			drawdown := (peak - point.Equity) / peak
			if drawdown > maximum {
				maximum = drawdown
			}
		}
	}
	return maximum
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func spotUnixTime(value int64) time.Time { return time.Unix(value, 0).UTC() }
