package schema

import (
	"strings"

	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

const (
	MarketExchangeBinance = "binance"
	MarketExchangeOKX     = "okx"
	MarketTypeSpot        = "spot"
	MarketTypeFutures     = "futures"
)

var supportedMarketIntervals = map[string]struct{}{
	"1m": {}, "5m": {}, "15m": {}, "1h": {}, "4h": {}, "1d": {}, "1w": {},
}

type MarketInstrumentsQuery struct {
	Exchange string `form:"exchange"`
	Market   string `form:"market"`
}

func (a *MarketInstrumentsQuery) Validate() error {
	a.Exchange = strings.ToLower(strings.TrimSpace(a.Exchange))
	a.Market = strings.ToLower(strings.TrimSpace(a.Market))
	return validateMarketScope(a.Exchange, a.Market)
}

type MarketSymbolQuery struct {
	Exchange string `form:"exchange"`
	Market   string `form:"market"`
	Symbol   string `form:"symbol"`
}

func (a *MarketSymbolQuery) Validate() error {
	a.Exchange = strings.ToLower(strings.TrimSpace(a.Exchange))
	a.Market = strings.ToLower(strings.TrimSpace(a.Market))
	a.Symbol = strings.ToUpper(strings.TrimSpace(a.Symbol))
	if err := validateMarketScope(a.Exchange, a.Market); err != nil {
		return err
	}
	if a.Symbol == "" || len(a.Symbol) > 40 {
		return errors.BadRequest("market_symbol_invalid", "Market symbol is required")
	}
	return nil
}

type MarketKlinesQuery struct {
	MarketSymbolQuery
	Interval string `form:"interval"`
	Limit    int    `form:"limit"`
}

type MarketContextQuery struct {
	MarketSymbolQuery
	Depth int `form:"depth"`
}

func (a *MarketContextQuery) Validate() error {
	if err := a.MarketSymbolQuery.Validate(); err != nil {
		return err
	}
	if a.Depth == 0 {
		a.Depth = 20
	}
	if a.Depth < 5 || a.Depth > 100 {
		return errors.BadRequest("market_depth_invalid", "Order book depth must be between 5 and 100")
	}
	return nil
}

func (a *MarketKlinesQuery) Validate() error {
	if err := a.MarketSymbolQuery.Validate(); err != nil {
		return err
	}
	a.Interval = strings.ToLower(strings.TrimSpace(a.Interval))
	if _, ok := supportedMarketIntervals[a.Interval]; !ok {
		return errors.BadRequest("market_interval_invalid", "Unsupported market interval")
	}
	if a.Limit == 0 {
		a.Limit = 300
	}
	if a.Limit < 1 || a.Limit > 300 {
		return errors.BadRequest("market_limit_invalid", "Kline limit must be between 1 and 300")
	}
	return nil
}

func validateMarketScope(exchange, market string) error {
	if exchange != MarketExchangeBinance && exchange != MarketExchangeOKX {
		return errors.BadRequest("market_exchange_invalid", "Unsupported market exchange")
	}
	if market != MarketTypeSpot && market != MarketTypeFutures {
		return errors.BadRequest("market_type_invalid", "Unsupported market type")
	}
	return nil
}

type MarketInstrument struct {
	Exchange   string `json:"exchange"`
	Market     string `json:"market"`
	Symbol     string `json:"symbol"`
	BaseAsset  string `json:"base_asset"`
	QuoteAsset string `json:"quote_asset"`
	Display    string `json:"display"`
}

type MarketTicker struct {
	Exchange        string `json:"exchange"`
	Market          string `json:"market"`
	Symbol          string `json:"symbol"`
	LastPrice       string `json:"last_price"`
	OpenPrice       string `json:"open_price"`
	HighPrice       string `json:"high_price"`
	LowPrice        string `json:"low_price"`
	PriceChange     string `json:"price_change"`
	PriceChangeRate string `json:"price_change_rate"`
	BaseVolume      string `json:"base_volume"`
	QuoteVolume     string `json:"quote_volume"`
	Timestamp       int64  `json:"timestamp"`
}

type MarketKline struct {
	Time        int64  `json:"time"`
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	Volume      string `json:"volume"`
	QuoteVolume string `json:"quote_volume"`
	Closed      bool   `json:"closed"`
}

type MarketDepthLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

type MarketOrderBook struct {
	Bids      []MarketDepthLevel `json:"bids"`
	Asks      []MarketDepthLevel `json:"asks"`
	SpreadBPS float64            `json:"spread_bps"`
	Imbalance float64            `json:"imbalance"`
	Timestamp int64              `json:"timestamp"`
}

type MarketDerivatives struct {
	OpenInterest      string  `json:"open_interest"`
	OpenInterestValue string  `json:"open_interest_value_usdt"`
	FundingRate       string  `json:"funding_rate"`
	NextFundingTime   int64   `json:"next_funding_time"`
	MarkPrice         string  `json:"mark_price"`
	TakerBuyVolume    string  `json:"taker_buy_volume"`
	TakerSellVolume   string  `json:"taker_sell_volume"`
	TakerBuySellRatio float64 `json:"taker_buy_sell_ratio"`
	FlowSource        string  `json:"flow_source"`
	FlowIsProxy       bool    `json:"flow_is_proxy"`
}

type MarketRiskRadar struct {
	SpreadBPS      float64 `json:"spread_bps"`
	BookImbalance  float64 `json:"book_imbalance"`
	FundingRatePct float64 `json:"funding_rate_pct"`
	LiquidityRisk  string  `json:"liquidity_risk"`
	CrowdingRisk   string  `json:"crowding_risk"`
	RiskScore      int     `json:"risk_score"`
}

type MarketContext struct {
	Exchange    string             `json:"exchange"`
	Market      string             `json:"market"`
	Symbol      string             `json:"symbol"`
	Ticker      *MarketTicker      `json:"ticker"`
	OrderBook   *MarketOrderBook   `json:"order_book"`
	Derivatives *MarketDerivatives `json:"derivatives,omitempty"`
	RiskRadar   *MarketRiskRadar   `json:"risk_radar"`
	GeneratedAt int64              `json:"generated_at"`
}

type ProviderCapability struct {
	Exchange         string   `json:"exchange"`
	Kind             string   `json:"kind"`
	MigrationStatus  string   `json:"migration_status"`
	AccountTypes     []string `json:"account_types"`
	Snapshot         bool     `json:"snapshot"`
	History          bool     `json:"history"`
	UserStream       bool     `json:"user_stream"`
	PublicMarketData bool     `json:"public_market_data"`
	Trading          bool     `json:"trading"`
	TradingGate      string   `json:"trading_gate"`
	CredentialModel  string   `json:"credential_model"`
	Notes            []string `json:"notes"`
}

type MarketDataSourceCapability struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Cost          string   `json:"cost"`
	Authorization string   `json:"authorization"`
	Features      []string `json:"features"`
	Limitations   []string `json:"limitations"`
}

type ProviderCapabilityMatrix struct {
	Providers   []ProviderCapability         `json:"providers"`
	DataSources []MarketDataSourceCapability `json:"data_sources"`
}
