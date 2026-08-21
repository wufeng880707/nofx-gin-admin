package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type strategyContext struct {
	GeneratedAt   time.Time                `json:"generated_at"`
	Trader        map[string]interface{}   `json:"trader"`
	CandidatePool strategyCandidatePool    `json:"candidate_pool"`
	Account       []*schema.AssetBalance   `json:"account_assets"`
	Positions     []*schema.Position       `json:"open_positions"`
	Markets       []strategyMarketSnapshot `json:"markets"`
}

type strategyCandidatePool struct {
	RequestedSource string   `json:"requested_source"`
	ResolvedSources []string `json:"resolved_sources"`
	Symbols         []string `json:"symbols"`
	Limitations     []string `json:"limitations,omitempty"`
}

type strategyMarketSnapshot struct {
	Symbol      string                          `json:"symbol"`
	Market      *schema.MarketContext           `json:"market_context,omitempty"`
	MarketError string                          `json:"market_context_error,omitempty"`
	Timeframes  map[string]strategyTimeframeCtx `json:"timeframes"`
}

type strategyTimeframeCtx struct {
	LastPrice string                  `json:"last_price"`
	EMA       map[string]float64      `json:"ema,omitempty"`
	MACD      *strategyMACD           `json:"macd,omitempty"`
	RSI       map[string]float64      `json:"rsi,omitempty"`
	ATR       map[string]float64      `json:"atr,omitempty"`
	BOLL      map[string]strategyBOLL `json:"boll,omitempty"`
	Volume    string                  `json:"volume,omitempty"`
	Klines    int                     `json:"klines"`
	RawKlines []*schema.MarketKline   `json:"raw_klines,omitempty"`
}

type strategyMACD struct {
	DIF       float64 `json:"dif"`
	DEA       float64 `json:"dea"`
	Histogram float64 `json:"histogram"`
}

type strategyBOLL struct {
	Upper  float64 `json:"upper"`
	Middle float64 `json:"middle"`
	Lower  float64 `json:"lower"`
}

type decisionEnvelope struct {
	Decisions []runtimeDecision `json:"decisions"`
}

type runtimeDecision struct {
	Symbol           string  `json:"symbol"`
	Action           string  `json:"action"`
	Side             string  `json:"side,omitempty"`
	Confidence       int     `json:"confidence"`
	Leverage         int     `json:"leverage,omitempty"`
	PositionSizeUSDT float64 `json:"position_size_usdt,omitempty"`
	StopLoss         float64 `json:"stop_loss,omitempty"`
	TakeProfit       float64 `json:"take_profit,omitempty"`
	Reason           string  `json:"reason"`
	Valid            bool    `json:"valid"`
	ValidationError  string  `json:"validation_error,omitempty"`
}

