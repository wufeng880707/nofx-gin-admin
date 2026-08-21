package runtime

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func TestRunSpotCoreSatelliteBacktestKeepsLotInventoryAndJSONFinite(t *testing.T) {
	config := schema.DefaultSpotCoreSatelliteConfig()
	config.LowAreaLookbackBars = 30
	config.TrendFilterPeriod = 20
	config.ResistanceLookbackBars = 10
	config.DistributionLookbackBars = 30
	config.LowAreaDrawdownPct = 0.2
	config.StabilizationBars = 2
	config.BreakoutConfirmBars = 2
	config.BreakoutVolumeMultiplier = 1
	config.ExecutionTimeframe = "4h"
	config.MaxSymbolExposureRatio = 0.30
	config.MaxDailyBuyRatio = 0.05

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]*schema.MarketKline, 0, 180)
	price := 100.0
	for index := 0; index < 180; index++ {
		switch {
		case index < 45:
			price = 100 + float64(index)*0.3
		case index < 85:
			price = 113.5 - float64(index-45)*0.8
		case index < 110:
			price = 82 + float64(index-85)*0.12
		default:
			price = 85 + float64(index-110)*0.8
		}
		bars = append(bars, &schema.MarketKline{Time: start.Add(time.Duration(index) * 4 * time.Hour).Unix(),
			Open: fmt.Sprintf("%.8f", price), High: fmt.Sprintf("%.8f", price*1.01),
			Low: fmt.Sprintf("%.8f", price*0.99), Close: fmt.Sprintf("%.8f", price),
			Volume: "1000", QuoteVolume: fmt.Sprintf("%.8f", price*1000), Closed: true})
	}

	output, err := RunSpotCoreSatelliteBacktest(config, bars, 10000)
	if err != nil {
		t.Fatalf("run backtest: %v", err)
	}
	if len(output.Benchmarks) != 3 {
		t.Fatalf("expected three benchmarks, got %d", len(output.Benchmarks))
	}
	if len(output.Lots) == 0 || len(output.Events) == 0 {
		t.Fatalf("expected lot and state event output")
	}
	if output.Metrics.MinimumCashRatio+1e-9 < config.MinimumCashRatio {
		t.Fatalf("minimum cash ratio breached: got %v want >= %v", output.Metrics.MinimumCashRatio, config.MinimumCashRatio)
	}
	if output.Metrics.MaximumExposureRatio > config.MaxSymbolExposureRatio+1e-9 {
		t.Fatalf("maximum exposure breached: got %v want <= %v", output.Metrics.MaximumExposureRatio, config.MaxSymbolExposureRatio)
	}
	if output.Metrics.MaximumDailyBuyRatio > config.MaxDailyBuyRatio+1e-9 {
		t.Fatalf("daily buy limit breached: got %v want <= %v", output.Metrics.MaximumDailyBuyRatio, config.MaxDailyBuyRatio)
	}
	if output.Metrics.TradeCount == 0 {
		if output.Metrics.ProfitFactorAvailable {
			t.Fatal("profit factor must be unavailable without realized sell allocations")
		}
		for _, item := range output.Acceptance {
			if item.Key == "profit_factor" && !item.NotApplicable {
				t.Fatal("profit factor acceptance must be marked not applicable without sells")
			}
		}
	}
	for _, lot := range output.Lots {
		if lot.RemainingQty < -1e-9 || lot.RemainingQty > lot.AcquiredQty+1e-9 {
			t.Fatalf("invalid lot inventory: %+v", lot)
		}
	}
	if _, err := json.Marshal(output); err != nil {
		t.Fatalf("backtest output must be JSON-compatible: %v", err)
	}
}

func TestRunSpotCoreSatelliteBacktestCanDisableDrawdownRiskControl(t *testing.T) {
	config := schema.DefaultSpotCoreSatelliteConfig()
	config.LowAreaLookbackBars = 20
	config.TrendFilterPeriod = 20
	config.ResistanceLookbackBars = 5
	config.DistributionLookbackBars = 20
	config.LowAreaDrawdownPct = 0.10
	config.StabilizationBars = 2
	config.ExecutionTimeframe = "4h"
	config.MaxStrategyDrawdownPct = 0.001
	config.MaxSymbolExposureRatio = 0.30
	config.MaxDailyBuyRatio = 0.10

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]*schema.MarketKline, 0, 90)
	for index := 0; index < 90; index++ {
		price := 100.0
		switch {
		case index < 25:
			price = 100 + float64(index)*0.5
		case index < 45:
			price = 112 - float64(index-25)*1.5
		case index < 50:
			price = 83 + float64(index-45)*0.4
		default:
			price = 85 - float64(index-50)*0.7
		}
		bars = append(bars, &schema.MarketKline{Time: start.Add(time.Duration(index) * 4 * time.Hour).Unix(),
			Open: fmt.Sprintf("%.8f", price), High: fmt.Sprintf("%.8f", price*1.01),
			Low: fmt.Sprintf("%.8f", price*0.99), Close: fmt.Sprintf("%.8f", price),
			Volume: "1000", QuoteVolume: fmt.Sprintf("%.8f", price*1000), Closed: true})
	}

	enabledOutput, err := RunSpotCoreSatelliteBacktest(config, bars, 10000)
	if err != nil {
		t.Fatalf("run enabled drawdown control backtest: %v", err)
	}
	if !spotBacktestHasState(enabledOutput, schema.SpotStateRiskOff) {
		t.Fatal("enabled drawdown risk control must enter RISK_OFF")
	}

	disabled := false
	config.EnableDrawdownRiskControl = &disabled
	disabledOutput, err := RunSpotCoreSatelliteBacktest(config, bars, 10000)
	if err != nil {
		t.Fatalf("run disabled drawdown control backtest: %v", err)
	}
	if spotBacktestHasState(disabledOutput, schema.SpotStateRiskOff) {
		t.Fatal("disabled drawdown risk control must not enter RISK_OFF")
	}
}

func spotBacktestHasState(output *schema.SpotBacktestOutput, state string) bool {
	for _, event := range output.Events {
		if event.ToState == state {
			return true
		}
	}
	return false
}
