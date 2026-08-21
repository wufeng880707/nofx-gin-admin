package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestShouldDeliverTraderNotificationByExecutionMode(t *testing.T) {
	tests := []struct {
		mode, event string
		want        bool
	}{
		{schema.TraderExecutionNotifyOnly, schema.TraderNotificationSignal, true},
		{schema.TraderExecutionNotifyOnly, schema.TraderNotificationApprovalRequest, false},
		{schema.TraderExecutionApprovalRequired, schema.TraderNotificationApprovalRequest, true},
		{schema.TraderExecutionApprovalRequired, schema.TraderNotificationExecutionResult, false},
		{schema.TraderExecutionAuto, schema.TraderNotificationExecutionResult, true},
		{schema.TraderExecutionAuto, schema.TraderNotificationSignal, false},
		{schema.TraderExecutionNotifyOnly, schema.TraderNotificationSystemError, true},
		{schema.TraderExecutionApprovalRequired, schema.TraderNotificationSystemError, true},
		{schema.TraderExecutionAuto, schema.TraderNotificationSystemError, true},
	}
	for _, test := range tests {
		if got := shouldDeliverTraderNotification(test.mode, test.event); got != test.want {
			t.Fatalf("mode=%s event=%s: got %v want %v", test.mode, test.event, got, test.want)
		}
	}
}

func TestAutomaticNotificationHasIndependentGlobalGate(t *testing.T) {
	previous := config.C.Trading.Notifications.Enabled
	config.C.Trading.Notifications.Enabled = false
	t.Cleanup(func() { config.C.Trading.Notifications.Enabled = previous })
	err := new(Catalog).DeliverTraderNotification(context.Background(), &schema.Trader{
		ExecutionMode: schema.TraderExecutionNotifyOnly}, schema.TraderNotificationEvent{EventType: schema.TraderNotificationSignal})
	if errorID(err) != "trading_notification_dispatch_disabled" {
		t.Fatalf("automatic notification must be disabled independently, got %v", err)
	}
}

func TestSendTelegramMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["chat_id"] != "-100123" || payload["text"] != "test message" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	if _, err := sendTelegramMessage(context.Background(), server.URL, "-100123", "test message"); err != nil {
		t.Fatal(err)
	}
}

func TestSendLarkMessageWithSignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["msg_type"] != "text" || payload["timestamp"] == "" || payload["sign"] == "" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	if _, err := sendLarkMessage(context.Background(), server.URL, "signing-secret", "test message"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLarkWebhookURLRejectsUntrustedHost(t *testing.T) {
	if err := validateLarkWebhookURL("https://example.com/open-apis/bot/v2/hook/test"); err == nil {
		t.Fatal("expected untrusted Lark webhook host to be rejected")
	}
	if err := validateLarkWebhookURL("https://open.feishu.cn/open-apis/bot/v2/hook/test"); err != nil {
		t.Fatalf("expected official Lark webhook URL to be accepted: %v", err)
	}
}

func TestTelegramBindingCodeStoresHashOnlyAndIsOwnerScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.NotificationChannel), new(schema.TraderNotificationBinding),
		new(schema.TelegramBindingCode), new(schema.TelegramSession)); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{CatalogDAL: &dal.Catalog{DB: db}, Credentials: credential.NewService(make([]byte, 32), nil)}
	ctx := util.NewUserID(context.Background(), "owner-a")
	channel, err := catalog.CreateNotificationChannel(ctx, &schema.NotificationChannelForm{Name: "Telegram",
		ChannelType: schema.NotificationChannelTelegram, Enabled: true, BotToken: "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"})
	if err != nil {
		t.Fatal(err)
	}
	if channel.ChatID != "" {
		t.Fatalf("unbound channel must not require chat id: %#v", channel)
	}
	view, err := catalog.CreateTelegramBindingCode(ctx, channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Code == "" || !strings.Contains(view.Code, "-") {
		t.Fatalf("unexpected one-time code: %#v", view)
	}
	var stored schema.TelegramBindingCode
	if err := db.First(&stored, "channel_id = ?", channel.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CodeHash == "" || stored.CodeHash == view.Code || strings.Contains(stored.CodeHash, view.Code) {
		t.Fatalf("binding code was stored in plaintext: %#v", stored)
	}
	session, err := catalog.GetTelegramSession(ctx, channel.ID)
	if err != nil || session.Bound || session.Status != schema.TelegramSessionUnbound {
		t.Fatalf("unexpected unbound session: %#v err=%v", session, err)
	}
	otherCtx := util.NewUserID(context.Background(), "owner-b")
	if _, err := catalog.CreateTelegramBindingCode(otherCtx, channel.ID); err == nil {
		t.Fatal("cross-owner binding code creation must be rejected")
	}
}