func (a *Registry) RunStrategyCycle(ctx context.Context, traderID string) error {
	var trader schema.Trader
	if err := a.DB.WithContext(ctx).Where("id = ?", traderID).First(&trader).Error; err != nil {
		return err
	}
	if trader.StrategyID == nil || strings.TrimSpace(*trader.StrategyID) == "" {
		return errors.New("trader has no strategy")
	}
	var exchange schema.Exchange
	if err := a.DB.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND enabled = ?", trader.ExchangeID, trader.OwnerUserID, true).First(&exchange).Error; err != nil {
		return err
	}
	strategy, configJSON, err := a.loadRuntimeStrategy(ctx, &trader)
	if err != nil {
		return err
	}
	config, err := schema.DecodeStrategyStudioConfig(json.RawMessage(configJSON))
	if err != nil {
		return fmt.Errorf("decode strategy config: %w", err)
	}
	if config.StrategyType == "grid" {
		return a.runGridStrategyCycle(ctx, &trader, &exchange, strategy, config)
	}
	if config.StrategyType == schema.SpotStrategyType {
		return a.runSpotCoreSatelliteStrategyCycle(ctx, &trader, &exchange, strategy, config)
	}
	if a.LLM == nil {
		return errors.New("AI runtime service is unavailable")
	}
	if trader.AIConsentAt == nil || trader.AIConsentModelID != trader.AIModelID {
		return errors.New("external AI context consent is missing or stale")
	}
	var model schema.AIModel
	if err := a.DB.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND enabled = ?", trader.AIModelID, trader.OwnerUserID, true).First(&model).Error; err != nil {
		return err
	}

	cycleNumber := trader.CycleNumber + 1
	generatedAt := time.Now().UTC()
	contextData, candidates, err := a.buildStrategyContext(ctx, &trader, &exchange, config, generatedAt)
	if err != nil {
		return err
	}
	inputJSON, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return err
	}
	systemPrompt := runtimeSystemPrompt(strategy, config)
	result, invokeErr := a.LLM.Invoke(ctx, &model, llm.InvocationRequest{
		Purpose: "decision", TraderID: &trader.ID, StrategyID: trader.StrategyID,
		SystemPrompt: systemPrompt, UserPrompt: string(inputJSON), MaxTokens: 1200,
	})
	decision := &schema.Decision{
		OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, ExchangeID: trader.ExchangeID,
		CycleNumber: cycleNumber, DecidedAt: generatedAt, SystemPrompt: systemPrompt,
		InputPrompt: string(inputJSON), CoTTrace: "", DecisionJSON: `{}`,
		CandidateCoinsJSON: mustJSON(candidates), ExecutionLogJSON: `[]`, ActionsJSON: `[]`, CreatedAt: generatedAt,
	}
	if invokeErr != nil {
		decision.ErrorMessage = invokeErr.Error()
		if err := a.DB.WithContext(ctx).Create(decision).Error; err != nil {
			return err
		}
		_ = a.updateTraderFields(ctx, trader.ID, map[string]interface{}{"cycle_number": cycleNumber, "safe_mode": true})
		return invokeErr
	}

	decision.RawResponse = result.Content
	decision.AIRequestDurationMs = result.DurationMS
	envelope, err := parseDecisionEnvelope(result.Content)
	if err != nil {
		decision.ErrorMessage = err.Error()
		if createErr := a.DB.WithContext(ctx).Create(decision).Error; createErr != nil {
			return createErr
		}
		_ = a.updateTraderFields(ctx, trader.ID, map[string]interface{}{"cycle_number": cycleNumber, "safe_mode": true})
		return err
	}
	validateRuntimeDecisions(&envelope, config, len(contextData.Positions))
	decision.Success, decision.DecisionJSON = true, mustJSON(envelope)
	decision.ActionsJSON = mustJSON(envelope.Decisions)
	decision.ExecutionLogJSON = mustJSON([]map[string]interface{}{{
		"stage": "dry_run", "status": "skipped", "message": "No trading command service was invoked",
	}})
	if err := a.DB.WithContext(ctx).Create(decision).Error; err != nil {
		return err
	}
	if a.DecisionWorkflow != nil {
		logs, workflowErr := a.DecisionWorkflow.ProcessStrategyDecision(ctx, &trader, &exchange, decision, json.RawMessage(decision.ActionsJSON))
		if logs != nil {
			decision.ExecutionLogJSON = mustJSON(logs)
			_ = a.DB.WithContext(ctx).Model(new(schema.Decision)).Where("id = ?", decision.ID).
				Update("execution_log_json", decision.ExecutionLogJSON).Error
		}
		if workflowErr != nil {
			_ = a.updateTraderFields(ctx, trader.ID, map[string]interface{}{
				"last_error_code": "decision_workflow_failed", "last_error_message": workflowErr.Error(),
			})
		}
	}
	now := time.Now().UTC()
	return a.updateTraderFields(ctx, trader.ID, map[string]interface{}{
		"cycle_number": cycleNumber, "last_decision_at": now, "strategy_state": "running",
		"safe_mode": true, "pause_reason": "", "last_error_code": "", "last_error_message": "",
	})
}

