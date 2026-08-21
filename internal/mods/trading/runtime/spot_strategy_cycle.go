package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func (a *Registry) runSpotCoreSatelliteStrategyCycle(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	_ *schema.Strategy, studio *schema.StrategyStudioConfig) error {
	if !trader.IsSimulation {
		return errors.New("spot core-satellite runtime is simulation-only; live execution is disabled")
	}
	if a.ProviderFactory == nil {
		return errors.New("market data provider is unavailable")
	}
	config := studio.SpotCoreSatelliteConfig
	config.Exchange = exchange.ExchangeType
	config.AllowLiveExecution = false
	config.Normalize()
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate spot strategy: %w", err)
	}

	barDuration := 24 * time.Hour
	if config.ExecutionTimeframe == "4h" {
		barDuration = 4 * time.Hour
	}
	lookback := maxInt(config.LowAreaLookbackBars, maxInt(config.TrendFilterPeriod, config.DistributionLookbackBars)) + 80
	end := time.Now().UTC()
	start := end.Add(-time.Duration(lookback) * barDuration)
	klines, err := a.ProviderFactory.MarketData().KlinesRange(ctx, config.Exchange, schema.MarketTypeSpot,
		config.Symbol, config.ExecutionTimeframe, start, end)
	if err != nil {
		return fmt.Errorf("load spot strategy market history: %w", err)
	}
	output, err := RunSpotCoreSatelliteBacktest(config, klines, config.StrategyBudgetUSDT)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	cycleNumber := trader.CycleNumber + 1
	input, _ := json.Marshal(map[string]interface{}{"config": config, "data_points": len(klines), "end_time": end})
	snapshot, _ := json.Marshal(map[string]interface{}{
		"metrics": output.Metrics, "benchmarks": output.Benchmarks, "acceptance": output.Acceptance,
		"lots": output.Lots, "events": recentSpotBacktestEvents(output.Events, 50),
	})
	actions, _ := json.Marshal([]map[string]interface{}{})
	logJSON, _ := json.Marshal([]map[string]interface{}{{
		"stage": "spot_core_satellite", "status": "simulated",
		"message": "Deterministic replay completed; no trade command was created",
	}})
	decision := &schema.Decision{OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, ExchangeID: exchange.ID,
		CycleNumber: cycleNumber, DecidedAt: now, SystemPrompt: "", InputPrompt: string(input), CoTTrace: "",
		DecisionJSON: string(snapshot), CandidateCoinsJSON: mustJSON([]string{config.Symbol}),
		ExecutionLogJSON: string(logJSON), ActionsJSON: string(actions), Success: true, CreatedAt: now}
	if err := a.DB.WithContext(ctx).Create(decision).Error; err != nil {
		return err
	}
	state := schema.SpotStateWait
	if len(output.EquityCurve) > 0 {
		state = output.EquityCurve[len(output.EquityCurve)-1].State
	}
	return a.updateTraderFields(ctx, trader.ID, map[string]interface{}{
		"cycle_number": cycleNumber, "last_decision_at": now, "strategy_state": state,
		"safe_mode": true, "pause_reason": "", "last_error_code": "", "last_error_message": "",
	})
}

func recentSpotBacktestEvents(events []schema.SpotBacktestEvent, limit int) []schema.SpotBacktestEvent {
	if len(events) <= limit {
		return events
	}
	return events[len(events)-limit:]
}
