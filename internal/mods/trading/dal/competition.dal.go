package dal

import (
	"context"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

type Competition struct {
	DB *gorm.DB
}

func (a *Competition) ListParticipants(ctx context.Context, limit int) ([]*schema.Trader, error) {
	var items []*schema.Trader
	if err := util.GetDB(ctx, a.DB).Where("show_in_competition = ? AND competition_public_id <> ?", true, "").
		Order("competition_joined_at ASC, id ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (a *Competition) GetParticipantByPublicID(ctx context.Context, publicID string) (*schema.Trader, error) {
	var item schema.Trader
	err := util.GetDB(ctx, a.DB).Where("show_in_competition = ? AND competition_public_id = ?", true, publicID).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &item, nil
}

func (a *Competition) ListClosedTrades(ctx context.Context, traderIDs []string, startAt, endAt time.Time) ([]*schema.ClosedTrade, error) {
	if len(traderIDs) == 0 {
		return nil, nil
	}
	var items []*schema.ClosedTrade
	if err := util.GetDB(ctx, a.DB).Select("trader_id", "net_pnl", "closed_at").
		Where("trader_id IN ? AND closed_at >= ? AND closed_at <= ?", traderIDs, startAt, endAt).
		Order("closed_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	return items, nil
}

func (a *Competition) UpdateParticipation(ctx context.Context, item *schema.Trader) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Select(
		"show_in_competition", "competition_public_id", "competition_alias", "competition_joined_at", "updated_at",
	).Updates(item).Error)
}
