package biz

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

func (a *Catalog) QueryMarketInstruments(ctx context.Context, params *schema.MarketInstrumentsQuery) ([]*schema.MarketInstrument, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	result, err := a.ProviderFactory.MarketData().Instruments(ctx, params.Exchange, params.Market)
	if err != nil {
		return nil, errors.InternalServerError("market_instruments_failed", "Unable to load market instruments: %v", err)
	}
	return result, nil
}

func (a *Catalog) GetLiquidationHeatmap(ctx context.Context, params *schema.LiquidationHeatmapQuery) (*schema.LiquidationHeatmap, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	to := time.Now().UTC()
	from := to.Add(-time.Duration(params.Hours) * time.Hour)
	var events []*schema.MarketLiquidationEvent
	if err := a.CatalogDAL.DB.WithContext(ctx).
		Where("exchange = ? AND market = ? AND symbol = ? AND event_at >= ? AND event_at <= ?", "binance", "futures", params.Symbol, from, to).
		Order("event_at ASC").Limit(10000).Find(&events).Error; err != nil {
		return nil, errors.InternalServerError("market_liquidation_query_failed", "Unable to load liquidation events: %v", err)
	}
	return buildLiquidationHeatmap(params, events, from, to), nil
}

func buildLiquidationHeatmap(params *schema.LiquidationHeatmapQuery, events []*schema.MarketLiquidationEvent, from, to time.Time) *schema.LiquidationHeatmap {
	result := &schema.LiquidationHeatmap{
		Exchange: "binance", Market: "futures", Symbol: params.Symbol, From: from, To: to,
		TimeBuckets: params.TimeBuckets, PriceBuckets: params.PriceBuckets, Cells: []schema.LiquidationHeatmapCell{},
		Source:         "binance_force_order_ws",
		CoverageNotice: "仅统计本服务启用公共清算采集器后收到的 Binance USD-M 实时强平快照，不包含启用前历史数据。",
	}
	type parsedEvent struct {
		item     *schema.MarketLiquidationEvent
		price    float64
		notional float64
	}
	parsed := make([]parsedEvent, 0, len(events))
	for _, item := range events {
		price, priceErr := strconv.ParseFloat(item.Price, 64)
		notional, notionalErr := strconv.ParseFloat(item.NotionalUSDT, 64)
		if priceErr != nil || notionalErr != nil || price <= 0 || notional <= 0 || item.EventAt.Before(from) || item.EventAt.After(to) {
			continue
		}
		if len(parsed) == 0 || price < result.MinPrice {
			result.MinPrice = price
		}
		if len(parsed) == 0 || price > result.MaxPrice {
			result.MaxPrice = price
		}
		parsed = append(parsed, parsedEvent{item: item, price: price, notional: notional})
	}
	if len(parsed) == 0 {
		return result
	}
	if result.MaxPrice <= result.MinPrice {
		padding := result.MinPrice * 0.005
		if padding <= 0 {
			padding = 1
		}
		result.MinPrice -= padding
		result.MaxPrice += padding
	}
	duration := to.Sub(from)
	priceSpan := result.MaxPrice - result.MinPrice
	cells := make(map[[2]int]*schema.LiquidationHeatmapCell)
	for _, event := range parsed {
		timeIndex := int(float64(event.item.EventAt.Sub(from)) / float64(duration) * float64(params.TimeBuckets))
		priceIndex := int((event.price - result.MinPrice) / priceSpan * float64(params.PriceBuckets))
		if timeIndex < 0 {
			timeIndex = 0
		} else if timeIndex >= params.TimeBuckets {
			timeIndex = params.TimeBuckets - 1
		}
		if priceIndex < 0 {
			priceIndex = 0
		} else if priceIndex >= params.PriceBuckets {
			priceIndex = params.PriceBuckets - 1
		}
		key := [2]int{timeIndex, priceIndex}
		cell := cells[key]
		if cell == nil {
			cell = &schema.LiquidationHeatmapCell{TimeIndex: timeIndex, PriceIndex: priceIndex}
			cells[key] = cell
		}
		if event.item.Side == "long" {
			cell.LongNotional += event.notional
		} else if event.item.Side == "short" {
			cell.ShortNotional += event.notional
		}
		cell.EventCount++
		result.TotalNotional += event.notional
		result.EventCount++
	}
	keys := make([][2]int, 0, len(cells))
	for key := range cells {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] == keys[j][0] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})
	for _, key := range keys {
		result.Cells = append(result.Cells, *cells[key])
	}
	result.DataAvailable = result.EventCount > 0
	return result
}

func (a *Catalog) GetMarketTicker(ctx context.Context, params *schema.MarketSymbolQuery) (*schema.MarketTicker, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	result, err := a.ProviderFactory.MarketData().Ticker(ctx, params.Exchange, params.Market, params.Symbol)
	if err != nil {
		return nil, errors.InternalServerError("market_ticker_failed", "Unable to load market ticker: %v", err)
	}
	return result, nil
}

