import { request } from '@umijs/max';

const query = <T>(path: string, params: API.PaginationParam) =>
  request<API.ResponseResult<T[]>>(path, {
    method: 'GET',
    params: { current: 1, pageSize: 10, ...params },
  });

export const fetchSnapshots = (params: API.PaginationParam) =>
  query<TradingAPI.AccountSnapshot>('/api/v1/trading/account-snapshots', params);

export const fetchPositions = (params: API.PaginationParam) =>
  query<TradingAPI.Position>('/api/v1/trading/positions', params);

export const fetchAssetBalances = (params: API.PaginationParam) =>
  query<TradingAPI.AssetBalance>('/api/v1/trading/asset-balances', params);

export const fetchAssetSummary = () =>
  request<API.ResponseResult<TradingAPI.AssetSummary>>('/api/v1/trading/asset-summary', {
    method: 'GET',
  });

export const fetchOrders = (params: API.PaginationParam) =>
  query<TradingAPI.ExchangeOrder>('/api/v1/trading/orders', params);

export const fetchFills = (params: API.PaginationParam) =>
  query<TradingAPI.Fill>('/api/v1/trading/fills', params);

export const fetchIncomeEvents = (params: API.PaginationParam) =>
  query<TradingAPI.IncomeEvent>('/api/v1/trading/income-events', params);

export const fetchUserStreamStates = () =>
  request<API.ResponseResult<TradingAPI.UserStreamState[]>>('/api/v1/trading/user-stream/states');

export const fetchUserStreamEvents = (
  params: API.PaginationParam & { exchangeID?: string; eventType?: string },
) => query<TradingAPI.UserStreamEvent>('/api/v1/trading/user-stream/events', params);

export const fetchDecisions = (params: API.PaginationParam) =>
  query<TradingAPI.Decision>('/api/v1/trading/decisions', params);

export const fetchDecision = (id: number) =>
  request<API.ResponseResult<TradingAPI.DecisionDetail>>(`/api/v1/trading/decisions/${id}`, {
    method: 'GET',
  });
