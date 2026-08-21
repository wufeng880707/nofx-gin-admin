package biz

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type strategyWorkflowAction struct {
	Symbol           string  `json:"symbol"`
	Action           string  `json:"action"`
	Side             string  `json:"side"`
	Confidence       int     `json:"confidence"`
	Leverage         int     `json:"leverage"`
	PositionSizeUSDT float64 `json:"position_size_usdt"`
	StopLoss         float64 `json:"stop_loss"`
	TakeProfit       float64 `json:"take_profit"`
	Reason           string  `json:"reason"`
	Valid            bool    `json:"valid"`
	ValidationError  string  `json:"validation_error"`
}

// ProcessStrategyDecision routes validated AI actions through the selected
// execution mode. Every external side effect remains behind its global gate.
func (a *Execution) ProcessStrategyDecision(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	decision *schema.Decision, raw json.RawMessage) ([]map[string]interface{}, error) {
	var actions []strategyWorkflowAction
	if err := json.Unmarshal(raw, &actions); err != nil {
		return nil, fmt.Errorf("decode strategy actions: %w", err)
	}
	logs := make([]map[string]interface{}, 0, len(actions))
	if trader.ExecutionMode == schema.TraderExecutionNotifyOnly {
		event := schema.TraderNotificationEvent{EventType: schema.TraderNotificationSignal,
			Title: "策略信号", Body: formatWorkflowActions(actions)}
		return append(logs, notificationWorkflowLog(a.deliverDecisionNotification(ctx, trader, event))), nil
	}
	if trader.ExecutionMode == schema.TraderExecutionAuto {
		return a.processAutomaticActions(ctx, trader, exchange, decision, actions), nil
	}
	if trader.ExecutionMode != schema.TraderExecutionApprovalRequired {
		return nil, fmt.Errorf("unsupported execution mode %q", trader.ExecutionMode)
	}

	for index, action := range actions {
		if !action.Valid || strings.EqualFold(action.Action, "HOLD") {
			logs = append(logs, map[string]interface{}{"stage": "approval", "action_index": index,
				"status": "skipped", "reason": firstWorkflowValue(action.ValidationError, "hold/no executable action")})
			continue
		}
		approval, err := a.buildStrategyApproval(ctx, trader, exchange, decision, index, action)
		if err != nil {
			logs = append(logs, map[string]interface{}{"stage": "approval", "action_index": index,
				"status": "failed", "error": err.Error()})
			continue
		}
		event := approvalEvent(approval, "system", "requested", "", schema.TradeApprovalStatusPending,
			map[string]interface{}{"decision_id": decision.ID, "action_index": index})
		if err := a.ExecutionDAL.CreateApprovalWithEvent(ctx, approval, event); err != nil {
			logs = append(logs, map[string]interface{}{"stage": "approval", "action_index": index,
				"status": "failed", "error": err.Error()})
			continue
		}
		logs = append(logs, map[string]interface{}{"stage": "approval", "action_index": index,
			"status": "pending", "approval_id": approval.ID})
		body := fmt.Sprintf("%s\n审批入口：%s", formatWorkflowAction(action), approvalURL(approval.ID))
		notifyLog := notificationWorkflowLog(a.deliverDecisionNotification(ctx, trader,
			schema.TraderNotificationEvent{EventType: schema.TraderNotificationApprovalRequest,
				Title: "交易审批", Body: body}))
		notifyLog["approval_id"] = approval.ID
		logs = append(logs, notifyLog)
	}
	return logs, nil
}

func (a *Execution) processAutomaticActions(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	decision *schema.Decision, actions []strategyWorkflowAction) []map[string]interface{} {
	logs := make([]map[string]interface{}, 0, len(actions)*2)
	ownerCtx := util.NewUserID(ctx, trader.OwnerUserID)
	for index, action := range actions {
		if !action.Valid || strings.EqualFold(action.Action, "HOLD") {
			logs = append(logs, map[string]interface{}{"stage": "auto_execution", "action_index": index,
				"status": "skipped", "reason": firstWorkflowValue(action.ValidationError, "hold/no executable action")})
			continue
		}
		approval, err := a.buildStrategyApproval(ownerCtx, trader, exchange, decision, index, action)
		if err != nil {
			logs = append(logs, map[string]interface{}{"stage": "auto_execution", "action_index": index,
				"status": "failed", "error": err.Error()})
			continue
		}
		form, err := a.approvalCommandForm(ownerCtx, approval)
		if err == nil {
			form.IdempotencyKey = fmt.Sprintf("strategy:%d:%d", decision.ID, index)
			form.ConfirmExecution = true
			err = form.Validate()
		}
		var command *schema.TradeCommand
		if err == nil {
			command, err = a.CreateCommand(ownerCtx, form)
		}
		status, body := "succeeded", formatWorkflowAction(action)
		entry := map[string]interface{}{"stage": "auto_execution", "action_index": index}
		if err != nil {
			status, body = "failed", body+"\n结果："+err.Error()
			entry["error"] = err.Error()
		} else {
			body += fmt.Sprintf("\n命令：%s\n状态：%s", command.ID, command.Status)
			entry["command_id"], entry["command_status"] = command.ID, command.Status
		}
		entry["status"] = status
		logs = append(logs, entry)
		logs = append(logs, notificationWorkflowLog(a.deliverDecisionNotification(ownerCtx, trader,
			schema.TraderNotificationEvent{EventType: schema.TraderNotificationExecutionResult,
				Title: "自动交易结果", Body: body})))
	}
	return logs
}

