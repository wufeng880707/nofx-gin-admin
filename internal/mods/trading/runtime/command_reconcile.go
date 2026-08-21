package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/gorm"
)

// ReconcileTradeCommands advances local command state from read-only order
// history. It never calls a trading endpoint and is safe to repeat.
func (a *Registry) ReconcileTradeCommands(ctx context.Context, ownerUserID, exchangeID string) error {
	var commands []*schema.TradeCommand
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND exchange_id = ? AND status IN ?",
		ownerUserID, exchangeID, []string{schema.TradeCommandStatusSubmitted, schema.TradeCommandStatusAccepted,
			schema.TradeCommandStatusPartial}).Find(&commands).Error; err != nil {
		return err
	}
	for _, command := range commands {
		order, err := a.findCommandOrder(ctx, command)
		if err != nil {
			return err
		}
		if order == nil {
			continue
		}
		status := reconciledCommandStatus(order.Status)
		if status == "" {
			continue
		}
		executed, average := zero(order.ExecutedQuantity), zero(order.AveragePrice)
		if command.Status == status && command.ExecutedQuantity == executed && command.AveragePrice == average &&
			command.ProviderStatus == order.Status {
			continue
		}
		now := time.Now().UTC()
		detail, _ := json.Marshal(map[string]interface{}{"exchange_order_id": order.ExchangeOrderID,
			"provider_status": order.Status, "executed_quantity": executed, "average_price": average})
		if err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			values := map[string]interface{}{"status": status, "provider_status": order.Status,
				"executed_quantity": executed, "average_price": average, "updated_at": now}
			if reconciledStatusTerminal(status) {
				values["finished_at"] = now
			}
			result := tx.Model(new(schema.TradeCommand)).Where("id = ? AND owner_user_id = ? AND status = ?",
				command.ID, ownerUserID, command.Status).Updates(values)
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			return tx.Create(&schema.TradeCommandEvent{CommandID: command.ID, OwnerUserID: ownerUserID,
				ActorUserID: "system", EventType: "exchange_reconciled", FromStatus: command.Status,
				ToStatus: status, DetailJSON: string(detail), CreatedAt: now}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Registry) findCommandOrder(ctx context.Context, command *schema.TradeCommand) (*schema.ExchangeOrder, error) {
	if command.ExchangeOrderID == "" && command.ExchangeClientOrderID == "" {
		return nil, nil
	}
	db := a.DB.WithContext(ctx).Where("owner_user_id = ? AND exchange_id = ? AND market = ?",
		command.OwnerUserID, command.ExchangeID, command.Market)
	switch {
	case command.ExchangeOrderID != "" && command.ExchangeClientOrderID != "":
		db = db.Where("(exchange_order_id = ? OR client_order_id = ?)", command.ExchangeOrderID, command.ExchangeClientOrderID)
	case command.ExchangeOrderID != "":
		db = db.Where("exchange_order_id = ?", command.ExchangeOrderID)
	default:
		db = db.Where("client_order_id = ?", command.ExchangeClientOrderID)
	}
	var order schema.ExchangeOrder
	if err := db.Order("exchange_updated_at DESC").First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func reconciledCommandStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "new", "open", "live", "pending":
		return schema.TradeCommandStatusAccepted
	case "partially_filled", "partiallyfilled", "partial-fill", "partial_fill":
		return schema.TradeCommandStatusPartial
	case "filled", "closed":
		return schema.TradeCommandStatusFilled
	case "cancelled", "canceled", "expired":
		return schema.TradeCommandStatusCancelled
	case "rejected", "failed":
		return schema.TradeCommandStatusRejected
	default:
		return ""
	}
}

func reconciledStatusTerminal(value string) bool {
	return value == schema.TradeCommandStatusFilled || value == schema.TradeCommandStatusCancelled ||
		value == schema.TradeCommandStatusRejected
}
