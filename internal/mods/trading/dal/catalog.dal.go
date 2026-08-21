package dal

import (
	"context"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Catalog struct {
	DB *gorm.DB
}

func (a *Catalog) GetExchange(ctx context.Context, id, ownerUserID string) (*schema.Exchange, error) {
	item := new(schema.Exchange)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) CreateExchange(ctx context.Context, item *schema.Exchange) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *Catalog) UpdateExchange(ctx context.Context, item *schema.Exchange) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error)
}

func (a *Catalog) DeleteExchange(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("exchange_id = ?", id).Delete(new(schema.HistorySymbol)).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(new(schema.Exchange)).Error
	}))
}

func (a *Catalog) ListHistorySymbols(ctx context.Context, exchangeID, ownerUserID string) ([]string, error) {
	var symbols []string
	err := scopedCatalogDB(ctx, a.DB.Model(new(schema.HistorySymbol)), ownerUserID).
		Where("exchange_id = ?", exchangeID).Order("symbol ASC").Pluck("symbol", &symbols).Error
	return symbols, errors.WithStack(err)
}

func (a *Catalog) ReplaceHistorySymbols(ctx context.Context, exchangeID, ownerUserID string, symbols []string) error {
	now := time.Now().UTC()
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("exchange_id = ? AND owner_user_id = ?", exchangeID, ownerUserID).
			Delete(new(schema.HistorySymbol)).Error; err != nil {
			return err
		}
		for _, symbol := range symbols {
			item := &schema.HistorySymbol{ID: util.NewXID(), OwnerUserID: ownerUserID, ExchangeID: exchangeID,
				Symbol: symbol, Source: "manual", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (a *Catalog) AddHistorySymbols(ctx context.Context, exchangeID, ownerUserID, source string, symbols []string) error {
	now := time.Now().UTC()
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		for _, symbol := range symbols {
			item := &schema.HistorySymbol{ID: util.NewXID(), OwnerUserID: ownerUserID, ExchangeID: exchangeID,
				Symbol: symbol, Source: source, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "exchange_id"}, {Name: "symbol"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"updated_at": now}),
			}).Create(item).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (a *Catalog) QueryHistorySyncStatuses(ctx context.Context, ownerUserID string) ([]*schema.HistorySyncStatusView, error) {
	var exchanges []*schema.Exchange
	if err := scopedCatalogDB(ctx, a.DB.Model(new(schema.Exchange)), ownerUserID).
		Order("created_at ASC").Find(&exchanges).Error; err != nil {
		return nil, errors.WithStack(err)
	}
	result := make([]*schema.HistorySyncStatusView, 0, len(exchanges))
	for _, exchange := range exchanges {
		view := &schema.HistorySyncStatusView{ExchangeID: exchange.ID, ExchangeName: exchange.Name,
			ExchangeType: exchange.ExchangeType, AccountType: exchange.AccountType, Enabled: exchange.Enabled,
			Status: "pending", RegisteredSymbols: []string{}}
		cursor := new(schema.SyncCursor)
		err := util.GetDB(ctx, a.DB).Where("trader_id = ? AND exchange_id = ? AND dataset = ? AND market = ? AND symbol = ?",
			"exchange:"+exchange.ID, exchange.ID, "history", "", "").First(cursor).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, errors.WithStack(err)
		}
		if err == nil {
			view.Status, view.SyncedUntil, view.LastRunAt = cursor.Status, cursor.SyncedUntil, cursor.LastRunAt
			view.ErrorMessage = cursor.ErrorMessage
		}
		symbols, err := a.ListHistorySymbols(ctx, exchange.ID, ownerUserID)
		if err != nil {
			return nil, err
		}
		view.RegisteredSymbols = symbols
		for model, target := range map[interface{}]*int64{
			new(schema.ExchangeOrder): &view.OrderCount, new(schema.Fill): &view.FillCount,
			new(schema.IncomeEvent): &view.IncomeEventCount, new(schema.ClosedTrade): &view.ClosedTradeCount,
		} {
			if err := util.GetDB(ctx, a.DB).Model(model).Where("exchange_id = ?", exchange.ID).Count(target).Error; err != nil {
				return nil, errors.WithStack(err)
			}
		}
		fillTable := new(schema.Fill).TableName()
		orderTable := new(schema.ExchangeOrder).TableName()
		if err := util.GetDB(ctx, a.DB).Table(fillTable+" AS f").
			Joins("LEFT JOIN "+orderTable+" AS o ON o.exchange_id = f.exchange_id AND o.market = f.market AND o.exchange_order_id = f.exchange_order_id").
			Where("f.exchange_id = ? AND f.exchange_order_id <> ? AND o.id IS NULL", exchange.ID, "").
			Count(&view.OrphanFillCount).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		if err := util.GetDB(ctx, a.DB).Table(orderTable+" AS o").
			Joins("LEFT JOIN "+fillTable+" AS f ON f.exchange_id = o.exchange_id AND f.market = o.market AND f.exchange_order_id = o.exchange_order_id").
			Where("o.exchange_id = ? AND LOWER(o.status) IN ? AND f.id IS NULL", exchange.ID, []string{"filled", "partially_filled"}).
			Count(&view.UnmatchedOrderCount).Error; err != nil {
			return nil, errors.WithStack(err)
		}
		view.GapCount = view.OrphanFillCount + view.UnmatchedOrderCount
		result = append(result, view)
	}
	return result, nil
}

func (a *Catalog) ExchangeAccountExists(ctx context.Context, ownerUserID, exchangeType, accountName, excludeID string) (bool, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.Exchange)), ownerUserID).
		Where("exchange_type = ? AND account_name = ?", exchangeType, accountName)
	if excludeID != "" {
		db = db.Where("id <> ?", excludeID)
	}
	ok, err := util.Exists(ctx, db)
	return ok, errors.WithStack(err)
}

