package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SpotStrategy struct{ DB *gorm.DB }

type SpotLotAllocationRequest struct {
	LotID    string `json:"lot_id"`
	Quantity string `json:"quantity"`
}

func (a *SpotStrategy) CreateBacktestTask(ctx context.Context, item *schema.SpotBacktestTask) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *SpotStrategy) GetBacktestTask(ctx context.Context, ownerUserID, id string) (*schema.SpotBacktestTask, error) {
	item := new(schema.SpotBacktestTask)
	err := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND id = ?", ownerUserID, id).First(item).Error
	return item, errors.WithStack(err)
}

func (a *SpotStrategy) QueryBacktestTasks(ctx context.Context, params schema.SpotBacktestQueryParam) (*schema.SpotBacktestQueryResult, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("owner_user_id = ?", params.OwnerUserID)
	if value := strings.TrimSpace(params.Status); value != "" {
		db = db.Where("status = ?", value)
	}
	if value := strings.ToUpper(strings.TrimSpace(params.Symbol)); value != "" {
		db = db.Where("symbol = ?", value)
	}
	var list []*schema.SpotBacktestTask
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.SpotBacktestQueryResult{Data: list, PageResult: page}, nil
}

func (a *SpotStrategy) TransitionBacktestTask(ctx context.Context, id, from, to string, values map[string]interface{}) error {
	values["status"] = to
	values["updated_at"] = time.Now().UTC()
	result := util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("id = ? AND status = ?", id, from).Updates(values)
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.Conflict("spot_backtest_state_conflict", "Backtest task state changed concurrently")
	}
	return nil
}

func (a *SpotStrategy) FailBacktestTask(ctx context.Context, id, code, message string) error {
	now := time.Now().UTC()
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("id = ? AND status NOT IN ?", id,
		[]string{schema.SpotBacktestCompleted, schema.SpotBacktestFailed, schema.SpotBacktestCancelled}).Updates(map[string]interface{}{
		"status": schema.SpotBacktestFailed, "error_code": code, "error_message": message, "finished_at": &now, "updated_at": now,
	}).Error)
}

func (a *SpotStrategy) MarkInterruptedBacktestsFailed(ctx context.Context) error {
	now := time.Now().UTC()
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("status IN ?", []string{
		schema.SpotBacktestLoadingData, schema.SpotBacktestRunning, schema.SpotBacktestCalculating,
	}).Updates(map[string]interface{}{"status": schema.SpotBacktestFailed, "error_code": "process_restarted",
		"error_message": "Backtest worker was interrupted by a process restart", "finished_at": &now, "updated_at": now}).Error)
}

func (a *SpotStrategy) UpdateBacktestProgress(ctx context.Context, id, status string, progress int, marketAt *time.Time) error {
	values := map[string]interface{}{"progress": progress, "processed_market_at": marketAt, "updated_at": time.Now().UTC()}
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("id = ? AND status = ?", id, status).Updates(values).Error)
}

