package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

var (
	ErrUnsupportedExchange    = errors.New("unsupported read-only exchange provider")
	ErrCredentialsIncomplete  = errors.New("read-only exchange credentials are incomplete")
	ErrAccountTypeUnsupported = errors.New("exchange account type is unsupported")
	ErrTestnetUnsupported     = errors.New("exchange account type does not support testnet")
	ErrHistoryUnsupported     = errors.New("exchange account history is unsupported")
)

type Account struct {
	TotalEquity      string
	Balance          string
	AvailableBalance string
	UnrealizedPnL    string
	MarginUsedPct    string
	Assets           []*Asset
}

type Asset struct {
	Asset         string
	Total         string
	Available     string
	Locked        string
	Borrowed      string
	Interest      string
	PriceUSDT     string
	ValueUSDT     string
	UnrealizedPnL string
}

type Position struct {
	ExchangePositionID string
	Symbol             string
	Side               string
	Quantity           string
	EntryPrice         string
	MarkPrice          string
	LiquidationPrice   string
	UnrealizedPnL      string
	Leverage           int
}

type HistoryRequest struct {
	Since   time.Time
	Until   time.Time
	Symbols []string
}

type Order struct {
	Market            string
	ExchangeOrderID   string
	ClientOrderID     string
	Symbol            string
	Side              string
	PositionSide      string
	OrderType         string
	Status            string
	Price             string
	AveragePrice      string
	Quantity          string
	ExecutedQuantity  string
	QuoteQuantity     string
	ReduceOnly        bool
	OrderedAt         time.Time
	ExchangeUpdatedAt time.Time
}

type Fill struct {
	Market          string
	ExchangeTradeID string
	ExchangeOrderID string
	Symbol          string
	Side            string
	PositionSide    string
	Price           string
	Quantity        string
	QuoteQuantity   string
	FeeAsset        string
	FeeQuantity     string
	FeeValueUSDT    string
	RealizedPnL     string
	Liquidity       string
	FilledAt        time.Time
}

type IncomeEvent struct {
	Market          string
	EventType       string
	ExchangeEventID string
	Symbol          string
	Asset           string
	Amount          string
	PriceUSDT       string
	ValueUSDT       string
	OccurredAt      time.Time
}

type HistoryBatch struct {
	Orders       []*Order
	Fills        []*Fill
	IncomeEvents []*IncomeEvent
	SyncedUntil  time.Time
}

// ReadOnly exposes observation methods only. Trading commands intentionally do not exist here.
type ReadOnly interface {
	Name() string
	Snapshot(context.Context) (*Account, []*Position, error)
	Health(context.Context) error
}

// CredentialValidator verifies signed, read-only account access without loading a full snapshot.
type CredentialValidator interface {
	ValidateCredentials(context.Context) error
}

// HistoricalReadOnly exposes incremental account history without any trading commands.
type HistoricalReadOnly interface {
	History(context.Context, HistoryRequest) (*HistoryBatch, error)
}

type Factory struct {
	Credentials *credential.Service
	HTTPClient  *http.Client
}

func NewFactory(credentials *credential.Service) *Factory {
	return &Factory{Credentials: credentials, HTTPClient: &http.Client{Timeout: 20 * time.Second}}
}

func (a *Factory) Build(exchange *schema.Exchange) (ReadOnly, error) {
	if exchange == nil {
		return nil, errors.New("exchange configuration is required")
	}
	exchangeType := strings.ToLower(strings.TrimSpace(exchange.ExchangeType))
	if exchangeType != "binance" && exchangeType != "bybit" && exchangeType != "okx" {
		return nil, ErrUnsupportedExchange
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	apiKey, err := a.decrypt(exchange.APIKeyCiphertext, exchange, "api_key")
	if err != nil {
		return nil, err
	}
	secretKey, err := a.decrypt(exchange.SecretKeyCiphertext, exchange, "secret_key")
	if err != nil {
		return nil, err
	}
	credentials := Credentials{
		APIKey:      apiKey,
		SecretKey:   secretKey,
		Testnet:     exchange.Testnet,
		AccountType: strings.ToLower(strings.TrimSpace(exchange.AccountType)),
	}
	if credentials.APIKey == "" || credentials.SecretKey == "" {
		return nil, ErrCredentialsIncomplete
	}
	switch exchangeType {
	case "binance":
		if !isBinanceAccountTypeSupported(credentials.AccountType) {
			return nil, fmt.Errorf("%w: binance %q", ErrAccountTypeUnsupported, credentials.AccountType)
		}
		if credentials.AccountType == BinanceAccountTypeUnified && credentials.Testnet {
			return nil, fmt.Errorf("%w: Binance Portfolio Margin", ErrTestnetUnsupported)
		}
		return NewBinance(credentials, client), nil
	case "bybit":
		return NewBybit(credentials, client), nil
	case "okx":
		passphrase, decryptErr := a.decrypt(exchange.PassphraseCiphertext, exchange, "passphrase")
		if decryptErr != nil {
			return nil, decryptErr
		}
		if passphrase == "" {
			return nil, ErrCredentialsIncomplete
		}
		credentials.Passphrase = passphrase
		return NewOKX(credentials, client), nil
	default:
		return nil, ErrUnsupportedExchange
	}
}

func (a *Factory) decrypt(value string, exchange *schema.Exchange, field string) (string, error) {
	if value == "" {
		return "", nil
	}
	if a.Credentials == nil {
		return "", credential.ErrKeyNotConfigured
	}
	return a.Credentials.DecryptField(value, exchange.OwnerUserID, exchange.ID, field)
}

type Credentials struct {
	APIKey      string
	SecretKey   string
	Passphrase  string
	Testnet     bool
	AccountType string
}
