package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	utilityprom "github.com/LyricTian/gin-admin/v10/internal/utility/prom"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLeaseHeld = errors.New("trader runtime lease is held by another instance")

type Registry struct {
	DB               *gorm.DB
	ProviderFactory  *provider.Factory
	LLM              *llm.Service
	InstanceID       string
	LeaseDuration    time.Duration
	DecisionWorkflow DecisionWorkflow

	mu            sync.Mutex
	running       map[string]*worker
	exchangeLocks sync.Map

	schedulerMu     sync.Mutex
	schedulerCancel context.CancelFunc
	schedulerDone   chan struct{}
	syncCycleHook   func(context.Context, bool)

	persistentSchedulerMu     sync.Mutex
	persistentSchedulerCancel context.CancelFunc
	persistentSchedulerDone   chan struct{}
	persistentTaskWG          sync.WaitGroup
	taskHandlerHook           func(context.Context, string, string) error

	userStreamMu        sync.Mutex
	userStreamCancel    context.CancelFunc
	userStreamDone      chan struct{}
	userStreamWorkers   map[string]*userStreamWorker
	userStreamRefresh   time.Duration
	userStreamBuildHook func(*schema.Exchange) (provider.UserDataStream, error)

	marketStreamMu       sync.Mutex
	marketStreamCancel   context.CancelFunc
	marketStreamDone     chan struct{}
	marketStreamEndpoint string
	marketStreamDialer   *websocket.Dialer
	marketStreamNow      func() time.Time

	telegramMu         sync.Mutex
	telegramCancel     context.CancelFunc
	telegramDone       chan struct{}
	telegramWorkers    map[string]*telegramWorker
	telegramAPIBase    string
	telegramHTTPClient *http.Client
}

type userStreamWorker struct {
	cancel  context.CancelFunc
	done    chan struct{}
	version int64
}

type DecisionWorkflow interface {
	ProcessStrategyDecision(context.Context, *schema.Trader, *schema.Exchange, *schema.Decision, json.RawMessage) ([]map[string]interface{}, error)
}

func (a *Registry) SetDecisionWorkflow(workflow DecisionWorkflow) {
	a.DecisionWorkflow = workflow
}

type worker struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func NewRegistry(db *gorm.DB, factory *provider.Factory, llmService *llm.Service) *Registry {
	hostname, _ := os.Hostname()
	return &Registry{DB: db, ProviderFactory: factory, LLM: llmService,
		InstanceID:    hostname + ":" + strconv.Itoa(os.Getpid()) + ":" + uuid.NewString()[:8],
		LeaseDuration: 90 * time.Second, running: make(map[string]*worker),
		userStreamWorkers: make(map[string]*userStreamWorker), userStreamRefresh: 30 * time.Second,
		telegramWorkers: make(map[string]*telegramWorker), telegramAPIBase: "https://api.telegram.org",
		telegramHTTPClient: &http.Client{Timeout: 35 * time.Second}}
}

