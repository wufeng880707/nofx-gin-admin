package runtime

import (
	"context"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

const (
	userStreamReconcileDebounce = 2 * time.Second
	userStreamReconcileTimeout  = 60 * time.Second
)

func (a *Registry) StartUserDataStreams() error {
	a.userStreamMu.Lock()
	defer a.userStreamMu.Unlock()
	if a.userStreamCancel != nil {
		return nil
	}
	if a.DB == nil || a.ProviderFactory == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.userStreamCancel, a.userStreamDone = cancel, done
	go a.runUserStreamSupervisor(ctx, done)
	return nil
}

func (a *Registry) StopUserDataStreams(ctx context.Context) error {
	a.userStreamMu.Lock()
	cancel, done := a.userStreamCancel, a.userStreamDone
	a.userStreamCancel, a.userStreamDone = nil, nil
	a.userStreamMu.Unlock()
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

func (a *Registry) runUserStreamSupervisor(ctx context.Context, done chan struct{}) {
	defer close(done)
	refresh := a.userStreamRefresh
	if refresh <= 0 {
		refresh = 30 * time.Second
	}
	a.reconcileUserStreamWorkers(ctx)
	ticker := time.NewTicker(refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.stopUserStreamWorkers()
			return
		case <-ticker.C:
			a.reconcileUserStreamWorkers(ctx)
		}
	}
}

// reconcileUserStreamWorkers detects account creation, enable/disable and
// credential updates without requiring an application restart.
func (a *Registry) reconcileUserStreamWorkers(parent context.Context) {
	var exchanges []*schema.Exchange
	if err := a.DB.WithContext(parent).Where("enabled = ? AND exchange_type IN ?", true, []string{"binance", "okx"}).
		Order("created_at ASC").Find(&exchanges).Error; err != nil {
		logging.Context(parent).Warn("query user stream exchanges failed", zap.Error(err))
		return
	}
	wanted := make(map[string]*schema.Exchange, len(exchanges))
	for _, exchange := range exchanges {
		wanted[exchange.ID] = exchange
	}

	var waits []<-chan struct{}
	a.userStreamMu.Lock()
	for id, worker := range a.userStreamWorkers {
		exchange := wanted[id]
		if exchange == nil || worker.version != exchange.UpdatedAt.UnixNano() {
			worker.cancel()
			waits = append(waits, worker.done)
			delete(a.userStreamWorkers, id)
		}
	}
	for id, exchange := range wanted {
		if _, exists := a.userStreamWorkers[id]; exists {
			continue
		}
		ctx, cancel := context.WithCancel(parent)
		worker := &userStreamWorker{cancel: cancel, done: make(chan struct{}), version: exchange.UpdatedAt.UnixNano()}
		a.userStreamWorkers[id] = worker
		go func(item *schema.Exchange, current *userStreamWorker) {
			defer close(current.done)
			a.runExchangeUserStream(ctx, item)
		}(exchange, worker)
	}
	a.userStreamMu.Unlock()
	for _, wait := range waits {
		<-wait
	}
}

func (a *Registry) stopUserStreamWorkers() {
	a.userStreamMu.Lock()
	workers := make([]*userStreamWorker, 0, len(a.userStreamWorkers))
	for id, worker := range a.userStreamWorkers {
		worker.cancel()
		workers = append(workers, worker)
		delete(a.userStreamWorkers, id)
	}
	a.userStreamMu.Unlock()
	for _, worker := range workers {
		<-worker.done
	}
}

func (a *Registry) runExchangeUserStream(ctx context.Context, exchange *schema.Exchange) {
	backoff := 2 * time.Second
	reconnects := 0
	for ctx.Err() == nil {
		status := schema.UserStreamStatusConnecting
		if reconnects > 0 {
			status = schema.UserStreamStatusReconnecting
		}
		a.saveUserStreamState(ctx, exchange, status, reconnects, "", nil, nil)
		a.reconcileUserStreamExchange(ctx, exchange)
		stream, err := a.buildUserDataStream(exchange)
		if err == nil {
			now := time.Now().UTC()
			a.saveUserStreamState(ctx, exchange, schema.UserStreamStatusConnected, reconnects, "", &now, nil)
			reconcileSignals := make(chan struct{}, 1)
			reconcileDone := make(chan struct{})
			go func() {
				defer close(reconcileDone)
				a.consumeUserStreamReconcileSignals(ctx, exchange, reconcileSignals)
			}()
			err = stream.Run(ctx, func(event provider.UserDataEvent) error {
				if persistErr := a.persistUserDataEvent(ctx, exchange, event); persistErr != nil {
					return persistErr
				}
				select {
				case reconcileSignals <- struct{}{}:
				default:
				}
				return nil
			})
			close(reconcileSignals)
			<-reconcileDone
		}
		if ctx.Err() != nil {
			a.saveUserStreamState(context.Background(), exchange, schema.UserStreamStatusStopped, reconnects, "", nil, nil)
			return
		}
		reconnects++
		message := "stream disconnected"
		if err != nil {
			message = strings.TrimSpace(err.Error())
			if len(message) > 1000 {
				message = message[:1000]
			}
		}
		a.saveUserStreamState(ctx, exchange, schema.UserStreamStatusReconnecting, reconnects, message, nil, nil)
		logging.Context(ctx).Warn("exchange user data stream reconnecting", zap.String("exchange_id", exchange.ID),
			zap.String("exchange_type", exchange.ExchangeType), zap.Int("reconnect_count", reconnects), zap.Error(err))
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

func (a *Registry) buildUserDataStream(exchange *schema.Exchange) (provider.UserDataStream, error) {
	if a.userStreamBuildHook != nil {
		return a.userStreamBuildHook(exchange)
	}
	return a.ProviderFactory.BuildUserDataStream(exchange)
}

func (a *Registry) persistUserDataEvent(ctx context.Context, exchange *schema.Exchange, event provider.UserDataEvent) error {
	now := time.Now().UTC()
	item := &schema.UserStreamEvent{OwnerUserID: exchange.OwnerUserID, ExchangeID: exchange.ID,
		ExchangeType: exchange.ExchangeType, EventType: event.Type, PayloadHash: event.PayloadHash,
		EventTime: event.EventTime, ReceivedAt: now}
	if !event.TransactionTime.IsZero() {
		transactionTime := event.TransactionTime
		item.TransactionTime = &transactionTime
	}
	if err := a.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error; err != nil {
		return err
	}
	a.saveUserStreamState(ctx, exchange, schema.UserStreamStatusConnected, -1, "", nil, &now)
	return nil
}

func (a *Registry) consumeUserStreamReconcileSignals(ctx context.Context, exchange *schema.Exchange, signals <-chan struct{}) {
	var last time.Time
	for range signals {
		if delay := userStreamReconcileDebounce - time.Since(last); delay > 0 {
			if waitSchedulerContext(ctx, delay) != nil {
				return
			}
		}
		a.reconcileUserStreamExchange(ctx, exchange)
		last = time.Now()
	}
}

func (a *Registry) reconcileUserStreamExchange(ctx context.Context, exchange *schema.Exchange) {
	readOnlyProvider, err := a.ProviderFactory.Build(exchange)
	if err != nil {
		return
	}
	reconcileCtx, cancel := context.WithTimeout(ctx, userStreamReconcileTimeout)
	defer cancel()
	if _, err := a.syncExchangeOnce(reconcileCtx, exchange, readOnlyProvider, false); err != nil {
		logging.Context(ctx).Warn("user stream REST reconciliation failed", zap.String("exchange_id", exchange.ID), zap.Error(err))
		return
	}
	now := time.Now().UTC()
	a.saveUserStreamState(ctx, exchange, schema.UserStreamStatusConnected, -1, "", nil, nil, &now)
}

func (a *Registry) saveUserStreamState(ctx context.Context, exchange *schema.Exchange, status string,
	reconnects int, lastError string, connectedAt, lastEventAt *time.Time, reconciledAt ...*time.Time) {
	if a.DB == nil || exchange == nil {
		return
	}
	values := map[string]interface{}{"owner_user_id": exchange.OwnerUserID, "exchange_type": exchange.ExchangeType,
		"status": status, "updated_at": time.Now().UTC()}
	if reconnects >= 0 {
		values["reconnect_count"] = reconnects
	}
	if lastError != "" || status == schema.UserStreamStatusConnected {
		values["last_error"] = lastError
	}
	if connectedAt != nil {
		values["connected_at"] = connectedAt
	}
	if lastEventAt != nil {
		values["last_event_at"] = lastEventAt
	}
	if len(reconciledAt) > 0 && reconciledAt[0] != nil {
		values["last_reconciled_at"] = reconciledAt[0]
	}
	state := &schema.UserStreamState{ExchangeID: exchange.ID, OwnerUserID: exchange.OwnerUserID,
		ExchangeType: exchange.ExchangeType, Status: status, UpdatedAt: time.Now().UTC()}
	if reconnects >= 0 {
		state.ReconnectCount = reconnects
	}
	state.LastError, state.ConnectedAt, state.LastEventAt = lastError, connectedAt, lastEventAt
	if len(reconciledAt) > 0 {
		state.LastReconciledAt = reconciledAt[0]
	}
	_ = a.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "exchange_id"}},
		DoUpdates: clause.Assignments(values)}).Create(state).Error
}