func (a *Registry) loadRuntimeStrategy(ctx context.Context, trader *schema.Trader) (*schema.Strategy, string, error) {
	if trader.StrategyID == nil || strings.TrimSpace(*trader.StrategyID) == "" {
		return nil, "", errors.New("trader has no strategy")
	}
	var strategy schema.Strategy
	var version schema.StrategyVersion
	err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ? AND is_active = ?", *trader.StrategyID, trader.OwnerUserID, true).
			First(&strategy).Error; err != nil {
			return err
		}
		targetVersion := trader.StrategyVersion
		// Runtime hot switching is an explicit simulation-only capability. Live
		// traders stay pinned until they are stopped and started again.
		if trader.IsSimulation && trader.AutoApplyStrategyUpdates && strategy.PublishedVersion != nil {
			targetVersion = *strategy.PublishedVersion
		}
		if targetVersion <= 0 {
			return errors.New("trader has no pinned published strategy version")
		}
		if err := tx.Where("strategy_id = ? AND owner_user_id = ? AND version = ? AND published_at IS NOT NULL",
			strategy.ID, trader.OwnerUserID, targetVersion).First(&version).Error; err != nil {
			return fmt.Errorf("load published strategy version %d: %w", targetVersion, err)
		}
		if _, err := schema.DecodeStrategyStudioConfig(json.RawMessage(version.ConfigJSON)); err != nil {
			return fmt.Errorf("validate published strategy version %d: %w", targetVersion, err)
		}
		if targetVersion == trader.StrategyVersion {
			return nil
		}
		now := time.Now().UTC()
		result := tx.Model(new(schema.Trader)).
			Where("id = ? AND owner_user_id = ? AND strategy_id = ? AND strategy_version = ?",
				trader.ID, trader.OwnerUserID, strategy.ID, trader.StrategyVersion).
			Updates(map[string]interface{}{"strategy_version": targetVersion, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("trader strategy version changed concurrently; retry next cycle")
		}
		if err := tx.Create(&schema.StrategyVersionSwitchEvent{
			ID: uuid.NewString(), OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, StrategyID: strategy.ID,
			FromVersion: trader.StrategyVersion, ToVersion: targetVersion,
			Reason: "published_version_changed", OccurredAt: now,
		}).Error; err != nil {
			return err
		}
		trader.StrategyVersion = targetVersion
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return &strategy, version.ConfigJSON, nil
}

func (a *Registry) buildStrategyContext(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	config *schema.StrategyStudioConfig, generatedAt time.Time) (*strategyContext, []string, error) {
	var assets []*schema.AssetBalance
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND exchange_id = ?", trader.OwnerUserID, exchange.ID).
		Order("value_usdt DESC").Limit(100).Find(&assets).Error; err != nil {
		return nil, nil, err
	}
	var positions []*schema.Position
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND trader_id = ? AND status = ?", trader.OwnerUserID, trader.ID, schema.PositionStatusOpen).
		Order("opened_at ASC").Find(&positions).Error; err != nil {
		return nil, nil, err
	}
	result := &strategyContext{GeneratedAt: generatedAt, Account: assets, Positions: positions,
		Trader: map[string]interface{}{"id": trader.ID, "strategy_version": trader.StrategyVersion,
			"execution_mode": trader.ExecutionMode, "simulation": true, "safe_mode": true}}
	if a.ProviderFactory == nil {
		candidates := normalizeCandidates(config.AIConfig.CoinSource.StaticCoins)
		if len(candidates) == 0 {
			candidates = []string{"BTCUSDT", "ETHUSDT"}
		}
		result.CandidatePool = strategyCandidatePool{RequestedSource: config.AIConfig.CoinSource.SourceType,
			ResolvedSources: []string{"static_fallback"}, Symbols: candidates,
			Limitations: []string{"provider factory unavailable; public candidate pools and market enrichment omitted"}}
		return result, candidates, nil
	}
	marketType := schema.MarketTypeFutures
	if exchange.AccountType == "spot" {
		marketType = schema.MarketTypeSpot
	}
	if exchange.ExchangeType != schema.MarketExchangeBinance && exchange.ExchangeType != schema.MarketExchangeOKX {
		return nil, nil, fmt.Errorf("strategy market context is unsupported for %s", exchange.ExchangeType)
	}
	marketData := a.ProviderFactory.MarketData()
	candidates, pool, err := resolveStrategyCandidates(ctx, marketData, exchange.ExchangeType, marketType, config.AIConfig.CoinSource)
	if err != nil {
		return nil, nil, err
	}
	result.CandidatePool = pool
	timeframes := config.AIConfig.Indicators.Klines.SelectedTimeframes
	if len(timeframes) == 0 {
		timeframes = []string{config.AIConfig.Indicators.Klines.PrimaryTimeframe}
	}
	limit := config.AIConfig.Indicators.Klines.PrimaryCount
	if limit < 20 {
		limit = 30
	}
	for _, candidate := range candidates {
		snapshot := strategyMarketSnapshot{Symbol: candidate, Timeframes: make(map[string]strategyTimeframeCtx)}
		providerSymbol := runtimeMarketSymbol(exchange.ExchangeType, marketType, candidate)
		if needsMarketIntelligence(config.AIConfig.Indicators) {
			marketContext, contextErr := marketData.Context(ctx, exchange.ExchangeType, marketType, providerSymbol, 20)
			if contextErr != nil {
				snapshot.MarketError = contextErr.Error()
			} else {
				snapshot.Market = filterMarketContext(marketContext, config.AIConfig.Indicators)
			}
		}
		for _, timeframe := range timeframes {
			klines, err := marketData.Klines(ctx, exchange.ExchangeType, marketType, providerSymbol, timeframe, limit)
			if err != nil {
				return nil, nil, fmt.Errorf("load %s %s klines: %w", candidate, timeframe, err)
			}
			snapshot.Timeframes[timeframe] = summarizeKlines(klines, config.AIConfig.Indicators,
				timeframe == config.AIConfig.Indicators.Klines.PrimaryTimeframe)
		}
		result.Markets = append(result.Markets, snapshot)
	}
	return result, candidates, nil
}

func resolveStrategyCandidates(ctx context.Context, marketData *provider.MarketData, exchange, market string,
	config schema.CoinSourceConfig) ([]string, strategyCandidatePool, error) {
	requested := strings.ToLower(strings.TrimSpace(config.SourceType))
	static := normalizeCandidates(config.StaticCoins)
	var liquidity, oi []string
	var sources, limitations []string
	if requested == "static" || requested == "mixed" {
		sources = append(sources, "static")
	}
	if requested == "ai500" || requested == "mixed" || config.UseAI500 {
		values, err := marketData.LiquidityCandidates(ctx, exchange, market, config.AI500Limit)
		if err != nil {
			return nil, strategyCandidatePool{}, fmt.Errorf("load public liquidity candidate pool: %w", err)
		}
		liquidity = normalizeCandidates(values)
		sources = append(sources, "public_24h_quote_volume")
		limitations = append(limitations, "public liquidity ranking replaces proprietary AI500 data")
	}
	if requested == "oi_top" || requested == "mixed" || config.UseOITop {
		if market == schema.MarketTypeFutures {
			values, err := marketData.OpenInterestCandidates(ctx, exchange, config.OITopLimit)
			if err != nil {
				return nil, strategyCandidatePool{}, fmt.Errorf("load open-interest candidate pool: %w", err)
			}
			oi = normalizeCandidates(values)
			sources = append(sources, "public_open_interest")
		} else {
			limitations = append(limitations, "open-interest ranking is unavailable for spot markets")
		}
	}
	candidates := mergeStrategyCandidates(static, liquidity, oi, config.ExcludedCoins, 12)
	if len(candidates) == 0 {
		candidates = []string{"BTCUSDT", "ETHUSDT"}
		sources = append(sources, "default_fallback")
	}
	return candidates, strategyCandidatePool{RequestedSource: requested, ResolvedSources: uniqueRuntimeStrings(sources),
		Symbols: candidates, Limitations: uniqueRuntimeStrings(limitations)}, nil
}

func mergeStrategyCandidates(static, liquidity, oi, excluded []string, limit int) []string {
	exclusions := make(map[string]struct{})
	for _, item := range normalizeCandidates(excluded) {
		exclusions[item] = struct{}{}
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, limit)
	for _, group := range [][]string{static, liquidity, oi} {
		for _, symbol := range normalizeCandidates(group) {
			if _, skip := exclusions[symbol]; skip {
				continue
			}
			if _, exists := seen[symbol]; exists {
				continue
			}
			seen[symbol] = struct{}{}
			result = append(result, symbol)
			if limit > 0 && len(result) >= limit {
				return result
			}
		}
	}
	return result
}

func uniqueRuntimeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func needsMarketIntelligence(config schema.IndicatorConfig) bool {
	return config.EnableOI || config.EnableFundingRate || config.EnableQuantData || config.EnableQuantOI ||
		config.EnableQuantNetflow || config.EnableOIRanking || config.EnableNetflowRanking
}

func filterMarketContext(value *schema.MarketContext, config schema.IndicatorConfig) *schema.MarketContext {
	if value == nil {
		return nil
	}
	copyValue := *value
	if copyValue.Derivatives != nil {
		metrics := *copyValue.Derivatives
		if !config.EnableOI && !config.EnableQuantOI {
			metrics.OpenInterest, metrics.OpenInterestValue = "", ""
		}
		if !config.EnableFundingRate {
			metrics.FundingRate, metrics.NextFundingTime, metrics.MarkPrice = "", 0, ""
		}
		if !config.EnableQuantData && !config.EnableQuantNetflow {
			metrics.TakerBuyVolume, metrics.TakerSellVolume = "", ""
			metrics.TakerBuySellRatio, metrics.FlowSource, metrics.FlowIsProxy = 0, "", false
		}
		copyValue.Derivatives = &metrics
	}
	return &copyValue
}

func runtimeSystemPrompt(strategy *schema.Strategy, config *schema.StrategyStudioConfig) string {
	risk := config.AIConfig.RiskControl
	return fmt.Sprintf(`You are XTrading's dry-run strategy analysis engine for strategy %q.
Return JSON only using this schema:
{"decisions":[{"symbol":"BTCUSDT","action":"HOLD|OPEN|ADD|REDUCE|CLOSE|CANCEL","side":"long|short","confidence":0,"leverage":1,"position_size_usdt":0,"stop_loss":0,"take_profit":0,"reason":"concise evidence"}]}
Never include hidden chain-of-thought. Do not claim that an order was submitted. Application hard limits: max positions=%d, BTC/ETH leverage=%d, alt leverage=%d, max margin=%.4f, min position=%.4f USDT, min risk/reward=%.4f, min confidence=%d.`,
		strategy.Name, risk.MaxPositions, risk.BTCETHMaxLeverage, risk.AltcoinMaxLeverage,
		risk.MaxMarginUsage, risk.MinPositionSize, risk.MinRiskRewardRatio, risk.MinConfidence)
}

func parseDecisionEnvelope(raw string) (decisionEnvelope, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	var envelope decisionEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &envelope); err != nil {
		return envelope, fmt.Errorf("AI response is not valid decision JSON: %w", err)
	}
	if len(envelope.Decisions) == 0 {
		return envelope, errors.New("AI response contains no decisions")
	}
	return envelope, nil
}