// Start begins account synchronization and strategy analysis. The strategy
// engine remains dry-run and cannot submit exchange trading commands.
func (a *Registry) Start(trader *schema.Trader, exchange *schema.Exchange) error {
	if trader == nil || exchange == nil {
		return errors.New("trader and exchange are required")
	}
	a.mu.Lock()
	if _, exists := a.running[trader.ID]; exists {
		a.mu.Unlock()
		return nil
	}
	if err := a.acquireLease(context.Background(), trader.ID); err != nil {
		a.mu.Unlock()
		return err
	}
	readOnlyProvider, err := a.ProviderFactory.Build(exchange)
	if err != nil {
		a.releaseLease(context.Background(), trader.ID)
		a.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &worker{cancel: cancel, done: make(chan struct{})}
	a.running[trader.ID] = w
	a.mu.Unlock()

	_ = a.updateTraderState(context.Background(), trader.ID, "starting", "", "")
	go a.run(ctx, w, trader, exchange, readOnlyProvider)
	return nil
}

func (a *Registry) Stop(ctx context.Context, traderID string) error {
	a.mu.Lock()
	w := a.running[traderID]
	if w != nil {
		delete(a.running, traderID)
		w.cancel()
	}
	a.mu.Unlock()
	if w != nil {
		select {
		case <-w.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_ = a.updateTraderState(context.Background(), traderID, schema.TraderRuntimeUnavailable, "", "")
	return a.releaseLease(context.Background(), traderID)
}

func (a *Registry) StopAll(ctx context.Context) error {
	a.mu.Lock()
	ids := make([]string, 0, len(a.running))
	for id := range a.running {
		ids = append(ids, id)
	}
	a.mu.Unlock()
	for _, id := range ids {
		if err := a.Stop(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// RequireRestartConfirmation prevents a stale desired state from silently
// resuming external AI context transmission after a process restart.
func (a *Registry) RequireRestartConfirmation(ctx context.Context) error {
	now := time.Now().UTC()
	return a.DB.WithContext(ctx).Model(new(schema.Trader)).Where("desired_state = ?", "running").Updates(map[string]interface{}{
		"desired_state": schema.TraderDesiredStateStopped, "runtime_state": "paused",
		"sync_state": "idle", "strategy_state": "paused", "safe_mode": true,
		"pause_reason":        "Service restarted; confirm external AI context sharing before resuming",
		"ai_consent_model_id": "", "ai_consent_at": nil, "auto_execution_consent_at": nil, "updated_at": now,
	}).Error
}

func (a *Registry) SyncOnce(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange, readOnlyProvider provider.ReadOnly) error {
	unlock := a.lockExchange(exchange.ID)
	defer unlock()

	account, positions, err := readOnlyProvider.Snapshot(ctx)
	if err != nil {
		_ = a.updateTraderState(ctx, trader.ID, "error", "provider_snapshot_failed", err.Error())
		return err
	}
	now := time.Now().UTC()
	err = a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seenAssets := make([]string, 0, len(account.Assets))
		for _, remote := range account.Assets {
			asset := strings.ToUpper(strings.TrimSpace(remote.Asset))
			if asset == "" {
				continue
			}
			seenAssets = append(seenAssets, asset)
			item := &schema.AssetBalance{OwnerUserID: trader.OwnerUserID, TraderID: accountScopeID(exchange.ID), ExchangeID: exchange.ID,
				AccountType: strings.ToLower(exchange.AccountType), Asset: asset, Total: zero(remote.Total),
				Available: zero(remote.Available), Locked: zero(remote.Locked), Borrowed: zero(remote.Borrowed),
				Interest: zero(remote.Interest), PriceUSDT: zero(remote.PriceUSDT), ValueUSDT: zero(remote.ValueUSDT),
				UnrealizedPnL: zero(remote.UnrealizedPnL), CapturedAt: now, Source: readOnlyProvider.Name(),
				CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "trader_id"}, {Name: "exchange_id"}, {Name: "account_type"}, {Name: "asset"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"total": item.Total, "available": item.Available,
					"locked": item.Locked, "borrowed": item.Borrowed, "interest": item.Interest,
					"price_usdt": item.PriceUSDT, "value_usdt": item.ValueUSDT, "unrealized_pnl": item.UnrealizedPnL,
					"captured_at": now, "source": item.Source, "updated_at": now}),
			}).Create(item).Error; err != nil {
				return err
			}
		}
		staleAssets := tx.Where("trader_id = ? AND exchange_id = ?", accountScopeID(exchange.ID), exchange.ID)
		if len(seenAssets) > 0 {
			staleAssets = staleAssets.Where("asset NOT IN ?", seenAssets)
		}
		if err := staleAssets.Delete(new(schema.AssetBalance)).Error; err != nil {
			return err
		}
		seen := make([]string, 0, len(positions))
		for _, remote := range positions {
			if remote.ExchangePositionID == "" {
				remote.ExchangePositionID = remote.Symbol + ":" + remote.Side
			}
			seen = append(seen, remote.ExchangePositionID)
			item := &schema.Position{ID: uuid.NewString(), OwnerUserID: trader.OwnerUserID,
				TraderID: trader.ID, ExchangeID: exchange.ID, ExchangePositionID: remote.ExchangePositionID,
				Symbol: strings.ToUpper(remote.Symbol), Side: strings.ToLower(remote.Side), Status: schema.PositionStatusOpen,
				EntryQuantity: zero(remote.Quantity), Quantity: zero(remote.Quantity), EntryPrice: zero(remote.EntryPrice),
				MarkPrice: zero(remote.MarkPrice), LiquidationPrice: zero(remote.LiquidationPrice),
				UnrealizedPnL: zero(remote.UnrealizedPnL), RealizedPnL: "0", Fee: "0", Leverage: remote.Leverage,
				Source: readOnlyProvider.Name(), OpenedAt: now, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "exchange_id"}, {Name: "exchange_position_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"quantity": item.Quantity, "mark_price": item.MarkPrice, "liquidation_price": item.LiquidationPrice,
					"unrealized_pnl": item.UnrealizedPnL, "leverage": item.Leverage, "status": schema.PositionStatusOpen,
					"last_synced_at": now, "updated_at": now, "closed_at": nil,
				}),
			}).Create(item).Error; err != nil {
				return err
			}
		}
		closeQuery := tx.Model(new(schema.Position)).Where("trader_id = ? AND exchange_id = ? AND status = ?", trader.ID, exchange.ID, schema.PositionStatusOpen)
		if len(seen) > 0 {
			closeQuery = closeQuery.Where("exchange_position_id NOT IN ?", seen)
		}
		return closeQuery.Updates(map[string]interface{}{"status": schema.PositionStatusClosed, "quantity": "0", "closed_at": now, "last_synced_at": now, "updated_at": now}).Error
	})
	if err != nil {
		_ = a.updateTraderState(ctx, trader.ID, "error", "account_data_persist_failed", err.Error())
		return err
	}
	if historical, ok := readOnlyProvider.(provider.HistoricalReadOnly); ok {
		if err := a.syncHistory(ctx, trader, exchange, historical, positions, now); err != nil {
			_ = a.updateTraderState(ctx, trader.ID, "error", "history_sync_failed", err.Error())
			return err
		}
	}
	return a.updateTraderState(ctx, trader.ID, "running", "", "")
}

