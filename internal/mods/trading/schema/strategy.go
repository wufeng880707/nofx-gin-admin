package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

var supportedStrategyTimeframes = map[string]struct{}{
	"1m": {}, "3m": {}, "5m": {}, "15m": {}, "30m": {}, "1h": {}, "2h": {}, "4h": {}, "6h": {}, "8h": {}, "12h": {}, "1d": {}, "3d": {}, "1w": {},
}

type StrategyStudioConfig struct {
	StrategyType            string                  `json:"strategy_type"`
	Language                string                  `json:"language"`
	AIConfig                AITradingConfig         `json:"ai_config"`
	GridConfig              GridStrategyConfig      `json:"grid_config"`
	SpotCoreSatelliteConfig SpotCoreSatelliteConfig `json:"spot_core_satellite_config"`
}

type GridStrategyConfig struct {
	Symbol          string  `json:"symbol"`
	Market          string  `json:"market"`
	Direction       string  `json:"direction"`
	PriceMode       string  `json:"price_mode"`
	LowerPrice      float64 `json:"lower_price"`
	UpperPrice      float64 `json:"upper_price"`
	GridCount       int     `json:"grid_count"`
	InvestmentUSDT  float64 `json:"investment_usdt"`
	MaxPositionUSDT float64 `json:"max_position_usdt"`
	MaxOpenOrders   int     `json:"max_open_orders"`
	ReserveRatio    float64 `json:"reserve_ratio"`
	Leverage        int     `json:"leverage"`
	StopLossPct     float64 `json:"stop_loss_pct"`
	TakeProfitPct   float64 `json:"take_profit_pct"`
	MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
}

type AITradingConfig struct {
	CoinSource     CoinSourceConfig     `json:"coin_source"`
	Indicators     IndicatorConfig      `json:"indicators"`
	RiskControl    RiskControlConfig    `json:"risk_control"`
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
	CustomPrompt   string               `json:"custom_prompt,omitempty"`
}

type PromptSectionsConfig struct {
	RoleDefinition   string `json:"role_definition,omitempty"`
	TradingFrequency string `json:"trading_frequency,omitempty"`
	EntryStandards   string `json:"entry_standards,omitempty"`
	DecisionProcess  string `json:"decision_process,omitempty"`
}

type CoinSourceConfig struct {
	SourceType    string   `json:"source_type"`
	StaticCoins   []string `json:"static_coins,omitempty"`
	ExcludedCoins []string `json:"excluded_coins,omitempty"`
	UseAI500      bool     `json:"use_ai500"`
	AI500Limit    int      `json:"ai500_limit,omitempty"`
	UseOITop      bool     `json:"use_oi_top"`
	OITopLimit    int      `json:"oi_top_limit,omitempty"`
}

type IndicatorConfig struct {
	Klines                 KlineConfig `json:"klines"`
	EnableRawKlines        bool        `json:"enable_raw_klines"`
	EnableEMA              bool        `json:"enable_ema"`
	EnableMACD             bool        `json:"enable_macd"`
	EnableRSI              bool        `json:"enable_rsi"`
	EnableATR              bool        `json:"enable_atr"`
	EnableBOLL             bool        `json:"enable_boll"`
	EnableVolume           bool        `json:"enable_volume"`
	EnableOI               bool        `json:"enable_oi"`
	EnableFundingRate      bool        `json:"enable_funding_rate"`
	EnableQuantData        bool        `json:"enable_quant_data"`
	EnableQuantOI          bool        `json:"enable_quant_oi"`
	EnableQuantNetflow     bool        `json:"enable_quant_netflow"`
	EnableOIRanking        bool        `json:"enable_oi_ranking"`
	EnableNetflowRanking   bool        `json:"enable_netflow_ranking"`
	EnablePriceRanking     bool        `json:"enable_price_ranking"`
	EMAPeriods             []int       `json:"ema_periods,omitempty"`
	RSIPeriods             []int       `json:"rsi_periods,omitempty"`
	ATRPeriods             []int       `json:"atr_periods,omitempty"`
	BOLLPeriods            []int       `json:"boll_periods,omitempty"`
	OIRankingDuration      string      `json:"oi_ranking_duration,omitempty"`
	OIRankingLimit         int         `json:"oi_ranking_limit,omitempty"`
	NetflowRankingDuration string      `json:"netflow_ranking_duration,omitempty"`
	NetflowRankingLimit    int         `json:"netflow_ranking_limit,omitempty"`
	PriceRankingDuration   string      `json:"price_ranking_duration,omitempty"`
	PriceRankingLimit      int         `json:"price_ranking_limit,omitempty"`
}

