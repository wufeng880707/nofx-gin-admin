package biz

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

func TestApprovalCreatesOneAuditedSimulationCommand(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	now := time.Now().UTC()
	approval := &schema.TradeApproval{ID: "approval-a", OwnerUserID: "owner-a", TraderID: "trader-a",
		ExchangeID: "exchange-a", DecisionID: 1, ActionIndex: 0, Action: "open", Market: "spot",
		Symbol: "BTCUSDT", Side: "buy", PositionSide: "long", OrderType: "market",
		Quantity: "0", QuoteQuantity: "20", Price: "0", StopLoss: "1900", TakeProfit: "2200", Leverage: 1,
		Status: schema.TradeApprovalStatusPending, RequestedAt: now, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
	if err := service.ExecutionDAL.CreateApprovalWithEvent(context.Background(), approval,
		approvalEvent(approval, "system", "requested", "", schema.TradeApprovalStatusPending, nil)); err != nil {
		t.Fatal(err)
	}
	ctx := util.NewUserID(context.Background(), "owner-a")
	form := &schema.TradeApprovalActForm{ConfirmationPhrase: "APPROVE TRADE", Comment: "reviewed"}
	if err := form.ValidateApprove(); err != nil {
		t.Fatal(err)
	}
	result, err := service.Approve(ctx, approval.ID, form)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != schema.TradeApprovalStatusSucceeded || result.CommandID == "" {
		t.Fatalf("unexpected approval result: %#v", result)
	}
	if factory.calls != 0 {
		t.Fatalf("approved simulation must not call provider, calls=%d", factory.calls)
	}
	var command schema.TradeCommand
	if err := db.First(&command, "id = ?", result.CommandID).Error; err != nil {
		t.Fatal(err)
	}
	if command.Status != schema.TradeCommandStatusSimulated || command.Quantity != "0.01" {
		t.Fatalf("unexpected command: %#v", command)
	}
	if _, err := service.Approve(ctx, approval.ID, form); errorID(err) != "trading_approval_not_pending" {
		t.Fatalf("second approval must be rejected, got %v", err)
	}
	var events []schema.TradeApprovalEvent
	if err := db.Where("approval_id = ?", approval.ID).Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[1].ToStatus != schema.TradeApprovalStatusApproved ||
		events[2].ToStatus != schema.TradeApprovalStatusExecuting || events[3].ToStatus != schema.TradeApprovalStatusSucceeded {
		t.Fatalf("unexpected approval audit: %#v", events)
	}
}

func TestApprovalRejectAndExpireAreTerminal(t *testing.T) {
	service, db, _ := newExecutionTestService(t, true)
	ctx := util.NewUserID(context.Background(), "owner-a")
	now := time.Now().UTC()
	items := []*schema.TradeApproval{
		{ID: "reject-a", OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a", DecisionID: 2,
			ActionIndex: 0, Action: "open", Market: "spot", Symbol: "ETHUSDT", Side: "buy", PositionSide: "long",
			OrderType: "market", Quantity: "0.01", QuoteQuantity: "0", Price: "0", Leverage: 1,
			Status: schema.TradeApprovalStatusPending, RequestedAt: now, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now},
		{ID: "expire-a", OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a", DecisionID: 3,
			ActionIndex: 0, Action: "open", Market: "spot", Symbol: "BNBUSDT", Side: "buy", PositionSide: "long",
			OrderType: "market", Quantity: "0.1", QuoteQuantity: "0", Price: "0", Leverage: 1,
			Status: schema.TradeApprovalStatusPending, RequestedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute), CreatedAt: now, UpdatedAt: now},
	}
	for _, item := range items {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	reject := &schema.TradeApprovalActForm{Comment: "risk rejected"}
	if err := reject.ValidateReject(); err != nil {
		t.Fatal(err)
	}
	if result, err := service.RejectApproval(ctx, "reject-a", reject); err != nil || result.Status != schema.TradeApprovalStatusRejected {
		t.Fatalf("reject approval: result=%#v err=%v", result, err)
	}
	if _, err := service.QueryApprovals(ctx, schema.TradeApprovalQueryParam{}); err != nil {
		t.Fatal(err)
	}
	var expired schema.TradeApproval
	if err := db.First(&expired, "id = ?", "expire-a").Error; err != nil {
		t.Fatal(err)
	}
	if expired.Status != schema.TradeApprovalStatusExpired {
		t.Fatalf("approval was not expired: %#v", expired)
	}
}

type workflowNotificationRecorder struct {
	events []schema.TraderNotificationEvent
}

func (a *workflowNotificationRecorder) DeliverTraderNotification(_ context.Context, _ *schema.Trader,
	event schema.TraderNotificationEvent) error {
	a.events = append(a.events, event)
	return nil
}

func TestApprovalWorkflowCreatesRequestAndNotification(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	notifier := new(workflowNotificationRecorder)
	service.Notifier = notifier
	decision := &schema.Decision{OwnerUserID: "owner-a", TraderID: "trader-a", ExchangeID: "exchange-a",
		CycleNumber: 1, DecidedAt: time.Now(), DecisionJSON: `{}`, CandidateCoinsJSON: `[]`,
		ExecutionLogJSON: `[]`, ActionsJSON: `[]`, CreatedAt: time.Now()}
	if err := db.Create(decision).Error; err != nil {
		t.Fatal(err)
	}
	var trader schema.Trader
	var exchange schema.Exchange
	if err := db.First(&trader, "id = ?", "trader-a").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&exchange, "id = ?", "exchange-a").Error; err != nil {
		t.Fatal(err)
	}
	actions, _ := json.Marshal([]map[string]interface{}{{"symbol": "BTCUSDT", "action": "OPEN", "side": "long",
		"confidence": 90, "leverage": 1, "position_size_usdt": 20, "stop_loss": 1900,
		"take_profit": 2200, "reason": "test", "valid": true}})
	logs, err := service.ProcessStrategyDecision(context.Background(), &trader, &exchange, decision, actions)
	if err != nil {
		t.Fatal(err)
	}
	if factory.calls != 0 {
		t.Fatalf("approval workflow must not call trading provider, calls=%d", factory.calls)
	}
	var approval schema.TradeApproval
	if err := db.Where("decision_id = ?", decision.ID).First(&approval).Error; err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(logs)
	if approval.Status != schema.TradeApprovalStatusPending || approval.StopLoss != "1900" || approval.TakeProfit != "2200" ||
		!strings.Contains(string(encoded), `"status":"delivered"`) || len(notifier.events) != 1 ||
		notifier.events[0].EventType != schema.TraderNotificationApprovalRequest {
		t.Fatalf("unexpected internal approval workflow: approval=%#v logs=%s", approval, encoded)
	}
}

