package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/gorm"
)

type tradeAccumulator struct {
	ownerUserID       string
	traderID          string
	strategyID        string
	strategyVersion   int
	attributionSource string
	exchangeID        string
	market            string
	symbol            string
	side              string
	openedAt          time.Time
	openQty           *big.Rat
	openValue         *big.Rat
	entryQty          *big.Rat
	entryValue        *big.Rat
	exitQty           *big.Rat
	exitValue         *big.Rat
	providerPnL       *big.Rat
	fee               *big.Rat
	fillIDs           []string
}

func newTradeAccumulator(fill *schema.Fill, side string) *tradeAccumulator {
	return &tradeAccumulator{
		ownerUserID: fill.OwnerUserID, traderID: fill.TraderID, exchangeID: fill.ExchangeID,
		strategyID: fill.StrategyID, strategyVersion: fill.StrategyVersion,
		attributionSource: fallbackAttribution(fill.AttributionSource),
		market:            strings.ToLower(fill.Market), symbol: strings.ToUpper(fill.Symbol), side: side,
		openQty: new(big.Rat), openValue: new(big.Rat), entryQty: new(big.Rat),
		entryValue: new(big.Rat), exitQty: new(big.Rat), exitValue: new(big.Rat),
		providerPnL: new(big.Rat), fee: new(big.Rat),
	}
}

// RebuildClosedTrades rebuilds deterministic round trips from immutable fills.
// It is safe to run after every incremental history synchronization.
func (a *Registry) RebuildClosedTrades(ctx context.Context, ownerUserID, exchangeID string) error {
	var fills []*schema.Fill
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND exchange_id = ?", ownerUserID, exchangeID).
		Order("filled_at ASC, id ASC").Find(&fills).Error; err != nil {
		return err
	}
	if err := a.applyCommandAttribution(ctx, ownerUserID, exchangeID, fills); err != nil {
		return err
	}
	trades, err := buildClosedTrades(fills, time.Now().UTC())
	if err != nil {
		return err
	}
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_user_id = ? AND exchange_id = ?", ownerUserID, exchangeID).
			Delete(new(schema.ClosedTrade)).Error; err != nil {
			return err
		}
		if len(trades) == 0 {
			return nil
		}
		return tx.CreateInBatches(trades, 200).Error
	})
}