type KlineConfig struct {
	PrimaryTimeframe     string   `json:"primary_timeframe"`
	PrimaryCount         int      `json:"primary_count"`
	SelectedTimeframes   []string `json:"selected_timeframes"`
	EnableMultiTimeframe bool     `json:"enable_multi_timeframe"`
}

type RiskControlConfig struct {
	MaxPositions                 int     `json:"max_positions"`
	BTCETHMaxLeverage            int     `json:"btc_eth_max_leverage"`
	AltcoinMaxLeverage           int     `json:"altcoin_max_leverage"`
	BTCETHMaxPositionValueRatio  float64 `json:"btc_eth_max_position_value_ratio"`
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`
	MaxMarginUsage               float64 `json:"max_margin_usage"`
	MinPositionSize              float64 `json:"min_position_size"`
	MinRiskRewardRatio           float64 `json:"min_risk_reward_ratio"`
	MinConfidence                int     `json:"min_confidence"`
}

type StrategyPreviewForm struct {
	Config        json.RawMessage `json:"config" binding:"required"`
	PromptVariant string          `json:"prompt_variant"`
}

type StrategyTokenEstimateForm struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

type StrategyTestForm struct {
	Config        json.RawMessage `json:"config" binding:"required"`
	PromptVariant string          `json:"prompt_variant"`
	AIModelID     string          `json:"ai_model_id" binding:"required"`
}

type StrategyDuplicateForm struct {
	Name string `json:"name" binding:"required,max=128"`
}

func (a *StrategyDuplicateForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return fmt.Errorf("strategy name is required")
	}
	return nil
}

type StrategyPreview struct {
	SystemPrompt  string                 `json:"system_prompt"`
	PromptVariant string                 `json:"prompt_variant"`
	Summary       map[string]interface{} `json:"config_summary"`
}

type StrategyTestResult struct {
	StrategyPreview
	AIModelName  string `json:"ai_model_name"`
	Provider     string `json:"provider"`
	ModelName    string `json:"model_name"`
	RequestID    string `json:"request_id"`
	AIResponse   string `json:"ai_response"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	Ready        bool   `json:"ready"`
	Note         string `json:"note"`
}

type StrategyTokenEstimate struct {
	Total       int                    `json:"total"`
	Breakdown   StrategyTokenBreakdown `json:"breakdown"`
	ModelLimits []StrategyModelLimit   `json:"model_limits"`
	Suggestions []string               `json:"suggestions"`
}

type StrategyTokenBreakdown struct {
	SystemPrompt  int `json:"system_prompt"`
	MarketData    int `json:"market_data"`
	RankingData   int `json:"ranking_data"`
	QuantData     int `json:"quant_data"`
	FixedOverhead int `json:"fixed_overhead"`
}

type StrategyModelLimit struct {
	Name         string `json:"name"`
	ContextLimit int    `json:"context_limit"`
	UsagePct     int    `json:"usage_pct"`
	Level        string `json:"level"`
}