func TestNotifyOnlyWorkflowDeliversSignalWithoutCommand(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	notifier := new(workflowNotificationRecorder)
	service.Notifier = notifier
	var trader schema.Trader
	var exchange schema.Exchange
	_ = db.First(&trader, "id = ?", "trader-a").Error
	_ = db.First(&exchange, "id = ?", "exchange-a").Error
	trader.ExecutionMode = schema.TraderExecutionNotifyOnly
	decision := &schema.Decision{ID: 10, OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, ExchangeID: exchange.ID}
	actions := json.RawMessage(`[{"symbol":"BTCUSDT","action":"OPEN","side":"long","confidence":90,"leverage":1,"position_size_usdt":20,"stop_loss":1900,"take_profit":2200,"valid":true}]`)
	logs, err := service.ProcessStrategyDecision(context.Background(), &trader, &exchange, decision, actions)
	if err != nil || factory.calls != 0 || len(notifier.events) != 1 ||
		notifier.events[0].EventType != schema.TraderNotificationSignal {
		t.Fatalf("notify-only workflow failed: logs=%#v events=%#v calls=%d err=%v", logs, notifier.events, factory.calls, err)
	}
	var count int64
	if err := db.Model(new(schema.TradeCommand)).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("notify-only workflow created commands: count=%d err=%v", count, err)
	}
}

func TestAutoWorkflowCreatesAuditedSimulationCommandAndResultNotification(t *testing.T) {
	service, db, factory := newExecutionTestService(t, true)
	notifier := new(workflowNotificationRecorder)
	service.Notifier = notifier
	var trader schema.Trader
	var exchange schema.Exchange
	_ = db.First(&trader, "id = ?", "trader-a").Error
	_ = db.First(&exchange, "id = ?", "exchange-a").Error
	trader.ExecutionMode = schema.TraderExecutionAuto
	if err := db.Model(&trader).Update("execution_mode", trader.ExecutionMode).Error; err != nil {
		t.Fatal(err)
	}
	decision := &schema.Decision{ID: 11, OwnerUserID: trader.OwnerUserID, TraderID: trader.ID, ExchangeID: exchange.ID}
	actions := json.RawMessage(`[{"symbol":"BTCUSDT","action":"OPEN","side":"long","confidence":90,"leverage":1,"position_size_usdt":20,"stop_loss":1900,"take_profit":2200,"valid":true}]`)
	logs, err := service.ProcessStrategyDecision(context.Background(), &trader, &exchange, decision, actions)
	if err != nil || factory.calls != 0 || len(notifier.events) != 1 ||
		notifier.events[0].EventType != schema.TraderNotificationExecutionResult {
		t.Fatalf("auto workflow failed: logs=%#v events=%#v calls=%d err=%v", logs, notifier.events, factory.calls, err)
	}
	var command schema.TradeCommand
	if err := db.First(&command, "idempotency_key = ?", "strategy:11:0").Error; err != nil {
		t.Fatal(err)
	}
	if command.Status != schema.TradeCommandStatusSimulated || command.Quantity != "0.01" {
		t.Fatalf("unexpected automatic simulation command: %#v", command)
	}
}
