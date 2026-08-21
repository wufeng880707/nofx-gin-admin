package runtime

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type telegramWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	token  string
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

type telegramAPIResponse struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description"`
	Result      []telegramUpdate `json:"result"`
}

func (a *Registry) StartTelegramBot() error {
	if a.ProviderFactory == nil || a.ProviderFactory.Credentials == nil {
		return fmt.Errorf("Telegram bot credential service is unavailable")
	}
	a.telegramMu.Lock()
	defer a.telegramMu.Unlock()
	if a.telegramCancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.telegramCancel, a.telegramDone = cancel, make(chan struct{})
	go a.runTelegramManager(ctx)
	return nil
}

func (a *Registry) StopTelegramBot(ctx context.Context) error {
	a.telegramMu.Lock()
	cancel, done := a.telegramCancel, a.telegramDone
	a.telegramCancel, a.telegramDone = nil, nil
	workers := make([]*telegramWorker, 0, len(a.telegramWorkers))
	for _, item := range a.telegramWorkers {
		workers = append(workers, item)
		item.cancel()
	}
	a.telegramWorkers = make(map[string]*telegramWorker)
	a.telegramMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	for _, item := range workers {
		select {
		case <-item.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Registry) runTelegramManager(ctx context.Context) {
	defer close(a.telegramDone)
	refresh := time.Duration(config.C.Trading.TelegramBot.RefreshSeconds) * time.Second
	if refresh < 5*time.Second {
		refresh = 30 * time.Second
	}
	ticker := time.NewTicker(refresh)
	defer ticker.Stop()
	for {
		a.refreshTelegramWorkers(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Registry) refreshTelegramWorkers(ctx context.Context) {
	var channels []schema.NotificationChannel
	if err := a.DB.WithContext(ctx).Where("channel_type = ? AND enabled = ?",
		schema.NotificationChannelTelegram, true).Find(&channels).Error; err != nil {
		return
	}
	active := make(map[string]bool, len(channels))
	for index := range channels {
		channel := &channels[index]
		token, err := a.ProviderFactory.Credentials.DecryptField(channel.BotTokenCiphertext,
			channel.OwnerUserID, channel.ID, "bot_token")
		if err != nil || strings.TrimSpace(token) == "" {
			continue
		}
		active[channel.ID] = true
		a.telegramMu.Lock()
		current := a.telegramWorkers[channel.ID]
		if current != nil && current.token == token {
			a.telegramMu.Unlock()
			continue
		}
		if current != nil {
			current.cancel()
		}
		workerCtx, cancel := context.WithCancel(ctx)
		worker := &telegramWorker{cancel: cancel, done: make(chan struct{}), token: token}
		a.telegramWorkers[channel.ID] = worker
		a.telegramMu.Unlock()
		go a.runTelegramWorker(workerCtx, worker, channel, token)
	}
	a.telegramMu.Lock()
	for id, item := range a.telegramWorkers {
		if !active[id] {
			item.cancel()
			delete(a.telegramWorkers, id)
		}
	}
	a.telegramMu.Unlock()
}

func (a *Registry) runTelegramWorker(ctx context.Context, worker *telegramWorker,
	channel *schema.NotificationChannel, token string) {
	defer close(worker.done)
	var state schema.TelegramPollState
	if err := a.DB.WithContext(ctx).First(&state, "channel_id = ?", channel.ID).Error; err != nil && err != gorm.ErrRecordNotFound {
		return
	}
	state.ChannelID = channel.ID
	for ctx.Err() == nil {
		updates, err := a.pollTelegram(ctx, token, state.LastUpdateID+1)
		if err != nil {
			a.saveTelegramPollState(ctx, &state, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, update := range updates {
			if update.UpdateID <= state.LastUpdateID {
				continue
			}
			if update.Message != nil {
				a.handleTelegramUpdate(ctx, channel, token, update)
			}
			state.LastUpdateID = update.UpdateID
			a.saveTelegramPollState(ctx, &state, nil)
		}
		if len(updates) == 0 {
			a.saveTelegramPollState(ctx, &state, nil)
		}
	}
}

func (a *Registry) pollTelegram(ctx context.Context, token string, offset int64) ([]telegramUpdate, error) {
	timeout := config.C.Trading.TelegramBot.PollTimeoutSeconds
	if timeout < 1 || timeout > 50 {
		timeout = 25
	}
	endpoint := strings.TrimRight(a.telegramAPIBase, "/") + "/bot" + token + "/getUpdates"
	query := url.Values{"offset": {strconv.FormatInt(offset, 10)}, "timeout": {strconv.Itoa(timeout)},
		"allowed_updates": {`["message"]`}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	response, err := a.telegramHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Telegram getUpdates returned HTTP %d", response.StatusCode)
	}
	var result telegramAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("Telegram getUpdates failed: %s", result.Description)
	}
	return result.Result, nil
}

func (a *Registry) saveTelegramPollState(ctx context.Context, state *schema.TelegramPollState, pollErr error) {
	now := time.Now().UTC()
	state.LastPolledAt, state.UpdatedAt, state.LastError = now, now, ""
	if pollErr != nil {
		state.LastError = truncateTelegramText(pollErr.Error(), 500)
	}
	_ = a.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_update_id", "last_polled_at", "last_error", "updated_at"})}).Create(state).Error
}

func (a *Registry) handleTelegramUpdate(ctx context.Context, channel *schema.NotificationChannel,
	token string, update telegramUpdate) {
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	userID := strconv.FormatInt(update.Message.From.ID, 10)
	command, argument := parseTelegramCommand(update.Message.Text)
	if command == "help" || command == "start" {
		a.replyTelegram(ctx, token, chatID, "可用命令：/bind CODE、/status、/unbind、/help。账户余额和持仓不会通过 Bot 外发。")
		return
	}
	if command == "bind" {
		a.replyTelegram(ctx, token, chatID, a.bindTelegramSession(ctx, channel, chatID, userID,
			update.Message.From.Username, argument, update.UpdateID))
		return
	}
	session := a.authorizedTelegramSession(ctx, channel.ID, chatID, userID)
	if session == nil {
		a.replyTelegram(ctx, token, chatID, "当前会话未绑定。请先在 XTrading 生成绑定码，然后发送 /bind CODE。")
		return
	}
	now := time.Now().UTC()
	_ = a.DB.WithContext(ctx).Model(session).Updates(map[string]interface{}{"last_message_at": now,
		"last_command": command, "last_telegram_update": update.UpdateID, "updated_at": now}).Error
	switch command {
	case "status":
		a.replyTelegram(ctx, token, chatID, "绑定有效。Bot 可接收策略信号、审批链接和执行结果；不提供账户数据或交易命令。")
	case "unbind":
		_ = a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(session).Updates(map[string]interface{}{"status": schema.TelegramSessionUnbound,
				"unbound_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			return tx.Table(channel.TableName()).Where("id = ?", channel.ID).
				Updates(map[string]interface{}{"chat_id": "", "updated_at": now}).Error
		})
		a.replyTelegram(ctx, token, chatID, "已解除绑定。")
	default:
		a.replyTelegram(ctx, token, chatID, "未知命令。发送 /help 查看可用命令。")
	}
}

func parseTelegramCommand(text string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
		return "", ""
	}
	command := strings.ToLower(strings.TrimPrefix(strings.SplitN(parts[0], "@", 2)[0], "/"))
	if len(parts) > 1 {
		return command, strings.Join(parts[1:], " ")
	}
	return command, ""
}

func (a *Registry) bindTelegramSession(ctx context.Context, channel *schema.NotificationChannel,
	chatID, userID, username, code string, updateID int64) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	publicID := strings.SplitN(code, "-", 2)[0]
	if code == "" || publicID == "" {
		return "绑定码不能为空。"
	}
	err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding schema.TelegramBindingCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id = ? AND channel_id = ?",
			publicID, channel.ID).First(&binding).Error; err != nil {
			return err
		}
		if binding.ConsumedAt != nil || !binding.ExpiresAt.After(time.Now().UTC()) || binding.Attempts >= binding.MaxAttempts {
			return fmt.Errorf("binding code is expired or unavailable")
		}
		if subtle.ConstantTimeCompare([]byte(binding.CodeHash), []byte(schema.HashTelegramBindingCode(code))) != 1 {
			if err := tx.Model(&binding).Update("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
				return err
			}
			return fmt.Errorf("binding code is invalid")
		}
		now := time.Now().UTC()
		if err := tx.Model(&binding).Update("consumed_at", now).Error; err != nil {
			return err
		}
		session := &schema.TelegramSession{ID: uuid.NewString(), OwnerUserID: binding.OwnerUserID,
			ChannelID: channel.ID, ChatID: chatID, TelegramUserID: userID, TelegramUsername: username,
			Status: schema.TelegramSessionActive, BoundAt: now, LastMessageAt: &now,
			LastCommand: "bind", LastTelegramUpdate: updateID, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "channel_id"}}, DoUpdates: clause.AssignmentColumns(
			[]string{"owner_user_id", "chat_id", "telegram_user_id", "telegram_username", "status", "bound_at", "unbound_at",
				"last_message_at", "last_command", "last_telegram_update", "updated_at"})}).Create(session).Error; err != nil {
			return err
		}
		return tx.Table(channel.TableName()).Where("id = ? AND owner_user_id = ?", channel.ID, binding.OwnerUserID).
			Updates(map[string]interface{}{"chat_id": chatID, "updated_at": now}).Error
	})
	if err != nil {
		return "绑定失败：" + err.Error()
	}
	return "绑定成功。此 Bot 仅发送策略信号、审批链接和执行结果。"
}

func (a *Registry) authorizedTelegramSession(ctx context.Context, channelID, chatID, userID string) *schema.TelegramSession {
	var session schema.TelegramSession
	if err := a.DB.WithContext(ctx).Where("channel_id = ? AND chat_id = ? AND telegram_user_id = ? AND status = ?",
		channelID, chatID, userID, schema.TelegramSessionActive).First(&session).Error; err != nil {
		return nil
	}
	return &session
}

func (a *Registry) replyTelegram(ctx context.Context, token, chatID, text string) {
	payload, _ := json.Marshal(map[string]interface{}{"chat_id": chatID,
		"text": truncateTelegramText(text, 4000), "disable_web_page_preview": true})
	endpoint := strings.TrimRight(a.telegramAPIBase, "/") + "/bot" + token + "/sendMessage"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.telegramHTTPClient.Do(request)
	if err == nil {
		_ = response.Body.Close()
	}
}

func truncateTelegramText(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}