// SyncExchangeOnce synchronizes current account data and history without creating an immutable account snapshot.
func (a *Registry) SyncExchangeOnce(ctx context.Context, exchange *schema.Exchange,
	readOnlyProvider provider.ReadOnly) (*schema.ExchangeDataSyncView, error) {
	return a.syncExchangeOnce(ctx, exchange, readOnlyProvider, false)
}

// CaptureExchangeSnapshotOnce persists the scheduled immutable account snapshot while synchronizing current data.
func (a *Registry) CaptureExchangeSnapshotOnce(ctx context.Context, exchange *schema.Exchange,
	readOnlyProvider provider.ReadOnly) (*schema.ExchangeDataSyncView, error) {
	return a.syncExchangeOnce(ctx, exchange, readOnlyProvider, true)
}

func (a *Registry) syncExchangeOnce(ctx context.Context, exchange *schema.Exchange,
	readOnlyProvider provider.ReadOnly, persistSnapshot bool) (*schema.ExchangeDataSyncView, error) {
	unlock := a.lockExchange(exchange.ID)
	defer unlock()

	account, positions, err := readOnlyProvider.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if persistSnapshot {
		snapshot := &schema.AccountSnapshot{OwnerUserID: exchange.OwnerUserID, TraderID: accountScopeID(exchange.ID),
			ExchangeID: exchange.ID, CapturedAt: now, TotalEquity: zero(account.TotalEquity), Balance: zero(account.Balance),
			AvailableBalance: zero(account.AvailableBalance), UnrealizedPnL: zero(account.UnrealizedPnL),
			MarginUsedPct: zero(account.MarginUsedPct), PositionCount: len(positions), Source: readOnlyProvider.Name(), CreatedAt: now}
		if err := a.DB.WithContext(ctx).Create(snapshot).Error; err != nil {
			return nil, err
		}
	}
	if err := a.persistExchangeAssets(ctx, exchange, readOnlyProvider.Name(), account.Assets, now); err != nil {
		return nil, err
	}
	if historical, ok := readOnlyProvider.(provider.HistoricalReadOnly); ok {
		accountOwner := &schema.Trader{ID: accountScopeID(exchange.ID), OwnerUserID: exchange.OwnerUserID, ExchangeID: exchange.ID}
		if err := a.syncHistory(ctx, accountOwner, exchange, historical, positions, now); err != nil {
			return nil, err
		}
	}
	result := &schema.ExchangeDataSyncView{ExchangeID: exchange.ID, ExchangeType: exchange.ExchangeType,
		AccountType: exchange.AccountType, SyncedAt: now}
	for model, target := range map[interface{}]*int64{
		new(schema.AssetBalance):  &result.AssetCount,
		new(schema.ExchangeOrder): &result.OrderCount,
		new(schema.Fill):          &result.FillCount,
		new(schema.IncomeEvent):   &result.IncomeEventCount,
	} {
		if err := a.DB.WithContext(ctx).Model(model).Where("exchange_id = ?", exchange.ID).Count(target).Error; err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *Registry) persistExchangeAssets(ctx context.Context, exchange *schema.Exchange, source string,
	assets []*provider.Asset, now time.Time) error {
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := make([]string, 0, len(assets))
		for _, remote := range assets {
			asset := strings.ToUpper(strings.TrimSpace(remote.Asset))
			if asset == "" {
				continue
			}
			seen = append(seen, asset)
			item := &schema.AssetBalance{OwnerUserID: exchange.OwnerUserID, TraderID: accountScopeID(exchange.ID),
				ExchangeID: exchange.ID, AccountType: strings.ToLower(exchange.AccountType), Asset: asset,
				Total: zero(remote.Total), Available: zero(remote.Available), Locked: zero(remote.Locked),
				Borrowed: zero(remote.Borrowed), Interest: zero(remote.Interest), PriceUSDT: zero(remote.PriceUSDT),
				ValueUSDT: zero(remote.ValueUSDT), UnrealizedPnL: zero(remote.UnrealizedPnL), CapturedAt: now,
				Source: source, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "trader_id"}, {Name: "exchange_id"}, {Name: "account_type"}, {Name: "asset"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"total": item.Total, "available": item.Available,
					"locked": item.Locked, "borrowed": item.Borrowed, "interest": item.Interest,
					"price_usdt": item.PriceUSDT, "value_usdt": item.ValueUSDT, "unrealized_pnl": item.UnrealizedPnL,
					"captured_at": now, "source": source, "updated_at": now}),
			}).Create(item).Error; err != nil {
				return err
			}
		}
		stale := tx.Where("trader_id = ? AND exchange_id = ?", accountScopeID(exchange.ID), exchange.ID)
		if len(seen) > 0 {
			stale = stale.Where("asset NOT IN ?", seen)
		}
		return stale.Delete(new(schema.AssetBalance)).Error
	})
}

