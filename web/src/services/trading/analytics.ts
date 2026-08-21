import { request } from '@umijs/max';

export type AnalyticsParams = {
  ownerUserID?: string;
  traderID?: string;
  strategyID?: string;
  strategyVersion?: number;
  exchangeID?: string;
  market?: string;
  symbol?: string;
  side?: string;
  startAt?: string;
  endAt?: string;
};

export type PerformanceReportParams = AnalyticsParams & {
  period: 'day' | 'week' | 'month';
};

export const fetchTradingOverview = (params: AnalyticsParams = {}) =>
  request<API.ResponseResult<TradingAPI.DashboardOverview>>(
    '/api/v1/trading/dashboard/overview',
    { method: 'GET', params },
  );

export const fetchEquityCurve = (params: AnalyticsParams = {}) =>
  request<API.ResponseResult<TradingAPI.EquityPoint[]>>(
    '/api/v1/trading/analytics/equity-curve',
    { method: 'GET', params },
  );

export const fetchPerformance = (params: AnalyticsParams = {}) =>
  request<API.ResponseResult<TradingAPI.PerformanceSummary>>(
    '/api/v1/trading/analytics/performance',
    { method: 'GET', params },
  );

export const fetchClosedTrades = (
  params: AnalyticsParams & API.PaginationParam,
) =>
  request<API.ResponseResult<TradingAPI.ClosedTrade[]>>(
    '/api/v1/trading/analytics/closed-trades',
    { method: 'GET', params },
  );

export const fetchAIInvocationSummary = (
  params: Pick<AnalyticsParams, 'traderID' | 'strategyID' | 'startAt' | 'endAt'> = {},
) =>
  request<API.ResponseResult<TradingAPI.AIInvocationSummary>>(
    '/api/v1/trading/ai-invocations/summary',
    { method: 'GET', params },
  );

export const fetchPerformanceReport = (params: PerformanceReportParams) =>
  request<API.ResponseResult<TradingAPI.PerformanceReportBucket[]>>(
    '/api/v1/trading/analytics/reports',
    { method: 'GET', params },
  );

export const downloadPerformanceReport = (params: PerformanceReportParams) =>
  request<Blob>('/api/v1/trading/analytics/reports/export', {
    method: 'GET',
    params,
    responseType: 'blob',
  });
