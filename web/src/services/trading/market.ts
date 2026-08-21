import { request } from "@umijs/max";

export type MarketScope = {
  exchange: TradingAPI.MarketExchange;
  market: TradingAPI.MarketType;
};

export type MarketSymbolScope = MarketScope & {
  symbol: string;
};

export const fetchMarketInstruments = (params: MarketScope) =>
  request<API.ResponseResult<TradingAPI.MarketInstrument[]>>(
    "/api/v1/market/instruments",
    {
      method: "GET",
      params,
    },
  );

export const fetchMarketTicker = (params: MarketSymbolScope) =>
  request<API.ResponseResult<TradingAPI.MarketTicker>>(
    "/api/v1/market/ticker",
    {
      method: "GET",
      params,
    },
  );

export const fetchMarketKlines = (
  params: MarketSymbolScope & {
    interval: TradingAPI.MarketInterval;
    limit?: number;
  },
) =>
  request<API.ResponseResult<TradingAPI.MarketKline[]>>(
    "/api/v1/market/klines",
    {
      method: "GET",
      params,
    },
  );

export const fetchMarketContext = (
  params: MarketSymbolScope & {
    depth?: number;
  },
) =>
  request<API.ResponseResult<TradingAPI.MarketContext>>(
    "/api/v1/market/context",
    {
      method: "GET",
      params,
    },
  );

export const fetchLiquidationHeatmap = (params: {
  symbol: string;
  hours?: number;
  timeBuckets?: number;
  priceBuckets?: number;
}) =>
  request<API.ResponseResult<TradingAPI.LiquidationHeatmap>>(
    "/api/v1/market/liquidations/heatmap",
    {
      method: "GET",
      params,
    },
  );