func (a *Catalog) GetAIModel(ctx context.Context, id, ownerUserID string) (*schema.AIModel, error) {
	item := new(schema.AIModel)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) CreateAIModel(ctx context.Context, item *schema.AIModel) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *Catalog) UpdateAIModel(ctx context.Context, item *schema.AIModel) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error)
}

func (a *Catalog) DeleteAIModel(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Where("id = ?", id).Delete(new(schema.AIModel)).Error)
}

func (a *Catalog) GetStrategy(ctx context.Context, id, ownerUserID string) (*schema.Strategy, error) {
	item := new(schema.Strategy)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) CreateStrategy(ctx context.Context, item *schema.Strategy) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if item.IsDefault {
			if err := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND is_default = ?", item.OwnerUserID, true).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if item.IsActive {
			if err := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND is_active = ?", item.OwnerUserID, true).
				Update("is_active", false).Error; err != nil {
				return err
			}
		}
		item.CurrentVersion = 1
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return tx.Create(strategyVersionSnapshot(item, 1, nil)).Error
	}))
}

func (a *Catalog) UpdateStrategy(ctx context.Context, item *schema.Strategy, createVersion bool) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if createVersion {
			var current schema.Strategy
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", item.ID).First(&current).Error; err != nil {
				return err
			}
			item.CurrentVersion = current.CurrentVersion + 1
			if err := tx.Create(strategyVersionSnapshot(item, item.CurrentVersion, nil)).Error; err != nil {
				return err
			}
		}
		if item.IsDefault {
			if err := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND id <> ? AND is_default = ?", item.OwnerUserID, item.ID, true).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if item.IsActive {
			if err := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND id <> ? AND is_active = ?", item.OwnerUserID, item.ID, true).
				Update("is_active", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error
	}))
}

func (a *Catalog) ListStrategyVersions(ctx context.Context, strategyID, ownerUserID string) ([]*schema.StrategyVersion, error) {
	var items []*schema.StrategyVersion
	err := util.GetDB(ctx, a.DB).Where("strategy_id = ? AND owner_user_id = ?", strategyID, ownerUserID).
		Order("version DESC").Find(&items).Error
	return items, errors.WithStack(err)
}

