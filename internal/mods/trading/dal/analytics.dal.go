package dal

import (
	"context"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

type Analytics struct {
	DB *gorm.DB
}

func (a *Analytics) QueryClosedTrades(ctx context.Context, params schema.AnalyticsQueryParam) (*schema.ClosedTradeQueryResult, error) {
	db := analyticsTradeDB(ctx, a.DB, params)
	var list []*schema.ClosedTrade
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "closed_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.ClosedTradeQueryResult{Data: list, PageResult: page}, nil
}

func (a *Analytics) ListClosedTrades(ctx context.Context, params schema.AnalyticsQueryParam) ([]*schema.ClosedTrade, error) {
	var list []*schema.ClosedTrade
	if err := analyticsTradeDB(ctx, a.DB, params).Order("closed_at ASC").Find(&list).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return list, nil
}

func (a *Analytics) QueryOverview(ctx context.Context, params schema.AnalyticsQueryParam) (*schema.DashboardOverview, error) {
	result := &schema.DashboardOverview{}
	assetDB := util.GetDB(ctx, a.DB).Model(new(schema.AssetBalance)).Where("owner_user_id = ?", params.OwnerUserID)
	if params.ExchangeID != "" {
		assetDB = assetDB.Where("exchange_id = ?", params.ExchangeID)
	}
	var assetTotals struct {
		TotalEquity      string `gorm:"column:total_equity"`
		AvailableBalance string `gorm:"column:available_balance"`
	}
	if err := assetDB.Select("COALESCE(SUM(value_usdt), 0) AS total_equity, COALESCE(SUM(available * price_usdt), 0) AS available_balance").
		Scan(&assetTotals).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	result.TotalEquity = assetTotals.TotalEquity
	result.AvailableBalance = assetTotals.AvailableBalance
	latestDB := util.GetDB(ctx, a.DB).Model(new(schema.AssetBalance)).Where("owner_user_id = ?", params.OwnerUserID)
	if params.ExchangeID != "" {
		latestDB = latestDB.Where("exchange_id = ?", params.ExchangeID)
	}
	var latest schema.AssetBalance
	if err := latestDB.Select("captured_at").Order("captured_at DESC").First(&latest).Error; err == nil {
		result.LastUpdatedAt = &latest.CapturedAt
	} else if err != gorm.ErrRecordNotFound {
		return nil, errors.WithStack(err)
	}

	positionDB := util.GetDB(ctx, a.DB).Model(new(schema.Position)).Where("owner_user_id = ? AND status = ?", params.OwnerUserID, schema.PositionStatusOpen)
	if params.ExchangeID != "" {
		positionDB = positionDB.Where("exchange_id = ?", params.ExchangeID)
	}
	var positionTotals struct {
		Count         int64  `gorm:"column:count"`
		UnrealizedPnL string `gorm:"column:unrealized_pnl"`
	}
	if err := positionDB.Select("COUNT(*) AS count, COALESCE(SUM(unrealized_pnl), 0) AS unrealized_pnl").Scan(&positionTotals).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	result.PositionCount = positionTotals.Count
	result.UnrealizedPnL = positionTotals.UnrealizedPnL

	tradeDB := analyticsTradeDB(ctx, a.DB, params)
	var tradeTotals struct {
		Count       int64  `gorm:"column:count"`
		RealizedPnL string `gorm:"column:realized_pnl"`
		TotalFee    string `gorm:"column:total_fee"`
	}
	if err := tradeDB.Select("COUNT(*) AS count, COALESCE(SUM(gross_pnl), 0) AS realized_pnl, COALESCE(SUM(fee), 0) AS total_fee").
		Scan(&tradeTotals).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	result.ClosedTradeCount = tradeTotals.Count
	result.RealizedPnL = tradeTotals.RealizedPnL
	result.TotalFee = tradeTotals.TotalFee

	funding, err := a.QueryFunding(ctx, params)
	if err != nil {
		return nil, err
	}
	result.Funding = funding
	return result, nil
}

func (a *Analytics) QueryFunding(ctx context.Context, params schema.AnalyticsQueryParam) (string, error) {
	db := fundingDB(ctx, a.DB, params)
	var total struct {
		Value string `gorm:"column:value"`
	}
	if err := db.Select("COALESCE(SUM(value_usdt), 0) AS value").Scan(&total).Error; err != nil {
		return "", errors.WithStack(err)
	}
	return total.Value, nil
}

func (a *Analytics) ListFundingEvents(ctx context.Context, params schema.AnalyticsQueryParam) ([]*schema.IncomeEvent, error) {
	db := fundingDB(ctx, a.DB, params)
	var items []*schema.IncomeEvent
	if err := db.Order("occurred_at ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func fundingDB(ctx context.Context, db *gorm.DB, params schema.AnalyticsQueryParam) *gorm.DB {
	db = util.GetDB(ctx, db).Model(new(schema.IncomeEvent)).Where("owner_user_id = ?", params.OwnerUserID).
		Where("event_type IN ?", []string{"funding", "funding_fee", "fundingfee"})
	// Funding events currently have account/trader attribution but no strategy
	// attribution. Returning an empty set is safer than assigning exchange-wide
	// funding to an individual strategy report.
	if strings.TrimSpace(params.StrategyID) != "" || params.StrategyVersion > 0 {
		return db.Where("1 = 0")
	}
	if params.TraderID != "" {
		db = db.Where("trader_id = ?", params.TraderID)
	}
	if params.ExchangeID != "" {
		db = db.Where("exchange_id = ?", params.ExchangeID)
	}
	if params.Market != "" {
		db = db.Where("market = ?", strings.ToLower(params.Market))
	}
	if params.Symbol != "" {
		db = db.Where("symbol = ?", strings.ToUpper(params.Symbol))
	}
	if params.StartAt != nil {
		db = db.Where("occurred_at >= ?", params.StartAt.UTC())
	}
	if params.EndAt != nil {
		db = db.Where("occurred_at <= ?", params.EndAt.UTC())
	}
	return db
}

func (a *Analytics) ListEquityPoints(ctx context.Context, params schema.AnalyticsQueryParam) ([]*schema.AccountSnapshot, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.AccountSnapshot)).Where("owner_user_id = ?", params.OwnerUserID)
	if params.ExchangeID != "" {
		db = db.Where("exchange_id = ?", params.ExchangeID)
	}
	if params.StartAt != nil {
		db = db.Where("captured_at >= ?", params.StartAt.UTC())
	}
	if params.EndAt != nil {
		db = db.Where("captured_at <= ?", params.EndAt.UTC())
	}
	var list []*schema.AccountSnapshot
	if err := db.Order("captured_at ASC").Find(&list).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return list, nil
}