func validateRuntimeDecisions(envelope *decisionEnvelope, config *schema.StrategyStudioConfig, openPositions int) {
	risk := config.AIConfig.RiskControl
	for index := range envelope.Decisions {
		decision := &envelope.Decisions[index]
		decision.Symbol = strings.ToUpper(strings.TrimSpace(decision.Symbol))
		decision.Action = strings.ToUpper(strings.TrimSpace(decision.Action))
		decision.Side = strings.ToLower(strings.TrimSpace(decision.Side))
		decision.Valid = true
		if !containsString([]string{"HOLD", "OPEN", "ADD", "REDUCE", "CLOSE", "CANCEL"}, decision.Action) {
			decision.Valid, decision.ValidationError = false, "unsupported action"
			continue
		}
		if decision.Symbol == "" {
			decision.Valid, decision.ValidationError = false, "symbol is required"
			continue
		}
		if decision.Action == "OPEN" || decision.Action == "ADD" {
			maxLeverage := risk.AltcoinMaxLeverage
			if strings.HasPrefix(decision.Symbol, "BTC") || strings.HasPrefix(decision.Symbol, "ETH") {
				maxLeverage = risk.BTCETHMaxLeverage
			}
			switch {
			case decision.Confidence < risk.MinConfidence:
				decision.Valid, decision.ValidationError = false, "confidence below strategy minimum"
			case decision.Leverage < 1 || decision.Leverage > maxLeverage:
				decision.Valid, decision.ValidationError = false, "leverage exceeds strategy limit"
			case decision.PositionSizeUSDT < risk.MinPositionSize:
				decision.Valid, decision.ValidationError = false, "position size below strategy minimum"
			case openPositions >= risk.MaxPositions && decision.Action == "OPEN":
				decision.Valid, decision.ValidationError = false, "maximum position count reached"
			case decision.StopLoss <= 0 || decision.TakeProfit <= 0:
				decision.Valid, decision.ValidationError = false, "stop loss and take profit are required"
			}
		}
	}
}