func (a *Registry) syncHistory(ctx context.Context, trader *schema.Trader, exchange *schema.Exchange,
	historical provider.HistoricalReadOnly, positions []*provider.Position, now time.Time) error {
	scopeID := accountScopeID(exchange.ID)
	cursor := &schema.SyncCursor{}
	err := a.DB.WithContext(ctx).Where("trader_id = ? AND exchange_id = ? AND dataset = ? AND market = ? AND symbol = ?",
		scopeID, exchange.ID, "history", "", "").First(cursor).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	since := now.Add(-7 * 24 * time.Hour)
	if cursor.SyncedUntil != nil {
		since = cursor.SyncedUntil.Add(-5 * time.Minute)
	}
	symbols, err := a.discoverHistorySymbols(ctx, exchange.ID, positions)
	if err != nil {
		return err
	}
	batch, err := historical.History(ctx, provider.HistoryRequest{Since: since, Until: now, Symbols: symbols})
	if err != nil {
		a.persistSyncCursor(ctx, scopeID, exchange.ID, since, nil, "error", err.Error())
		return err
	}
	if batch == nil {
		batch = &provider.HistoryBatch{SyncedUntil: now}
	}
	if batch.SyncedUntil.IsZero() {
		batch.SyncedUntil = now
	}
	if err := a.persistHistoryBatch(ctx, trader.OwnerUserID, exchange, batch, now, true, since); err != nil {
		return err
	}
	if err := a.ReconcileTradeCommands(ctx, trader.OwnerUserID, exchange.ID); err != nil {
		return err
	}
	return a.RebuildClosedTrades(ctx, trader.OwnerUserID, exchange.ID)
}

