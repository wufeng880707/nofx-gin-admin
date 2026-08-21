package dal

import (
	"context"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

func (a *Execution) CreateApprovalWithEvent(ctx context.Context, approval *schema.TradeApproval, event *schema.TradeApprovalEvent) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(approval).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}))
}

func (a *Execution) GetApproval(ctx context.Context, id, ownerUserID string) (*schema.TradeApproval, error) {
	item := new(schema.TradeApproval)
	err := util.GetDB(ctx, a.DB).Where("id = ? AND owner_user_id = ?", id, ownerUserID).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return item, errors.WithStack(err)
}

func (a *Execution) QueryApprovals(ctx context.Context, params schema.TradeApprovalQueryParam) (*schema.TradeApprovalQueryResult, error) {
	db := util.GetDB(ctx, a.DB).Model(new(schema.TradeApproval)).Where("owner_user_id = ?", params.OwnerUserID)
	if value := strings.TrimSpace(params.TraderID); value != "" {
		db = db.Where("trader_id = ?", value)
	}
	if value := strings.ToLower(strings.TrimSpace(params.Status)); value != "" {
		db = db.Where("status = ?", value)
	}
	if value := strings.ToUpper(strings.TrimSpace(params.Symbol)); value != "" {
		db = db.Where("symbol = ?", value)
	}
	var list []*schema.TradeApproval
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "requested_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.TradeApprovalQueryResult{Data: list, PageResult: page}, nil
}

func (a *Execution) TransitionApproval(ctx context.Context, id, ownerUserID, fromStatus string,
	values map[string]interface{}, event *schema.TradeApprovalEvent) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(new(schema.TradeApproval)).Where("id = ? AND owner_user_id = ? AND status = ?",
			id, ownerUserID, fromStatus).Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrInvalidTransaction
		}
		return tx.Create(event).Error
	}))
}

func (a *Execution) ExpireApprovals(ctx context.Context, ownerUserID string, now time.Time) error {
	var pending []*schema.TradeApproval
	if err := util.GetDB(ctx, a.DB).Where("owner_user_id = ? AND status = ? AND expires_at <= ?",
		ownerUserID, schema.TradeApprovalStatusPending, now).Find(&pending).Error; err != nil {
		return errors.WithStack(err)
	}
	for _, item := range pending {
		event := &schema.TradeApprovalEvent{ApprovalID: item.ID, OwnerUserID: ownerUserID,
			ActorUserID: "system", EventType: "expired", FromStatus: schema.TradeApprovalStatusPending,
			ToStatus: schema.TradeApprovalStatusExpired, DetailJSON: `{"reason":"approval timeout"}`, CreatedAt: now}
		if err := a.TransitionApproval(ctx, item.ID, ownerUserID, schema.TradeApprovalStatusPending,
			map[string]interface{}{"status": schema.TradeApprovalStatusExpired, "updated_at": now}, event); err != nil && !errors.Is(err, gorm.ErrInvalidTransaction) {
			return err
		}
	}
	return nil
}

func (a *Execution) QueryApprovalEvents(ctx context.Context, approvalID, ownerUserID string) ([]*schema.TradeApprovalEvent, error) {
	var list []*schema.TradeApprovalEvent
	err := util.GetDB(ctx, a.DB).Where("approval_id = ? AND owner_user_id = ?", approvalID, ownerUserID).
		Order("id ASC").Find(&list).Error
	return list, errors.WithStack(err)
}
