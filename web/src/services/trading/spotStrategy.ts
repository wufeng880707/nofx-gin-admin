import { request } from '@umijs/max';

export type SpotBacktestCreateForm = {
  strategy_id?: string;
  exchange: 'binance' | 'okx';
  symbol: string;
  interval: '4h' | '1d';
  start_time: string;
  end_time: string;
  initial_capital: string;
  config: TradingAPI.SpotCoreSatelliteConfig;
  cost_scenario: { fee_rate_bps: number; slippage_bps: number };
  benchmarks: string[];
};

export const createSpotBacktest = (data: SpotBacktestCreateForm) =>
  request<API.ResponseResult<TradingAPI.SpotBacktestTask>>('/api/v1/trading/backtests', { method: 'POST', data });

export const fetchSpotBacktests = (params: API.PaginationParam & { status?: string; symbol?: string }) =>
  request<API.ResponseResult<TradingAPI.SpotBacktestTask[]>>('/api/v1/trading/backtests', { method: 'GET', params });

export const getSpotBacktestResult = (id: string) =>
  request<API.ResponseResult<TradingAPI.SpotBacktestResult>>(`/api/v1/trading/backtests/${id}/result`);

export const cancelSpotBacktest = (id: string) =>
  request<API.ResponseResult<void>>(`/api/v1/trading/backtests/${id}/cancel`, { method: 'POST' });

export const fetchSpotLots = (params: API.PaginationParam & { strategyID?: string; cycleID?: string; bucket?: string; status?: string }) =>
  request<API.ResponseResult<TradingAPI.SpotPositionLot[]>>('/api/v1/trading/spot-strategy/lots', { method: 'GET', params });
