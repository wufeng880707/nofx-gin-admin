package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

type GridPlan struct {
	Symbol          string            `json:"symbol"`
	Market          string            `json:"market"`
	Direction       string            `json:"direction"`
	PriceMode       string            `json:"price_mode"`
	CurrentPrice    float64           `json:"current_price"`
	LowerPrice      float64           `json:"lower_price"`
	UpperPrice      float64           `json:"upper_price"`
	InvestmentUSDT  float64           `json:"investment_usdt"`
	MaxPositionUSDT float64           `json:"max_position_usdt"`
	ReserveUSDT     float64           `json:"reserve_usdt"`
	StopLossPrice   float64           `json:"stop_loss_price"`
	TakeProfitPrice float64           `json:"take_profit_price"`
	MaxDrawdownPct  float64           `json:"max_drawdown_pct"`
	RiskTriggered   bool              `json:"risk_triggered"`
	RiskReason      string            `json:"risk_reason,omitempty"`
	Levels          []float64         `json:"levels"`
	Orders          []GridOrderIntent `json:"orders"`
}

type GridOrderIntent struct {
	Level        int     `json:"level"`
	Side         string  `json:"side"`
	Price        float64 `json:"price"`
	NotionalUSDT float64 `json:"notional_usdt"`
	ReduceOnly   bool    `json:"reduce_only"`
}

func BuildGridPlan(config schema.GridStrategyConfig, currentPrice float64) (*GridPlan, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if currentPrice <= 0 || math.IsNaN(currentPrice) || math.IsInf(currentPrice, 0) {
		return nil, errors.New("current price must be positive and finite")
	}
	plan := &GridPlan{Symbol: config.Symbol, Market: config.Market, Direction: config.Direction,
		PriceMode: config.PriceMode, CurrentPrice: currentPrice, LowerPrice: config.LowerPrice,
		UpperPrice: config.UpperPrice, InvestmentUSDT: config.InvestmentUSDT,
		MaxPositionUSDT: config.MaxPositionUSDT, ReserveUSDT: config.InvestmentUSDT * config.ReserveRatio,
		StopLossPrice:   roundGridNumber(config.LowerPrice * (1 - config.StopLossPct/100)),
		TakeProfitPrice: roundGridNumber(config.UpperPrice * (1 + config.TakeProfitPct/100)),
		MaxDrawdownPct:  config.MaxDrawdownPct}
	plan.Levels = make([]float64, 0, config.GridCount+1)
	for index := 0; index <= config.GridCount; index++ {
		ratio := float64(index) / float64(config.GridCount)
		price := config.LowerPrice + (config.UpperPrice-config.LowerPrice)*ratio
		if config.PriceMode == "geometric" {
			price = config.LowerPrice * math.Pow(config.UpperPrice/config.LowerPrice, ratio)
		}
		plan.Levels = append(plan.Levels, roundGridNumber(price))
	}
	if currentPrice < config.LowerPrice || currentPrice > config.UpperPrice {
		plan.RiskTriggered = true
		plan.RiskReason = "current price is outside the configured grid range"
		return plan, nil
	}
	orderNotional := math.Min((config.InvestmentUSDT-plan.ReserveUSDT)/float64(config.MaxOpenOrders),
		config.MaxPositionUSDT/float64(config.MaxOpenOrders))
	orders := make([]GridOrderIntent, 0, config.GridCount)
	for index, price := range plan.Levels {
		if price == currentPrice {
			continue
		}
		side := "buy"
		if price > currentPrice {
			side = "sell"
		}
		orders = append(orders, GridOrderIntent{Level: index, Side: side, Price: price,
			NotionalUSDT: roundGridNumber(orderNotional), ReduceOnly: config.Direction != "neutral" &&
				((config.Direction == "long" && side == "sell") || (config.Direction == "short" && side == "buy"))})
	}
	sort.SliceStable(orders, func(i, j int) bool {
		return math.Abs(orders[i].Price-currentPrice) < math.Abs(orders[j].Price-currentPrice)
	})
	if len(orders) > config.MaxOpenOrders {
		orders = orders[:config.MaxOpenOrders]
	}
	plan.Orders = orders
	return plan, nil
}

func roundGridNumber(value float64) float64 {
	return math.Round(value*1e8) / 1e8
}

func (a *Registry) runGridStrategyCycle(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	_ *schema.Strategy, config *schema.StrategyStudioConfig) error {
	if !trader.IsSimulation {
		return errors.New("grid runtime is simulation-only; live grid execution is disabled")
	}
	if a.ProviderFactory == nil {
		return errors.New("market data provider is unavailable")
	}
	if exchange.ExchangeType != schema.MarketExchangeBinance && exchange.ExchangeType != schema.MarketExchangeOKX {
		return fmt.Errorf("grid market data is unsupported for %s", exchange.ExchangeType)
	}
	providerSymbol := runtimeMarketSymbol(exchange.ExchangeType, config.GridConfig.Market, config.GridConfig.Symbol)
	ticker, err := a.ProviderFactory.MarketData().Ticker(ctx, exchange.ExchangeType, config.GridConfig.Market, providerSymbol)
	if err != nil {
		return fmt.Errorf("load grid ticker: %w", err)
	}
	currentPrice, err := strconv.ParseFloat(ticker.LastPrice, 64)
	if err != nil {
		return fmt.Errorf("parse grid ticker price: %w", err)
	}
	plan, err := BuildGridPlan(config.GridConfig, currentPrice)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	cycleNumber := trader.CycleNumber + 1
	input, _ := json.Marshal(map[string]interface{}{"ticker": ticker, "grid_config": config.GridConfig})
	decisionJSON, _ := json.Marshal(plan)
	actionsJSON, _ := json.Marshal(plan.Orders)
	logJSON, _ := json.Marshal([]map[string]interface{}{{
		"stage": "grid_plan", "status": "simulated", "message": "Simulation plan only; no trade command was created",
	}})
	decision := &schema.Decision{OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, ExchangeID: exchange.ID,
		CycleNumber: cycleNumber, DecidedAt: now, SystemPrompt: "", InputPrompt: string(input), CoTTrace: "",
		DecisionJSON: string(decisionJSON), CandidateCoinsJSON: mustJSON([]string{config.GridConfig.Symbol}),
		ExecutionLogJSON: string(logJSON), ActionsJSON: string(actionsJSON), Success: true,
		RawResponse: "", CreatedAt: now}
	if err := a.DB.WithContext(ctx).Create(decision).Error; err != nil {
		return err
	}
	state := "running"
	pauseReason := ""
	if plan.RiskTriggered {
		state = "paused"
		pauseReason = plan.RiskReason
	}
	return a.updateTraderFields(ctx, trader.ID, map[string]interface{}{
		"cycle_number": cycleNumber, "last_decision_at": now, "strategy_state": state,
		"safe_mode": true, "pause_reason": pauseReason, "last_error_code": "", "last_error_message": "",
	})
}