func DefaultStrategyStudioConfig() StrategyStudioConfig {
	return StrategyStudioConfig{
		StrategyType: "ai_trading",
		Language:     "zh",
		AIConfig: AITradingConfig{
			CoinSource: CoinSourceConfig{SourceType: "static", StaticCoins: []string{"BTCUSDT", "ETHUSDT"}},
			Indicators: IndicatorConfig{
				Klines:          KlineConfig{PrimaryTimeframe: "15m", PrimaryCount: 30, SelectedTimeframes: []string{"5m", "15m", "1h"}, EnableMultiTimeframe: true},
				EnableRawKlines: true, EnableEMA: true, EnableMACD: true, EnableRSI: true, EnableATR: true,
				EnableVolume: true, EnableOI: true, EnableFundingRate: true,
				EMAPeriods: []int{20, 50}, RSIPeriods: []int{7, 14}, ATRPeriods: []int{14}, BOLLPeriods: []int{20},
				OIRankingDuration: "1h", OIRankingLimit: 10, NetflowRankingDuration: "1h", NetflowRankingLimit: 10,
				PriceRankingDuration: "1h,4h,24h", PriceRankingLimit: 10,
			},
			RiskControl: RiskControlConfig{
				MaxPositions: 3, BTCETHMaxLeverage: 5, AltcoinMaxLeverage: 3,
				BTCETHMaxPositionValueRatio: 0.3, AltcoinMaxPositionValueRatio: 0.15,
				MaxMarginUsage: 0.6, MinPositionSize: 10, MinRiskRewardRatio: 2, MinConfidence: 75,
			},
		},
		GridConfig: GridStrategyConfig{Symbol: "BTCUSDT", Market: "spot", Direction: "neutral", PriceMode: "arithmetic",
			LowerPrice: 50000, UpperPrice: 100000, GridCount: 20, InvestmentUSDT: 1000,
			MaxPositionUSDT: 500, MaxOpenOrders: 10, ReserveRatio: 0.2, Leverage: 1,
			StopLossPct: 8, TakeProfitPct: 20, MaxDrawdownPct: 15},
		SpotCoreSatelliteConfig: DefaultSpotCoreSatelliteConfig(),
	}
}

func DecodeStrategyStudioConfig(raw json.RawMessage) (*StrategyStudioConfig, error) {
	var config StrategyStudioConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("decode strategy studio config: %w", err)
	}
	config.Normalize()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