func (a *Registry) discoverHistorySymbols(ctx context.Context, exchangeID string, positions []*provider.Position) ([]string, error) {
	symbolSet := make(map[string]struct{})
	for _, position := range positions {
		if symbol := strings.ToUpper(strings.TrimSpace(position.Symbol)); symbol != "" {
			symbolSet[symbol] = struct{}{}
		}
	}
	var registeredSymbols []string
	if err := a.DB.WithContext(ctx).Model(new(schema.HistorySymbol)).Where("exchange_id = ?", exchangeID).
		Distinct("symbol").Pluck("symbol", &registeredSymbols).Error; err != nil {
		return nil, err
	}
	for _, value := range registeredSymbols {
		if symbol := strings.ToUpper(strings.TrimSpace(value)); symbol != "" {
			symbolSet[symbol] = struct{}{}
		}
	}
	for _, model := range []interface{}{new(schema.ExchangeOrder), new(schema.Fill), new(schema.IncomeEvent)} {
		var knownSymbols []string
		if err := a.DB.WithContext(ctx).Model(model).Where("exchange_id = ? AND symbol <> ?", exchangeID, "").
			Distinct("symbol").Pluck("symbol", &knownSymbols).Error; err != nil {
			return nil, err
		}
		for _, value := range knownSymbols {
			if symbol := strings.ToUpper(strings.TrimSpace(value)); symbol != "" {
				symbolSet[symbol] = struct{}{}
			}
		}
	}
	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols, nil
}

