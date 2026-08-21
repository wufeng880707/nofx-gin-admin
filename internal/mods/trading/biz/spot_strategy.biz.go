package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	tradingruntime "github.com/LyricTian/gin-admin/v10/internal/mods/trading/runtime"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

type SpotStrategy struct {
	SpotDAL         *dal.SpotStrategy
	CatalogDAL      *dal.Catalog
	ProviderFactory *provider.Factory

	workersMu sync.Mutex
	workers   map[string]context.CancelFunc
}

func NewSpotStrategy(spotDAL *dal.SpotStrategy, catalogDAL *dal.Catalog, factory *provider.Factory) *SpotStrategy {
	return &SpotStrategy{SpotDAL: spotDAL, CatalogDAL: catalogDAL, ProviderFactory: factory,
		workers: make(map[string]context.CancelFunc)}
}

func (a *SpotStrategy) Recover(ctx context.Context) error {
	return a.SpotDAL.MarkInterruptedBacktestsFailed(ctx)
}

func (a *SpotStrategy) CreateBacktest(ctx context.Context, form *schema.SpotBacktestCreateForm) (*schema.SpotBacktestTask, error) {
	owner, err := resolveMutationOwner(ctx, "")
	if err != nil {
		return nil, err
	}
	if form.StrategyID != "" {
		strategy, getErr := a.CatalogDAL.GetStrategy(ctx, form.StrategyID, owner)
		if getErr != nil {
			return nil, getErr
		}
		if strategy == nil {
			return nil, errors.NotFound("spot_strategy_not_found", "Strategy not found")
		}
		if strategy.StrategyType != schema.SpotStrategyType {
			return nil, errors.BadRequest("spot_strategy_type_required", "Strategy must be spot_core_satellite")
		}
		config, decodeErr := decodeStoredSpotConfig(strategy.ConfigJSON)
		if decodeErr != nil {
			return nil, errors.BadRequest("spot_strategy_config_invalid", "Stored strategy config is invalid: %v", decodeErr)
		}
		form.Config = config
	}
	if err := form.Validate(); err != nil {
		return nil, errors.BadRequest("spot_backtest_form_invalid", "%s", err.Error())
	}
	capital, ok := new(big.Rat).SetString(form.InitialCapital)
	if !ok || capital.Sign() <= 0 {
		return nil, errors.BadRequest("spot_backtest_capital_invalid", "Initial capital must be a positive decimal")
	}
	if form.EndTime.Sub(form.StartTime) > 10*365*24*time.Hour {
		return nil, errors.BadRequest("spot_backtest_range_too_large", "Backtest range cannot exceed ten years")
	}
	configJSON, _ := json.Marshal(form.Config)
	costJSON, _ := json.Marshal(form.CostScenario)
	benchmarksJSON, _ := json.Marshal(form.Benchmarks)
	now := time.Now().UTC()
	item := &schema.SpotBacktestTask{ID: util.MustNewUUID(), OwnerUserID: owner, StrategyID: form.StrategyID,
		Status: schema.SpotBacktestQueued, Exchange: form.Exchange, Symbol: form.Symbol, Interval: form.Interval,
		StartTime: form.StartTime.UTC(), EndTime: form.EndTime.UTC(), InitialCapital: form.InitialCapital,
		ConfigJSON: string(configJSON), CostScenarioJSON: string(costJSON), BenchmarksJSON: string(benchmarksJSON),
		CodeVersion: "spot-core-satellite-v1", CreatedAt: now, UpdatedAt: now}
	if err := a.SpotDAL.CreateBacktestTask(ctx, item); err != nil {
		return nil, err
	}
	a.startWorker(item.ID)
	return item, nil
}

func (a *SpotStrategy) QueryBacktests(ctx context.Context, params schema.SpotBacktestQueryParam) (*schema.SpotBacktestQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.SpotDAL.QueryBacktestTasks(ctx, params)
}

func (a *SpotStrategy) GetBacktest(ctx context.Context, id string) (*schema.SpotBacktestTask, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	item, err := a.SpotDAL.GetBacktestTask(ctx, owner, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NotFound("spot_backtest_not_found", "Backtest task not found")
	}
	return item, err
}

