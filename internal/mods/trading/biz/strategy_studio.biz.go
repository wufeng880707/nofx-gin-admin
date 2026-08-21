package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

func (a *Catalog) DefaultStrategyConfig() *schema.StrategyStudioConfig {
	config := schema.DefaultStrategyStudioConfig()
	return &config
}

func (a *Catalog) PreviewStrategy(_ context.Context, form *schema.StrategyPreviewForm) (*schema.StrategyPreview, error) {
	config, err := schema.DecodeStrategyStudioConfig(form.Config)
	if err != nil {
		return nil, errors.BadRequest("trading_strategy_preview_invalid", "Unable to preview strategy: %v", err)
	}
	return buildStrategyPreview(config, normalizePromptVariant(form.PromptVariant)), nil
}

func (a *Catalog) EstimateStrategyTokens(_ context.Context, form *schema.StrategyTokenEstimateForm) (*schema.StrategyTokenEstimate, error) {
	config, err := schema.DecodeStrategyStudioConfig(form.Config)
	if err != nil {
		return nil, errors.BadRequest("trading_strategy_token_estimate_invalid", "Unable to estimate strategy tokens: %v", err)
	}
	if config.StrategyType == "grid" {
		return &schema.StrategyTokenEstimate{Suggestions: []string{"网格策略由本地确定性运行时计算，不调用 AI 模型"}}, nil
	}
	estimate := config.EstimateTokens()
	return &estimate, nil
}

// TestStrategy performs a real model invocation with synthetic context. It never
// creates a trade command or contacts an exchange trading endpoint.
func (a *Catalog) TestStrategy(ctx context.Context, form *schema.StrategyTestForm) (*schema.StrategyTestResult, error) {
	config, configErr := schema.DecodeStrategyStudioConfig(form.Config)
	if configErr != nil {
		return nil, errors.BadRequest("trading_strategy_test_invalid", "Unable to test strategy: %v", configErr)
	}
	if config.StrategyType == "grid" {
		return nil, errors.Conflict("trading_grid_ai_test_not_applicable", "Grid strategy is deterministic and does not call an AI model")
	}
	preview, err := a.PreviewStrategy(ctx, &schema.StrategyPreviewForm{Config: form.Config, PromptVariant: form.PromptVariant})
	if err != nil {
		return nil, err
	}
	model, err := a.getAIModel(ctx, strings.TrimSpace(form.AIModelID))
	if err != nil {
		return nil, err
	}
	if !model.Enabled {
		return nil, errors.Conflict("trading_model_disabled", "AI model is disabled")
	}
	if model.APIKeyCiphertext == "" {
		return nil, errors.BadRequest("trading_model_api_key_missing", "AI model API key is missing")
	}
	result, err := a.LLM.Invoke(ctx, model, llm.InvocationRequest{
		Purpose: "strategy_test", SystemPrompt: preview.SystemPrompt,
		UserPrompt: `This is a dry-run strategy test. Do not execute or request a trade.
Return a concise structured example decision using this synthetic context:
- Account equity: 1000 USDT; available balance: 1000 USDT; positions: none.
- BTCUSDT: price 60000, EMA20 59800, EMA50 59500, RSI14 56, ATR14 800.
Explain whether the result is HOLD or a hypothetical trade candidate.`,
		MaxTokens: 512,
	})
	if err != nil {
		return nil, errors.BadRequest("trading_strategy_ai_test_failed", "Strategy AI test failed: %v", err)
	}
	return &schema.StrategyTestResult{
		StrategyPreview: *preview,
		AIModelName:     model.Name,
		Provider:        result.Provider,
		ModelName:       result.ModelName,
		RequestID:       result.RequestID,
		AIResponse:      result.Content,
		InputTokens:     result.InputTokens,
		OutputTokens:    result.OutputTokens,
		TotalTokens:     result.TotalTokens,
		LatencyMS:       result.DurationMS,
		Ready:           true,
		Note:            "Real AI dry-run completed with synthetic market data. No exchange order was submitted.",
	}, nil
}