func (a *Catalog) PublishStrategy(ctx context.Context, strategyID, ownerUserID string, publishedAt time.Time) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		var item schema.Strategy
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ?", strategyID, ownerUserID).First(&item).Error; err != nil {
			return err
		}
		if item.CurrentVersion <= 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(new(schema.StrategyVersion)).
			Where("strategy_id = ? AND version = ?", item.ID, item.CurrentVersion).
			Update("published_at", publishedAt).Error; err != nil {
			return err
		}
		return tx.Model(new(schema.Strategy)).Where("id = ?", item.ID).
			Updates(map[string]interface{}{"published_version": item.CurrentVersion, "updated_at": publishedAt}).Error
	}))
}

func (a *Catalog) RollbackStrategy(ctx context.Context, strategyID, ownerUserID string, sourceVersion int, updatedAt time.Time) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		var item schema.Strategy
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ?", strategyID, ownerUserID).First(&item).Error; err != nil {
			return err
		}
		var source schema.StrategyVersion
		if err := tx.Where("strategy_id = ? AND owner_user_id = ? AND version = ?", strategyID, ownerUserID, sourceVersion).
			First(&source).Error; err != nil {
			return err
		}
		item.Name = source.Name
		item.Description = source.Description
		item.StrategyType = source.StrategyType
		item.ConfigJSON = source.ConfigJSON
		item.CurrentVersion++
		item.UpdatedAt = updatedAt
		rollbackFrom := sourceVersion
		if err := tx.Create(strategyVersionSnapshot(&item, item.CurrentVersion, &rollbackFrom)).Error; err != nil {
			return err
		}
		return tx.Model(new(schema.Strategy)).Where("id = ?", item.ID).
			Updates(map[string]interface{}{
				"name": item.Name, "description": item.Description, "strategy_type": item.StrategyType,
				"config_json": item.ConfigJSON, "current_version": item.CurrentVersion, "updated_at": updatedAt,
			}).Error
	}))
}

func (a *Catalog) ActivateStrategy(ctx context.Context, ownerUserID, id string, updatedAt time.Time) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND is_active = ?", ownerUserID, true).
			Updates(map[string]interface{}{"is_active": false, "updated_at": updatedAt}).Error; err != nil {
			return err
		}
		result := tx.Model(new(schema.Strategy)).Where("owner_user_id = ? AND id = ?", ownerUserID, id).
			Updates(map[string]interface{}{"is_active": true, "updated_at": updatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}))
}

func (a *Catalog) DeleteStrategy(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("strategy_id = ?", id).Delete(new(schema.StrategyVersion)).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(new(schema.Strategy)).Error
	}))
}

func (a *Catalog) BackfillStrategyVersions(ctx context.Context) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		var items []*schema.Strategy
		if err := tx.Where("current_version = ?", 0).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			item.CurrentVersion = 1
			if item.IsActive {
				published := 1
				item.PublishedVersion = &published
			}
			version := strategyVersionSnapshot(item, 1, nil)
			if item.PublishedVersion != nil {
				publishedAt := item.UpdatedAt
				version.PublishedAt = &publishedAt
			}
			if err := tx.Create(version).Error; err != nil {
				return err
			}
			if err := tx.Model(new(schema.Strategy)).Where("id = ?", item.ID).
				Updates(map[string]interface{}{"current_version": 1, "published_version": item.PublishedVersion}).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func strategyVersionSnapshot(item *schema.Strategy, version int, rollbackFrom *int) *schema.StrategyVersion {
	return &schema.StrategyVersion{
		ID: util.NewXID(), StrategyID: item.ID, OwnerUserID: item.OwnerUserID, Version: version,
		Name: item.Name, Description: item.Description, StrategyType: item.StrategyType,
		ConfigJSON: item.ConfigJSON, RollbackFrom: rollbackFrom, CreatedAt: item.UpdatedAt,
	}
}

func (a *Catalog) ClearDefaultStrategies(ctx context.Context, ownerUserID, excludeID string) error {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.Strategy)), ownerUserID).Where("is_default = ?", true)
	if excludeID != "" {
		db = db.Where("id <> ?", excludeID)
	}
	return errors.WithStack(db.Update("is_default", false).Error)
}