// BackfillExchangeHistory imports an explicit historical range without moving
// the incremental synchronization cursor backwards.
func (a *Registry) BackfillExchangeHistory(ctx context.Context, exchange *schema.Exchange,
	historical provider.HistoricalReadOnly, request provider.HistoryRequest) (*schema.HistoryBackfillView, error) {
	unlock := a.lockExchange(exchange.ID)
	defer unlock()

	batch, err := historical.History(ctx, request)
	if err != nil {
		return nil, err
	}
	if batch == nil {
		batch = &provider.HistoryBatch{SyncedUntil: request.Until}
	}
	if batch.SyncedUntil.IsZero() {
		batch.SyncedUntil = request.Until
	}
	now := time.Now().UTC()
	if err := a.persistHistoryBatch(ctx, exchange.OwnerUserID, exchange, batch, now, false, request.Since); err != nil {
		return nil, err
	}
	if err := a.ReconcileTradeCommands(ctx, exchange.OwnerUserID, exchange.ID); err != nil {
		return nil, err
	}
	if err := a.RebuildClosedTrades(ctx, exchange.OwnerUserID, exchange.ID); err != nil {
		return nil, err
	}
	result := &schema.HistoryBackfillView{
		ExchangeDataSyncView: schema.ExchangeDataSyncView{ExchangeID: exchange.ID, ExchangeType: exchange.ExchangeType,
			AccountType: exchange.AccountType, SyncedAt: now},
		StartAt: request.Since, EndAt: request.Until, Symbols: request.Symbols,
		FetchedOrderCount: len(batch.Orders), FetchedFillCount: len(batch.Fills),
		FetchedIncomeEventCount: len(batch.IncomeEvents),
	}
	for model, target := range map[interface{}]*int64{
		new(schema.AssetBalance):  &result.AssetCount,
		new(schema.ExchangeOrder): &result.OrderCount,
		new(schema.Fill):          &result.FillCount,
		new(schema.IncomeEvent):   &result.IncomeEventCount,
		new(schema.ClosedTrade):   &result.ClosedTradeCount,
	} {
		if err := a.DB.WithContext(ctx).Model(model).Where("exchange_id = ?", exchange.ID).Count(target).Error; err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *Registry) persistHistoryBatch(ctx context.Context, ownerUserID string, exchange *schema.Exchange,
	batch *provider.HistoryBatch, now time.Time, updateCursor bool, since time.Time) error {
	scopeID := accountScopeID(exchange.ID)
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, remote := range batch.Orders {
			orderedAt := nonZeroTime(remote.OrderedAt, now)
			updatedAt := nonZeroTime(remote.ExchangeUpdatedAt, orderedAt)
			item := &schema.ExchangeOrder{ID: uuid.NewString(), OwnerUserID: ownerUserID, TraderID: scopeID,
				ExchangeID: exchange.ID, Market: strings.ToLower(remote.Market), ExchangeOrderID: remote.ExchangeOrderID,
				ClientOrderID: remote.ClientOrderID, Origin: orderOrigin(remote.ClientOrderID), Symbol: strings.ToUpper(remote.Symbol),
				Side: strings.ToLower(remote.Side), PositionSide: strings.ToLower(remote.PositionSide),
				OrderType: strings.ToLower(remote.OrderType), Status: strings.ToLower(remote.Status), Price: zero(remote.Price),
				AveragePrice: zero(remote.AveragePrice), Quantity: zero(remote.Quantity), ExecutedQuantity: zero(remote.ExecutedQuantity),
				QuoteQuantity: zero(remote.QuoteQuantity), ReduceOnly: remote.ReduceOnly, OrderedAt: orderedAt,
				ExchangeUpdatedAt: updatedAt, LastSyncedAt: now, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "exchange_id"}, {Name: "market"}, {Name: "exchange_order_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"client_order_id": item.ClientOrderID, "status": item.Status,
					"average_price": item.AveragePrice, "executed_quantity": item.ExecutedQuantity, "quote_quantity": item.QuoteQuantity,
					"exchange_updated_at": updatedAt, "last_synced_at": now, "updated_at": now})}).Create(item).Error; err != nil {
				return err
			}
		}
		for _, remote := range batch.Fills {
			item := &schema.Fill{ID: uuid.NewString(), OwnerUserID: ownerUserID, TraderID: scopeID,
				ExchangeID: exchange.ID, Market: strings.ToLower(remote.Market), ExchangeTradeID: remote.ExchangeTradeID,
				ExchangeOrderID: remote.ExchangeOrderID, Symbol: strings.ToUpper(remote.Symbol), Side: strings.ToLower(remote.Side),
				PositionSide: strings.ToLower(remote.PositionSide), Price: zero(remote.Price), Quantity: zero(remote.Quantity),
				QuoteQuantity: zero(remote.QuoteQuantity), FeeAsset: strings.ToUpper(remote.FeeAsset), FeeQuantity: zero(remote.FeeQuantity),
				FeeValueUSDT: zero(remote.FeeValueUSDT), RealizedPnL: zero(remote.RealizedPnL), Liquidity: strings.ToLower(remote.Liquidity),
				FilledAt: nonZeroTime(remote.FilledAt, now), Source: exchange.ExchangeType, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "exchange_id"}, {Name: "market"}, {Name: "exchange_trade_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"fee_quantity": item.FeeQuantity, "fee_value_usdt": item.FeeValueUSDT,
					"realized_pnl": item.RealizedPnL, "updated_at": now})}).Create(item).Error; err != nil {
				return err
			}
		}
		for _, remote := range batch.IncomeEvents {
			item := &schema.IncomeEvent{ID: uuid.NewString(), OwnerUserID: ownerUserID, TraderID: scopeID,
				ExchangeID: exchange.ID, EventType: strings.ToLower(remote.EventType), ExchangeEventID: remote.ExchangeEventID,
				Market: strings.ToLower(remote.Market), Symbol: strings.ToUpper(remote.Symbol), Asset: strings.ToUpper(remote.Asset),
				Amount: zero(remote.Amount), PriceUSDT: zero(remote.PriceUSDT), ValueUSDT: zero(remote.ValueUSDT),
				OccurredAt: nonZeroTime(remote.OccurredAt, now), Source: exchange.ExchangeType, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "exchange_id"}, {Name: "event_type"}, {Name: "exchange_event_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"amount": item.Amount, "price_usdt": item.PriceUSDT,
					"value_usdt": item.ValueUSDT, "updated_at": now})}).Create(item).Error; err != nil {
				return err
			}
		}
		if updateCursor {
			return persistSyncCursorTx(tx, scopeID, exchange.ID, since, &batch.SyncedUntil, "success", "")
		}
		return nil
	})
}

