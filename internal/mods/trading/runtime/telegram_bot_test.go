package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTelegramBindingAndOwnerScopedSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.NotificationChannel), new(schema.TelegramBindingCode),
		new(schema.TelegramSession), new(schema.TelegramPollState)); err != nil {
		t.Fatal(err)
	}
	replies := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/sendMessage") {
			t.Fatalf("unexpected Telegram path %s", request.URL.Path)
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		replies = append(replies, payload.Text)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	registry := &Registry{DB: db, telegramAPIBase: server.URL, telegramHTTPClient: server.Client()}
	channel := &schema.NotificationChannel{ID: "channel-a", OwnerUserID: "owner-a", Name: "Telegram",
		ChannelType: schema.NotificationChannelTelegram, Enabled: true, BotTokenCiphertext: "ENC:v1:test"}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	code := "A1B2C3D4E5F6-ABCDEFGHJKLMNPQR"
	if err := db.Create(&schema.TelegramBindingCode{ID: "binding-a", PublicID: "A1B2C3D4E5F6",
		OwnerUserID: "owner-a", ChannelID: channel.ID, CodeHash: schema.HashTelegramBindingCode(code),
		MaxAttempts: 5, ExpiresAt: time.Now().UTC().Add(time.Minute), CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	update := telegramUpdate{UpdateID: 10}
	update.Message = new(struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
	})
	update.Message.Text, update.Message.Chat.ID = "/bind "+code, -100123
	update.Message.From.ID, update.Message.From.Username = 42, "tester"
	registry.handleTelegramUpdate(context.Background(), channel, "token", update)
	var session schema.TelegramSession
	if err := db.First(&session, "channel_id = ?", channel.ID).Error; err != nil {
		t.Fatal(err)
	}
	if session.OwnerUserID != "owner-a" || session.ChatID != "-100123" || session.TelegramUserID != "42" ||
		session.Status != schema.TelegramSessionActive || len(replies) != 1 || !strings.Contains(replies[0], "绑定成功") {
		t.Fatalf("unexpected Telegram binding: session=%#v replies=%#v", session, replies)
	}
	update.UpdateID, update.Message.Text = 11, "/status"
	update.Message.From.ID = 99
	registry.handleTelegramUpdate(context.Background(), channel, "token", update)
	if !strings.Contains(replies[len(replies)-1], "未绑定") {
		t.Fatalf("another Telegram user must not inherit the session: %#v", replies)
	}
}

func TestParseTelegramCommand(t *testing.T) {
	command, argument := parseTelegramCommand(" /BIND@XTradingBot code-123 ")
	if command != "bind" || argument != "code-123" {
		t.Fatalf("unexpected command=%q argument=%q", command, argument)
	}
}
