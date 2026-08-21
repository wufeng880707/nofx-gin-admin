package schema

import (
	"math/big"
	"strings"

	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type TradeCommandForm struct {
	TraderID              string  `json:"trader_id" binding:"required,max=64"`
	DecisionID            *uint64 `json:"decision_id"`
	IdempotencyKey        string  `json:"idempotency_key" binding:"required,max=128"`
	Action                string  `json:"action" binding:"required"`
	Market                string  `json:"market" binding:"required"`
	Symbol                string  `json:"symbol" binding:"required,max=64"`
	Side                  string  `json:"side"`
	PositionSide          string  `json:"position_side"`
	OrderType             string  `json:"order_type"`
	TimeInForce           string  `json:"time_in_force"`
	Quantity              string  `json:"quantity"`
	QuoteQuantity         string  `json:"quote_quantity"`
	Price                 string  `json:"price"`
	StopLoss              string  `json:"stop_loss"`
	TakeProfit            string  `json:"take_profit"`
	Leverage              int     `json:"leverage"`
	ReduceOnly            bool    `json:"reduce_only"`
	TargetExchangeOrderID string  `json:"target_exchange_order_id"`
	ConfirmExecution      bool    `json:"confirm_execution"`
}

func (a *TradeCommandForm) Validate() error {
	a.TraderID = strings.TrimSpace(a.TraderID)
	a.IdempotencyKey = strings.TrimSpace(a.IdempotencyKey)
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	a.Market = strings.ToLower(strings.TrimSpace(a.Market))
	a.Symbol = strings.ToUpper(strings.TrimSpace(a.Symbol))
	a.Side = strings.ToLower(strings.TrimSpace(a.Side))
	a.PositionSide = strings.ToLower(strings.TrimSpace(a.PositionSide))
	a.OrderType = strings.ToLower(strings.TrimSpace(a.OrderType))
	a.TimeInForce = strings.ToUpper(strings.TrimSpace(a.TimeInForce))
	a.Quantity = normalizeExecutionDecimal(a.Quantity)
	a.QuoteQuantity = normalizeExecutionDecimal(a.QuoteQuantity)
	a.Price = normalizeExecutionDecimal(a.Price)
	a.StopLoss = normalizeExecutionDecimal(a.StopLoss)
	a.TakeProfit = normalizeExecutionDecimal(a.TakeProfit)
	a.TargetExchangeOrderID = strings.TrimSpace(a.TargetExchangeOrderID)
	if a.TraderID == "" || a.IdempotencyKey == "" || a.Symbol == "" {
		return errors.BadRequest("trading_command_required", "Trader, idempotency key, and symbol are required")
	}
	if a.Market != MarketTypeSpot && a.Market != MarketTypeFutures {
		return errors.BadRequest("trading_command_market_invalid", "Market must be spot or futures")
	}
	switch a.Action {
	case "open", "add", "reduce", "close":
		if a.Side != "buy" && a.Side != "sell" {
			return errors.BadRequest("trading_command_side_invalid", "Side must be buy or sell")
		}
		if a.OrderType == "" {
			a.OrderType = "market"
		}
		if a.OrderType != "market" && a.OrderType != "limit" {
			return errors.BadRequest("trading_command_type_invalid", "Order type must be market or limit")
		}
		if !positiveExecutionDecimal(a.Quantity) && !(a.Market == MarketTypeSpot && a.Side == "buy" && positiveExecutionDecimal(a.QuoteQuantity)) {
			return errors.BadRequest("trading_command_quantity_invalid", "A positive quantity is required")
		}
		if a.OrderType == "limit" && !positiveExecutionDecimal(a.Price) {
			return errors.BadRequest("trading_command_price_invalid", "A positive limit price is required")
		}
		if a.Action == "open" || a.Action == "add" {
			if a.ReduceOnly {
				return errors.BadRequest("trading_command_reduce_only_invalid", "Open and add commands cannot be reduce-only")
			}
			if !positiveExecutionDecimal(a.StopLoss) || !positiveExecutionDecimal(a.TakeProfit) {
				return errors.BadRequest("trading_command_protection_required", "Open and add commands require positive stop-loss and take-profit prices")
			}
		} else {
			a.ReduceOnly = true
			if positiveExecutionDecimal(a.StopLoss) || positiveExecutionDecimal(a.TakeProfit) {
				return errors.BadRequest("trading_command_protection_invalid", "Reduce and close commands cannot create stop-loss or take-profit levels")
			}
		}
	case "cancel":
		if a.TargetExchangeOrderID == "" {
			return errors.BadRequest("trading_command_target_required", "Target exchange order ID is required for cancellation")
		}
		if a.ReduceOnly || positiveExecutionDecimal(a.StopLoss) || positiveExecutionDecimal(a.TakeProfit) {
			return errors.BadRequest("trading_command_cancel_semantics_invalid", "Cancel commands cannot contain position protection fields")
		}
	default:
		return errors.BadRequest("trading_command_action_invalid", "Unsupported trade command action")
	}
	if a.PositionSide != "" && a.PositionSide != "long" && a.PositionSide != "short" {
		return errors.BadRequest("trading_command_position_side_invalid", "Position side must be long or short")
	}
	if a.Leverage == 0 {
		a.Leverage = 1
	}
	if a.Leverage < 1 || a.Leverage > 125 {
		return errors.BadRequest("trading_command_leverage_invalid", "Leverage must be between 1 and 125")
	}
	return nil
}

type TradingSafetyEnableForm struct {
	ConfirmationPhrase string `json:"confirmation_phrase"`
	AcknowledgeRisk    bool   `json:"acknowledge_risk"`
	Reason             string `json:"reason" binding:"max=500"`
}

func (a *TradingSafetyEnableForm) Validate() error {
	a.ConfirmationPhrase = strings.TrimSpace(a.ConfirmationPhrase)
	a.Reason = strings.TrimSpace(a.Reason)
	if !a.AcknowledgeRisk || a.ConfirmationPhrase != "ENABLE LIVE TRADING" {
		return errors.BadRequest("trading_live_confirmation_required",
			"Live trading requires risk acknowledgement and the exact confirmation phrase ENABLE LIVE TRADING")
	}
	return nil
}

type TradingSafetyDisableForm struct {
	Reason string `json:"reason" binding:"required,max=500"`
}

func (a *TradingSafetyDisableForm) Validate() error {
	a.Reason = strings.TrimSpace(a.Reason)
	if a.Reason == "" {
		return errors.BadRequest("trading_live_disable_reason_required", "A disable reason is required")
	}
	return nil
}

type TradeCommandQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	TraderID    string `form:"traderID"`
	Status      string `form:"status"`
	Symbol      string `form:"symbol"`
}

