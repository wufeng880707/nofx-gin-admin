package biz

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"gorm.io/gorm"
)

func (a *Execution) QueryApprovals(ctx context.Context, params schema.TradeApprovalQueryParam) (*schema.TradeApprovalQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if err := a.ExecutionDAL.ExpireApprovals(ctx, owner, time.Now().UTC()); err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ExecutionDAL.QueryApprovals(ctx, params)
}

func (a *Execution) QueryApprovalEvents(ctx context.Context, approvalID string) ([]*schema.TradeApprovalEvent, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	if item, getErr := a.ExecutionDAL.GetApproval(ctx, approvalID, owner); getErr != nil {
		return nil, getErr
	} else if item == nil {
		return nil, errors.NotFound("trading_approval_not_found", "Trade approval not found")
	}
	return a.ExecutionDAL.QueryApprovalEvents(ctx, approvalID, owner)
}

func (a *Execution) Approve(ctx context.Context, approvalID string, form *schema.TradeApprovalActForm) (*schema.TradeApproval, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	approval, err := a.ExecutionDAL.GetApproval(ctx, approvalID, owner)
	if err != nil {
		return nil, err
	}
	if approval == nil {
		return nil, errors.NotFound("trading_approval_not_found", "Trade approval not found")
	}
	if approval.Status != schema.TradeApprovalStatusPending && approval.Status != schema.TradeApprovalStatusApproved {
		return nil, errors.Conflict("trading_approval_not_pending", "Trade approval is no longer pending")
	}
	now := time.Now().UTC()
	if approval.Status == schema.TradeApprovalStatusPending && !approval.ExpiresAt.After(now) {
		_ = a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner, schema.TradeApprovalStatusPending,
			map[string]interface{}{"status": schema.TradeApprovalStatusExpired, "updated_at": now},
			approvalEvent(approval, "system", "expired", schema.TradeApprovalStatusPending,
				schema.TradeApprovalStatusExpired, map[string]interface{}{"reason": "approval timeout"}))
		return nil, errors.Conflict("trading_approval_expired", "Trade approval has expired")
	}
	if approval.Status == schema.TradeApprovalStatusPending {
		if err := a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner, schema.TradeApprovalStatusPending,
			map[string]interface{}{"status": schema.TradeApprovalStatusApproved, "acted_by": owner,
				"acted_at": now, "updated_at": now},
			approvalEvent(approval, owner, "approved", schema.TradeApprovalStatusPending,
				schema.TradeApprovalStatusApproved, map[string]interface{}{"comment": form.Comment})); err != nil {
			if errors.Is(err, gorm.ErrInvalidTransaction) {
				return nil, errors.Conflict("trading_approval_concurrent_action", "Trade approval was already handled")
			}
			return nil, err
		}
		approval.Status, approval.ActedBy, approval.ActedAt = schema.TradeApprovalStatusApproved, owner, &now
	}
	if err := a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner, schema.TradeApprovalStatusApproved,
		map[string]interface{}{"status": schema.TradeApprovalStatusExecuting, "updated_at": now},
		approvalEvent(approval, owner, "execution_started", schema.TradeApprovalStatusApproved,
			schema.TradeApprovalStatusExecuting, nil)); err != nil {
		if errors.Is(err, gorm.ErrInvalidTransaction) {
			return nil, errors.Conflict("trading_approval_concurrent_action", "Trade approval was already handled")
		}
		return nil, err
	}
	approval.Status = schema.TradeApprovalStatusExecuting
	commandForm, err := a.approvalCommandForm(ctx, approval)
	if err == nil {
		err = commandForm.Validate()
	}
	var commandID string
	if err == nil {
		var command *schema.TradeCommand
		command, err = a.CreateCommand(ctx, commandForm)
		if command != nil {
			commandID = command.ID
		}
	}
	finished := time.Now().UTC()
	if err != nil {
		message := err.Error()
		transitionErr := a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner,
			schema.TradeApprovalStatusExecuting, map[string]interface{}{"status": schema.TradeApprovalStatusFailed,
				"failure_code": "trade_command_failed", "failure_message": message, "updated_at": finished},
			approvalEvent(approval, owner, "execution_failed", schema.TradeApprovalStatusExecuting,
				schema.TradeApprovalStatusFailed, map[string]interface{}{"error": message}))
		if transitionErr != nil {
			return nil, transitionErr
		}
		return nil, err
	}
	if err := a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner, schema.TradeApprovalStatusExecuting,
		map[string]interface{}{"status": schema.TradeApprovalStatusSucceeded, "command_id": commandID, "updated_at": finished},
		approvalEvent(approval, owner, "command_created", schema.TradeApprovalStatusExecuting,
			schema.TradeApprovalStatusSucceeded, map[string]interface{}{"command_id": commandID})); err != nil {
		return nil, err
	}
	approval.Status, approval.CommandID, approval.UpdatedAt = schema.TradeApprovalStatusSucceeded, commandID, finished
	return approval, nil
}

