package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

var ErrTradingProviderDisabled = errors.New("external trading provider is disabled by configuration")

type InstrumentRules struct {
	Symbol      string `json:"symbol"`
	MinQuantity string `json:"min_quantity"`
	StepSize    string `json:"step_size"`
	TickSize    string `json:"tick_size"`
	MinNotional string `json:"min_notional"`
}

type TradingOrderRequest struct {
	ClientOrderID string
	Market        string
	Symbol        string
	Side          string
	PositionSide  string
	OrderType     string
	TimeInForce   string
	Quantity      string
	QuoteQuantity string
	Price         string
	StopLoss      string
	TakeProfit    string
	ReduceOnly    bool
}

type TradingOrderResult struct {
	ExchangeOrderID  string
	ClientOrderID    string
	Status           string
	ExecutedQuantity string
	AveragePrice     string
}

type CancelOrderRequest struct {
	Market          string
	Symbol          string
	ExchangeOrderID string
	ClientOrderID   string
}

// Trading is deliberately separate from ReadOnly. Construction is protected
// by process-wide and testnet-only gates.
type Trading interface {
	Name() string
	PlaceOrder(context.Context, TradingOrderRequest) (*TradingOrderResult, error)
	CancelOrder(context.Context, CancelOrderRequest) (*TradingOrderResult, error)
}

func (a *Factory) BuildTrading(exchange *schema.Exchange) (Trading, error) {
	if !config.C.Trading.Execution.Enabled {
		return nil, ErrTradingProviderDisabled
	}
	if exchange == nil {
		return nil, errors.New("exchange configuration is required")
	}
	if config.C.Trading.Execution.TestnetOnly && !exchange.Testnet {
		return nil, fmt.Errorf("trading provider is restricted to testnet")
	}
	if strings.EqualFold(exchange.AccountType, BinanceAccountTypeUnified) {
		return nil, fmt.Errorf("trading is not implemented for unified accounts")
	}
	readOnly, err := a.Build(exchange)
	if err != nil {
		return nil, err
	}
	switch item := readOnly.(type) {
	case *Binance:
		return item, nil
	case *OKX:
		return item, nil
	default:
		return nil, ErrTradingProviderDisabled
	}
}