type TradeCommandQueryResult struct {
	Data       []*TradeCommand
	PageResult *util.PaginationResult
}

type BulkClosePositionsForm struct {
	TraderID           string `json:"trader_id" binding:"max=64"`
	OperationID        string `json:"operation_id" binding:"required,max=64"`
	ConfirmationPhrase string `json:"confirmation_phrase"`
	AcknowledgeRisk    bool   `json:"acknowledge_risk"`
	ConfirmExecution   bool   `json:"confirm_execution"`
}

func (a *BulkClosePositionsForm) Validate() error {
	a.TraderID = strings.TrimSpace(a.TraderID)
	a.OperationID = strings.TrimSpace(a.OperationID)
	a.ConfirmationPhrase = strings.TrimSpace(a.ConfirmationPhrase)
	if a.OperationID == "" {
		return errors.BadRequest("trading_bulk_close_operation_required", "Operation ID is required")
	}
	if !a.AcknowledgeRisk || a.ConfirmationPhrase != "CLOSE ALL POSITIONS" {
		return errors.BadRequest("trading_bulk_close_confirmation_required",
			"Bulk close requires risk acknowledgement and the exact confirmation phrase CLOSE ALL POSITIONS")
	}
	return nil
}

type BulkClosePositionResult struct {
	PositionID string        `json:"position_id"`
	Symbol     string        `json:"symbol"`
	Command    *TradeCommand `json:"command,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type BulkClosePositionsResult struct {
	Requested int                        `json:"requested"`
	Succeeded int                        `json:"succeeded"`
	Failed    int                        `json:"failed"`
	Results   []*BulkClosePositionResult `json:"results"`
}

func normalizeExecutionDecimal(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0"
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		return value
	}
	result := parsed.FloatString(18)
	result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	if result == "" || result == "-0" {
		return "0"
	}
	return result
}

func positiveExecutionDecimal(value string) bool {
	parsed, ok := new(big.Rat).SetString(value)
	return ok && parsed.Sign() > 0
}
