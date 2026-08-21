import { request } from "@umijs/max";

const query = <T>(path: string, params: API.PaginationParam) =>
  request<API.ResponseResult<T[]>>(path, {
    method: "GET",
    params: { current: 1, pageSize: 10, ...params },
  });

export const fetchExchanges = (params: API.PaginationParam) =>
  query<TradingAPI.Exchange>("/api/v1/trading/exchanges", params);

export const fetchModels = (params: API.PaginationParam) =>
  query<TradingAPI.AIModel>("/api/v1/trading/models", params);

export const fetchStrategies = (params: API.PaginationParam) =>
  query<TradingAPI.Strategy>("/api/v1/trading/strategies", params);

export const fetchTraders = (params: API.PaginationParam) =>
  query<TradingAPI.Trader>("/api/v1/trading/traders", params);

export const fetchNotificationChannels = (params: API.PaginationParam) =>
  query<TradingAPI.NotificationChannel>(
    "/api/v1/trading/notification-channels",
    params,
  );

export const fetchScheduledTasks = (params: API.PaginationParam) =>
  query<TradingAPI.ScheduledTask>("/api/v1/trading/scheduled-tasks", params);

export const fetchScheduledTaskRuns = (
  params: API.PaginationParam & { taskID?: string; status?: string },
) =>
  query<TradingAPI.ScheduledTaskRun>(
    "/api/v1/trading/scheduled-task-runs",
    params,
  );

export const fetchScheduledTaskAudits = (
  params: API.PaginationParam & { taskID?: string; action?: string },
) =>
  query<TradingAPI.ScheduledTaskAudit>(
    "/api/v1/trading/scheduled-task-audits",
    params,
  );

const create = <T, F>(path: string, data: F) =>
  request<API.ResponseResult<T>>(path, { method: "POST", data });

const update = <F>(path: string, data: F) =>
  request<API.ResponseResult<void>>(path, { method: "PUT", data });

const remove = (path: string) =>
  request<API.ResponseResult<void>>(path, { method: "DELETE" });

export const getExchange = (id: string) =>
  request<API.ResponseResult<TradingAPI.Exchange>>(
    `/api/v1/trading/exchanges/${id}`,
  );
export const createExchange = (data: TradingAPI.ExchangeForm) =>
  create<TradingAPI.Exchange, TradingAPI.ExchangeForm>(
    "/api/v1/trading/exchanges",
    data,
  );
export const updateExchange = (id: string, data: TradingAPI.ExchangeForm) =>
  update(`/api/v1/trading/exchanges/${id}`, data);
export const deleteExchange = (id: string) =>
  remove(`/api/v1/trading/exchanges/${id}`);
export const testExchange = (id: string) =>
  request<API.ResponseResult<TradingAPI.ExchangeConnectionTest>>(
    `/api/v1/trading/exchanges/${id}/test`,
    { method: "POST" },
  );
export const syncExchange = (id: string) =>
  request<API.ResponseResult<TradingAPI.ExchangeDataSync>>(
    `/api/v1/trading/exchanges/${id}/sync`,
    { method: "POST" },
  );
export const backfillExchangeHistory = (
  id: string,
  data: TradingAPI.HistoryBackfillForm,
) =>
  request<API.ResponseResult<TradingAPI.HistoryBackfill>>(
    `/api/v1/trading/exchanges/${id}/history/backfill`,
    { method: "POST", data },
  );
export const fetchHistorySyncStatuses = () =>
  request<API.ResponseResult<TradingAPI.HistorySyncStatus[]>>(
    "/api/v1/trading/history-status",
  );
export const replaceExchangeHistorySymbols = (id: string, symbols: string[]) =>
  request<API.ResponseResult<string[]>>(
    `/api/v1/trading/exchanges/${id}/history/symbols`,
    {
      method: "PUT",
      data: { symbols },
    },
  );

export const getModel = (id: string) =>
  request<API.ResponseResult<TradingAPI.AIModel>>(
    `/api/v1/trading/models/${id}`,
  );
export const createModel = (data: TradingAPI.AIModelForm) =>
  create<TradingAPI.AIModel, TradingAPI.AIModelForm>(
    "/api/v1/trading/models",
    data,
  );
export const updateModel = (id: string, data: TradingAPI.AIModelForm) =>
  update(`/api/v1/trading/models/${id}`, data);
export const deleteModel = (id: string) =>
  remove(`/api/v1/trading/models/${id}`);
export const testModel = (id: string) =>
  request<API.ResponseResult<TradingAPI.AIModelConnectionTest>>(
    `/api/v1/trading/models/${id}/test`,
    { method: "POST" },
  );

export const getStrategy = (id: string) =>
  request<API.ResponseResult<TradingAPI.StrategyDetail>>(
    `/api/v1/trading/strategies/${id}`,
  );
export const createStrategy = (data: TradingAPI.StrategyForm) =>
  create<TradingAPI.StrategyDetail, TradingAPI.StrategyForm>(
    "/api/v1/trading/strategies",
    data,
  );
export const updateStrategy = (id: string, data: TradingAPI.StrategyForm) =>
  update(`/api/v1/trading/strategies/${id}`, data);
export const deleteStrategy = (id: string) =>
  remove(`/api/v1/trading/strategies/${id}`);