func (a *StrategyStudioConfig) Normalize() {
	defaults := DefaultStrategyStudioConfig()
	if a.StrategyType == "" {
		a.StrategyType = defaults.StrategyType
	}
	if a.Language == "" {
		a.Language = defaults.Language
	}
	a.SpotCoreSatelliteConfig.Normalize()
	grid := &a.GridConfig
	grid.Symbol = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(grid.Symbol), "/", ""))
	grid.Market = strings.ToLower(strings.TrimSpace(grid.Market))
	grid.Direction = strings.ToLower(strings.TrimSpace(grid.Direction))
	grid.PriceMode = strings.ToLower(strings.TrimSpace(grid.PriceMode))
	if grid.Symbol == "" {
		grid.Symbol = defaults.GridConfig.Symbol
	}
	if grid.Market == "" {
		grid.Market = defaults.GridConfig.Market
	}
	if grid.Direction == "" {
		grid.Direction = defaults.GridConfig.Direction
	}
	if grid.PriceMode == "" {
		grid.PriceMode = defaults.GridConfig.PriceMode
	}
	if grid.LowerPrice == 0 {
		grid.LowerPrice = defaults.GridConfig.LowerPrice
	}
	if grid.UpperPrice == 0 {
		grid.UpperPrice = defaults.GridConfig.UpperPrice
	}
	if grid.GridCount == 0 {
		grid.GridCount = defaults.GridConfig.GridCount
	}
	if grid.InvestmentUSDT == 0 {
		grid.InvestmentUSDT = defaults.GridConfig.InvestmentUSDT
	}
	if grid.MaxPositionUSDT == 0 {
		grid.MaxPositionUSDT = defaults.GridConfig.MaxPositionUSDT
	}
	if grid.MaxOpenOrders == 0 {
		grid.MaxOpenOrders = defaults.GridConfig.MaxOpenOrders
	}
	if grid.ReserveRatio == 0 {
		grid.ReserveRatio = defaults.GridConfig.ReserveRatio
	}
	if grid.Leverage == 0 {
		grid.Leverage = defaults.GridConfig.Leverage
	}
	if grid.StopLossPct == 0 {
		grid.StopLossPct = defaults.GridConfig.StopLossPct
	}
	if grid.TakeProfitPct == 0 {
		grid.TakeProfitPct = defaults.GridConfig.TakeProfitPct
	}
	if grid.MaxDrawdownPct == 0 {
		grid.MaxDrawdownPct = defaults.GridConfig.MaxDrawdownPct
	}
	coin := &a.AIConfig.CoinSource
	coin.SourceType = strings.ToLower(strings.TrimSpace(coin.SourceType))
	if coin.SourceType == "" {
		coin.SourceType = defaults.AIConfig.CoinSource.SourceType
	}
	coin.UseAI500 = coin.SourceType == "ai500" || coin.SourceType == "mixed"
	coin.UseOITop = coin.SourceType == "oi_top" || coin.SourceType == "mixed"
	coin.StaticCoins = normalizeSymbols(coin.StaticCoins)
	coin.ExcludedCoins = normalizeSymbols(coin.ExcludedCoins)
	if coin.AI500Limit == 0 {
		coin.AI500Limit = 10
	}
	if coin.OITopLimit == 0 {
		coin.OITopLimit = 10
	}
	indicators := &a.AIConfig.Indicators
	indicators.EnableRawKlines = true
	if indicators.Klines.PrimaryTimeframe == "" {
		indicators.Klines.PrimaryTimeframe = defaults.AIConfig.Indicators.Klines.PrimaryTimeframe
	}
	if indicators.Klines.PrimaryCount == 0 {
		indicators.Klines.PrimaryCount = defaults.AIConfig.Indicators.Klines.PrimaryCount
	}
	if len(indicators.Klines.SelectedTimeframes) == 0 {
		indicators.Klines.SelectedTimeframes = []string{indicators.Klines.PrimaryTimeframe}
	}
	indicators.Klines.SelectedTimeframes = uniqueStrings(indicators.Klines.SelectedTimeframes)
	indicators.Klines.EnableMultiTimeframe = len(indicators.Klines.SelectedTimeframes) > 1
	indicators.EMAPeriods = normalizePeriods(indicators.EMAPeriods, defaults.AIConfig.Indicators.EMAPeriods)
	indicators.RSIPeriods = normalizePeriods(indicators.RSIPeriods, defaults.AIConfig.Indicators.RSIPeriods)
	indicators.ATRPeriods = normalizePeriods(indicators.ATRPeriods, defaults.AIConfig.Indicators.ATRPeriods)
	indicators.BOLLPeriods = normalizePeriods(indicators.BOLLPeriods, defaults.AIConfig.Indicators.BOLLPeriods)
	if indicators.OIRankingDuration == "" {
		indicators.OIRankingDuration = defaults.AIConfig.Indicators.OIRankingDuration
	}
	if indicators.OIRankingLimit == 0 {
		indicators.OIRankingLimit = defaults.AIConfig.Indicators.OIRankingLimit
	}
	if indicators.NetflowRankingDuration == "" {
		indicators.NetflowRankingDuration = defaults.AIConfig.Indicators.NetflowRankingDuration
	}
	if indicators.NetflowRankingLimit == 0 {
		indicators.NetflowRankingLimit = defaults.AIConfig.Indicators.NetflowRankingLimit
	}
	if indicators.PriceRankingDuration == "" {
		indicators.PriceRankingDuration = defaults.AIConfig.Indicators.PriceRankingDuration
	}
	if indicators.PriceRankingLimit == 0 {
		indicators.PriceRankingLimit = defaults.AIConfig.Indicators.PriceRankingLimit
	}
	risk := &a.AIConfig.RiskControl
	if risk.MaxPositions == 0 {
		risk.MaxPositions = defaults.AIConfig.RiskControl.MaxPositions
	}
	if risk.BTCETHMaxLeverage == 0 {
		risk.BTCETHMaxLeverage = defaults.AIConfig.RiskControl.BTCETHMaxLeverage
	}
	if risk.AltcoinMaxLeverage == 0 {
		risk.AltcoinMaxLeverage = defaults.AIConfig.RiskControl.AltcoinMaxLeverage
	}
	if risk.BTCETHMaxPositionValueRatio == 0 {
		risk.BTCETHMaxPositionValueRatio = defaults.AIConfig.RiskControl.BTCETHMaxPositionValueRatio
	}
	if risk.AltcoinMaxPositionValueRatio == 0 {
		risk.AltcoinMaxPositionValueRatio = defaults.AIConfig.RiskControl.AltcoinMaxPositionValueRatio
	}
	if risk.MaxMarginUsage == 0 {
		risk.MaxMarginUsage = defaults.AIConfig.RiskControl.MaxMarginUsage
	}
	if risk.MinPositionSize == 0 {
		risk.MinPositionSize = defaults.AIConfig.RiskControl.MinPositionSize
	}
	if risk.MinRiskRewardRatio == 0 {
		risk.MinRiskRewardRatio = defaults.AIConfig.RiskControl.MinRiskRewardRatio
	}
	if risk.MinConfidence == 0 {
		risk.MinConfidence = defaults.AIConfig.RiskControl.MinConfidence
	}
}