func (a *SpotStrategy) RequestBacktestCancel(ctx context.Context, ownerUserID, id string) error {
	result := util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Where("owner_user_id = ? AND id = ? AND status NOT IN ?",
		ownerUserID, id, []string{schema.SpotBacktestCompleted, schema.SpotBacktestFailed, schema.SpotBacktestCancelled}).
		Updates(map[string]interface{}{"cancel_requested": true, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected != 1 {
		return errors.Conflict("spot_backtest_not_cancellable", "Backtest task is already finished")
	}
	return nil
}

func (a *SpotStrategy) IsBacktestCancelRequested(ctx context.Context, id string) (bool, error) {
	var value bool
	err := util.GetDB(ctx, a.DB).Model(new(schema.SpotBacktestTask)).Select("cancel_requested").Where("id = ?", id).Scan(&value).Error
	return value, errors.WithStack(err)
}

func (a *SpotStrategy) SaveBacktestResult(ctx context.Context, taskID string, output *schema.SpotBacktestOutput, dataPoints int) error {
	metrics, err := json.Marshal(output.Metrics)
	if err != nil {
		return err
	}
	benchmarks, err := json.Marshal(output.Benchmarks)
	if err != nil {
		return err
	}
	curve, err := json.Marshal(output.EquityCurve)
	if err != nil {
		return err
	}
	lots, err := json.Marshal(output.Lots)
	if err != nil {
		return err
	}
	events, err := json.Marshal(output.Events)
	if err != nil {
		return err
	}
	acceptance, err := json.Marshal(output.Acceptance)
	if err != nil {
		return err
	}
	result := &schema.SpotBacktestResult{TaskID: taskID, MetricsJSON: string(metrics), BenchmarksJSON: string(benchmarks),
		EquityCurveJSON: string(curve), LotsJSON: string(lots), EventsJSON: string(events), AcceptanceJSON: string(acceptance),
		DataPoints: dataPoints, CreatedAt: time.Now().UTC()}
	return errors.WithStack(util.GetDB(ctx, a.DB).Clauses(clause.OnConflict{UpdateAll: true}).Create(result).Error)
}

func (a *SpotStrategy) GetBacktestResult(ctx context.Context, taskID string) (*schema.SpotBacktestResult, error) {
	item := new(schema.SpotBacktestResult)
	err := util.GetDB(ctx, a.DB).Where("task_id = ?", taskID).First(item).Error
	return item, errors.WithStack(err)
}

func (a *SpotStrategy) QueryLots(ctx context.Context, params schema.SpotLotQueryParam) (*schema.SpotLotQueryResult, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.SpotPositionLot)).Where("owner_user_id = ?", params.OwnerUserID)
	if value := strings.TrimSpace(params.StrategyID); value != "" {
		db = db.Where("strategy_id = ?", value)
	}
	if value := strings.TrimSpace(params.CycleID); value != "" {
		db = db.Where("cycle_id = ?", value)
	}
	if value := strings.TrimSpace(params.Bucket); value != "" {
		db = db.Where("bucket = ?", value)
	}
	if value := strings.TrimSpace(params.Status); value != "" {
		db = db.Where("status = ?", value)
	}
	var list []*schema.SpotPositionLot
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "opened_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.SpotLotQueryResult{Data: list, PageResult: page}, nil
}

func (a *SpotStrategy) QueryStateEvents(ctx context.Context, ownerUserID, cycleID string, limit int) ([]*schema.SpotStateEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	var list []*schema.SpotStateEvent
	err := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND cycle_id = ?", ownerUserID, cycleID).
		Order("market_time DESC").Limit(limit).Find(&list).Error
	return list, errors.WithStack(err)
}

// CreateLotFromBuyFill makes the exchange fill the immutable source of a lot.
func (a *SpotStrategy) CreateLotFromBuyFill(ctx context.Context, lot *schema.SpotPositionLot, fill *schema.Fill, feeRateBPS float64) error {
	if fill == nil || !strings.EqualFold(fill.Side, "buy") {
		return fmt.Errorf("a buy fill is required")
	}
	quantity, err := spotDecimal(fill.Quantity)
	if err != nil || quantity.Sign() <= 0 {
		return fmt.Errorf("invalid buy quantity")
	}
	quote, err := spotDecimal(fill.QuoteQuantity)
	if err != nil || quote.Sign() <= 0 {
		price, priceErr := spotDecimal(fill.Price)
		if priceErr != nil {
			return fmt.Errorf("invalid buy price")
		}
		quote = new(big.Rat).Mul(quantity, price)
	}
	fee := spotDecimalOrZero(fill.FeeValueUSDT)
	acquired := new(big.Rat).Set(quantity)
	base := strings.TrimSuffix(strings.ReplaceAll(fill.Symbol, "-", ""), "USDT")
	if strings.EqualFold(fill.FeeAsset, base) {
		acquired.Sub(acquired, spotDecimalOrZero(fill.FeeQuantity))
	}
	if acquired.Sign() <= 0 {
		return fmt.Errorf("buy fee consumes the acquired quantity")
	}
	totalCost := new(big.Rat).Add(quote, fee)
	unitCost := new(big.Rat).Quo(totalCost, acquired)
	targetRate := spotDecimalOrZero(lot.TargetNetProfitRate)
	feeRate := new(big.Rat).Quo(big.NewRat(int64(mathRound(feeRateBPS*1e6)), 1e6), big.NewRat(10000, 1))
	target := new(big.Rat).Mul(unitCost, new(big.Rat).Add(big.NewRat(1, 1), targetRate))
	target.Quo(target, new(big.Rat).Sub(big.NewRat(1, 1), feeRate))
	now := fill.FilledAt
	lot.SourceBuyFillID, lot.AcquiredQty, lot.RemainingQty = fill.ID, spotDecimalText(acquired), spotDecimalText(acquired)
	lot.TotalCost, lot.UnitCost, lot.TargetPrice = spotDecimalText(totalCost), spotDecimalText(unitCost), spotDecimalText(target)
	lot.Status, lot.OpenedAt, lot.CreatedAt, lot.UpdatedAt = schema.SpotLotOpen, now, now, now
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(lot).Error)
}

