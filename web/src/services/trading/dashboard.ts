import { request } from '@umijs/max';

export type TraderSummary = {
  id: string;
  name: string;
  runtime_state: string;
  is_simulation: boolean;
  last_heartbeat_at?: string;
  last_error_message?: string;
};

export type DashboardSummary = {
  exchanges: number;
  models: number;
  strategies: number;
  traders: number;
  runningTraders: number;
  recentTraders: TraderSummary[];
};

type CountResponse<T> = API.ResponseResult<T[]>;

const countParams = { current: 1, pageSize: 5 };

export async function fetchDashboardSummary(): Promise<DashboardSummary> {
  const [exchanges, models, strategies, traders, runningTraders] = await Promise.all([
    request<CountResponse<unknown>>('/api/v1/trading/exchanges', {
      method: 'GET',
      params: countParams,
    }),
    request<CountResponse<unknown>>('/api/v1/trading/models', {
      method: 'GET',
      params: countParams,
    }),
    request<CountResponse<unknown>>('/api/v1/trading/strategies', {
      method: 'GET',
      params: countParams,
    }),
    request<CountResponse<TraderSummary>>('/api/v1/trading/traders', {
      method: 'GET',
      params: countParams,
    }),
    request<CountResponse<TraderSummary>>('/api/v1/trading/traders', {
      method: 'GET',
      params: { current: 1, pageSize: 1, runtimeState: 'running' },
    }),
  ]);

  const recentTraders = traders.data ?? [];
  return {
    exchanges: exchanges.total ?? exchanges.data?.length ?? 0,
    models: models.total ?? models.data?.length ?? 0,
    strategies: strategies.total ?? strategies.data?.length ?? 0,
    traders: traders.total ?? recentTraders.length,
    runningTraders: runningTraders.total ?? runningTraders.data?.length ?? 0,
    recentTraders,
  };
}
