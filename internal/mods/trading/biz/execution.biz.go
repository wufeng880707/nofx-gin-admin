package biz

import (
	"context"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type executionMarketRules interface {
	InstrumentRules(context.Context, string, string, string) (*provider.InstrumentRules, error)
	Ticker(context.Context, string, string, string) (*schema.MarketTicker, error)
}

type executionTradingFactory interface {
	BuildTrading(*schema.Exchange) (provider.Trading, error)
}

type decisionNotifier interface {
	DeliverTraderNotification(context.Context, *schema.Trader, schema.TraderNotificationEvent) error
}

type Execution struct {
	ExecutionDAL   *dal.Execution
	CatalogDAL     *dal.Catalog
	CatalogBIZ     *Catalog
	Notifier       decisionNotifier
	MarketRules    executionMarketRules
	TradingFactory executionTradingFactory
}

func NewExecution(executionDAL *dal.Execution, catalogDAL *dal.Catalog, catalogBIZ *Catalog, factory *provider.Factory) *Execution {
	return &Execution{ExecutionDAL: executionDAL, CatalogDAL: catalogDAL,
		CatalogBIZ: catalogBIZ, Notifier: catalogBIZ, MarketRules: factory.MarketData(), TradingFactory: factory}
}

func (a *Execution) GetSafetyState(ctx context.Context, requestedOwner string) (*schema.TradingSafetyState, error) {
	owner, err := resolveOwnerScope(ctx, requestedOwner)
	if err != nil {
		return nil, err
	}
	return a.ExecutionDAL.GetSafetyState(ctx, owner)
}

func (a *Execution) EnableLiveTrading(ctx context.Context, form *schema.TradingSafetyEnableForm) (*schema.TradingSafetyState, error) {
	if !config.C.Trading.Execution.Enabled {
		return nil, errors.Forbidden("trading_execution_global_disabled",
			"Exchange execution is disabled by the server operator")
	}
	if config.C.Trading.Execution.TestnetOnly {
		return nil, errors.Forbidden("trading_execution_testnet_only",
			"Production trading cannot be enabled while the server is restricted to testnet")
	}
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	state, err := a.ExecutionDAL.GetSafetyState(ctx, owner)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state.LiveEnabled, state.EnabledBy, state.EnabledAt = true, owner, &now
	state.DisabledBy, state.DisabledAt, state.UpdatedAt = "", nil, now
	state.Version++
	event := &schema.TradingSafetyEvent{OwnerUserID: owner, ActorUserID: owner, Action: "enable",
		Version: state.Version, Reason: form.Reason, CreatedAt: now}
	if err := a.ExecutionDAL.SaveSafetyStateWithEvent(ctx, state, event); err != nil {
		return nil, err
	}
	return state, nil
}

func (a *Execution) DisableLiveTrading(ctx context.Context, form *schema.TradingSafetyDisableForm) (*schema.TradingSafetyState, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	state, err := a.ExecutionDAL.GetSafetyState(ctx, owner)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state.LiveEnabled, state.DisabledBy, state.DisabledAt = false, owner, &now
	state.EnabledBy, state.EnabledAt, state.UpdatedAt = "", nil, now
	state.Version++
	event := &schema.TradingSafetyEvent{OwnerUserID: owner, ActorUserID: owner, Action: "disable",
		Version: state.Version, Reason: form.Reason, CreatedAt: now}
	if err := a.ExecutionDAL.SaveSafetyStateWithEvent(ctx, state, event); err != nil {
		return nil, err
	}
	return state, nil
}

func (a *Execution) QueryCommands(ctx context.Context, params schema.TradeCommandQueryParam) (*schema.TradeCommandQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ExecutionDAL.QueryCommands(ctx, params)
}

func (a *Execution) QueryCommandEvents(ctx context.Context, commandID string) ([]*schema.TradeCommandEvent, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	return a.ExecutionDAL.QueryEvents(ctx, commandID, owner)
}

func (a *Execution) CreateCommand(ctx context.Context, form *schema.TradeCommandForm) (*schema.TradeCommand, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	if existing, err := a.ExecutionDAL.FindByIdempotency(ctx, owner, form.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if !sameCommand(existing, form) {
			return nil, errors.Conflict("trading_command_idempotency_conflict",
				"Idempotency key is already bound to a different command")
		}
		return existing, nil
	}
	trader, err := a.CatalogDAL.GetTrader(ctx, form.TraderID, owner)
	if err != nil {
		return nil, err
	}
	if trader == nil {
		return nil, errors.NotFound("trading_trader_not_found", "Trader not found")
	}
	exchange, err := a.CatalogDAL.GetExchange(ctx, trader.ExchangeID, owner)
	if err != nil {
		return nil, err
	}
	if exchange == nil || !exchange.Enabled {
		return nil, errors.Conflict("trading_exchange_unavailable", "Exchange is missing or disabled")
	}
	if err := validateCommandMarketAccount(exchange, form.Market); err != nil {
		return nil, err
	}
	if form.Action == "reduce" || form.Action == "close" {
		form.ReduceOnly = true
	}
	riskSnapshot, err := a.validateCommandRisk(ctx, owner, trader, exchange, form)
	if err != nil {
		return nil, err
	}
	simulation := trader.IsSimulation
	if !simulation {
		if !config.C.Trading.Execution.Enabled {
			return nil, errors.Forbidden("trading_execution_global_disabled",
				"Exchange execution is disabled by the server operator")
		}
		if config.C.Trading.Execution.TestnetOnly && !exchange.Testnet {
			return nil, errors.Forbidden("trading_execution_testnet_only",
				"Production exchange commands are disabled while the server is restricted to testnet")
		}
		if !form.ConfirmExecution {
			return nil, errors.BadRequest("trading_command_confirmation_required",
				"This exchange command requires explicit confirmation")
		}
		if trader.ExecutionMode == schema.TraderExecutionNotifyOnly {
			return nil, errors.Forbidden("trading_command_notify_only",
				"The trader is configured for notification only")
		}
		if !exchange.Testnet {
			state, stateErr := a.ExecutionDAL.GetSafetyState(ctx, owner)
			if stateErr != nil {
				return nil, stateErr
			}
			if !state.LiveEnabled {
				return nil, errors.Forbidden("trading_live_user_disabled",
					"Live trading is disabled for the current user")
			}
		}
	}
	now := time.Now().UTC()
	command := &schema.TradeCommand{ID: util.MustNewUUID(), OwnerUserID: owner, TraderID: trader.ID,
		StrategyID: trader.StrategyID, StrategyVersion: trader.StrategyVersion,
		ExchangeID: exchange.ID, DecisionID: form.DecisionID, IdempotencyKey: form.IdempotencyKey,
		Action: form.Action, Market: form.Market, Symbol: form.Symbol, Side: form.Side,
		PositionSide: form.PositionSide, OrderType: form.OrderType, TimeInForce: form.TimeInForce,
		Quantity: form.Quantity, QuoteQuantity: form.QuoteQuantity, Price: form.Price,
		StopLoss: form.StopLoss, TakeProfit: form.TakeProfit, Leverage: form.Leverage,
		ReduceOnly: form.ReduceOnly, TargetExchangeOrderID: form.TargetExchangeOrderID,
		Simulation: simulation, Testnet: exchange.Testnet, RiskSnapshotJSON: string(riskSnapshot),
		Status: schema.TradeCommandStatusValidated, CreatedAt: now, UpdatedAt: now}
	if simulation {
		command.Status, command.FinishedAt = schema.TradeCommandStatusSimulated, &now
	}
	event := commandEvent(command, owner, "validated", "", command.Status, map[string]interface{}{
		"simulation": simulation, "testnet": exchange.Testnet,
	})
	if err := a.ExecutionDAL.CreateCommandWithEvent(ctx, command, event); err != nil {
		return nil, err
	}
	if simulation {
		return command, nil
	}
	return a.submitCommand(ctx, command, exchange, form)
}

func (a *Execution) BulkClosePositions(ctx context.Context, form *schema.BulkClosePositionsForm) (*schema.BulkClosePositionsResult, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	positions, err := a.ExecutionDAL.QueryOpenPositions(ctx, owner, form.TraderID, 101)
	if err != nil {
		return nil, err
	}
	if len(positions) > 100 {
		return nil, errors.Conflict("trading_bulk_close_limit", "Bulk close is limited to 100 open positions per operation")
	}
	result := &schema.BulkClosePositionsResult{Requested: len(positions), Results: make([]*schema.BulkClosePositionResult, 0, len(positions))}
	for _, position := range positions {
		item := &schema.BulkClosePositionResult{PositionID: position.ID, Symbol: position.Symbol}
		if strings.TrimSpace(position.TraderID) == "" {
			item.Error = "position is not attributed to a trader"
			result.Failed++
			result.Results = append(result.Results, item)
			continue
		}
		side := "sell"
		if position.Side == "short" {
			side = "buy"
		}
		commandForm := &schema.TradeCommandForm{
			TraderID: position.TraderID, IdempotencyKey: "bulk-close-" + form.OperationID + "-" + position.ID,
			Action: "close", Market: schema.MarketTypeFutures, Symbol: position.Symbol, Side: side,
			PositionSide: position.Side, OrderType: "market", Quantity: position.Quantity,
			Leverage: position.Leverage, ReduceOnly: true, ConfirmExecution: form.ConfirmExecution,
		}
		if commandErr := commandForm.Validate(); commandErr != nil {
			item.Error = commandErr.Error()
			result.Failed++
			result.Results = append(result.Results, item)
			continue
		}
		command, commandErr := a.CreateCommand(ctx, commandForm)
		if commandErr != nil {
			item.Error = commandErr.Error()
			result.Failed++
		} else {
			item.Command = command
			result.Succeeded++
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}

func (a *Execution) submitCommand(ctx context.Context, command *schema.TradeCommand,
	exchange *schema.Exchange, form *schema.TradeCommandForm) (*schema.TradeCommand, error) {
	trading, err := a.TradingFactory.BuildTrading(exchange)
	if err != nil {
		return a.rejectCommand(ctx, command, "trading_provider_disabled", err.Error())
	}
	now := time.Now().UTC()
	if err := a.ExecutionDAL.Transition(ctx, command.ID, command.OwnerUserID, command.Status,
		map[string]interface{}{"status": schema.TradeCommandStatusSubmitted, "submitted_at": now, "updated_at": now},
		commandEvent(command, command.OwnerUserID, "submitted", command.Status, schema.TradeCommandStatusSubmitted, nil)); err != nil {
		return nil, err
	}
	command.Status, command.SubmittedAt = schema.TradeCommandStatusSubmitted, &now
	var result *provider.TradingOrderResult
	if form.Action == "cancel" {
		result, err = trading.CancelOrder(ctx, provider.CancelOrderRequest{Market: form.Market, Symbol: form.Symbol,
			ExchangeOrderID: form.TargetExchangeOrderID})
	} else {
		result, err = trading.PlaceOrder(ctx, provider.TradingOrderRequest{ClientOrderID: compactClientOrderID(command.ID),
			Market: form.Market, Symbol: form.Symbol, Side: form.Side, PositionSide: form.PositionSide,
			OrderType: form.OrderType, TimeInForce: form.TimeInForce, Quantity: form.Quantity,
			QuoteQuantity: form.QuoteQuantity, Price: form.Price, StopLoss: form.StopLoss,
			TakeProfit: form.TakeProfit, ReduceOnly: form.ReduceOnly})
	}
	if err != nil {
		return a.rejectCommand(ctx, command, "trading_provider_rejected", err.Error())
	}
	acknowledged := time.Now().UTC()
	status := providerCommandStatus(result.Status)
	if form.Action == "cancel" {
		status = schema.TradeCommandStatusCancelled
	}
	values := map[string]interface{}{"status": status, "exchange_order_id": result.ExchangeOrderID,
		"exchange_client_order_id": result.ClientOrderID, "provider_status": result.Status,
		"executed_quantity": normalizeProviderDecimal(result.ExecutedQuantity),
		"average_price":     normalizeProviderDecimal(result.AveragePrice),
		"updated_at":        acknowledged}
	if commandStatusTerminal(status) {
		values["finished_at"] = acknowledged
	}
	if err := a.ExecutionDAL.Transition(ctx, command.ID, command.OwnerUserID, command.Status, values,
		commandEvent(command, command.OwnerUserID, "provider_acknowledged", command.Status, status,
			map[string]interface{}{"provider_status": result.Status})); err != nil {
		return nil, err
	}
	command.Status, command.ExchangeOrderID, command.ExchangeClientOrderID = status, result.ExchangeOrderID, result.ClientOrderID
	command.ProviderStatus, command.UpdatedAt = result.Status, acknowledged
	if commandStatusTerminal(status) {
		command.FinishedAt = &acknowledged
	}
	command.ExecutedQuantity, command.AveragePrice = normalizeProviderDecimal(result.ExecutedQuantity), normalizeProviderDecimal(result.AveragePrice)
	return command, nil
}

func (a *Execution) rejectCommand(ctx context.Context, command *schema.TradeCommand, code, message string) (*schema.TradeCommand, error) {
	now := time.Now().UTC()
	values := map[string]interface{}{"status": schema.TradeCommandStatusRejected, "error_code": code,
		"error_message": message, "finished_at": now, "updated_at": now}
	if err := a.ExecutionDAL.Transition(ctx, command.ID, command.OwnerUserID, command.Status, values,
		commandEvent(command, command.OwnerUserID, "rejected", command.Status, schema.TradeCommandStatusRejected,
			map[string]interface{}{"error_code": code})); err != nil {
		return nil, err
	}
	command.Status, command.ErrorCode, command.ErrorMessage = schema.TradeCommandStatusRejected, code, message
	command.FinishedAt, command.UpdatedAt = &now, now
	return command, nil
}

func (a *Execution) validateCommandRisk(ctx context.Context, owner string, trader *schema.Trader,
	exchange *schema.Exchange, form *schema.TradeCommandForm) ([]byte, error) {
	if form.Action == "cancel" {
		return json.Marshal(map[string]interface{}{"result": "cancel_target_validated"})
	}
	if a.MarketRules == nil {
		return nil, errors.Conflict("trading_market_rules_unavailable", "Market rule service is unavailable")
	}
	rules, err := a.MarketRules.InstrumentRules(ctx, exchange.ExchangeType, form.Market, form.Symbol)
	if err != nil {
		return nil, errors.BadRequest("trading_market_rules_failed", "Unable to load instrument rules: %v", err)
	}
	quantity, quantityOK := decimalRat(form.Quantity)
	price, priceOK := decimalRat(form.Price)
	if form.OrderType == "market" || !priceOK || price.Sign() <= 0 {
		ticker, tickerErr := a.MarketRules.Ticker(ctx, exchange.ExchangeType, form.Market, form.Symbol)
		if tickerErr != nil {
			return nil, errors.BadRequest("trading_market_price_failed", "Unable to load market price: %v", tickerErr)
		}
		price, priceOK = decimalRat(ticker.LastPrice)
	}
	if !priceOK || price.Sign() <= 0 {
		return nil, errors.BadRequest("trading_market_price_invalid", "Market price is unavailable")
	}
	quote, quoteOK := decimalRat(form.QuoteQuantity)
	notional := new(big.Rat)
	if quantityOK && quantity.Sign() > 0 {
		notional.Mul(quantity, price)
	} else if quoteOK && quote.Sign() > 0 {
		notional.Set(quote)
	}
	if quantityOK && quantity.Sign() > 0 {
		if err := validateDecimalRule("quantity", quantity, rules.MinQuantity, rules.StepSize); err != nil {
			return nil, err
		}
	}
	if form.OrderType == "limit" {
		if err := validateDecimalRule("price", price, "0", rules.TickSize); err != nil {
			return nil, err
		}
	}
	if minimum, ok := decimalRat(rules.MinNotional); ok && minimum.Sign() > 0 && notional.Cmp(minimum) < 0 {
		return nil, errors.BadRequest("trading_command_min_notional", "Order notional is below exchange minimum")
	}
	strategyRisk, err := a.loadCommandStrategyRisk(ctx, owner, trader)
	if err != nil {
		return nil, err
	}
	if strategyRisk != nil && (form.Action == "open" || form.Action == "add") {
		maxLeverage := strategyRisk.AltcoinMaxLeverage
		maxRatio := strategyRisk.AltcoinMaxPositionValueRatio
		baseSymbol := strings.TrimSuffix(strings.ReplaceAll(form.Symbol, "-", ""), "USDT")
		if baseSymbol == "BTC" || baseSymbol == "ETH" {
			maxLeverage, maxRatio = strategyRisk.BTCETHMaxLeverage, strategyRisk.BTCETHMaxPositionValueRatio
		}
		if form.Leverage > maxLeverage {
			return nil, errors.BadRequest("trading_command_leverage_limit", "Leverage exceeds strategy limit")
		}
		if minimum, ok := new(big.Rat).SetString(strconv.FormatFloat(strategyRisk.MinPositionSize, 'f', -1, 64)); ok && minimum.Sign() > 0 && notional.Cmp(minimum) < 0 {
			return nil, errors.BadRequest("trading_command_position_minimum", "Order notional is below strategy minimum position size")
		}
		if err := validateProtectionLevels(form, price, rules, strategyRisk.MinRiskRewardRatio); err != nil {
			return nil, err
		}
		var positionCount int64
		if err := a.ExecutionDAL.DB.WithContext(ctx).Model(new(schema.Position)).
			Where("owner_user_id = ? AND trader_id = ? AND status = ?", owner, trader.ID, schema.PositionStatusOpen).
			Count(&positionCount).Error; err != nil {
			return nil, err
		}
		if form.Action == "open" && positionCount >= int64(strategyRisk.MaxPositions) {
			return nil, errors.BadRequest("trading_command_position_limit", "Maximum strategy position count reached")
		}
		var snapshot schema.AccountSnapshot
		dbErr := a.ExecutionDAL.DB.WithContext(ctx).Where("owner_user_id = ? AND trader_id = ?", owner, trader.ID).
			Order("captured_at DESC").First(&snapshot).Error
		if dbErr != nil {
			return nil, errors.Conflict("trading_command_account_snapshot_missing", "A current account snapshot is required for opening risk")
		}
		if dbErr == nil {
			equity, equityOK := decimalRat(snapshot.TotalEquity)
			available, availableOK := decimalRat(snapshot.AvailableBalance)
			requiredCapital := new(big.Rat).Set(notional)
			if form.Market == schema.MarketTypeFutures && form.Leverage > 1 {
				requiredCapital.Quo(requiredCapital, new(big.Rat).SetInt64(int64(form.Leverage)))
			}
			if availableOK && requiredCapital.Cmp(available) > 0 {
				return nil, errors.BadRequest("trading_command_balance_limit", "Order notional exceeds available balance")
			}
			if equityOK && equity.Sign() > 0 && maxRatio > 0 {
				limit := new(big.Rat).Mul(equity, new(big.Rat).SetFloat64(maxRatio))
				if notional.Cmp(limit) > 0 {
					return nil, errors.BadRequest("trading_command_position_ratio_limit", "Order notional exceeds strategy position ratio")
				}
			}
			if equityOK && equity.Sign() > 0 && strategyRisk.MaxMarginUsage > 0 {
				existingMargin, marginErr := a.aggregateOpenMargin(ctx, owner, trader.ID)
				if marginErr != nil {
					return nil, marginErr
				}
				totalMargin := new(big.Rat).Add(existingMargin, requiredCapital)
				limit := new(big.Rat).Mul(equity, new(big.Rat).SetFloat64(strategyRisk.MaxMarginUsage))
				if totalMargin.Cmp(limit) > 0 {
					return nil, errors.BadRequest("trading_command_margin_usage_limit", "Aggregate margin usage exceeds strategy limit")
				}
			}
		}
	} else if form.Action == "open" || form.Action == "add" {
		if err := validateProtectionLevels(form, price, rules, 0); err != nil {
			return nil, err
		}
	}
	if form.Action == "reduce" || form.Action == "close" {
		if err := a.validateReduction(ctx, owner, trader, form); err != nil {
			return nil, err
		}
	}
	return json.Marshal(map[string]interface{}{"result": "passed", "rules": rules,
		"reference_price": decimalText(price), "notional_usdt": decimalText(notional)})
}

func (a *Execution) loadCommandStrategyRisk(ctx context.Context, owner string, trader *schema.Trader) (*schema.RiskControlConfig, error) {
	if trader.StrategyID == nil || trader.StrategyVersion <= 0 {
		return nil, nil
	}
	var version schema.StrategyVersion
	err := a.ExecutionDAL.DB.WithContext(ctx).Where("strategy_id = ? AND owner_user_id = ? AND version = ? AND published_at IS NOT NULL",
		*trader.StrategyID, owner, trader.StrategyVersion).First(&version).Error
	if err != nil {
		return nil, errors.Conflict("trading_command_strategy_version_missing", "Pinned strategy version is unavailable")
	}
	studio, err := schema.DecodeStrategyStudioConfig(json.RawMessage(version.ConfigJSON))
	if err != nil {
		return nil, errors.BadRequest("trading_command_strategy_invalid", "Pinned strategy configuration is invalid")
	}
	return &studio.AIConfig.RiskControl, nil
}

func validateCommandMarketAccount(exchange *schema.Exchange, market string) error {
	account := strings.ToLower(strings.TrimSpace(exchange.AccountType))
	if exchange.ExchangeType == schema.MarketExchangeBinance && account != "unified" && account != market {
		return errors.BadRequest("trading_command_account_market_mismatch",
			"Binance account type %s cannot execute %s commands", account, market)
	}
	return nil
}

func validateDecimalRule(name string, value *big.Rat, minimumText, stepText string) error {
	if minimum, ok := decimalRat(minimumText); ok && minimum.Sign() > 0 && value.Cmp(minimum) < 0 {
		return errors.BadRequest("trading_command_"+name+"_minimum", "%s is below exchange minimum", name)
	}
	if step, ok := decimalRat(stepText); ok && step.Sign() > 0 {
		quotient := new(big.Rat).Quo(value, step)
		if !quotient.IsInt() {
			return errors.BadRequest("trading_command_"+name+"_step", "%s does not match exchange step size", name)
		}
	}
	return nil
}

func validateProtectionLevels(form *schema.TradeCommandForm, entry *big.Rat, rules *provider.InstrumentRules, minimumRatio float64) error {
	stop, stopOK := decimalRat(form.StopLoss)
	take, takeOK := decimalRat(form.TakeProfit)
	if !stopOK || !takeOK || stop.Sign() <= 0 || take.Sign() <= 0 {
		return errors.BadRequest("trading_command_protection_required", "Positive stop-loss and take-profit prices are required")
	}
	if err := validateDecimalRule("stop_loss", stop, "0", rules.TickSize); err != nil {
		return err
	}
	if err := validateDecimalRule("take_profit", take, "0", rules.TickSize); err != nil {
		return err
	}
	positionSide := strings.ToLower(strings.TrimSpace(form.PositionSide))
	if positionSide == "" {
		if form.Side == "buy" {
			positionSide = "long"
		} else {
			positionSide = "short"
		}
	}
	validDirection := positionSide == "long" && stop.Cmp(entry) < 0 && take.Cmp(entry) > 0 ||
		positionSide == "short" && take.Cmp(entry) < 0 && stop.Cmp(entry) > 0
	if !validDirection {
		return errors.BadRequest("trading_command_protection_direction", "Stop-loss and take-profit are on the wrong side of the entry price")
	}
	risk := new(big.Rat).Sub(entry, stop)
	reward := new(big.Rat).Sub(take, entry)
	if risk.Sign() < 0 {
		risk.Neg(risk)
	}
	if reward.Sign() < 0 {
		reward.Neg(reward)
	}
	if risk.Sign() <= 0 {
		return errors.BadRequest("trading_command_risk_reward_invalid", "Stop-loss distance must be positive")
	}
	if minimumRatio > 0 {
		minimum := new(big.Rat).SetFloat64(minimumRatio)
		if new(big.Rat).Quo(reward, risk).Cmp(minimum) < 0 {
			return errors.BadRequest("trading_command_risk_reward_limit", "Risk/reward ratio is below strategy minimum")
		}
	}
	return nil
}

func (a *Execution) validateReduction(ctx context.Context, owner string, trader *schema.Trader, form *schema.TradeCommandForm) error {
	db := a.ExecutionDAL.DB.WithContext(ctx).Where("owner_user_id = ? AND trader_id = ? AND symbol = ? AND status = ?",
		owner, trader.ID, form.Symbol, schema.PositionStatusOpen)
	if form.PositionSide != "" {
		db = db.Where("side = ?", form.PositionSide)
	}
	var position schema.Position
	if err := db.Order("opened_at ASC").First(&position).Error; err != nil {
		return errors.BadRequest("trading_command_position_missing", "No matching open position is available to reduce")
	}
	expectedSide := "sell"
	if position.Side == "short" {
		expectedSide = "buy"
	}
	if form.Side != expectedSide {
		return errors.BadRequest("trading_command_reduce_side_invalid", "Reduce-only side does not close the matching position")
	}
	requested, requestedOK := decimalRat(form.Quantity)
	available, availableOK := decimalRat(position.Quantity)
	if !requestedOK || !availableOK || requested.Sign() <= 0 || requested.Cmp(available) > 0 {
		return errors.BadRequest("trading_command_reduce_quantity_invalid", "Reduce-only quantity exceeds the matching open position")
	}
	if form.Action == "close" && requested.Cmp(available) != 0 {
		return errors.BadRequest("trading_command_close_quantity_invalid", "Close command must use the full open position quantity")
	}
	form.PositionSide = position.Side
	form.ReduceOnly = true
	return nil
}

func (a *Execution) aggregateOpenMargin(ctx context.Context, owner, traderID string) (*big.Rat, error) {
	var positions []*schema.Position
	if err := a.ExecutionDAL.DB.WithContext(ctx).Where("owner_user_id = ? AND trader_id = ? AND status = ?",
		owner, traderID, schema.PositionStatusOpen).Find(&positions).Error; err != nil {
		return nil, err
	}
	total := new(big.Rat)
	for _, position := range positions {
		quantity, quantityOK := decimalRat(position.Quantity)
		price, priceOK := decimalRat(position.MarkPrice)
		if !priceOK || price.Sign() <= 0 {
			price, priceOK = decimalRat(position.EntryPrice)
		}
		if !quantityOK || !priceOK || quantity.Sign() <= 0 || price.Sign() <= 0 {
			continue
		}
		margin := new(big.Rat).Mul(quantity, price)
		leverage := position.Leverage
		if leverage < 1 {
			leverage = 1
		}
		margin.Quo(margin, new(big.Rat).SetInt64(int64(leverage)))
		total.Add(total, margin)
	}
	return total, nil
}

func decimalRat(value string) (*big.Rat, bool) {
	result, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return result, ok
}

func decimalText(value *big.Rat) string {
	result := value.FloatString(18)
	result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	if result == "" || result == "-0" {
		return "0"
	}
	return result
}

func normalizeProviderDecimal(value string) string {
	parsed, ok := decimalRat(value)
	if !ok {
		return "0"
	}
	return decimalText(parsed)
}

func providerCommandStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "partially_filled", "partiallyfilled", "partial-fill", "partial_fill":
		return schema.TradeCommandStatusPartial
	case "filled", "closed":
		return schema.TradeCommandStatusFilled
	case "cancelled", "canceled", "expired":
		return schema.TradeCommandStatusCancelled
	case "rejected", "failed":
		return schema.TradeCommandStatusRejected
	default:
		return schema.TradeCommandStatusAccepted
	}
}

func commandStatusTerminal(value string) bool {
	return value == schema.TradeCommandStatusFilled || value == schema.TradeCommandStatusCancelled ||
		value == schema.TradeCommandStatusRejected || value == schema.TradeCommandStatusSimulated
}

func sameCommand(command *schema.TradeCommand, form *schema.TradeCommandForm) bool {
	return command.TraderID == form.TraderID && command.Action == form.Action && command.Market == form.Market &&
		command.Symbol == form.Symbol && command.Side == form.Side && command.Quantity == form.Quantity &&
		command.QuoteQuantity == form.QuoteQuantity && command.Price == form.Price &&
		command.StopLoss == form.StopLoss && command.TakeProfit == form.TakeProfit &&
		command.Leverage == form.Leverage && command.ReduceOnly == form.ReduceOnly &&
		command.TargetExchangeOrderID == form.TargetExchangeOrderID
}

func commandEvent(command *schema.TradeCommand, actor, eventType, from, to string, detail interface{}) *schema.TradeCommandEvent {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	encoded, _ := json.Marshal(detail)
	return &schema.TradeCommandEvent{CommandID: command.ID, OwnerUserID: command.OwnerUserID,
		ActorUserID: actor, EventType: eventType, FromStatus: from, ToStatus: to,
		DetailJSON: string(encoded), CreatedAt: time.Now().UTC()}
}

func compactClientOrderID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}