func (a *Execution) RejectApproval(ctx context.Context, approvalID string, form *schema.TradeApprovalActForm) (*schema.TradeApproval, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	approval, err := a.ExecutionDAL.GetApproval(ctx, approvalID, owner)
	if err != nil {
		return nil, err
	}
	if approval == nil {
		return nil, errors.NotFound("trading_approval_not_found", "Trade approval not found")
	}
	if approval.Status != schema.TradeApprovalStatusPending {
		return nil, errors.Conflict("trading_approval_not_pending", "Trade approval is no longer pending")
	}
	now := time.Now().UTC()
	if err := a.ExecutionDAL.TransitionApproval(ctx, approval.ID, owner, schema.TradeApprovalStatusPending,
		map[string]interface{}{"status": schema.TradeApprovalStatusRejected, "acted_by": owner,
			"acted_at": now, "updated_at": now},
		approvalEvent(approval, owner, "rejected", schema.TradeApprovalStatusPending,
			schema.TradeApprovalStatusRejected, map[string]interface{}{"comment": form.Comment})); err != nil {
		if errors.Is(err, gorm.ErrInvalidTransaction) {
			return nil, errors.Conflict("trading_approval_concurrent_action", "Trade approval was already handled")
		}
		return nil, err
	}
	approval.Status, approval.ActedBy, approval.ActedAt, approval.UpdatedAt = schema.TradeApprovalStatusRejected, owner, &now, now
	return approval, nil
}

func (a *Execution) approvalCommandForm(ctx context.Context, approval *schema.TradeApproval) (*schema.TradeCommandForm, error) {
	form := &schema.TradeCommandForm{TraderID: approval.TraderID, DecisionID: &approval.DecisionID,
		IdempotencyKey: "approval:" + approval.ID, Action: approval.Action, Market: approval.Market,
		Symbol: approval.Symbol, Side: approval.Side, PositionSide: approval.PositionSide,
		OrderType: approval.OrderType, Quantity: approval.Quantity, QuoteQuantity: approval.QuoteQuantity,
		Price: approval.Price, StopLoss: approval.StopLoss, TakeProfit: approval.TakeProfit,
		Leverage: approval.Leverage, ReduceOnly: approval.ReduceOnly,
		TargetExchangeOrderID: approval.TargetExchangeOrderID, ConfirmExecution: true}
	quantity, quantityOK := decimalRat(form.Quantity)
	if (quantityOK && quantity.Sign() > 0) || form.Action == "cancel" {
		return form, nil
	}
	quote, ok := decimalRat(form.QuoteQuantity)
	if !ok || quote.Sign() <= 0 {
		return nil, errors.BadRequest("trading_approval_quantity_missing", "Approval has no executable quantity")
	}
	exchange, err := a.CatalogDAL.GetExchange(ctx, approval.ExchangeID, approval.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if exchange == nil {
		return nil, errors.NotFound("trading_exchange_not_found", "Exchange not found")
	}
	ticker, err := a.MarketRules.Ticker(ctx, exchange.ExchangeType, approval.Market, approval.Symbol)
	if err != nil {
		return nil, errors.BadRequest("trading_market_price_failed", "Unable to load market price: %v", err)
	}
	price, ok := decimalRat(ticker.LastPrice)
	if !ok || price.Sign() <= 0 {
		return nil, errors.BadRequest("trading_market_price_invalid", "Market price is unavailable")
	}
	calculatedQuantity := new(big.Rat).Quo(quote, price)
	rules, err := a.MarketRules.InstrumentRules(ctx, exchange.ExchangeType, approval.Market, approval.Symbol)
	if err != nil {
		return nil, errors.BadRequest("trading_market_rules_failed", "Unable to load instrument rules: %v", err)
	}
	if step, stepOK := decimalRat(rules.StepSize); stepOK && step.Sign() > 0 {
		units := new(big.Rat).Quo(calculatedQuantity, step)
		whole := new(big.Int).Quo(units.Num(), units.Denom())
		calculatedQuantity.Mul(new(big.Rat).SetInt(whole), step)
	}
	if calculatedQuantity.Sign() <= 0 {
		return nil, errors.BadRequest("trading_approval_quantity_too_small", "Approval size is below exchange quantity step")
	}
	form.Quantity, form.QuoteQuantity = decimalText(calculatedQuantity), "0"
	return form, nil
}

func approvalEvent(approval *schema.TradeApproval, actor, eventType, from, to string, detail interface{}) *schema.TradeApprovalEvent {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	encoded, _ := json.Marshal(detail)
	return &schema.TradeApprovalEvent{ApprovalID: approval.ID, OwnerUserID: approval.OwnerUserID,
		ActorUserID: strings.TrimSpace(actor), EventType: eventType, FromStatus: from,
		ToStatus: to, DetailJSON: string(encoded), CreatedAt: time.Now().UTC()}
}
