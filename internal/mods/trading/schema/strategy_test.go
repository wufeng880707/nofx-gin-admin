package schema

import (
	"encoding/json"
	"testing"
)

func TestDefaultStrategyStudioConfigIsValid(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
	if !config.AIConfig.Indicators.EnableRawKlines {
		t.Fatal("raw klines must always be enabled")
	}
}

func TestDecodeStrategyStudioConfigNormalizesSourceFlagsAndSymbols(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.AIConfig.CoinSource.SourceType = "mixed"
	config.AIConfig.CoinSource.UseAI500 = false
	config.AIConfig.CoinSource.UseOITop = false
	config.AIConfig.CoinSource.StaticCoins = []string{" btc/usdt ", "BTCUSDT", "ethusdt"}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStrategyStudioConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.AIConfig.CoinSource.UseAI500 || !decoded.AIConfig.CoinSource.UseOITop {
		t.Fatalf("source flags were not normalized: %+v", decoded.AIConfig.CoinSource)
	}
	if len(decoded.AIConfig.CoinSource.StaticCoins) != 2 || decoded.AIConfig.CoinSource.StaticCoins[1] != "ETHUSDT" {
		t.Fatalf("symbols were not normalized: %v", decoded.AIConfig.CoinSource.StaticCoins)
	}
}

func TestStrategyStudioConfigRejectsPrimaryOutsideSelectedTimeframes(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.AIConfig.Indicators.Klines.PrimaryTimeframe = "4h"
	if err := config.Validate(); err == nil {
		t.Fatal("expected primary timeframe validation error")
	}
}

func TestDecodeStrategyStudioConfigNormalizesIndicatorPeriods(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.AIConfig.Indicators.EMAPeriods = []int{50, 20, 20}
	config.AIConfig.Indicators.RSIPeriods = nil
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeStrategyStudioConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.AIConfig.Indicators.EMAPeriods; len(got) != 2 || got[0] != 20 || got[1] != 50 {
		t.Fatalf("EMA periods were not normalized: %v", got)
	}
	if got := decoded.AIConfig.Indicators.RSIPeriods; len(got) != 2 || got[0] != 7 || got[1] != 14 {
		t.Fatalf("RSI defaults were not applied: %v", got)
	}
}

func TestEstimateTokensIncludesPromptAndMarketData(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.AIConfig.PromptSections.RoleDefinition = "严格遵守风险边界的交易分析模型"
	estimate := config.EstimateTokens()
	if estimate.Total <= 0 {
		t.Fatalf("expected positive total, got %d", estimate.Total)
	}
	if estimate.Breakdown.SystemPrompt <= 0 || estimate.Breakdown.MarketData <= 0 {
		t.Fatalf("unexpected token breakdown: %+v", estimate.Breakdown)
	}
	if len(estimate.ModelLimits) == 0 {
		t.Fatal("expected model context limits")
	}
}

func TestStrategyStudioConfigRejectsInvalidRankingDuration(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.AIConfig.Indicators.OIRankingDuration = "7d"
	if err := config.Validate(); err == nil {
		t.Fatal("expected ranking duration validation error")
	}
}

func TestGridStrategyConfigHasIndependentRiskValidation(t *testing.T) {
	config := DefaultStrategyStudioConfig()
	config.StrategyType = "grid"
	if err := config.Validate(); err != nil {
		t.Fatalf("default grid config is invalid: %v", err)
	}
	config.GridConfig.MaxPositionUSDT = config.GridConfig.InvestmentUSDT + 1
	if err := config.Validate(); err == nil {
		t.Fatal("expected grid max position validation error")
	}
	config = DefaultStrategyStudioConfig()
	config.StrategyType = "grid"
	config.GridConfig.Market = "spot"
	config.GridConfig.Direction = "short"
	if err := config.Validate(); err == nil {
		t.Fatal("expected spot short grid validation error")
	}
}