func (a StrategyStudioConfig) Validate() error {
	if a.StrategyType != "ai_trading" && a.StrategyType != "grid" && a.StrategyType != SpotStrategyType {
		return fmt.Errorf("unsupported strategy type %q", a.StrategyType)
	}
	if a.StrategyType == "grid" {
		return a.GridConfig.Validate()
	}
	if a.StrategyType == SpotStrategyType {
		return a.SpotCoreSatelliteConfig.Validate()
	}
	coin := a.AIConfig.CoinSource
	if coin.SourceType != "static" && coin.SourceType != "ai500" && coin.SourceType != "oi_top" && coin.SourceType != "mixed" {
		return fmt.Errorf("unsupported coin source %q", coin.SourceType)
	}
	if coin.SourceType == "static" && len(coin.StaticCoins) == 0 {
		return fmt.Errorf("static coin source requires at least one symbol")
	}
	if coin.UseAI500 && (coin.AI500Limit < 1 || coin.AI500Limit > 30) {
		return fmt.Errorf("AI500 candidate coin limit must be between 1 and 30")
	}
	if coin.UseOITop && (coin.OITopLimit < 1 || coin.OITopLimit > 30) {
		return fmt.Errorf("candidate coin limit must be between 1 and 30")
	}
	klines := a.AIConfig.Indicators.Klines
	if _, ok := supportedStrategyTimeframes[klines.PrimaryTimeframe]; !ok {
		return fmt.Errorf("unsupported primary timeframe %q", klines.PrimaryTimeframe)
	}
	if klines.PrimaryCount < 20 || klines.PrimaryCount > 200 {
		return fmt.Errorf("kline count must be between 20 and 200")
	}
	if len(klines.SelectedTimeframes) > 6 {
		return fmt.Errorf("at most 6 timeframes may be selected")
	}
	primarySelected := false
	for _, timeframe := range klines.SelectedTimeframes {
		if _, ok := supportedStrategyTimeframes[timeframe]; !ok {
			return fmt.Errorf("unsupported timeframe %q", timeframe)
		}
		primarySelected = primarySelected || timeframe == klines.PrimaryTimeframe
	}
	if !primarySelected {
		return fmt.Errorf("primary timeframe must be included in selected timeframes")
	}
	indicators := a.AIConfig.Indicators
	if err := validatePeriods("EMA", indicators.EMAPeriods); err != nil {
		return err
	}
	if err := validatePeriods("RSI", indicators.RSIPeriods); err != nil {
		return err
	}
	if err := validatePeriods("ATR", indicators.ATRPeriods); err != nil {
		return err
	}
	if err := validatePeriods("BOLL", indicators.BOLLPeriods); err != nil {
		return err
	}
	if err := validateRanking("OI", indicators.OIRankingDuration, indicators.OIRankingLimit, false); err != nil {
		return err
	}
	if err := validateRanking("NetFlow", indicators.NetflowRankingDuration, indicators.NetflowRankingLimit, false); err != nil {
		return err
	}
	if err := validateRanking("price", indicators.PriceRankingDuration, indicators.PriceRankingLimit, true); err != nil {
		return err
	}
	risk := a.AIConfig.RiskControl
	if risk.MaxPositions < 1 || risk.MaxPositions > 20 {
		return fmt.Errorf("max positions must be between 1 and 20")
	}
	if risk.BTCETHMaxLeverage < 1 || risk.BTCETHMaxLeverage > 125 || risk.AltcoinMaxLeverage < 1 || risk.AltcoinMaxLeverage > 125 {
		return fmt.Errorf("leverage must be between 1 and 125")
	}
	if risk.MaxMarginUsage <= 0 || risk.MaxMarginUsage > 1 {
		return fmt.Errorf("max margin usage must be between 0 and 1")
	}
	if risk.MinPositionSize < 5 {
		return fmt.Errorf("minimum position size must be at least 5 USDT")
	}
	if risk.MinRiskRewardRatio < 0.5 || risk.MinRiskRewardRatio > 20 {
		return fmt.Errorf("risk reward ratio must be between 0.5 and 20")
	}
	if risk.MinConfidence < 1 || risk.MinConfidence > 100 {
		return fmt.Errorf("minimum confidence must be between 1 and 100")
	}
	if len(a.AIConfig.CustomPrompt) > 8000 {
		return fmt.Errorf("custom prompt must not exceed 8000 characters")
	}
	sections := a.AIConfig.PromptSections
	for name, value := range map[string]string{
		"role definition":   sections.RoleDefinition,
		"trading frequency": sections.TradingFrequency,
		"entry standards":   sections.EntryStandards,
		"decision process":  sections.DecisionProcess,
	} {
		if len(value) > 4000 {
			return fmt.Errorf("%s prompt must not exceed 4000 characters", name)
		}
	}
	return nil
}