func (a *Analytics) ResolveTraderExchange(ctx context.Context, ownerUserID, traderID string) (string, error) {
	if strings.TrimSpace(traderID) == "" {
		return "", nil
	}
	var trader schema.Trader
	if err := util.GetDB(ctx, a.DB).Select("exchange_id").Where("owner_user_id = ? AND id = ?", ownerUserID, traderID).
		First(&trader).Error; err != nil {
		return "", errors.WithStack(err)
	}
	return trader.ExchangeID, nil
}

func analyticsTradeDB(ctx context.Context, db *gorm.DB, params schema.AnalyticsQueryParam) *gorm.DB {
	db = util.GetDB(ctx, db).Model(new(schema.ClosedTrade)).Where("owner_user_id = ?", params.OwnerUserID)
	for column, value := range map[string]string{
		"exchange_id": params.ExchangeID, "market": strings.ToLower(params.Market),
		"symbol": strings.ToUpper(params.Symbol), "side": strings.ToLower(params.Side),
		"trader_id": params.TraderID, "strategy_id": params.StrategyID,
	} {
		if value = strings.TrimSpace(value); value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	if params.StrategyVersion > 0 {
		db = db.Where("strategy_version = ?", params.StrategyVersion)
	}
	if params.StartAt != nil {
		db = db.Where("closed_at >= ?", params.StartAt.UTC())
	}
	if params.EndAt != nil {
		db = db.Where("closed_at <= ?", params.EndAt.UTC())
	}
	return db
}