func (a *Registry) persistSyncCursor(ctx context.Context, traderID, exchangeID string, since time.Time,
	syncedUntil *time.Time, status, message string) {
	_ = a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return persistSyncCursorTx(tx, traderID, exchangeID, since, syncedUntil, status, message)
	})
}

func persistSyncCursorTx(tx *gorm.DB, traderID, exchangeID string, since time.Time,
	syncedUntil *time.Time, status, message string) error {
	now := time.Now().UTC()
	item := &schema.SyncCursor{TraderID: traderID, ExchangeID: exchangeID, Dataset: "history", Market: "", Symbol: "",
		Cursor: since.Format(time.RFC3339Nano), SyncedUntil: syncedUntil, Status: status, ErrorMessage: message,
		LastRunAt: &now, CreatedAt: now, UpdatedAt: now}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "trader_id"}, {Name: "exchange_id"},
		{Name: "dataset"}, {Name: "market"}, {Name: "symbol"}}, DoUpdates: clause.Assignments(map[string]interface{}{
		"cursor": item.Cursor, "synced_until": syncedUntil, "status": status, "error_message": message,
		"last_run_at": now, "updated_at": now,
	})}).Create(item).Error
}

func orderOrigin(clientOrderID string) string {
	value := strings.ToLower(strings.TrimSpace(clientOrderID))
	if strings.HasPrefix(value, "xt-") || strings.HasPrefix(value, "xtrading-") {
		return schema.OrderOriginSystem
	}
	return schema.OrderOriginExternal
}

func nonZeroTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func accountScopeID(exchangeID string) string {
	return "exchange:" + exchangeID
}