func (a GridStrategyConfig) Validate() error {
	if a.Symbol == "" || len(a.Symbol) > 40 {
		return fmt.Errorf("grid symbol is required")
	}
	if a.Market != "spot" && a.Market != "futures" {
		return fmt.Errorf("grid market must be spot or futures")
	}
	if a.Direction != "neutral" && a.Direction != "long" && a.Direction != "short" {
		return fmt.Errorf("grid direction must be neutral, long or short")
	}
	if a.Market == "spot" && a.Direction == "short" {
		return fmt.Errorf("spot grid does not support short direction")
	}
	if a.PriceMode != "arithmetic" && a.PriceMode != "geometric" {
		return fmt.Errorf("grid price mode must be arithmetic or geometric")
	}
	if a.LowerPrice <= 0 || a.UpperPrice <= a.LowerPrice {
		return fmt.Errorf("grid upper price must be greater than lower price")
	}
	if a.GridCount < 2 || a.GridCount > 200 {
		return fmt.Errorf("grid count must be between 2 and 200")
	}
	if a.InvestmentUSDT < 10 || a.MaxPositionUSDT < 10 || a.MaxPositionUSDT > a.InvestmentUSDT {
		return fmt.Errorf("grid position limit must be between 10 and total investment")
	}
	if a.MaxOpenOrders < 1 || a.MaxOpenOrders > 50 || a.MaxOpenOrders > a.GridCount {
		return fmt.Errorf("grid open order limit must be between 1 and min(50, grid count)")
	}
	if a.ReserveRatio < 0.05 || a.ReserveRatio > 0.9 {
		return fmt.Errorf("grid reserve ratio must be between 0.05 and 0.9")
	}
	if a.Leverage < 1 || a.Leverage > 20 || (a.Market == "spot" && a.Leverage != 1) {
		return fmt.Errorf("grid leverage must be 1 for spot and between 1 and 20 for futures")
	}
	if a.StopLossPct <= 0 || a.StopLossPct > 50 || a.TakeProfitPct <= 0 || a.TakeProfitPct > 200 ||
		a.MaxDrawdownPct <= 0 || a.MaxDrawdownPct > 50 {
		return fmt.Errorf("grid stop loss, take profit or drawdown boundary is invalid")
	}
	return nil
}