func (a *Registry) applyCommandAttribution(ctx context.Context, ownerUserID, exchangeID string, fills []*schema.Fill) error {
	var commands []*schema.TradeCommand
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND exchange_id = ? AND exchange_order_id <> ?",
		ownerUserID, exchangeID, "").Find(&commands).Error; err != nil {
		return err
	}
	byOrderID := make(map[string]*schema.TradeCommand, len(commands))
	for _, command := range commands {
		byOrderID[command.ExchangeOrderID] = command
	}
	return a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, fill := range fills {
			command := byOrderID[fill.ExchangeOrderID]
			if command == nil {
				if fill.AttributionSource == "" {
					fill.AttributionSource = "external"
				}
				continue
			}
			strategyID := ""
			if command.StrategyID != nil {
				strategyID = *command.StrategyID
			}
			if fill.TraderID == command.TraderID && fill.StrategyID == strategyID &&
				fill.StrategyVersion == command.StrategyVersion && fill.AttributionSource == "system" {
				continue
			}
			fill.TraderID, fill.StrategyID, fill.StrategyVersion, fill.AttributionSource =
				command.TraderID, strategyID, command.StrategyVersion, "system"
			if err := tx.Model(new(schema.Fill)).Where("id = ? AND owner_user_id = ?", fill.ID, ownerUserID).Updates(map[string]interface{}{
				"trader_id": fill.TraderID, "strategy_id": fill.StrategyID, "strategy_version": fill.StrategyVersion,
				"attribution_source": fill.AttributionSource, "updated_at": time.Now().UTC(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func buildClosedTrades(fills []*schema.Fill, rebuiltAt time.Time) ([]*schema.ClosedTrade, error) {
	states := make(map[string]*tradeAccumulator)
	trades := make([]*schema.ClosedTrade, 0)
	for _, fill := range fills {
		qty, err := decimalRat(fill.Quantity)
		if err != nil || qty.Sign() <= 0 {
			continue
		}
		price, err := decimalRat(fill.Price)
		if err != nil || price.Sign() < 0 {
			continue
		}
		fee := decimalRatOrZero(fill.FeeValueUSDT)
		providerPnL := decimalRatOrZero(fill.RealizedPnL)
		positionSide := strings.ToLower(strings.TrimSpace(fill.PositionSide))
		key := strings.Join([]string{fill.ExchangeID, strings.ToLower(fill.Market), strings.ToUpper(fill.Symbol), positionSide}, "|")
		state := states[key]
		fillSide := strings.ToLower(strings.TrimSpace(fill.Side))

		explicitDirection := ""
		openingSide := ""
		switch positionSide {
		case "long":
			explicitDirection, openingSide = "long", "buy"
		case "short":
			explicitDirection, openingSide = "short", "sell"
		}
		if explicitDirection != "" {
			if state == nil {
				state = newTradeAccumulator(fill, explicitDirection)
				states[key] = state
			}
			if fillSide == openingSide {
				state.addOpen(fill, qty, price, fee)
				continue
			}
			if state.openQty.Sign() == 0 {
				continue
			}
			closed := state.addClose(fill, qty, price, fee, providerPnL, rebuiltAt)
			if closed != nil {
				trades = append(trades, closed)
				delete(states, key)
			}
			continue
		}

		incomingDirection := "long"
		if fillSide == "sell" {
			incomingDirection = "short"
		}
		if state == nil || state.openQty.Sign() == 0 {
			// Spot history can begin with a sell whose corresponding buy predates
			// the retained data. Do not manufacture a synthetic short position.
			if strings.EqualFold(fill.Market, "spot") && fillSide == "sell" {
				continue
			}
			state = newTradeAccumulator(fill, incomingDirection)
			states[key] = state
			state.addOpen(fill, qty, price, fee)
			continue
		}
		if state.side == incomingDirection {
			state.addOpen(fill, qty, price, fee)
			continue
		}

		closeQty := minRat(qty, state.openQty)
		closeRatio := new(big.Rat).Quo(closeQty, qty)
		closed := state.addClose(fill, closeQty, price, mulRat(fee, closeRatio), providerPnL, rebuiltAt)
		if closed != nil {
			trades = append(trades, closed)
			delete(states, key)
		}
		remaining := new(big.Rat).Sub(qty, closeQty)
		if remaining.Sign() > 0 && !strings.EqualFold(fill.Market, "spot") {
			remainingRatio := new(big.Rat).Quo(remaining, qty)
			next := newTradeAccumulator(fill, incomingDirection)
			next.addOpen(fill, remaining, price, mulRat(fee, remainingRatio))
			states[key] = next
		}
	}
	sort.Slice(trades, func(i, j int) bool { return trades[i].ClosedAt.Before(trades[j].ClosedAt) })
	return trades, nil
}

func (a *tradeAccumulator) addOpen(fill *schema.Fill, qty, price, fee *big.Rat) {
	if a.openQty.Sign() == 0 {
		a.openedAt = fill.FilledAt.UTC()
	}
	value := mulRat(qty, price)
	a.openQty.Add(a.openQty, qty)
	a.openValue.Add(a.openValue, value)
	a.entryQty.Add(a.entryQty, qty)
	a.entryValue.Add(a.entryValue, value)
	a.fee.Add(a.fee, fee)
	a.fillIDs = append(a.fillIDs, fill.ID)
}

func (a *tradeAccumulator) addClose(fill *schema.Fill, requestedQty, price, fee, providerPnL *big.Rat, rebuiltAt time.Time) *schema.ClosedTrade {
	qty := minRat(requestedQty, a.openQty)
	if qty.Sign() <= 0 {
		return nil
	}
	averageEntry := new(big.Rat).Quo(a.openValue, a.openQty)
	a.openQty.Sub(a.openQty, qty)
	a.openValue.Sub(a.openValue, mulRat(averageEntry, qty))
	a.exitQty.Add(a.exitQty, qty)
	a.exitValue.Add(a.exitValue, mulRat(qty, price))
	a.providerPnL.Add(a.providerPnL, providerPnL)
	a.fee.Add(a.fee, fee)
	a.fillIDs = append(a.fillIDs, fill.ID)
	if a.openQty.Sign() != 0 {
		return nil
	}
	entryPrice := new(big.Rat).Quo(a.entryValue, a.entryQty)
	exitPrice := new(big.Rat).Quo(a.exitValue, a.exitQty)
	gross := new(big.Rat).Set(a.providerPnL)
	if gross.Sign() == 0 {
		gross = new(big.Rat).Sub(exitPrice, entryPrice)
		gross.Mul(gross, a.exitQty)
		if a.side == "short" {
			gross.Neg(gross)
		}
	}
	net := new(big.Rat).Sub(gross, a.fee)
	closedAt := fill.FilledAt.UTC()
	return &schema.ClosedTrade{
		ID: tradeID(a, closedAt), OwnerUserID: a.ownerUserID, TraderID: a.traderID,
		StrategyID: a.strategyID, StrategyVersion: a.strategyVersion, AttributionSource: a.attributionSource,
		ExchangeID: a.exchangeID, Market: a.market, Symbol: a.symbol, Side: a.side,
		EntryQuantity: ratString(a.entryQty), ExitQuantity: ratString(a.exitQty),
		EntryPrice: ratString(entryPrice), ExitPrice: ratString(exitPrice), GrossPnL: ratString(gross),
		Fee: ratString(a.fee), Funding: "0", NetPnL: ratString(net), OpenedAt: a.openedAt,
		ClosedAt: closedAt, DurationSeconds: maxInt64(0, int64(closedAt.Sub(a.openedAt).Seconds())),
		SourceFillCount: len(a.fillIDs), RebuiltAt: rebuiltAt, CreatedAt: rebuiltAt, UpdatedAt: rebuiltAt,
	}
}

func fallbackAttribution(value string) string {
	if strings.TrimSpace(value) == "" {
		return "external"
	}
	return value
}

func decimalRat(value string) (*big.Rat, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return new(big.Rat), nil
	}
	result, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	return result, nil
}

func decimalRatOrZero(value string) *big.Rat {
	result, err := decimalRat(value)
	if err != nil {
		return new(big.Rat)
	}
	return result
}

func ratString(value *big.Rat) string {
	if value == nil || value.Sign() == 0 {
		return "0"
	}
	result := value.FloatString(18)
	result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	if result == "-0" {
		return "0"
	}
	return result
}

func minRat(left, right *big.Rat) *big.Rat {
	if left.Cmp(right) <= 0 {
		return new(big.Rat).Set(left)
	}
	return new(big.Rat).Set(right)
}

func mulRat(left, right *big.Rat) *big.Rat {
	if left == nil || right == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Mul(left, right)
}

func tradeID(a *tradeAccumulator, closedAt time.Time) string {
	source := strings.Join([]string{a.exchangeID, a.market, a.symbol, a.side, a.openedAt.Format(time.RFC3339Nano),
		closedAt.Format(time.RFC3339Nano), strings.Join(a.fillIDs, ",")}, "|")
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
