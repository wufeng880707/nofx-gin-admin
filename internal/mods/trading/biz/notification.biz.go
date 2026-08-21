package biz

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

var (
	telegramTokenPattern = regexp.MustCompile(`^[0-9]{6,16}:[A-Za-z0-9_-]{20,}$`)
	telegramChatPattern  = regexp.MustCompile(`^(?:-?[0-9]+|@[A-Za-z0-9_]{5,})$`)
)

func (a *Catalog) QueryNotificationChannels(ctx context.Context, params schema.NotificationChannelQueryParam) (*schema.NotificationChannelViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.CatalogDAL.QueryNotificationChannels(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.NotificationChannelView, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, buildNotificationChannelView(item))
	}
	return &schema.NotificationChannelViewResult{Data: data, PageResult: result.PageResult}, nil
}

func (a *Catalog) GetNotificationChannel(ctx context.Context, id string) (*schema.NotificationChannelView, error) {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildNotificationChannelView(item), nil
}

func (a *Catalog) CreateNotificationChannel(ctx context.Context, form *schema.NotificationChannelForm) (*schema.NotificationChannelView, error) {
	owner, err := resolveMutationOwner(ctx, form.OwnerUserID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &schema.NotificationChannel{ID: util.MustNewUUID(), OwnerUserID: owner, Name: form.Name,
		ChannelType: form.ChannelType, Enabled: form.Enabled, ChatID: form.ChatID,
		CredentialVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := a.applyNotificationCredentials(item, form); err != nil {
		return nil, err
	}
	if err := a.CatalogDAL.CreateNotificationChannel(ctx, item); err != nil {
		return nil, err
	}
	return buildNotificationChannelView(item), nil
}

func (a *Catalog) UpdateNotificationChannel(ctx context.Context, id string, form *schema.NotificationChannelForm) error {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return err
	}
	previousType := item.ChannelType
	item.Name, item.ChannelType, item.Enabled, item.ChatID = form.Name, form.ChannelType, form.Enabled, form.ChatID
	item.UpdatedAt = time.Now()
	if previousType != item.ChannelType {
		item.BotTokenCiphertext, item.WebhookURLCiphertext, item.SigningSecretCiphertext = "", "", ""
	}
	if err := a.applyNotificationCredentials(item, form); err != nil {
		return err
	}
	return a.CatalogDAL.UpdateNotificationChannel(ctx, item)
}

func (a *Catalog) DeleteNotificationChannel(ctx context.Context, id string) error {
	if _, err := a.getNotificationChannel(ctx, id); err != nil {
		return err
	}
	return a.CatalogDAL.DeleteNotificationChannel(ctx, id)
}

func (a *Catalog) TestNotificationChannel(ctx context.Context, id string) (*schema.NotificationChannelTestView, error) {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	message, sendErr := a.sendNotification(ctx, item, "XTrading 通知渠道测试成功。后续交易信号、审批和执行结果将发送到此渠道。")
	now := time.Now()
	item.LastTestedAt = &now
	item.LastTestStatus = "success"
	item.LastTestMessage = message
	if sendErr != nil {
		item.LastTestStatus = "failed"
		item.LastTestMessage = sendErr.Error()
	}
	item.UpdatedAt = now
	if updateErr := a.CatalogDAL.UpdateNotificationChannel(ctx, item); updateErr != nil {
		return nil, updateErr
	}
	if sendErr != nil {
		return nil, errors.BadRequest("trading_notification_test_failed", "Notification delivery failed: %v", sendErr)
	}
	return &schema.NotificationChannelTestView{ChannelID: item.ID, ChannelType: item.ChannelType,
		Delivered: true, LatencyMS: time.Since(started).Milliseconds(), Message: message}, nil
}

func (a *Catalog) CreateTelegramBindingCode(ctx context.Context, id string) (*schema.TelegramBindingCodeView, error) {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.ChannelType != schema.NotificationChannelTelegram {
		return nil, errors.BadRequest("trading_telegram_channel_required", "Telegram binding is only available for Telegram channels")
	}
	if !item.Enabled {
		return nil, errors.BadRequest("trading_telegram_channel_disabled", "Enable the Telegram channel before creating a binding code")
	}
	publicBytes, secretBytes := make([]byte, 6), make([]byte, 10)
	if _, err := rand.Read(publicBytes); err != nil {
		return nil, err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	publicID := strings.ToUpper(hex.EncodeToString(publicBytes))
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	code := publicID + "-" + secret
	ttl := config.C.Trading.TelegramBot.BindingTTLMinutes
	if ttl <= 0 {
		ttl = 10
	}
	now := time.Now().UTC()
	binding := &schema.TelegramBindingCode{ID: util.MustNewUUID(), PublicID: publicID,
		OwnerUserID: item.OwnerUserID, ChannelID: item.ID, CodeHash: schema.HashTelegramBindingCode(code),
		MaxAttempts: schema.TelegramBindingMaxTries, ExpiresAt: now.Add(time.Duration(ttl) * time.Minute), CreatedAt: now}
	if err := a.CatalogDAL.ReplaceTelegramBindingCode(ctx, binding); err != nil {
		return nil, err
	}
	return &schema.TelegramBindingCodeView{ChannelID: item.ID, Code: code, ExpiresAt: binding.ExpiresAt}, nil
}

func (a *Catalog) GetTelegramSession(ctx context.Context, id string) (*schema.TelegramSessionView, error) {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	if item.ChannelType != schema.NotificationChannelTelegram {
		return nil, errors.BadRequest("trading_telegram_channel_required", "Telegram session is only available for Telegram channels")
	}
	session, err := a.CatalogDAL.GetTelegramSession(ctx, item.ID, item.OwnerUserID)
	if err != nil {
		return nil, err
	}
	view := &schema.TelegramSessionView{ChannelID: item.ID, Status: schema.TelegramSessionUnbound}
	if session != nil && session.Status == schema.TelegramSessionActive {
		boundAt := session.BoundAt
		view.Bound, view.Status, view.ChatID = true, session.Status, session.ChatID
		view.TelegramUsername, view.BoundAt, view.LastMessageAt = session.TelegramUsername, &boundAt, session.LastMessageAt
	}
	return view, nil
}

func (a *Catalog) UnbindTelegramSession(ctx context.Context, id string) error {
	item, err := a.getNotificationChannel(ctx, id)
	if err != nil {
		return err
	}
	if item.ChannelType != schema.NotificationChannelTelegram {
		return errors.BadRequest("trading_telegram_channel_required", "Telegram session is only available for Telegram channels")
	}
	return a.CatalogDAL.UnbindTelegramSession(ctx, item.ID, item.OwnerUserID)
}

// DeliverTraderNotification sends one trading event to every enabled channel bound to the Trader.
// Order execution remains the caller's responsibility; this method only performs outbound delivery.
func (a *Catalog) DeliverTraderNotification(ctx context.Context, trader *schema.Trader, event schema.TraderNotificationEvent) error {
	if trader == nil {
		return fmt.Errorf("trader is required")
	}
	if !config.C.Trading.Notifications.Enabled {
		return errors.Forbidden("trading_notification_dispatch_disabled",
			"Automatic trading notifications are disabled by the server operator")
	}
	if !shouldDeliverTraderNotification(trader.ExecutionMode, event.EventType) {
		return nil
	}
	channels, err := a.CatalogDAL.ListEnabledNotificationChannelsForTrader(ctx, trader.ID, trader.OwnerUserID)
	if err != nil {
		return err
	}
	text := formatTraderNotification(trader, event)
	failed := make([]string, 0)
	for _, channel := range channels {
		if _, sendErr := a.sendNotification(ctx, channel, text); sendErr != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", channel.Name, sendErr))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("deliver trader notification: %s", strings.Join(failed, "; "))
	}
	return nil
}

func shouldDeliverTraderNotification(executionMode, eventType string) bool {
	if eventType == schema.TraderNotificationSystemError {
		return true
	}
	switch executionMode {
	case schema.TraderExecutionNotifyOnly:
		return eventType == schema.TraderNotificationSignal
	case schema.TraderExecutionApprovalRequired:
		return eventType == schema.TraderNotificationApprovalRequest
	case schema.TraderExecutionAuto:
		return eventType == schema.TraderNotificationExecutionResult
	default:
		return false
	}
}

func formatTraderNotification(trader *schema.Trader, event schema.TraderNotificationEvent) string {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = "交易通知"
	}
	return fmt.Sprintf("【XTrading】%s\nTrader：%s\n模式：%s\n%s", title, trader.Name, trader.ExecutionMode, strings.TrimSpace(event.Body))
}

func (a *Catalog) getNotificationChannel(ctx context.Context, id string) (*schema.NotificationChannel, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetNotificationChannel(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_notification_channel_not_found", "Notification channel not found")
	}
	return item, nil
}

func (a *Catalog) applyNotificationCredentials(item *schema.NotificationChannel, form *schema.NotificationChannelForm) error {
	encrypt := func(plain, field string, target *string) error {
		if plain == "" {
			return nil
		}
		value, err := a.Credentials.EncryptField(plain, item.OwnerUserID, item.ID, field)
		if err != nil {
			return errors.BadRequest("trading_credential_encryption_failed", "Unable to encrypt notification credential: %v", err)
		}
		*target = value
		return nil
	}
	if err := encrypt(form.BotToken, "bot_token", &item.BotTokenCiphertext); err != nil {
		return err
	}
	if err := encrypt(form.WebhookURL, "webhook_url", &item.WebhookURLCiphertext); err != nil {
		return err
	}
	if err := encrypt(form.SigningSecret, "signing_secret", &item.SigningSecretCiphertext); err != nil {
		return err
	}
	switch item.ChannelType {
	case schema.NotificationChannelTelegram:
		if form.BotToken != "" && !telegramTokenPattern.MatchString(form.BotToken) {
			return errors.BadRequest("trading_telegram_token_invalid", "Telegram Bot Token format is invalid")
		}
		if item.ChatID != "" && !telegramChatPattern.MatchString(item.ChatID) {
			return errors.BadRequest("trading_telegram_chat_invalid", "Telegram Chat ID must be a numeric ID or @channel username")
		}
		if item.BotTokenCiphertext == "" {
			return errors.BadRequest("trading_telegram_token_required", "Telegram Bot Token is required")
		}
		item.WebhookURLCiphertext, item.SigningSecretCiphertext = "", ""
	case schema.NotificationChannelLark:
		if form.WebhookURL != "" {
			if err := validateLarkWebhookURL(form.WebhookURL); err != nil {
				return err
			}
		}
		if item.WebhookURLCiphertext == "" {
			return errors.BadRequest("trading_lark_webhook_required", "Lark webhook URL is required")
		}
		item.BotTokenCiphertext, item.ChatID = "", ""
	default:
		return errors.BadRequest("trading_notification_channel_type_invalid", "Unsupported notification channel type")
	}
	return nil
}

func validateLarkWebhookURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return errors.BadRequest("trading_lark_webhook_invalid", "Lark webhook URL must use HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "open.feishu.cn" && host != "open.larksuite.com" {
		return errors.BadRequest("trading_lark_webhook_invalid", "Lark webhook host is not allowed")
	}
	if !strings.HasPrefix(parsed.Path, "/open-apis/bot/v2/hook/") {
		return errors.BadRequest("trading_lark_webhook_invalid", "Lark webhook path is invalid")
	}
	return nil
}

