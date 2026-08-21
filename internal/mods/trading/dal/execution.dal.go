package dal

import (
	"context"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

type Execution struct{ DB *gorm.DB }

func (a *Execution) GetSafetyState(ctx context.Context, ownerUserID string) (*schema.TradingSafetyState, error) {
	item := new(schema.TradingSafetyState)
	err := util.GetDB(ctx, a.DB).Where("owner_user_id = ?", ownerUserID).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &schema.TradingSafetyState{OwnerUserID: ownerUserID, LiveEnabled: false, Version: 1}, nil
	}
	return item, errors.WithStack(err)
}

func (a *Execution) SaveSafetyStateWithEvent(ctx context.Context, state *schema.TradingSafetyState, event *schema.TradingSafetyEvent) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(state).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}))
}

func (a *Execution) FindByIdempotency(ctx context.Context, ownerUserID, key string) (*schema.TradeCommand, error) {
	item := new(schema.TradeCommand)
	err := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND idempotency_key = ?", ownerUserID, key).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return item, errors.WithStack(err)
}

func (a *Execution) CreateCommandWithEvent(ctx context.Context, command *schema.TradeCommand, event *schema.TradeCommandEvent) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(command).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}))
}

func (a *Execution) Transition(ctx context.Context, id, ownerUserID, fromStatus string,
	values map[string]interface{}, event *schema.TradeCommandEvent) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(new(schema.TradeCommand)).Where("id = ? AND owner_user_id = ? AND status = ?", id, ownerUserID, fromStatus).Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrInvalidTransaction
		}
		return tx.Create(event).Error
	}))
}

func (a *Execution) QueryCommands(ctx context.Context, params schema.TradeCommandQueryParam) (*schema.TradeCommandQueryResult, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.TradeCommand)).Where("owner_user_id = ?", params.OwnerUserID)
	if value := strings.TrimSpace(params.TraderID); value != "" {
		db = db.Where("trader_id = ?", value)
	}
	if value := strings.ToLower(strings.TrimSpace(params.Status)); value != "" {
		db = db.Where("status = ?", value)
	}
	if value := strings.ToUpper(strings.TrimSpace(params.Symbol)); value != "" {
		db = db.Where("symbol = ?", value)
	}
	var list []*schema.TradeCommand
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.TradeCommandQueryResult{Data: list, PageResult: page}, nil
}

func (a *Execution) QueryEvents(ctx context.Context, commandID, ownerUserID string) ([]*schema.TradeCommandEvent, error) {
	var list []*schema.TradeCommandEvent
	err := util.GetDB(ctx, a.DB).Where("command_id = ? AND owner_user_id = ?", commandID, ownerUserID).
		Order("id ASC").Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Execution) QueryOpenPositions(ctx context.Context, ownerUserID, traderID string, limit int) ([]*schema.Position, error) {
	db := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND status = ?", ownerUserID, schema.PositionStatusOpen)
	if traderID != "" {
		db = db.Where("trader_id = ?", traderID)
	}
	var list []*schema.Position
	err := db.Order("opened_at ASC").Limit(limit).Find(&list).Error
	return list, errors.WithStack(err)
}