// AllocateSellFill atomically consumes explicitly selected lots. Reprocessing
// the same fill is idempotent and returns the original allocations.
func (a *SpotStrategy) AllocateSellFill(ctx context.Context, ownerUserID, bucket string, fill *schema.Fill,
	requests []SpotLotAllocationRequest) ([]*schema.SpotSellAllocation, error) {
	if fill == nil || !strings.EqualFold(fill.Side, "sell") {
		return nil, fmt.Errorf("a sell fill is required")
	}
	fillQty, err := spotDecimal(fill.Quantity)
	if err != nil || fillQty.Sign() <= 0 {
		return nil, fmt.Errorf("invalid sell quantity")
	}
	var result []*schema.SpotSellAllocation
	err = util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if queryErr := tx.Where("sell_fill_id = ?", fill.ID).Order("created_at ASC").Find(&result).Error; queryErr != nil {
			return queryErr
		}
		if len(result) > 0 {
			total := new(big.Rat)
			existing := make(map[string]*big.Rat, len(result))
			for _, item := range result {
				quantity := spotDecimalOrZero(item.AllocatedQty)
				total.Add(total, quantity)
				existing[item.LotID] = quantity
			}
			if total.Cmp(fillQty) != 0 {
				return fmt.Errorf("existing sell allocation does not match fill quantity")
			}
			if len(existing) != len(requests) {
				return fmt.Errorf("existing sell allocation does not match requested lots")
			}
			for _, request := range requests {
				quantity, ok := existing[request.LotID]
				if !ok || quantity.Cmp(spotDecimalOrZero(request.Quantity)) != 0 {
					return fmt.Errorf("existing sell allocation does not match requested lots")
				}
			}
			return nil
		}
		requestedTotal := new(big.Rat)
		for _, request := range requests {
			quantity, parseErr := spotDecimal(request.Quantity)
			if parseErr != nil || quantity.Sign() <= 0 {
				return fmt.Errorf("invalid allocation quantity")
			}
			requestedTotal.Add(requestedTotal, quantity)
		}
		if requestedTotal.Cmp(fillQty) != 0 {
			return fmt.Errorf("allocation quantity must equal sell fill quantity")
		}
		sellPrice, parseErr := spotDecimal(fill.Price)
		if parseErr != nil {
			return fmt.Errorf("invalid sell price")
		}
		totalSellFee := spotDecimalOrZero(fill.FeeValueUSDT)
		cyclePnL := new(big.Rat)
		netCash := new(big.Rat)
		cycleID := ""
		for _, request := range requests {
			lot := new(schema.SpotPositionLot)
			if queryErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", request.LotID, ownerUserID).First(lot).Error; queryErr != nil {
				return queryErr
			}
			if lot.Bucket != bucket || lot.Symbol != fill.Symbol || lot.StrategyID != fill.StrategyID {
				return fmt.Errorf("sell fill and source lot attribution do not match")
			}
			if cycleID == "" {
				cycleID = lot.CycleID
			} else if cycleID != lot.CycleID {
				return fmt.Errorf("one sell fill cannot cross strategy cycles")
			}
			quantity := spotDecimalOrZero(request.Quantity)
			remaining := spotDecimalOrZero(lot.RemainingQty)
			reserved := spotDecimalOrZero(lot.ReservedQty)
			if quantity.Cmp(remaining) > 0 {
				return fmt.Errorf("allocation exceeds lot inventory")
			}
			if reserved.Sign() > 0 && quantity.Cmp(reserved) > 0 {
				return fmt.Errorf("allocation exceeds reserved lot quantity")
			}
			unitCost := spotDecimalOrZero(lot.UnitCost)
			cost := new(big.Rat).Mul(unitCost, quantity)
			gross := new(big.Rat).Mul(sellPrice, quantity)
			feeShare := new(big.Rat).Mul(totalSellFee, new(big.Rat).Quo(quantity, fillQty))
			proceeds := new(big.Rat).Sub(gross, feeShare)
			pnl := new(big.Rat).Sub(proceeds, cost)
			pnlRate := new(big.Rat)
			if cost.Sign() > 0 {
				pnlRate.Quo(pnl, cost)
			}
			allocation := &schema.SpotSellAllocation{ID: util.MustNewUUID(), OwnerUserID: ownerUserID, SellFillID: fill.ID,
				LotID: lot.ID, AllocatedQty: spotDecimalText(quantity), AllocatedCost: spotDecimalText(cost),
				AllocatedSellFee: spotDecimalText(feeShare), NetProceeds: spotDecimalText(proceeds), RealizedPnL: spotDecimalText(pnl),
				RealizedPnLRate: spotDecimalText(pnlRate), CreatedAt: fill.FilledAt}
			buyFill := new(schema.Fill)
			if queryErr := tx.Where("id = ?", lot.SourceBuyFillID).First(buyFill).Error; queryErr == nil {
				allocation.AllocatedBuyFee = spotDecimalText(new(big.Rat).Mul(spotDecimalOrZero(buyFill.FeeValueUSDT), new(big.Rat).Quo(quantity, spotDecimalOrZero(lot.AcquiredQty))))
			}
			if createErr := tx.Create(allocation).Error; createErr != nil {
				return createErr
			}
			remaining.Sub(remaining, quantity)
			if reserved.Sign() > 0 {
				reserved.Sub(reserved, quantity)
				if reserved.Sign() < 0 {
					reserved.SetInt64(0)
				}
			}
			status := schema.SpotLotPartialClosed
			values := map[string]interface{}{"remaining_qty": spotDecimalText(remaining), "reserved_qty": spotDecimalText(reserved), "status": status,
				"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}
			if remaining.Sign() == 0 {
				status = schema.SpotLotClosed
				closed := fill.FilledAt
				values["status"], values["closed_at"] = status, &closed
			}
			if updateErr := tx.Model(new(schema.SpotPositionLot)).Where("id = ?", lot.ID).Updates(values).Error; updateErr != nil {
				return updateErr
			}
			cyclePnL.Add(cyclePnL, pnl)
			netCash.Add(netCash, proceeds)
			result = append(result, allocation)
		}
		if cycleID != "" {
			cycle := new(schema.SpotStrategyCycle)
			if queryErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", cycleID, ownerUserID).First(cycle).Error; queryErr != nil {
				return queryErr
			}
			cash := new(big.Rat).Add(spotDecimalOrZero(cycle.AvailableCash), netCash)
			pnl := new(big.Rat).Add(spotDecimalOrZero(cycle.RealizedPnL), cyclePnL)
			if updateErr := tx.Model(cycle).Updates(map[string]interface{}{"available_cash": spotDecimalText(cash), "realized_pnl": spotDecimalText(pnl),
				"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error; updateErr != nil {
				return updateErr
			}
		}
		return nil
	})
	return result, errors.WithStack(err)
}

func spotDecimal(value string) (*big.Rat, error) {
	result, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	return result, nil
}
func spotDecimalOrZero(value string) *big.Rat {
	result, err := spotDecimal(value)
	if err != nil {
		return new(big.Rat)
	}
	return result
}
func spotDecimalText(value *big.Rat) string {
	text := value.FloatString(18)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}
func mathRound(value float64) int64 {
	if value < 0 {
		return int64(value - 0.5)
	}
	return int64(value + 0.5)
}