func (a *Registry) lockExchange(exchangeID string) func() {
	value, _ := a.exchangeLocks.LoadOrStore(exchangeID, new(sync.Mutex))
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (a *Registry) run(ctx context.Context, w *worker, trader *schema.Trader, exchange *schema.Exchange, readOnlyProvider provider.ReadOnly) {
	defer func() {
		a.mu.Lock()
		if a.running[trader.ID] == w {
			delete(a.running, trader.ID)
		}
		a.mu.Unlock()
		_ = a.releaseLease(context.Background(), trader.ID)
		close(w.done)
	}()
	interval := time.Duration(trader.ScanIntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		cycleStartedAt := time.Now()
		if err := a.renewLease(ctx, trader.ID); err != nil {
			_ = a.updateTraderState(context.Background(), trader.ID, "error", "runtime_lease_lost", err.Error())
			a.recordRuntimeCycle(context.Background(), trader.ID, cycleStartedAt, err)
			return
		}
		_ = a.updateTraderFields(ctx, trader.ID, map[string]interface{}{"sync_state": "syncing", "strategy_state": "waiting"})
		if err := a.SyncOnce(ctx, trader, exchange, readOnlyProvider); err != nil {
			_ = a.updateTraderFields(context.Background(), trader.ID, map[string]interface{}{
				"sync_state": "error", "strategy_state": "paused", "pause_reason": err.Error(),
			})
			a.recordRuntimeCycle(context.Background(), trader.ID, cycleStartedAt, err)
		} else {
			_ = a.updateTraderFields(ctx, trader.ID, map[string]interface{}{"sync_state": "running", "strategy_state": "analyzing", "pause_reason": ""})
			if err := a.RunStrategyCycle(ctx, trader.ID); err != nil {
				_ = a.updateTraderFields(context.Background(), trader.ID, map[string]interface{}{
					"strategy_state": "error", "pause_reason": err.Error(),
					"last_error_code": "strategy_cycle_failed", "last_error_message": err.Error(),
				})
				a.recordRuntimeCycle(context.Background(), trader.ID, cycleStartedAt, err)
			} else {
				a.recordRuntimeCycle(ctx, trader.ID, cycleStartedAt, nil)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Registry) recordRuntimeCycle(ctx context.Context, traderID string, startedAt time.Time, cycleErr error) {
	duration := time.Since(startedAt)
	fields := map[string]interface{}{"last_cycle_duration_ms": duration.Milliseconds()}
	code := "success"
	if cycleErr != nil {
		fields["consecutive_failures"] = gorm.Expr("consecutive_failures + 1")
		code = "error"
	} else {
		fields["consecutive_failures"] = 0
	}
	_ = a.updateTraderFields(ctx, traderID, fields)
	if utilityprom.Ins != nil {
		utilityprom.Ins.RequestLog("trading_runtime", "strategy_cycle", "run", code)
		utilityprom.Ins.SummaryLatencyLog("trading_runtime", "strategy_cycle", "run", duration.Seconds())
		if cycleErr != nil {
			utilityprom.Ins.ExceptionLog("trading_runtime", "strategy_cycle_failed")
		}
	}
}

func (a *Registry) updateTraderFields(ctx context.Context, traderID string, fields map[string]interface{}) error {
	now := time.Now().UTC()
	fields["last_heartbeat_at"], fields["updated_at"] = now, now
	return a.DB.WithContext(ctx).Model(new(schema.Trader)).Where("id = ?", traderID).Updates(fields).Error
}

func (a *Registry) acquireLease(ctx context.Context, traderID string) error {
	now := time.Now().UTC()
	until := now.Add(a.LeaseDuration)
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lease schema.RuntimeLease
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trader_id = ?", traderID).First(&lease).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&schema.RuntimeLease{TraderID: traderID, OwnerInstanceID: a.InstanceID, LeaseUntil: until, HeartbeatAt: now}).Error
		}
		if err != nil {
			return err
		}
		if lease.OwnerInstanceID != a.InstanceID && lease.LeaseUntil.After(now) {
			return ErrLeaseHeld
		}
		return tx.Model(&lease).Updates(map[string]interface{}{"owner_instance_id": a.InstanceID, "lease_until": until, "heartbeat_at": now}).Error
	})
}

func (a *Registry) renewLease(ctx context.Context, traderID string) error {
	now := time.Now().UTC()
	result := a.DB.WithContext(ctx).Model(new(schema.RuntimeLease)).Where("trader_id = ? AND owner_instance_id = ?", traderID, a.InstanceID).
		Updates(map[string]interface{}{"lease_until": now.Add(a.LeaseDuration), "heartbeat_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLeaseHeld
	}
	return nil
}

func (a *Registry) releaseLease(ctx context.Context, traderID string) error {
	return a.DB.WithContext(ctx).Where("trader_id = ? AND owner_instance_id = ?", traderID, a.InstanceID).Delete(new(schema.RuntimeLease)).Error
}

func (a *Registry) updateTraderState(ctx context.Context, traderID, state, code, message string) error {
	now := time.Now().UTC()
	return a.DB.WithContext(ctx).Model(new(schema.Trader)).Where("id = ?", traderID).Updates(map[string]interface{}{
		"runtime_state": state, "last_heartbeat_at": now, "last_error_code": code,
		"last_error_message": message, "updated_at": now,
	}).Error
}

func zero(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return "0"
	}
	return value
}

func (a *Registry) Status(traderID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, exists := a.running[traderID]
	return exists
}

func (a *Registry) String() string { return fmt.Sprintf("runtime-registry[%s]", a.InstanceID) }
