package dal

import (
	"context"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

type Observation struct {
	DB *gorm.DB
}

func (a *Observation) QueryAssetSummary(ctx context.Context, ownerUserID string) (*schema.AssetSummary, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.AssetBalance)).Where("owner_user_id = ?", ownerUserID)
	var total struct {
		ValueUSDT string
		Count     int64
	}
	if err := db.Select("COALESCE(SUM(value_usdt), 0) AS value_usdt, COUNT(*) AS count").Scan(&total).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	exchanges := make([]*schema.AssetSummaryExchange, 0)
	if err := db.Select("exchange_id, account_type, COUNT(*) AS asset_count, COALESCE(SUM(value_usdt), 0) AS value_usdt, MAX(captured_at) AS last_updated_at").
		Group("exchange_id, account_type").Order("value_usdt DESC").Scan(&exchanges).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.AssetSummary{TotalValueUSDT: total.ValueUSDT, AssetCount: total.Count, Exchanges: exchanges}, nil
}

func (a *Observation) QueryAccountSnapshots(ctx context.Context, params schema.ObservationQueryParam) (*schema.AccountSnapshotQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.AccountSnapshot)), params, "captured_at")
	var list []*schema.AccountSnapshot
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("captured_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.AccountSnapshotQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryUserStreamStates(ctx context.Context, ownerUserID string) ([]*schema.UserStreamState, error) {
	var list []*schema.UserStreamState
	err := util.GetDB(ctx, a.DB).Where("owner_user_id = ?", ownerUserID).Order("updated_at DESC").Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Observation) QueryUserStreamEvents(ctx context.Context, params schema.UserStreamEventQueryParam) (*schema.UserStreamEventQueryResult, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.UserStreamEvent)).Where("owner_user_id = ?", params.OwnerUserID)
	if value := strings.TrimSpace(params.ExchangeID); value != "" {
		db = db.Where("exchange_id = ?", value)
	}
	if value := strings.TrimSpace(params.EventType); value != "" {
		db = db.Where("event_type = ?", value)
	}
	var list []*schema.UserStreamEvent
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("received_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.UserStreamEventQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryPositions(ctx context.Context, params schema.PositionQueryParam) (*schema.PositionQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.Position)), params.ObservationQueryParam, "last_synced_at")
	if value := strings.TrimSpace(params.Symbol); value != "" {
		db = db.Where("symbol = ?", strings.ToUpper(value))
	}
	if value := strings.ToLower(strings.TrimSpace(params.Side)); value != "" {
		db = db.Where("side = ?", value)
	}
	if value := strings.ToLower(strings.TrimSpace(params.Status)); value != "" {
		db = db.Where("status = ?", value)
	}
	var list []*schema.Position
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("last_synced_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.PositionQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryAssetBalances(ctx context.Context, params schema.AssetBalanceQueryParam) (*schema.AssetBalanceQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.AssetBalance)), params.ObservationQueryParam, "captured_at")
	if value := strings.ToUpper(strings.TrimSpace(params.Asset)); value != "" {
		db = db.Where("asset = ?", value)
	}
	if value := strings.ToLower(strings.TrimSpace(params.AccountType)); value != "" {
		db = db.Where("account_type = ?", value)
	}
	var list []*schema.AssetBalance
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("value_usdt"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.AssetBalanceQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryOrders(ctx context.Context, params schema.OrderQueryParam) (*schema.OrderQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.ExchangeOrder)), params.ObservationQueryParam, "ordered_at")
	for column, value := range map[string]string{"symbol": strings.ToUpper(params.Symbol), "market": strings.ToLower(params.Market),
		"side": strings.ToLower(params.Side), "status": strings.ToLower(params.Status), "origin": strings.ToLower(params.Origin)} {
		if value = strings.TrimSpace(value); value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	var list []*schema.ExchangeOrder
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("ordered_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.OrderQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryFills(ctx context.Context, params schema.FillQueryParam) (*schema.FillQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.Fill)), params.ObservationQueryParam, "filled_at")
	for column, value := range map[string]string{"symbol": strings.ToUpper(params.Symbol), "market": strings.ToLower(params.Market),
		"side": strings.ToLower(params.Side)} {
		if value = strings.TrimSpace(value); value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	var list []*schema.Fill
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("filled_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.FillQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryIncomeEvents(ctx context.Context, params schema.IncomeEventQueryParam) (*schema.IncomeEventQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.IncomeEvent)), params.ObservationQueryParam, "occurred_at")
	for column, value := range map[string]string{"symbol": strings.ToUpper(params.Symbol), "market": strings.ToLower(params.Market),
		"asset": strings.ToUpper(params.Asset), "event_type": strings.ToLower(params.EventType)} {
		if value = strings.TrimSpace(value); value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	var list []*schema.IncomeEvent
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("occurred_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.IncomeEventQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) QueryDecisions(ctx context.Context, params schema.DecisionQueryParam) (*schema.DecisionQueryResult, error) {
	db := observationDB(ctx, a.DB.Model(new(schema.Decision)), params.ObservationQueryParam, "decided_at")
	if params.Success != nil {
		db = db.Where("success = ?", *params.Success)
	}
	var list []*schema.Decision
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, newestFirst("decided_at"), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.DecisionQueryResult{Data: list, PageResult: page}, nil
}

func (a *Observation) GetDecision(ctx context.Context, id, ownerUserID string) (*schema.Decision, error) {
	item := new(schema.Decision)
	err := util.GetDB(ctx, a.DB).Where("id = ? AND owner_user_id = ?", id, ownerUserID).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return item, errors.WithStack(err)
}

func (a *Observation) ListAIInvocations(ctx context.Context, params schema.AIInvocationSummaryQueryParam) ([]*schema.AIInvocation, error) {
	db := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND started_at >= ? AND started_at <= ?",
		params.OwnerUserID, params.StartAt.UTC(), params.EndAt.UTC())
	for column, value := range map[string]string{"ai_model_id": params.AIModelID, "trader_id": params.TraderID,
		"strategy_id": params.StrategyID, "purpose": params.Purpose} {
		if value = strings.TrimSpace(value); value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	var items []*schema.AIInvocation
	if err := db.Order("started_at ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func observationDB(ctx context.Context, db *gorm.DB, params schema.ObservationQueryParam, timeColumn string) *gorm.DB {
	db = util.GetDB(ctx, db)
	if value := strings.TrimSpace(params.OwnerUserID); value != "" {
		db = db.Where("owner_user_id = ?", value)
	}
	if value := strings.TrimSpace(params.TraderID); value != "" {
		db = db.Where("trader_id = ?", value)
	}
	if value := strings.TrimSpace(params.ExchangeID); value != "" {
		db = db.Where("exchange_id = ?", value)
	}
	if params.StartAt != nil {
		db = db.Where(timeColumn+" >= ?", params.StartAt.UTC())
	}
	if params.EndAt != nil {
		db = db.Where(timeColumn+" <= ?", params.EndAt.UTC())
	}
	return db
}

func newestFirst(field string) util.QueryOptions {
	return util.QueryOptions{OrderFields: []util.OrderByParam{{Field: field, Direction: util.DESC}}}
}
