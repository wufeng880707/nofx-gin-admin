import { request } from '@umijs/max';

export const fetchTradeApprovals = (
  params: API.PaginationParam & { traderID?: string; status?: string; symbol?: string },
) =>
  request<API.ResponseResult<TradingAPI.TradeApproval[]>>('/api/v1/trading/trade-approvals', {
    method: 'GET',
    params: { current: 1, pageSize: 10, ...params },
  });

export const fetchTradeApprovalEvents = (id: string) =>
  request<API.ResponseResult<TradingAPI.TradeApprovalEvent[]>>(
    `/api/v1/trading/trade-approvals/${id}/events`,
  );

export const approveTradeApproval = (id: string, data: TradingAPI.TradeApprovalActForm) =>
  request<API.ResponseResult<TradingAPI.TradeApproval>>(
    `/api/v1/trading/trade-approvals/${id}/approve`,
    { method: 'POST', data },
  );

export const rejectTradeApproval = (id: string, data: TradingAPI.TradeApprovalActForm) =>
  request<API.ResponseResult<TradingAPI.TradeApproval>>(
    `/api/v1/trading/trade-approvals/${id}/reject`,
    { method: 'POST', data },
  );