func (a *Catalog) GetTrader(ctx context.Context, id, ownerUserID string) (*schema.Trader, error) {
	item := new(schema.Trader)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) CreateTrader(ctx context.Context, item *schema.Trader) error {
	db := util.GetDB(ctx, a.DB)
	simulation := item.IsSimulation
	if err := db.Create(item).Error; err != nil {
		return errors.WithStack(err)
	}
	if !simulation {
		if err := db.Model(new(schema.Trader)).Where("id = ?", item.ID).UpdateColumn("is_simulation", false).Error; err != nil {
			return errors.WithStack(err)
		}
		item.IsSimulation = false
	}
	return nil
}

func (a *Catalog) CreateTraderWithBindings(ctx context.Context, item *schema.Trader, channelIDs []string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		simulation := item.IsSimulation
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if !simulation {
			if err := tx.Model(new(schema.Trader)).Where("id = ?", item.ID).UpdateColumn("is_simulation", false).Error; err != nil {
				return err
			}
			item.IsSimulation = false
		}
		return replaceTraderNotificationBindings(tx, item.ID, channelIDs)
	}))
}

func (a *Catalog) UpdateTrader(ctx context.Context, item *schema.Trader) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error)
}

func (a *Catalog) UpdateTraderWithBindings(ctx context.Context, item *schema.Trader, channelIDs []string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error; err != nil {
			return err
		}
		return replaceTraderNotificationBindings(tx, item.ID, channelIDs)
	}))
}

func (a *Catalog) UpdateTraderFields(ctx context.Context, id string, values map[string]interface{}) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(new(schema.Trader)).Where("id = ?", id).Updates(values).Error)
}

func (a *Catalog) DeleteTrader(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("trader_id = ?", id).Delete(new(schema.TraderNotificationBinding)).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(new(schema.Trader)).Error
	}))
}

func replaceTraderNotificationBindings(tx *gorm.DB, traderID string, channelIDs []string) error {
	if err := tx.Where("trader_id = ?", traderID).Delete(new(schema.TraderNotificationBinding)).Error; err != nil {
		return err
	}
	for _, channelID := range channelIDs {
		if err := tx.Create(&schema.TraderNotificationBinding{TraderID: traderID, ChannelID: channelID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *Catalog) ListTraderNotificationChannelIDs(ctx context.Context, traderID string) ([]string, error) {
	var ids []string
	err := util.GetDB(ctx, a.DB).Model(new(schema.TraderNotificationBinding)).Where("trader_id = ?", traderID).
		Order("id ASC").Pluck("channel_id", &ids).Error
	return ids, errors.WithStack(err)
}

func (a *Catalog) CountEnabledNotificationChannels(ctx context.Context, ownerUserID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := scopedCatalogDB(ctx, a.DB.Model(new(schema.NotificationChannel)), ownerUserID).
		Where("id IN ? AND enabled = ?", ids, true).Count(&count).Error
	return count, errors.WithStack(err)
}

func (a *Catalog) ListEnabledNotificationChannelsForTrader(ctx context.Context, traderID, ownerUserID string) ([]*schema.NotificationChannel, error) {
	channelTable := new(schema.NotificationChannel).TableName()
	bindingTable := new(schema.TraderNotificationBinding).TableName()
	var list []*schema.NotificationChannel
	err := util.GetDB(ctx, a.DB).
		Table(channelTable+" AS channel").
		Select("channel.*").
		Joins("JOIN "+bindingTable+" AS binding ON binding.channel_id = channel.id").
		Where("binding.trader_id = ? AND channel.owner_user_id = ? AND channel.enabled = ?", traderID, ownerUserID, true).
		Order("binding.id ASC").
		Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Catalog) GetNotificationChannel(ctx context.Context, id, ownerUserID string) (*schema.NotificationChannel, error) {
	item := new(schema.NotificationChannel)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) CreateNotificationChannel(ctx context.Context, item *schema.NotificationChannel) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *Catalog) UpdateNotificationChannel(ctx context.Context, item *schema.NotificationChannel) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error)
}