export const activateStrategy = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/strategies/${id}/activate`,
    { method: "POST" },
  );
export const duplicateStrategy = (id: string, name: string) =>
  request<API.ResponseResult<TradingAPI.StrategyDetail>>(
    `/api/v1/trading/strategies/${id}/duplicate`,
    { method: "POST", data: { name } },
  );
export const fetchStrategyVersions = (id: string) =>
  request<API.ResponseResult<TradingAPI.StrategyVersion[]>>(
    `/api/v1/trading/strategies/${id}/versions`,
  );
export const publishStrategy = (id: string) =>
  request<API.ResponseResult<TradingAPI.StrategyDetail>>(
    `/api/v1/trading/strategies/${id}/publish`,
    { method: "POST" },
  );
export const rollbackStrategy = (id: string, version: number) =>
  request<API.ResponseResult<TradingAPI.StrategyDetail>>(
    `/api/v1/trading/strategies/${id}/rollback`,
    { method: "POST", data: { version } },
  );
export const getDefaultStrategyConfig = () =>
  request<API.ResponseResult<TradingAPI.StrategyStudioConfig>>(
    "/api/v1/trading/strategies/default-config",
  );
export const previewStrategy = (
  config: TradingAPI.StrategyStudioConfig,
  promptVariant: string,
) =>
  request<API.ResponseResult<TradingAPI.StrategyPreview>>(
    "/api/v1/trading/strategies/preview",
    {
      method: "POST",
      data: { config, prompt_variant: promptVariant },
    },
  );
export const estimateStrategyTokens = (
  config: TradingAPI.StrategyStudioConfig,
) =>
  request<API.ResponseResult<TradingAPI.StrategyTokenEstimate>>(
    "/api/v1/trading/strategies/estimate-tokens",
    { method: "POST", data: { config } },
  );
export const testStrategy = (
  config: TradingAPI.StrategyStudioConfig,
  promptVariant: string,
  aiModelID: string,
) =>
  request<API.ResponseResult<TradingAPI.StrategyTestResult>>(
    "/api/v1/trading/strategies/test",
    {
      method: "POST",
      data: { config, prompt_variant: promptVariant, ai_model_id: aiModelID },
    },
  );

export const getTrader = (id: string) =>
  request<API.ResponseResult<TradingAPI.Trader>>(
    `/api/v1/trading/traders/${id}`,
  );
export const createTrader = (data: TradingAPI.TraderForm) =>
  create<TradingAPI.Trader, TradingAPI.TraderForm>(
    "/api/v1/trading/traders",
    data,
  );
export const updateTrader = (id: string, data: TradingAPI.TraderForm) =>
  update(`/api/v1/trading/traders/${id}`, data);
export const deleteTrader = (id: string) =>
  remove(`/api/v1/trading/traders/${id}`);
export const startTrader = (id: string, data: TradingAPI.TraderStartForm) =>
  request<API.ResponseResult<void>>(`/api/v1/trading/traders/${id}/start`, {
    method: "POST",
    data,
  });
export const stopTrader = (id: string) =>
  request<API.ResponseResult<void>>(`/api/v1/trading/traders/${id}/stop`, {
    method: "POST",
  });
export const syncTrader = (id: string) =>
  request<API.ResponseResult<void>>(`/api/v1/trading/traders/${id}/sync`, {
    method: "POST",
  });

export const getNotificationChannel = (id: string) =>
  request<API.ResponseResult<TradingAPI.NotificationChannel>>(
    `/api/v1/trading/notification-channels/${id}`,
  );
export const createNotificationChannel = (
  data: TradingAPI.NotificationChannelForm,
) =>
  create<TradingAPI.NotificationChannel, TradingAPI.NotificationChannelForm>(
    "/api/v1/trading/notification-channels",
    data,
  );
export const updateNotificationChannel = (
  id: string,
  data: TradingAPI.NotificationChannelForm,
) => update(`/api/v1/trading/notification-channels/${id}`, data);
export const deleteNotificationChannel = (id: string) =>
  remove(`/api/v1/trading/notification-channels/${id}`);
export const testNotificationChannel = (id: string) =>
  request<API.ResponseResult<TradingAPI.NotificationChannelTest>>(
    `/api/v1/trading/notification-channels/${id}/test`,
    { method: "POST" },
  );
export const createTelegramBindingCode = (id: string) =>
  request<API.ResponseResult<TradingAPI.TelegramBindingCode>>(
    `/api/v1/trading/notification-channels/${id}/telegram/binding-code`,
    { method: "POST" },
  );
export const getTelegramSession = (id: string) =>
  request<API.ResponseResult<TradingAPI.TelegramSession>>(
    `/api/v1/trading/notification-channels/${id}/telegram/session`,
  );
export const unbindTelegramSession = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/notification-channels/${id}/telegram/session`,
    { method: "DELETE" },
  );

export const createScheduledTask = (data: TradingAPI.ScheduledTaskForm) =>
  create<TradingAPI.ScheduledTask, TradingAPI.ScheduledTaskForm>(
    "/api/v1/trading/scheduled-tasks",
    data,
  );
export const updateScheduledTask = (
  id: string,
  data: TradingAPI.ScheduledTaskForm,
) => update(`/api/v1/trading/scheduled-tasks/${id}`, data);
export const deleteScheduledTask = (id: string) =>
  remove(`/api/v1/trading/scheduled-tasks/${id}`);
export const triggerScheduledTask = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/scheduled-tasks/${id}/trigger`,
    {
      method: "POST",
    },
  );
export const pauseScheduledTask = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/scheduled-tasks/${id}/pause`,
    {
      method: "POST",
    },
  );
export const resumeScheduledTask = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/scheduled-tasks/${id}/resume`,
    {
      method: "POST",
    },
  );
export const disableScheduledTask = (id: string) =>
  request<API.ResponseResult<void>>(
    `/api/v1/trading/scheduled-tasks/${id}/disable`,
    {
      method: "POST",
    },
  );