func (a StrategyStudioConfig) EstimateTokens() StrategyTokenEstimate {
	indicators := a.AIConfig.Indicators
	coinCount := 3
	switch a.AIConfig.CoinSource.SourceType {
	case "static":
		coinCount = len(a.AIConfig.CoinSource.StaticCoins)
	case "ai500":
		coinCount = a.AIConfig.CoinSource.AI500Limit
	case "oi_top":
		coinCount = a.AIConfig.CoinSource.OITopLimit
	case "mixed":
		coinCount = a.AIConfig.CoinSource.AI500Limit + a.AIConfig.CoinSource.OITopLimit
	}
	if coinCount < 1 {
		coinCount = 1
	}
	timeframeCount := len(indicators.Klines.SelectedTimeframes)
	if timeframeCount < 1 {
		timeframeCount = 1
	}
	indicatorChars := 0
	if indicators.EnableEMA {
		indicatorChars += 10 * len(indicators.EMAPeriods)
	}
	if indicators.EnableMACD {
		indicatorChars += 30
	}
	if indicators.EnableRSI {
		indicatorChars += 10 * len(indicators.RSIPeriods)
	}
	if indicators.EnableATR {
		indicatorChars += 10 * len(indicators.ATRPeriods)
	}
	if indicators.EnableBOLL {
		indicatorChars += 18 * len(indicators.BOLLPeriods)
	}
	if indicators.EnableVolume {
		indicatorChars += 10
	}
	promptChars := 3000 + len(a.AIConfig.CustomPrompt) + len(a.AIConfig.PromptSections.RoleDefinition) +
		len(a.AIConfig.PromptSections.TradingFrequency) + len(a.AIConfig.PromptSections.EntryStandards) + len(a.AIConfig.PromptSections.DecisionProcess)
	promptDivisor := 2
	if a.Language == "en" {
		promptDivisor = 4
	}
	breakdown := StrategyTokenBreakdown{
		SystemPrompt:  promptChars / promptDivisor,
		MarketData:    coinCount * timeframeCount * indicators.Klines.PrimaryCount * (80 + indicatorChars) / 4,
		FixedOverhead: 200,
	}
	if indicators.EnableOI || indicators.EnableFundingRate {
		breakdown.MarketData += coinCount * 25
	}
	if indicators.EnableQuantData {
		perCoin := 0
		if indicators.EnableQuantOI {
			perCoin += 75
		}
		if indicators.EnableQuantNetflow {
			perCoin += 75
		}
		breakdown.QuantData = coinCount * perCoin
	}
	if indicators.EnableOIRanking {
		breakdown.RankingData += indicators.OIRankingLimit * 15
	}
	if indicators.EnableNetflowRanking {
		breakdown.RankingData += indicators.NetflowRankingLimit * 20
	}
	if indicators.EnablePriceRanking {
		breakdown.RankingData += indicators.PriceRankingLimit * len(strings.Split(indicators.PriceRankingDuration, ",")) * 10
	}
	subtotal := breakdown.SystemPrompt + breakdown.MarketData + breakdown.RankingData + breakdown.QuantData + breakdown.FixedOverhead
	total := subtotal * 115 / 100
	limits := map[string]int{"deepseek": 131072, "openai": 128000, "claude": 200000, "qwen": 131072, "gemini": 1000000}
	modelLimits := make([]StrategyModelLimit, 0, len(limits))
	for name, limit := range limits {
		pct, level := total*100/limit, "ok"
		if pct >= 100 {
			level = "danger"
		} else if pct >= 80 {
			level = "warning"
		}
		modelLimits = append(modelLimits, StrategyModelLimit{Name: name, ContextLimit: limit, UsagePct: pct, Level: level})
	}
	sort.Slice(modelLimits, func(i, j int) bool { return modelLimits[i].Name < modelLimits[j].Name })
	suggestions := make([]string, 0, 2)
	if total > 100000 && timeframeCount > 1 {
		suggestions = append(suggestions, "减少分析周期可以显著降低上下文占用")
	}
	if total > 100000 && coinCount > 5 {
		suggestions = append(suggestions, "减少候选币数量可以降低模型输入成本")
	}
	return StrategyTokenEstimate{Total: total, Breakdown: breakdown, ModelLimits: modelLimits, Suggestions: suggestions}
}

func normalizeSymbols(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "/", ""))
		if value != "" {
			result = append(result, value)
		}
	}
	return uniqueStrings(result)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizePeriods(values, defaults []int) []int {
	if len(values) == 0 {
		return append([]int(nil), defaults...)
	}
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func validatePeriods(name string, values []int) error {
	if len(values) < 1 || len(values) > 6 {
		return fmt.Errorf("%s periods must contain between 1 and 6 values", name)
	}
	for _, value := range values {
		if value < 2 || value > 400 {
			return fmt.Errorf("%s period must be between 2 and 400", name)
		}
	}
	return nil
}

func validateRanking(name, duration string, limit int, multiple bool) error {
	if limit < 1 || limit > 50 {
		return fmt.Errorf("%s ranking limit must be between 1 and 50", name)
	}
	parts := strings.Split(duration, ",")
	if !multiple && len(parts) != 1 {
		return fmt.Errorf("%s ranking supports one duration", name)
	}
	if len(parts) > 3 {
		return fmt.Errorf("%s ranking supports at most 3 durations", name)
	}
	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case "1h", "4h", "24h":
		default:
			return fmt.Errorf("unsupported %s ranking duration %q", name, part)
		}
	}
	return nil
}