func (a *Catalog) DeleteNotificationChannel(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(new(schema.TraderNotificationBinding)).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(new(schema.TelegramBindingCode)).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(new(schema.TelegramSession)).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(new(schema.NotificationChannel)).Error
	}))
}

func (a *Catalog) ReplaceTelegramBindingCode(ctx context.Context, item *schema.TelegramBindingCode) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(new(schema.TelegramBindingCode)).Where("channel_id = ? AND consumed_at IS NULL", item.ChannelID).
			Update("consumed_at", now).Error; err != nil {
			return err
		}
		return tx.Create(item).Error
	}))
}

func (a *Catalog) GetTelegramSession(ctx context.Context, channelID, ownerUserID string) (*schema.TelegramSession, error) {
	item := new(schema.TelegramSession)
	ok, err := util.FindOne(ctx, util.GetDB(ctx, a.DB).Model(item).
		Where("channel_id = ? AND owner_user_id = ?", channelID, ownerUserID), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) UnbindTelegramSession(ctx context.Context, channelID, ownerUserID string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(new(schema.TelegramSession)).Where("channel_id = ? AND owner_user_id = ?", channelID, ownerUserID).
			Updates(map[string]interface{}{"status": schema.TelegramSessionUnbound, "unbound_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Table(new(schema.NotificationChannel).TableName()).Where("id = ? AND owner_user_id = ?", channelID, ownerUserID).
			Updates(map[string]interface{}{"chat_id": "", "updated_at": now}).Error
	}))
}

func (a *Catalog) QueryNotificationChannels(ctx context.Context, params schema.NotificationChannelQueryParam) (*schema.NotificationChannelQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.NotificationChannel)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Keyword); value != "" {
		db = db.Where("name LIKE ? OR chat_id LIKE ?", "%"+value+"%", "%"+value+"%")
	}
	if value := strings.TrimSpace(params.ChannelType); value != "" {
		db = db.Where("channel_type = ?", value)
	}
	if params.Enabled != nil {
		db = db.Where("enabled = ?", *params.Enabled)
	}
	var list []*schema.NotificationChannel
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.NotificationChannelQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) CountTradersByReference(ctx context.Context, field, id string) (int64, error) {
	if field != "exchange_id" && field != "ai_model_id" && field != "strategy_id" {
		return 0, errors.BadRequest("trading_reference_invalid", "Invalid trader reference field")
	}
	var count int64
	err := util.GetDB(ctx, a.DB).Model(new(schema.Trader)).Where(field+" = ?", id).Count(&count).Error
	return count, errors.WithStack(err)
}

func (a *Catalog) QueryExchanges(ctx context.Context, params schema.ExchangeQueryParam) (*schema.ExchangeQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.Exchange)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Keyword); value != "" {
		like := "%" + value + "%"
		db = db.Where("name LIKE ? OR account_name LIKE ?", like, like)
	}
	if value := strings.TrimSpace(params.ExchangeType); value != "" {
		db = db.Where("exchange_type = ?", value)
	}
	if params.Enabled != nil {
		db = db.Where("enabled = ?", *params.Enabled)
	}
	var list []*schema.Exchange
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.ExchangeQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) QueryAIModels(ctx context.Context, params schema.AIModelQueryParam) (*schema.AIModelQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.AIModel)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Keyword); value != "" {
		db = db.Where("name LIKE ? OR custom_model_name LIKE ?", "%"+value+"%", "%"+value+"%")
	}
	if value := strings.TrimSpace(params.Provider); value != "" {
		db = db.Where("provider = ?", value)
	}
	if params.Enabled != nil {
		db = db.Where("enabled = ?", *params.Enabled)
	}
	var list []*schema.AIModel
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.AIModelQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) QueryStrategies(ctx context.Context, params schema.StrategyQueryParam) (*schema.StrategyQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.Strategy)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Keyword); value != "" {
		like := "%" + value + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if value := strings.TrimSpace(params.StrategyType); value != "" {
		db = db.Where("strategy_type = ?", value)
	}
	if params.Active != nil {
		db = db.Where("is_active = ?", *params.Active)
	}
	var list []*schema.Strategy
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.StrategyQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) QueryTraders(ctx context.Context, params schema.TraderQueryParam) (*schema.TraderQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.Trader)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Keyword); value != "" {
		db = db.Where("name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(params.RuntimeState); value != "" {
		db = db.Where("runtime_state = ?", value)
	}
	var list []*schema.Trader
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.TraderQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) GetScheduledTask(ctx context.Context, id, ownerUserID string) (*schema.ScheduledTask, error) {
	item := new(schema.ScheduledTask)
	ok, err := util.FindOne(ctx, scopedCatalogDB(ctx, a.DB.Model(item), ownerUserID).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (a *Catalog) ScheduledTaskNameExists(ctx context.Context, ownerUserID, name, excludeID string) (bool, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.ScheduledTask)), ownerUserID).Where("name = ?", name)
	if excludeID != "" {
		db = db.Where("id <> ?", excludeID)
	}
	ok, err := util.Exists(ctx, db)
	return ok, errors.WithStack(err)
}

