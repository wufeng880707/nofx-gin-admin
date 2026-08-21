package schema

import (
	"testing"
	"time"
)

func TestSpotCoreSatelliteConfigNormalizesExchangeSymbol(t *testing.T) {
	binance := DefaultSpotCoreSatelliteConfig()
	binance.Symbol = "BTC-USDT"
	binance.Normalize()
	if binance.Symbol != "BTCUSDT" {
		t.Fatalf("unexpected Binance symbol %q", binance.Symbol)
	}

	okx := DefaultSpotCoreSatelliteConfig()
	okx.Exchange = MarketExchangeOKX
	okx.Symbol = "BTCUSDT"
	okx.CoreMinimumRetentionRatio = 0
	okx.Normalize()
	if okx.Symbol != "BTC-USDT" {
		t.Fatalf("unexpected OKX symbol %q", okx.Symbol)
	}
	if okx.CoreMinimumRetentionRatio != 0 {
		t.Fatalf("explicit zero retention ratio was overwritten: %v", okx.CoreMinimumRetentionRatio)
	}
	if err := okx.Validate(); err != nil {
		t.Fatalf("zero retention ratio should be valid: %v", err)
	}
}

func TestSpotCoreSatelliteConfigRejectsUnsafeCapitalRatios(t *testing.T) {
	config := DefaultSpotCoreSatelliteConfig()
	config.MinimumCashRatio = 0.1
	config.CoreBudgetRatio = 0.6
	config.SatelliteBudgetRatio = 0.3
	if err := config.Validate(); err == nil {
		t.Fatal("expected unsafe cash ratio to fail validation")
	}
}

func TestSpotCoreSatelliteConfigPreservesZeroExecutionCosts(t *testing.T) {
	config := DefaultSpotCoreSatelliteConfig()
	config.FeeRateBPS = 0
	config.SlippageBPS = 0
	config.Normalize()
	if config.FeeRateBPS != 0 || config.SlippageBPS != 0 {
		t.Fatalf("explicit zero execution costs were overwritten: fee=%v slippage=%v", config.FeeRateBPS, config.SlippageBPS)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("zero execution costs should be valid: %v", err)
	}
}

func TestSpotCoreSatelliteConfigDefaultsDrawdownControlAndPreservesDisabled(t *testing.T) {
	config := DefaultSpotCoreSatelliteConfig()
	config.EnableDrawdownRiskControl = nil
	config.Normalize()
	if !config.DrawdownRiskControlEnabled() {
		t.Fatal("missing drawdown risk control switch must default to enabled")
	}

	disabled := false
	config.EnableDrawdownRiskControl = &disabled
	config.Normalize()
	if config.DrawdownRiskControlEnabled() {
		t.Fatal("explicitly disabled drawdown risk control was overwritten")
	}
}

func TestSpotBacktestCreateFormNormalizesTaskSymbol(t *testing.T) {
	form := SpotBacktestCreateForm{
		Exchange:       MarketExchangeOKX,
		Symbol:         "BTCUSDT",
		Interval:       "1d",
		StartTime:      time.Now().UTC().AddDate(0, -1, 0),
		EndTime:        time.Now().UTC().Add(-time.Hour),
		InitialCapital: "10000",
		Config:         DefaultSpotCoreSatelliteConfig(),
	}
	if err := form.Validate(); err != nil {
		t.Fatalf("validate form: %v", err)
	}
	if form.Symbol != "BTC-USDT" || form.Config.Symbol != "BTC-USDT" {
		t.Fatalf("OKX symbol was not normalized consistently: task=%q config=%q", form.Symbol, form.Config.Symbol)
	}
}
