import { request } from '@umijs/max';

export type TradeCommandForm = {
  trader_id: string;
  idempotency_key: string;
  action: 'open' | 'add' | 'reduce' | 'close' | 'cancel';
  market: TradingAPI.MarketType;
  symbol: string;
  side?: 'buy' | 'sell';
  position_side?: 'long' | 'short';
  order_type?: 'market' | 'limit';
  quantity?: string;
  quote_quantity?: string;
  price?: string;
  stop_loss?: string;
  take_profit?: string;
  leverage?: number;
  reduce_only?: boolean;
  target_exchange_order_id?: string;
  confirm_execution: boolean;
};

export const fetchTradeCommands = (
  params: API.PaginationParam & { traderID?: string; status?: string; symbol?: string },
) =>
  request<API.ResponseResult<TradingAPI.TradeCommand[]>>('/api/v1/trading/trade-commands', {
    method: 'GET',
    params: { current: 1, pageSize: 10, ...params },
  });

export const fetchTradeCommandEvents = (id: string) =>
  request<API.ResponseResult<TradingAPI.TradeCommandEvent[]>>(
    `/api/v1/trading/trade-commands/${id}/events`,
  );

export const createTradeCommand = (data: TradeCommandForm) =>
  request<API.ResponseResult<TradingAPI.TradeCommand>>('/api/v1/trading/trade-commands', {
    method: 'POST',
    data,
  });

export const bulkClosePositions = (data: {
  trader_id?: string;
  operation_id: string;
  confirmation_phrase: string;
  acknowledge_risk: boolean;
  confirm_execution: boolean;
}) =>
  request<
    API.ResponseResult<{
      requested: number;
      succeeded: number;
      failed: number;
      results: Array<{ position_id: string; symbol: string; command?: TradingAPI.TradeCommand; error?: string }>;
    }>
  >('/api/v1/trading/trade-commands/bulk-close', { method: 'POST', data });

export const fetchTradingSafetyState = () =>
  request<API.ResponseResult<TradingAPI.TradingSafetyState>>(
    '/api/v1/trading/execution/safety',
  );