func (a *Catalog) CreateScheduledTask(ctx context.Context, item *schema.ScheduledTask) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *Catalog) UpdateScheduledTask(ctx context.Context, item *schema.ScheduledTask) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Model(item).Where("id = ?", item.ID).Select("*").Omit("created_at").Updates(item).Error)
}

func (a *Catalog) DeleteScheduledTask(ctx context.Context, id string) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Where("id = ?", id).Delete(new(schema.ScheduledTask)).Error)
}

func (a *Catalog) QueryScheduledTasks(ctx context.Context, params schema.ScheduledTaskQueryParam) (*schema.ScheduledTaskQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.ScheduledTask)), params.OwnerUserID)
	if value := strings.TrimSpace(params.Status); value != "" {
		db = db.Where("status = ?", value)
	}
	if value := strings.TrimSpace(params.Handler); value != "" {
		db = db.Where("handler = ?", value)
	}
	var list []*schema.ScheduledTask
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam, catalogQueryOptions(), &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.ScheduledTaskQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) QueryScheduledTaskRuns(ctx context.Context, params schema.ScheduledTaskRunQueryParam) (*schema.ScheduledTaskRunQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.ScheduledTaskRun)), params.OwnerUserID)
	if value := strings.TrimSpace(params.TaskID); value != "" {
		db = db.Where("task_id = ?", value)
	}
	if value := strings.TrimSpace(params.Status); value != "" {
		db = db.Where("status = ?", value)
	}
	var list []*schema.ScheduledTaskRun
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "scheduled_at", Direction: util.DESC},
			{Field: "created_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.ScheduledTaskRunQueryResult{Data: list, PageResult: page}, nil
}

func (a *Catalog) CreateScheduledTaskAudit(ctx context.Context, item *schema.ScheduledTaskAudit) error {
	return errors.WithStack(util.GetDB(ctx, a.DB).Create(item).Error)
}

func (a *Catalog) QueryScheduledTaskAudits(ctx context.Context, params schema.ScheduledTaskAuditQueryParam) (*schema.ScheduledTaskAuditQueryResult, error) {
	db := scopedCatalogDB(ctx, a.DB.Model(new(schema.ScheduledTaskAudit)), params.OwnerUserID)
	if value := strings.TrimSpace(params.TaskID); value != "" {
		db = db.Where("task_id = ?", value)
	}
	if value := strings.TrimSpace(params.Action); value != "" {
		db = db.Where("action = ?", value)
	}
	var list []*schema.ScheduledTaskAudit
	page, err := util.WrapPageQuery(ctx, db, params.PaginationParam,
		util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.ScheduledTaskAuditQueryResult{Data: list, PageResult: page}, nil
}

func scopedCatalogDB(ctx context.Context, db *gorm.DB, ownerUserID string) *gorm.DB {
	model := db.Statement.Model
	db = util.GetDB(ctx, db)
	if model != nil {
		db = db.Model(model)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	return db
}

func catalogQueryOptions() util.QueryOptions {
	return util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}
}
