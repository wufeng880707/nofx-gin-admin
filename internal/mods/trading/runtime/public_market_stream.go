package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

const defaultBinanceLiquidationStream = "wss://fstream.binance.com/ws/!forceOrder@arr"

type binanceLiquidationOrder struct {
	Symbol       string `json:"s"`
	Side         string `json:"S"`
	Quantity     string `json:"q"`
	Price        string `json:"p"`
	AveragePrice string `json:"ap"`
	TradeTime    int64  `json:"T"`
}

type binanceLiquidationPayload struct {
	EventType string                  `json:"e"`
	EventTime int64                   `json:"E"`
	Order     binanceLiquidationOrder `json:"o"`
}

func (a *Registry) StartPublicMarketStream() error {
	a.marketStreamMu.Lock()
	defer a.marketStreamMu.Unlock()
	if a.marketStreamCancel != nil || a.DB == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.marketStreamCancel, a.marketStreamDone = cancel, done
	go a.runPublicMarketStream(ctx, done)
	return nil
}

func (a *Registry) StopPublicMarketStream(ctx context.Context) error {
	a.marketStreamMu.Lock()
	cancel, done := a.marketStreamCancel, a.marketStreamDone
	a.marketStreamCancel, a.marketStreamDone = nil, nil
	a.marketStreamMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Registry) runPublicMarketStream(ctx context.Context, done chan struct{}) {
	defer close(done)
	endpoint := strings.TrimSpace(a.marketStreamEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(config.C.Trading.MarketStream.Endpoint)
	}
	if endpoint == "" {
		endpoint = defaultBinanceLiquidationStream
	}
	dialer := a.marketStreamDialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		if err := a.consumeBinanceLiquidationStream(ctx, dialer, endpoint); err != nil && ctx.Err() == nil {
			logging.Context(ctx).Warn("public liquidation stream reconnecting", zap.Error(err))
		}
		if waitSchedulerContext(ctx, backoff) != nil {
			return
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (a *Registry) consumeBinanceLiquidationStream(ctx context.Context, dialer *websocket.Dialer, endpoint string) error {
	connection, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	closed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.Close()
		case <-closed:
		}
	}()
	defer close(closed)

	lastPrune := time.Time{}
	for ctx.Err() == nil {
		_, payload, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		events, err := parseBinanceLiquidationEvents(payload)
		if err != nil {
			logging.Context(ctx).Warn("discard invalid public liquidation event", zap.Error(err))
			continue
		}
		for _, event := range events {
			if err := a.persistMarketLiquidation(ctx, event); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if a.marketStreamNow != nil {
			now = a.marketStreamNow().UTC()
		}
		if lastPrune.IsZero() || now.Sub(lastPrune) >= time.Hour {
			if err := a.pruneMarketLiquidations(ctx, now); err != nil {
				return err
			}
			lastPrune = now
		}
	}
	return ctx.Err()
}

func parseBinanceLiquidationEvents(payload []byte) ([]*schema.MarketLiquidationEvent, error) {
	raw := json.RawMessage(payload)
	var combined struct {
		Data json.RawMessage `json:"data"`
	}
	if len(raw) > 0 && raw[0] == '{' && json.Unmarshal(raw, &combined) == nil && len(combined.Data) > 0 {
		raw = combined.Data
	}
	var entries []json.RawMessage
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, err
		}
	} else {
		entries = []json.RawMessage{raw}
	}
	result := make([]*schema.MarketLiquidationEvent, 0, len(entries))
	for _, entry := range entries {
		item, ok, err := parseBinanceLiquidationEvent(entry)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, item)
		}
	}
	return result, nil
}

func parseBinanceLiquidationEvent(payload []byte) (*schema.MarketLiquidationEvent, bool, error) {
	var event binanceLiquidationPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, false, err
	}
	if event.EventType != "forceOrder" {
		return nil, false, nil
	}
	price := strings.TrimSpace(event.Order.AveragePrice)
	if price == "" || price == "0" {
		price = strings.TrimSpace(event.Order.Price)
	}
	quantity := strings.TrimSpace(event.Order.Quantity)
	notional, err := multiplyDecimalStrings(price, quantity)
	if err != nil {
		return nil, false, fmt.Errorf("invalid liquidation price or quantity: %w", err)
	}
	symbol := strings.ToUpper(strings.TrimSpace(event.Order.Symbol))
	if symbol == "" || notional == "0" {
		return nil, false, nil
	}
	side := ""
	switch strings.ToUpper(strings.TrimSpace(event.Order.Side)) {
	case "SELL":
		side = "long"
	case "BUY":
		side = "short"
	default:
		return nil, false, nil
	}
	eventMillis := event.Order.TradeTime
	if eventMillis <= 0 {
		eventMillis = event.EventTime
	}
	if eventMillis <= 0 {
		return nil, false, fmt.Errorf("liquidation event time is missing")
	}
	normalized, _ := json.Marshal([]interface{}{symbol, side, price, quantity, eventMillis})
	hash := sha256.Sum256(normalized)
	return &schema.MarketLiquidationEvent{
		Exchange: "binance", Market: "futures", Symbol: symbol, Side: side,
		Price: price, Quantity: quantity, NotionalUSDT: notional,
		EventAt: time.UnixMilli(eventMillis).UTC(), PayloadHash: hex.EncodeToString(hash[:]),
		Source: "binance_force_order_ws",
	}, true, nil
}

func multiplyDecimalStrings(left, right string) (string, error) {
	a, ok := new(big.Rat).SetString(left)
	if !ok {
		return "", fmt.Errorf("invalid decimal %q", left)
	}
	b, ok := new(big.Rat).SetString(right)
	if !ok {
		return "", fmt.Errorf("invalid decimal %q", right)
	}
	value := new(big.Rat).Mul(a, b).FloatString(18)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" || value == "-0" {
		return "0", nil
	}
	return value, nil
}

func (a *Registry) persistMarketLiquidation(ctx context.Context, event *schema.MarketLiquidationEvent) error {
	if event == nil {
		return nil
	}
	return a.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

func (a *Registry) pruneMarketLiquidations(ctx context.Context, now time.Time) error {
	retention := config.C.Trading.MarketStream.RetentionHours
	if retention <= 0 {
		retention = 168
	}
	return a.DB.WithContext(ctx).Where("event_at < ?", now.Add(-time.Duration(retention)*time.Hour)).
		Delete(new(schema.MarketLiquidationEvent)).Error
}