func normalizeCandidates(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "/", ""))
		value = strings.ReplaceAll(value, "-", "")
		value = strings.TrimSuffix(value, "SWAP")
		if value == "" {
			continue
		}
		if !strings.HasSuffix(value, "USDT") {
			value += "USDT"
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func runtimeMarketSymbol(exchange, market, symbol string) string {
	if exchange != schema.MarketExchangeOKX {
		return symbol
	}
	base := strings.TrimSuffix(symbol, "USDT")
	if market == schema.MarketTypeFutures {
		return base + "-USDT-SWAP"
	}
	return base + "-USDT"
}

func summarizeKlines(klines []*schema.MarketKline, config schema.IndicatorConfig, primary bool) strategyTimeframeCtx {
	if len(klines) == 0 {
		return strategyTimeframeCtx{}
	}
	closes := make([]float64, 0, len(klines))
	highs := make([]float64, 0, len(klines))
	lows := make([]float64, 0, len(klines))
	for _, item := range klines {
		closes = append(closes, parseNumber(item.Close))
		highs = append(highs, parseNumber(item.High))
		lows = append(lows, parseNumber(item.Low))
	}
	last := klines[len(klines)-1]
	result := strategyTimeframeCtx{LastPrice: last.Close, Klines: len(klines)}
	if config.EnableEMA {
		result.EMA = calculatePeriodValues(config.EMAPeriods, func(period int) float64 { return ema(closes, period) })
	}
	if config.EnableMACD {
		result.MACD = calculateMACD(closes)
	}
	if config.EnableRSI {
		result.RSI = calculatePeriodValues(config.RSIPeriods, func(period int) float64 { return rsi(closes, period) })
	}
	if config.EnableATR {
		result.ATR = calculatePeriodValues(config.ATRPeriods, func(period int) float64 { return atr(highs, lows, closes, period) })
	}
	if config.EnableBOLL {
		result.BOLL = make(map[string]strategyBOLL, len(config.BOLLPeriods))
		for _, period := range config.BOLLPeriods {
			result.BOLL[strconv.Itoa(period)] = calculateBOLL(closes, period)
		}
	}
	if config.EnableVolume {
		result.Volume = last.Volume
	}
	if config.EnableRawKlines && primary {
		start := len(klines) - 60
		if start < 0 {
			start = 0
		}
		result.RawKlines = klines[start:]
	}
	return result
}

func calculatePeriodValues(periods []int, calculate func(int) float64) map[string]float64 {
	result := make(map[string]float64, len(periods))
	for _, period := range periods {
		if period > 0 {
			result[strconv.Itoa(period)] = round6(calculate(period))
		}
	}
	return result
}

func calculateMACD(values []float64) *strategyMACD {
	if len(values) == 0 {
		return &strategyMACD{}
	}
	fast := emaSeries(values, 12)
	slow := emaSeries(values, 26)
	dif := make([]float64, len(values))
	for index := range values {
		dif[index] = fast[index] - slow[index]
	}
	dea := emaSeries(dif, 9)
	last := len(values) - 1
	return &strategyMACD{DIF: round6(dif[last]), DEA: round6(dea[last]), Histogram: round6((dif[last] - dea[last]) * 2)}
}

func emaSeries(values []float64, period int) []float64 {
	result := make([]float64, len(values))
	if len(values) == 0 {
		return result
	}
	alpha, current := 2.0/float64(period+1), values[0]
	result[0] = current
	for index, value := range values[1:] {
		current = alpha*value + (1-alpha)*current
		result[index+1] = current
	}
	return result
}

func calculateBOLL(values []float64, period int) strategyBOLL {
	if len(values) == 0 || period <= 0 {
		return strategyBOLL{}
	}
	start := len(values) - period
	if start < 0 {
		start = 0
	}
	window := values[start:]
	var total float64
	for _, value := range window {
		total += value
	}
	middle := total / float64(len(window))
	var variance float64
	for _, value := range window {
		variance += math.Pow(value-middle, 2)
	}
	deviation := math.Sqrt(variance / float64(len(window)))
	return strategyBOLL{Upper: round6(middle + 2*deviation), Middle: round6(middle), Lower: round6(middle - 2*deviation)}
}

func ema(values []float64, period int) float64 {
	if len(values) == 0 {
		return 0
	}
	alpha, current := 2.0/float64(period+1), values[0]
	for _, value := range values[1:] {
		current = alpha*value + (1-alpha)*current
	}
	return current
}

func rsi(values []float64, period int) float64 {
	if len(values) < 2 {
		return 0
	}
	start := len(values) - period - 1
	if start < 0 {
		start = 0
	}
	var gains, losses float64
	for index := start + 1; index < len(values); index++ {
		delta := values[index] - values[index-1]
		if delta >= 0 {
			gains += delta
		} else {
			losses -= delta
		}
	}
	if losses == 0 {
		return 100
	}
	return 100 - 100/(1+gains/losses)
}

func atr(highs, lows, closes []float64, period int) float64 {
	if len(closes) < 2 {
		return 0
	}
	start := len(closes) - period
	if start < 1 {
		start = 1
	}
	var total float64
	for index := start; index < len(closes); index++ {
		total += math.Max(highs[index]-lows[index], math.Max(math.Abs(highs[index]-closes[index-1]), math.Abs(lows[index]-closes[index-1])))
	}
	return total / float64(len(closes)-start)
}

func parseNumber(value string) float64 {
	number, _ := strconv.ParseFloat(value, 64)
	return number
}

func round6(value float64) float64 { return math.Round(value*1e6) / 1e6 }

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(data)
}