func (a *Catalog) sendNotification(ctx context.Context, item *schema.NotificationChannel, text string) (string, error) {
	switch item.ChannelType {
	case schema.NotificationChannelTelegram:
		if strings.TrimSpace(item.ChatID) == "" {
			return "", fmt.Errorf("Telegram channel is not bound to a chat")
		}
		token, err := a.Credentials.DecryptField(item.BotTokenCiphertext, item.OwnerUserID, item.ID, "bot_token")
		if err != nil {
			return "", fmt.Errorf("decrypt Telegram credential: %w", err)
		}
		endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"
		return sendTelegramMessage(ctx, endpoint, item.ChatID, text)
	case schema.NotificationChannelLark:
		webhook, err := a.Credentials.DecryptField(item.WebhookURLCiphertext, item.OwnerUserID, item.ID, "webhook_url")
		if err != nil {
			return "", fmt.Errorf("decrypt Lark credential: %w", err)
		}
		if err := validateLarkWebhookURL(webhook); err != nil {
			return "", fmt.Errorf("validate Lark webhook: %w", err)
		}
		secret, err := a.Credentials.DecryptField(item.SigningSecretCiphertext, item.OwnerUserID, item.ID, "signing_secret")
		if err != nil {
			return "", fmt.Errorf("decrypt Lark signing secret: %w", err)
		}
		return sendLarkMessage(ctx, webhook, secret, text)
	default:
		return "", fmt.Errorf("unsupported notification channel type %q", item.ChannelType)
	}
}