func (a *Catalog) QueryMarketKlines(ctx context.Context, params *schema.MarketKlinesQuery) ([]*schema.MarketKline, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	result, err := a.ProviderFactory.MarketData().Klines(
		ctx,
		params.Exchange,
		params.Market,
		params.Symbol,
		params.Interval,
		params.Limit,
	)
	if err != nil {
		return nil, errors.InternalServerError("market_klines_failed", "Unable to load market klines: %v", err)
	}
	return result, nil
}

func (a *Catalog) GetMarketContext(ctx context.Context, params *schema.MarketContextQuery) (*schema.MarketContext, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	result, err := a.ProviderFactory.MarketData().Context(ctx, params.Exchange, params.Market, params.Symbol, params.Depth)
	if err != nil {
		return nil, errors.InternalServerError("market_context_failed", "Unable to load market context: %v", err)
	}
	return result, nil
}

func (a *Catalog) GetProviderCapabilities() *schema.ProviderCapabilityMatrix {
	return &schema.ProviderCapabilityMatrix{
		Providers: []schema.ProviderCapability{
			{Exchange: "binance", Kind: "cex", MigrationStatus: "implemented", AccountTypes: []string{"spot", "futures", "unified"},
				Snapshot: true, History: true, UserStream: true, PublicMarketData: true, Trading: true,
				TradingGate: "global_enabled+testnet_only+owner_safety+command_confirmation", CredentialModel: "api_key+secret",
				Notes: []string{"Unified account remains read-only", "spot protected entry requires atomic OTOCO limit order", "production execution is disabled by default"}},
			{Exchange: "okx", Kind: "cex", MigrationStatus: "implemented", AccountTypes: []string{"spot", "futures"},
				Snapshot: true, History: true, UserStream: true, PublicMarketData: true, Trading: true,
				TradingGate: "global_enabled+testnet_only+owner_safety+command_confirmation", CredentialModel: "api_key+secret+passphrase",
				Notes: []string{"private stream waits for login acknowledgement before subscribing", "orders attach TP/SL where configured", "production execution is disabled by default"}},
			{Exchange: "bybit", Kind: "cex", MigrationStatus: "implemented_read_only", AccountTypes: []string{"spot", "futures"},
				Snapshot: true, History: true, UserStream: false, PublicMarketData: false, Trading: false,
				TradingGate: "not_implemented", CredentialModel: "api_key+secret", Notes: []string{"large-account acceptance still requires external credentials"}},
			{Exchange: "bitget", Kind: "cex", MigrationStatus: "evaluated_not_scheduled", AccountTypes: []string{"futures"},
				TradingGate: "not_implemented", CredentialModel: "api_key+secret+passphrase",
				Notes: []string{"NOFX source has account, position, order and order-sync implementations", "requires normalized pagination and error-contract tests before enablement"}},
			{Exchange: "gate", Kind: "cex", MigrationStatus: "evaluated_not_scheduled", AccountTypes: []string{"futures"},
				TradingGate: "not_implemented", CredentialModel: "api_key+secret",
				Notes: []string{"NOFX source uses gateapi-go v6", "SDK dependency and pagination behavior require independent security review"}},
			{Exchange: "kucoin", Kind: "cex", MigrationStatus: "evaluated_not_scheduled", AccountTypes: []string{"futures"},
				TradingGate: "not_implemented", CredentialModel: "api_key+secret+passphrase",
				Notes: []string{"NOFX source has account, position, order and fill synchronization", "requires passphrase encryption and time-window acceptance tests"}},
			{Exchange: "hyperliquid", Kind: "dex", MigrationStatus: "deferred_by_product_and_security", AccountTypes: []string{"futures", "unified"},
				TradingGate: "not_implemented", CredentialModel: "agent_private_key+main_wallet_address",
				Notes: []string{"never request or store the main wallet private key", "agent wallet authorization, nonce ownership and builder fee require a separate threat model", "X402 is not part of exchange account access"}},
		},
		DataSources: []schema.MarketDataSourceCapability{
			{Name: "Binance Public API", Status: "implemented", Cost: "free_with_rate_limits", Authorization: "none",
				Features:    []string{"spot/futures ticker", "klines", "order book", "open interest", "funding rate", "mark price", "recent-trade taker proxy", "USD-M real-time liquidation snapshots"},
				Limitations: []string{"taker flow is a proxy and must not be labeled institutional NetFlow", "liquidation history starts only after the optional collector is enabled"}},
			{Name: "OKX Public API", Status: "implemented", Cost: "free_with_rate_limits", Authorization: "none",
				Features:    []string{"spot/swap ticker", "klines", "order book", "open interest", "funding rate", "recent-trade taker proxy"},
				Limitations: []string{"taker flow is a proxy", "OKX no longer exposes a public all-market liquidation feed"}},
			{Name: "NOFXOS", Status: "optional_not_required", Cost: "external_contract", Authorization: "api_key",
				Features:    []string{"AI500", "OI ranking", "NetFlow", "gainers/losers", "coin aggregation"},
				Limitations: []string{"commercial availability and SLA require owner confirmation", "not used as a mandatory runtime dependency"}},
		},
	}
}