func buildStrategyPreview(config *schema.StrategyStudioConfig, variant string) *schema.StrategyPreview {
	if config.StrategyType == "grid" {
		grid := config.GridConfig
		return &schema.StrategyPreview{PromptVariant: "deterministic", SystemPrompt: fmt.Sprintf(
			"Deterministic grid plan: %s %s %s, range %.8f-%.8f, %d %s levels, investment %.2f USDT, max position %.2f USDT, reserve %.0f%%, max open orders %d, stop loss %.2f%%, take profit %.2f%%, max drawdown %.2f%%. Simulation only; no AI call and no trade command.",
			grid.Symbol, grid.Market, grid.Direction, grid.LowerPrice, grid.UpperPrice, grid.GridCount,
			grid.PriceMode, grid.InvestmentUSDT, grid.MaxPositionUSDT, grid.ReserveRatio*100,
			grid.MaxOpenOrders, grid.StopLossPct, grid.TakeProfitPct, grid.MaxDrawdownPct),
			Summary: map[string]interface{}{"strategy_type": "grid", "symbol": grid.Symbol, "market": grid.Market,
				"grid_count": grid.GridCount, "simulation_only": true}}
	}
	ai := config.AIConfig
	coin := ai.CoinSource
	indicatorNames := make([]string, 0, 10)
	for name, enabled := range map[string]bool{
		"EMA": ai.Indicators.EnableEMA, "MACD": ai.Indicators.EnableMACD,
		"RSI": ai.Indicators.EnableRSI, "ATR": ai.Indicators.EnableATR,
		"BOLL": ai.Indicators.EnableBOLL, "Volume": ai.Indicators.EnableVolume,
		"Open Interest": ai.Indicators.EnableOI, "Funding Rate": ai.Indicators.EnableFundingRate,
		"Quant OI": ai.Indicators.EnableQuantOI, "NetFlow": ai.Indicators.EnableQuantNetflow,
	} {
		if enabled {
			indicatorNames = append(indicatorNames, name)
		}
	}
	sort.Strings(indicatorNames)
	sourceDescription := coin.SourceType
	switch coin.SourceType {
	case "static":
		sourceDescription = "static symbols: " + strings.Join(coin.StaticCoins, ", ")
	case "ai500":
		sourceDescription = fmt.Sprintf("AI500 top %d", coin.AI500Limit)
	case "oi_top":
		sourceDescription = fmt.Sprintf("open-interest growth top %d", coin.OITopLimit)
	case "mixed":
		sourceDescription = fmt.Sprintf("mixed AI500 top %d and OI top %d", coin.AI500Limit, coin.OITopLimit)
	}
	risk := ai.RiskControl
	prompt := fmt.Sprintf(`You are XTrading's risk-controlled trading analysis model.

Objective:
- Analyze configured candidates and return structured trade decisions. Never bypass application risk controls.
- Prompt profile: %s.

Market scope:
- Candidate source: %s.
- Timeframes: %s; primary timeframe: %s; candles per timeframe: %d.
- Indicators and market context: %s.

Hard risk boundaries:
- Maximum concurrent positions: %d.
- Maximum leverage: BTC/ETH %dx, altcoins %dx.
- Maximum margin usage: %.0f%%.
- Minimum position size: %.2f USDT.
- Minimum risk/reward ratio: %.2f.
- Minimum confidence to open: %d/100.

Output rules:
- Return HOLD when evidence conflicts or confidence is below the threshold.
- Every OPEN decision must include symbol, side, confidence, leverage, position size, stop loss, take profit and concise evidence.
- Do not reveal hidden reasoning or credentials. Return conclusions and short supporting reasons only.`,
		variant, sourceDescription, strings.Join(ai.Indicators.Klines.SelectedTimeframes, ", "),
		ai.Indicators.Klines.PrimaryTimeframe, ai.Indicators.Klines.PrimaryCount,
		strings.Join(indicatorNames, ", "), risk.MaxPositions, risk.BTCETHMaxLeverage,
		risk.AltcoinMaxLeverage, risk.MaxMarginUsage*100, risk.MinPositionSize,
		risk.MinRiskRewardRatio, risk.MinConfidence)
	if custom := strings.TrimSpace(ai.CustomPrompt); custom != "" {
		prompt += "\n\nUser strategy constraints:\n" + custom
	}
	sections := []struct {
		title string
		value string
	}{
		{"Role definition", ai.PromptSections.RoleDefinition},
		{"Trading frequency", ai.PromptSections.TradingFrequency},
		{"Entry standards", ai.PromptSections.EntryStandards},
		{"Decision process", ai.PromptSections.DecisionProcess},
	}
	for _, section := range sections {
		if value := strings.TrimSpace(section.value); value != "" {
			prompt += "\n\n" + section.title + ":\n" + value
		}
	}
	return &schema.StrategyPreview{
		SystemPrompt:  prompt,
		PromptVariant: variant,
		Summary: map[string]interface{}{
			"coin_source":       coin.SourceType,
			"primary_timeframe": ai.Indicators.Klines.PrimaryTimeframe,
			"timeframes":        ai.Indicators.Klines.SelectedTimeframes,
			"indicator_count":   len(indicatorNames),
			"ema_periods":       ai.Indicators.EMAPeriods,
			"rsi_periods":       ai.Indicators.RSIPeriods,
			"atr_periods":       ai.Indicators.ATRPeriods,
			"boll_periods":      ai.Indicators.BOLLPeriods,
			"max_positions":     risk.MaxPositions,
			"max_margin_usage":  risk.MaxMarginUsage,
		},
	}
}

func normalizePromptVariant(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "conservative", "aggressive":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "balanced"
	}
}