func (a *Execution) deliverDecisionNotification(ctx context.Context, trader *schema.Trader,
	event schema.TraderNotificationEvent) error {
	if a.Notifier == nil {
		return stderrors.New("notification service is unavailable")
	}
	return a.Notifier.DeliverTraderNotification(util.NewUserID(ctx, trader.OwnerUserID), trader, event)
}

func notificationWorkflowLog(err error) map[string]interface{} {
	result := map[string]interface{}{"stage": "notification", "status": "delivered"}
	if err != nil {
		result["status"], result["error"] = "failed", err.Error()
	}
	return result
}

func formatWorkflowActions(actions []strategyWorkflowAction) string {
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		if action.Valid && !strings.EqualFold(action.Action, "HOLD") {
			lines = append(lines, formatWorkflowAction(action))
		}
	}
	if len(lines) == 0 {
		return "本轮没有可执行信号。"
	}
	return strings.Join(lines, "\n")
}

func formatWorkflowAction(action strategyWorkflowAction) string {
	return fmt.Sprintf("%s %s %s，仓位 %.2f USDT，杠杆 %dx，止损 %.8g，止盈 %.8g，置信度 %d%%",
		strings.ToUpper(action.Symbol), strings.ToUpper(action.Action), strings.ToUpper(action.Side),
		action.PositionSizeUSDT, maxWorkflowInt(action.Leverage, 1), action.StopLoss, action.TakeProfit, action.Confidence)
}

func approvalURL(id string) string {
	base := strings.TrimRight(strings.TrimSpace(config.C.Trading.Notifications.ApprovalBaseURL), "/")
	if base == "" {
		return "审批中心（请登录 XTrading）"
	}
	return base + "?approval=" + id
}

func (a *Execution) buildStrategyApproval(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	decision *schema.Decision, index int, action strategyWorkflowAction) (*schema.TradeApproval, error) {
	verb := strings.ToLower(strings.TrimSpace(action.Action))
	if verb == "cancel" {
		return nil, stderrors.New("strategy cancellation requires a target exchange order id")
	}
	if verb != "open" && verb != "add" && verb != "reduce" && verb != "close" {
		return nil, fmt.Errorf("unsupported strategy action %q", action.Action)
	}
	market := schema.MarketTypeFutures
	if strings.EqualFold(exchange.AccountType, schema.MarketTypeSpot) {
		market = schema.MarketTypeSpot
	}
	positionSide := strings.ToLower(strings.TrimSpace(action.Side))
	if positionSide != "long" && positionSide != "short" {
		return nil, stderrors.New("strategy action has no valid position side")
	}
	if market == schema.MarketTypeSpot && positionSide == "short" {
		return nil, stderrors.New("spot account cannot execute a short action")
	}
	side := "buy"
	if ((verb == "open" || verb == "add") && positionSide == "short") ||
		((verb == "reduce" || verb == "close") && positionSide == "long") {
		side = "sell"
	}
	quantity, quoteQuantity := "0", "0"
	if verb == "open" || verb == "add" {
		if action.PositionSizeUSDT <= 0 {
			return nil, stderrors.New("strategy action has no positive position size")
		}
		quoteQuantity = strconv.FormatFloat(action.PositionSizeUSDT, 'f', -1, 64)
	} else {
		var position schema.Position
		err := a.ExecutionDAL.DB.WithContext(ctx).Where(
			"owner_user_id = ? AND trader_id = ? AND symbol = ? AND side = ? AND status = ?",
			trader.OwnerUserID, trader.ID, strings.ToUpper(action.Symbol), positionSide, schema.PositionStatusOpen).
			Order("opened_at ASC").First(&position).Error
		if err != nil {
			return nil, fmt.Errorf("load reducible position: %w", err)
		}
		quantity = position.Quantity
	}
	now := time.Now().UTC()
	return &schema.TradeApproval{ID: util.MustNewUUID(), OwnerUserID: trader.OwnerUserID,
		TraderID: trader.ID, ExchangeID: exchange.ID, DecisionID: decision.ID, ActionIndex: index,
		Action: verb, Market: market, Symbol: strings.ToUpper(action.Symbol), Side: side,
		PositionSide: positionSide, OrderType: "market", Quantity: quantity, QuoteQuantity: quoteQuantity,
		Price: "0", StopLoss: strconv.FormatFloat(action.StopLoss, 'f', -1, 64),
		TakeProfit: strconv.FormatFloat(action.TakeProfit, 'f', -1, 64),
		Leverage:   maxWorkflowInt(action.Leverage, 1), ReduceOnly: verb == "reduce" || verb == "close",
		Reason: action.Reason, Status: schema.TradeApprovalStatusPending,
		RequestedAt: now, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now}, nil
}

func firstWorkflowValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxWorkflowInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