func (a *SpotStrategy) GetBacktestResult(ctx context.Context, id string) (*schema.SpotBacktestResultView, error) {
	task, err := a.GetBacktest(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status != schema.SpotBacktestCompleted {
		return nil, errors.Conflict("spot_backtest_not_completed", "Backtest result is not available yet")
	}
	result, err := a.SpotDAL.GetBacktestResult(ctx, task.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.NotFound("spot_backtest_result_not_found", "Backtest result not found")
	}
	if err != nil {
		return nil, err
	}
	view, err := schema.DecodeSpotBacktestResult(task, result)
	return view, errors.WithStack(err)
}

func (a *SpotStrategy) CancelBacktest(ctx context.Context, id string) error {
	task, err := a.GetBacktest(ctx, id)
	if err != nil {
		return err
	}
	if err := a.SpotDAL.RequestBacktestCancel(ctx, task.OwnerUserID, task.ID); err != nil {
		return err
	}
	a.workersMu.Lock()
	cancel := a.workers[id]
	a.workersMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (a *SpotStrategy) QueryLots(ctx context.Context, params schema.SpotLotQueryParam) (*schema.SpotLotQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.SpotDAL.QueryLots(ctx, params)
}

func (a *SpotStrategy) QueryStateEvents(ctx context.Context, cycleID string, limit int) ([]*schema.SpotStateEvent, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	return a.SpotDAL.QueryStateEvents(ctx, owner, cycleID, limit)
}

func (a *SpotStrategy) startWorker(taskID string) {
	ctx, cancel := context.WithCancel(context.Background())
	a.workersMu.Lock()
	if existing := a.workers[taskID]; existing != nil {
		existing()
	}
	a.workers[taskID] = cancel
	a.workersMu.Unlock()
	go func() {
		defer func() {
			a.workersMu.Lock()
			delete(a.workers, taskID)
			a.workersMu.Unlock()
		}()
		a.runBacktest(ctx, taskID)
	}()
}

func (a *SpotStrategy) runBacktest(ctx context.Context, taskID string) {
	started := time.Now().UTC()
	if err := a.SpotDAL.TransitionBacktestTask(ctx, taskID, schema.SpotBacktestQueued, schema.SpotBacktestLoadingData,
		map[string]interface{}{"progress": 5, "started_at": &started}); err != nil {
		return
	}
	task := new(schema.SpotBacktestTask)
	if err := a.SpotDAL.DB.Where("id = ?", taskID).First(task).Error; err != nil {
		return
	}
	if a.cancelled(ctx, taskID) {
		a.markCancelled(taskID, schema.SpotBacktestLoadingData)
		return
	}
	var config schema.SpotCoreSatelliteConfig
	if err := json.Unmarshal([]byte(task.ConfigJSON), &config); err != nil {
		a.fail(taskID, "config_decode_failed", err)
		return
	}
	klines, err := a.ProviderFactory.MarketData().KlinesRange(ctx, task.Exchange, schema.MarketTypeSpot, task.Symbol,
		task.Interval, task.StartTime, task.EndTime)
	if err != nil {
		if ctx.Err() != nil {
			a.markCancelled(taskID, schema.SpotBacktestLoadingData)
		} else {
			a.fail(taskID, "market_data_failed", err)
		}
		return
	}
	if a.cancelled(ctx, taskID) {
		a.markCancelled(taskID, schema.SpotBacktestLoadingData)
		return
	}
	marketAt := task.StartTime
	if len(klines) > 0 {
		marketAt = time.Unix(klines[len(klines)-1].Time, 0).UTC()
	}
	if err := a.SpotDAL.TransitionBacktestTask(ctx, taskID, schema.SpotBacktestLoadingData, schema.SpotBacktestRunning,
		map[string]interface{}{"progress": 35, "processed_market_at": &marketAt}); err != nil {
		return
	}
	capitalRat, _ := new(big.Rat).SetString(task.InitialCapital)
	capital, _ := capitalRat.Float64()
	output, err := tradingruntime.RunSpotCoreSatelliteBacktest(config, klines, capital)
	if err != nil {
		a.fail(taskID, "backtest_execution_failed", err)
		return
	}
	if a.cancelled(ctx, taskID) {
		a.markCancelled(taskID, schema.SpotBacktestRunning)
		return
	}
	if err := a.SpotDAL.TransitionBacktestTask(ctx, taskID, schema.SpotBacktestRunning, schema.SpotBacktestCalculating,
		map[string]interface{}{"progress": 85}); err != nil {
		return
	}
	if err := a.SpotDAL.SaveBacktestResult(ctx, taskID, output, len(klines)); err != nil {
		a.fail(taskID, "backtest_result_save_failed", err)
		return
	}
	finished := time.Now().UTC()
	_ = a.SpotDAL.TransitionBacktestTask(ctx, taskID, schema.SpotBacktestCalculating, schema.SpotBacktestCompleted,
		map[string]interface{}{"progress": 100, "finished_at": &finished})
}

func (a *SpotStrategy) cancelled(ctx context.Context, taskID string) bool {
	if ctx.Err() != nil {
		return true
	}
	requested, err := a.SpotDAL.IsBacktestCancelRequested(ctx, taskID)
	return err == nil && requested
}

func (a *SpotStrategy) markCancelled(taskID, from string) {
	finished := time.Now().UTC()
	_ = a.SpotDAL.TransitionBacktestTask(context.Background(), taskID, from, schema.SpotBacktestCancelled,
		map[string]interface{}{"finished_at": &finished})
}

func (a *SpotStrategy) fail(taskID, code string, err error) {
	message := strings.TrimSpace(err.Error())
	if len(message) > 1000 {
		message = message[:1000]
	}
	_ = a.SpotDAL.FailBacktestTask(context.Background(), taskID, code, message)
}

func decodeStoredSpotConfig(raw string) (schema.SpotCoreSatelliteConfig, error) {
	var envelope struct {
		StrategyType string                         `json:"strategy_type"`
		Config       schema.SpotCoreSatelliteConfig `json:"spot_core_satellite_config"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return schema.SpotCoreSatelliteConfig{}, err
	}
	config := envelope.Config
	if envelope.StrategyType == "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return config, err
		}
	}
	config.Normalize()
	return config, config.Validate()
}

func (a *SpotStrategy) Shutdown() {
	a.workersMu.Lock()
	defer a.workersMu.Unlock()
	for _, cancel := range a.workers {
		cancel()
	}
}

func (a *SpotStrategy) DebugWorkerCount() int {
	a.workersMu.Lock()
	defer a.workersMu.Unlock()
	return len(a.workers)
}

func spotBacktestTaskLabel(task *schema.SpotBacktestTask) string {
	return fmt.Sprintf("%s/%s %s", task.Exchange, task.Symbol, task.Interval)
}