func sendTelegramMessage(ctx context.Context, endpoint, chatID, text string) (string, error) {
	payload := map[string]interface{}{"chat_id": chatID, "text": text, "disable_web_page_preview": true}
	var response struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := postNotificationJSON(ctx, endpoint, payload, &response); err != nil {
		return "", err
	}
	if !response.OK {
		return "", fmt.Errorf("Telegram rejected message: %s", response.Description)
	}
	return "Telegram test message delivered", nil
}

func sendLarkMessage(ctx context.Context, endpoint, secret, text string) (string, error) {
	payload := map[string]interface{}{"msg_type": "text", "content": map[string]string{"text": text}}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		payload["timestamp"] = timestamp
		payload["sign"] = larkSignature(timestamp, secret)
	}
	var response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := postNotificationJSON(ctx, endpoint, payload, &response); err != nil {
		return "", err
	}
	if response.Code != 0 {
		return "", fmt.Errorf("Lark rejected message: code=%d message=%s", response.Code, response.Msg)
	}
	return "Lark test message delivered", nil
}

func larkSignature(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	_, _ = mac.Write(nil)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func postNotificationJSON(ctx context.Context, endpoint string, payload, response interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	httpResponse, err := client.Do(request)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()
	data, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return fmt.Errorf("notification provider returned HTTP %d", httpResponse.StatusCode)
	}
	if err := json.Unmarshal(data, response); err != nil {
		return fmt.Errorf("decode notification provider response: %w", err)
	}
	return nil
}

func buildNotificationChannelView(item *schema.NotificationChannel) *schema.NotificationChannelView {
	return &schema.NotificationChannelView{ID: item.ID, OwnerUserID: item.OwnerUserID, Name: item.Name,
		ChannelType: item.ChannelType, Enabled: item.Enabled, ChatID: item.ChatID,
		HasBotToken: item.BotTokenCiphertext != "", HasWebhookURL: item.WebhookURLCiphertext != "",
		HasSigningSecret: item.SigningSecretCiphertext != "", LastTestedAt: item.LastTestedAt,
		LastTestStatus: item.LastTestStatus, LastTestMessage: item.LastTestMessage,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
