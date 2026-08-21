import { request } from '@umijs/max';

export const fetchCompetitionLeaderboard = (params: API.PaginationParam & { days?: number }) =>
  request<API.ResponseResult<TradingAPI.CompetitionLeaderboard>>(
    '/api/v1/competition/leaderboard',
    { method: 'GET', params },
  );

export const fetchCompetitionEquity = (publicID: string, days: number) =>
  request<API.ResponseResult<TradingAPI.CompetitionEquityHistory>>(
    `/api/v1/competition/traders/${encodeURIComponent(publicID)}/equity`,
    { method: 'GET', params: { days } },
  );

export const getCompetitionParticipation = (traderID: string) =>
  request<API.ResponseResult<TradingAPI.CompetitionParticipationState>>(
    `/api/v1/trading/traders/${traderID}/competition`,
    { method: 'GET' },
  );

export const setCompetitionParticipation = (
  traderID: string,
  data: TradingAPI.CompetitionParticipationForm,
) =>
  request<API.ResponseResult<TradingAPI.CompetitionParticipationState>>(
    `/api/v1/trading/traders/${traderID}/competition`,
    { method: 'PUT', data },
  );
