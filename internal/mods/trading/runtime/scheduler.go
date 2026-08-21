package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
	"go.uber.org/zap"
)

const (
	defaultAccountSyncInterval = 5 * time.Minute
	defaultAccountSyncTimeout  = time.Minute
	defaultSnapshotInterval    = 12 * time.Hour
	defaultSnapshotTimeZone    = "Asia/Shanghai"
)

// StartAccountSyncScheduler starts one process-local scheduler. Each cycle
// synchronizes every enabled exchange account. Immutable account snapshots use
// a separate clock aligned to midnight in the configured time zone.
func (a *Registry) StartAccountSyncScheduler(interval, timeout, snapshotInterval time.Duration, snapshotTimeZone string) {
	if interval <= 0 {
		interval = defaultAccountSyncInterval
	}
	if timeout <= 0 {
		timeout = defaultAccountSyncTimeout
	}
	if snapshotInterval <= 0 {
		snapshotInterval = defaultSnapshotInterval
	}
	if snapshotTimeZone == "" {
		snapshotTimeZone = defaultSnapshotTimeZone
	}
	snapshotLocation, err := time.LoadLocation(snapshotTimeZone)
	if err != nil {
		logging.Context(context.Background()).Warn("invalid account snapshot time zone; falling back to UTC",
			zap.String("time_zone", snapshotTimeZone), zap.Error(err))
		snapshotLocation = time.UTC
		snapshotTimeZone = "UTC"
	}

	a.schedulerMu.Lock()
	if a.schedulerCancel != nil {
		a.schedulerMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.schedulerCancel = cancel
	a.schedulerDone = done
	a.schedulerMu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		nextSnapshot := nextAlignedSnapshotTime(time.Now(), snapshotInterval, snapshotLocation)
		snapshotTimer := time.NewTimer(time.Until(nextSnapshot))
		defer snapshotTimer.Stop()
		logging.Context(ctx).Info("account synchronization scheduler started",
			zap.Duration("data_interval", interval), zap.Duration("snapshot_interval", snapshotInterval),
			zap.String("snapshot_time_zone", snapshotTimeZone), zap.Time("next_snapshot_at", nextSnapshot))

		_ = a.runAccountSyncCycle(ctx, timeout, false)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = a.runAccountSyncCycle(ctx, timeout, false)
			case <-snapshotTimer.C:
				_ = a.runAccountSyncCycle(ctx, timeout, true)
				nextSnapshot = nextAlignedSnapshotTime(time.Now(), snapshotInterval, snapshotLocation)
				snapshotTimer.Reset(time.Until(nextSnapshot))
			}
		}
	}()
}

func (a *Registry) StopAccountSyncScheduler(ctx context.Context) error {
	a.schedulerMu.Lock()
	cancel := a.schedulerCancel
	done := a.schedulerDone
	a.schedulerCancel = nil
	a.schedulerDone = nil
	a.schedulerMu.Unlock()
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

func (a *Registry) runAccountSyncCycle(ctx context.Context, timeout time.Duration, persistSnapshot bool) error {
	return a.runAccountSyncCycleForOwner(ctx, timeout, persistSnapshot, "")
}

func (a *Registry) runAccountSyncCycleForOwner(ctx context.Context, timeout time.Duration, persistSnapshot bool, ownerUserID string) error {
	if a.syncCycleHook != nil {
		a.syncCycleHook(ctx, persistSnapshot)
		return nil
	}
	if a.DB == nil || a.ProviderFactory == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = defaultAccountSyncTimeout
	}

	var exchanges []*schema.Exchange
	query := a.DB.WithContext(ctx).Where("enabled = ?", true)
	if ownerUserID != "" {
		query = query.Where("owner_user_id = ?", ownerUserID)
	}
	if err := query.Order("created_at ASC").Find(&exchanges).Error; err != nil {
		logging.Context(ctx).Error("query enabled exchanges for scheduled synchronization", zap.Error(err))
		return err
	}
	var cycleErrors []error
	for _, exchange := range exchanges {
		if ctx.Err() != nil {
			return errors.Join(append(cycleErrors, ctx.Err())...)
		}
		readOnlyProvider, err := a.ProviderFactory.Build(exchange)
		if err != nil {
			logging.Context(ctx).Error("build scheduled exchange provider",
				zap.String("exchange_id", exchange.ID), zap.String("exchange_type", exchange.ExchangeType), zap.Error(err))
			cycleErrors = append(cycleErrors, fmt.Errorf("build provider for exchange %s: %w", exchange.ID, err))
			continue
		}
		syncCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := a.syncExchangeOnce(syncCtx, exchange, readOnlyProvider, persistSnapshot)
		cancel()
		if err != nil {
			logging.Context(ctx).Error("scheduled exchange synchronization failed",
				zap.String("exchange_id", exchange.ID), zap.String("exchange_type", exchange.ExchangeType), zap.Error(err))
			cycleErrors = append(cycleErrors, fmt.Errorf("synchronize exchange %s: %w", exchange.ID, err))
			continue
		}
		logging.Context(ctx).Info("scheduled exchange synchronization completed",
			zap.String("exchange_id", exchange.ID), zap.String("exchange_type", exchange.ExchangeType),
			zap.Bool("snapshot_persisted", persistSnapshot), zap.Int64("orders", result.OrderCount),
			zap.Int64("fills", result.FillCount))
	}
	return errors.Join(cycleErrors...)
}

func nextAlignedSnapshotTime(now time.Time, interval time.Duration, location *time.Location) time.Time {
	if interval <= 0 {
		interval = defaultSnapshotInterval
	}
	if location == nil {
		location = time.UTC
	}
	localNow := now.In(location)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	next := dayStart.Add(interval)
	for !next.After(localNow) {
		next = next.Add(interval)
	}
	return next
}
