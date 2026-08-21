declare namespace TradingAPI {
  type Exchange = {
    id: string;
    owner_user_id: string;
    exchange_type: string;
    account_name: string;
    name: string;
    account_type: string;
    enabled: boolean;
    testnet: boolean;
    has_api_key: boolean;
    has_secret_key: boolean;
    has_passphrase: boolean;
    wallet_address: string;
    account_user: string;
    signer_address: string;
    api_key_index: number;
    unified_account: boolean;
    builder_approved: boolean;
    updated_at: string;
  };

  type ExchangeForm = Omit<
    Exchange,
    "id" | "has_api_key" | "has_secret_key" | "has_passphrase" | "updated_at"
  > & {
    api_key?: string;
    secret_key?: string;
    passphrase?: string;
    private_key?: string;
    api_key_private_key?: string;
  };

  type ExchangeConnectionTest = {
    exchange_id: string;
    exchange_type: string;
    account_type: string;
    testnet: boolean;
    environment: "production" | "testnet";
    network_ok: boolean;
    authentication_ok: boolean;
    network_latency_ms: number;
    authentication_latency_ms: number;
    latency_ms: number;
  };

  type ExchangeDataSync = {
    exchange_id: string;
    exchange_type: string;
    account_type: string;
    asset_count: number;
    order_count: number;
    fill_count: number;
    income_event_count: number;
    synced_at: string;
  };

  type HistoryBackfillForm = {
    start_at: string;
    end_at: string;
    symbols: string[];
  };

  type HistoryBackfill = ExchangeDataSync & {
    start_at: string;
    end_at: string;
    symbols: string[];
    fetched_order_count: number;
    fetched_fill_count: number;
    fetched_income_event_count: number;
    closed_trade_count: number;
  };

  type HistorySyncStatus = {
    exchange_id: string;
    exchange_name: string;
    exchange_type: string;
    account_type: string;
    enabled: boolean;
    status: string;
    coverage_status:
      "healthy" | "pending" | "stale" | "incomplete" | "error" | "disabled";
    warning: string;
    synced_until?: string;
    last_run_at?: string;
    error_message: string;
    registered_symbols: string[];
    order_count: number;
    fill_count: number;
    income_event_count: number;
    closed_trade_count: number;
    orphan_fill_count: number;
    unmatched_order_count: number;
    gap_count: number;
    gap_details: string[];
  };

  type AIModel = {
    id: string;
    owner_user_id: string;
    name: string;
    provider: string;
    enabled: boolean;
    has_api_key: boolean;
    custom_api_url: string;
    custom_model_name: string;
    input_price_per_m_tokens: string;
    output_price_per_m_tokens: string;
    updated_at: string;
  };

  type AIModelForm = Omit<AIModel, "id" | "has_api_key" | "updated_at"> & {
    api_key?: string;
  };

  type AIModelConnectionTest = {
    ai_model_id: string;
    provider: string;
    model_name: string;
    request_id: string;
    response: string;
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
    latency_ms: number;
    success: boolean;
  };

  type Strategy = {
    id: string;
    owner_user_id: string;
    name: string;
    description: string;
    strategy_type: string;
    is_active: boolean;
    is_default: boolean;
    is_public: boolean;
    config_visible: boolean;
    current_version: number;
    published_version?: number;
    updated_at: string;
  };

  type StrategyDetail = Strategy & { config: Record<string, unknown> };

  type StrategyForm = Omit<
    Strategy,
    "id" | "updated_at" | "current_version" | "published_version"
  > & {
    config: Record<string, unknown>;
  };

  type StrategyVersion = {
    id: string;
    strategy_id: string;
    version: number;
    name: string;
    description: string;
    strategy_type: string;
    config: Record<string, unknown>;
    rollback_from?: number;
    published_at?: string;
    created_at: string;
  };

  type StrategyStudioConfig = {
    strategy_type: "ai_trading" | "grid" | "spot_core_satellite";
    language: "zh" | "en";
    grid_config: {
      symbol: string;
      market: "spot" | "futures";
      direction: "neutral" | "long" | "short";
      price_mode: "arithmetic" | "geometric";
      lower_price: number;
      upper_price: number;
      grid_count: number;
      investment_usdt: number;
      max_position_usdt: number;
      max_open_orders: number;
      reserve_ratio: number;
      leverage: number;
      stop_loss_pct: number;
      take_profit_pct: number;
      max_drawdown_pct: number;
    };
    spot_core_satellite_config: SpotCoreSatelliteConfig;
    ai_config: {
      coin_source: {
        source_type: "static" | "ai500" | "oi_top" | "mixed";
        static_coins: string[];
        excluded_coins: string[];
        use_ai500: boolean;
        ai500_limit: number;
        use_oi_top: boolean;
        oi_top_limit: number;
      };
      indicators: {
        klines: {
          primary_timeframe: string;
          primary_count: number;
          selected_timeframes: string[];
          enable_multi_timeframe: boolean;
        };
        enable_raw_klines: boolean;
        enable_ema: boolean;
        enable_macd: boolean;
        enable_rsi: boolean;
        enable_atr: boolean;
        enable_boll: boolean;
        enable_volume: boolean;
        enable_oi: boolean;
        enable_funding_rate: boolean;
        enable_quant_data: boolean;
        enable_quant_oi: boolean;
        enable_quant_netflow: boolean;
        enable_oi_ranking: boolean;
        enable_netflow_ranking: boolean;
        enable_price_ranking: boolean;
        ema_periods: number[];
        rsi_periods: number[];
        atr_periods: number[];
        boll_periods: number[];
        oi_ranking_duration: "1h" | "4h" | "24h";
        oi_ranking_limit: number;
        netflow_ranking_duration: "1h" | "4h" | "24h";
        netflow_ranking_limit: number;
        price_ranking_duration: string;
        price_ranking_limit: number;
      };
      risk_control: {
        max_positions: number;
        btc_eth_max_leverage: number;
        altcoin_max_leverage: number;
        btc_eth_max_position_value_ratio: number;
        altcoin_max_position_value_ratio: number;
        max_margin_usage: number;
        min_position_size: number;
        min_risk_reward_ratio: number;
        min_confidence: number;
      };
      prompt_sections: {
        role_definition: string;
        trading_frequency: string;
        entry_standards: string;
        decision_process: string;
      };
      custom_prompt: string;
    };
  };

  type StrategyPreview = {
    system_prompt: string;
    prompt_variant: string;
    config_summary: Record<string, unknown>;
  };

  type StrategyTestResult = StrategyPreview & {
    ai_model_name: string;
    provider: string;
    model_name: string;
    request_id: string;
    ai_response: string;
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
    latency_ms: number;
    ready: boolean;
    note: string;
  };

  type StrategyTokenEstimate = {
    total: number;
    breakdown: {
      system_prompt: number;
      market_data: number;
      ranking_data: number;
      quant_data: number;
      fixed_overhead: number;
    };
    model_limits: Array<{
      name: string;
      context_limit: number;
      usage_pct: number;
      level: "ok" | "warning" | "danger";
    }>;
    suggestions: string[];
  };

  type Trader = {
    id: string;
    owner_user_id: string;
    name: string;
    ai_model_id: string;
    exchange_id: string;
    strategy_id?: string;
    strategy_version: number;
    initial_balance: string;
    scan_interval_minutes: number;
    is_cross_margin: boolean;
    is_simulation: boolean;
    auto_apply_strategy_updates: boolean;
    show_in_competition: boolean;
    competition_public_id: string;
    competition_alias: string;
    competition_joined_at?: string;
    last_cycle_duration_ms: number;
    consecutive_failures: number;
    execution_mode: "notify_only" | "approval_required" | "auto";
    notification_channel_ids: string[];
    desired_state: string;
    runtime_state: string;
    sync_state: string;
    strategy_state: string;
    cycle_number: number;
    last_decision_at?: string;
    safe_mode: boolean;
    pause_reason?: string;
    ai_consent_model_id?: string;
    ai_consent_at?: string;
    last_heartbeat_at?: string;
    last_error_message?: string;
    updated_at: string;
  };

  type NotificationChannel = {
    id: string;
    owner_user_id: string;
    name: string;
    channel_type: "telegram" | "lark";
    enabled: boolean;
    chat_id: string;
    has_bot_token: boolean;
    has_webhook_url: boolean;
    has_signing_secret: boolean;
    last_tested_at?: string;
    last_test_status?: "success" | "failed";
    last_test_message?: string;
    created_at: string;
    updated_at: string;
  };

  type NotificationChannelForm = {
    owner_user_id?: string;
    name: string;
    channel_type: "telegram" | "lark";
    enabled: boolean;
    bot_token?: string;
    chat_id?: string;
    webhook_url?: string;
    signing_secret?: string;
  };

  type NotificationChannelTest = {
    channel_id: string;
    channel_type: "telegram" | "lark";
    delivered: boolean;
    latency_ms: number;
    message: string;
  };

  type TelegramBindingCode = {
    channel_id: string;
    code: string;
    expires_at: string;
  };

  type TelegramSession = {
    channel_id: string;
    bound: boolean;
    status: "active" | "unbound";
    chat_id?: string;
    telegram_username?: string;
    bound_at?: string;
    last_message_at?: string;
  };

  type TraderForm = Omit<
    Trader,
    | "id"
    | "desired_state"
    | "runtime_state"
    | "last_heartbeat_at"
    | "last_error_message"
    | "updated_at"
    | "show_in_competition"
    | "competition_public_id"
    | "competition_alias"
    | "competition_joined_at"
  >;

  type CompetitionParticipationForm = {
    participating: boolean;
    public_alias?: string;
    confirm_public_disclosure: boolean;
  };

  type CompetitionParticipationState = {
    participating: boolean;
    public_id: string;
    public_alias: string;
    joined_at?: string;
  };

  type CompetitionTrader = {
    public_id: string;
    public_alias: string;
    rank: number;
    is_simulation: boolean;
    total_trades: number;
    winning_trades: number;
    win_rate: number;
    return_pct: number;
    max_drawdown_pct: number;
    equity_index: number;
    joined_at?: string;
    last_updated_at?: string;
  };

  type CompetitionLeaderboard = {
    period_start: string;
    period_end: string;
    items: CompetitionTrader[];
    page: { total: number; current: number; pageSize: number };
  };

  type CompetitionEquityPoint = {
    captured_at: string;
    equity_index: number;
    return_pct: number;
  };

  type CompetitionEquityHistory = {
    trader: CompetitionTrader;
    period_start: string;
    period_end: string;
    points: CompetitionEquityPoint[];
  };

  type TraderStartForm = {
    confirm_external_ai: boolean;
    confirm_automated_execution?: boolean;
    automation_phrase?: string;
  };

  type AccountSnapshot = {
    id: number;
    trader_id: string;
    exchange_id: string;
    captured_at: string;
    total_equity: string;
    balance: string;
    available_balance: string;
    unrealized_pnl: string;
    position_count: number;
    margin_used_pct: string;
    source: string;
  };

  type Position = {
    id: string;
    trader_id: string;
    exchange_id: string;
    symbol: string;
    side: string;
    status: string;
    quantity: string;
    entry_price: string;
    mark_price: string;
    liquidation_price: string;
    unrealized_pnl: string;
    realized_pnl: string;
    leverage: number;
    opened_at: string;
    closed_at?: string;
    last_synced_at: string;
  };

  type AssetBalance = {
    id: number;
    trader_id: string;
    exchange_id: string;
    account_type: string;
    asset: string;
    total: string;
    available: string;
    locked: string;
    borrowed: string;
    interest: string;
    price_usdt: string;
    value_usdt: string;
    unrealized_pnl: string;
    captured_at: string;
    source: string;
  };

  type AssetSummary = {
    total_value_usdt: string;
    asset_count: number;
    exchanges: Array<{
      exchange_id: string;
      account_type: string;
      asset_count: number;
      value_usdt: string;
      last_updated_at?: string;
    }>;
  };

  type ExchangeOrder = {
    id: string;
    trader_id: string;
    exchange_id: string;
    market: string;
    exchange_order_id: string;
    client_order_id: string;
    origin: string;
    symbol: string;
    side: string;
    position_side: string;
    order_type: string;
    status: string;
    price: string;
    average_price: string;
    quantity: string;
    executed_quantity: string;
    quote_quantity: string;
    reduce_only: boolean;
    ordered_at: string;
    exchange_updated_at: string;
  };

  type Fill = {
    id: string;
    trader_id: string;
    exchange_id: string;
    market: string;
    exchange_trade_id: string;
    exchange_order_id: string;
    symbol: string;
    side: string;
    position_side: string;
    price: string;
    quantity: string;
    quote_quantity: string;
    fee_asset: string;
    fee_quantity: string;
    fee_value_usdt: string;
    realized_pnl: string;
    liquidity: string;
    filled_at: string;
  };

  type IncomeEvent = {
    id: string;
    trader_id: string;
    exchange_id: string;
    event_type: string;
    market: string;
    symbol: string;
    asset: string;
    amount: string;
    price_usdt: string;
    value_usdt: string;
    occurred_at: string;
  };

  type Decision = {
    id: number;
    trader_id: string;
    exchange_id: string;
    cycle_number: number;
    decided_at: string;
    success: boolean;
    error_message: string;
    ai_request_duration_ms: number;
    candidate_coins: unknown;
    execution_log: unknown;
    actions: unknown;
  };

  type DecisionDetail = Decision & {
    input_prompt: string;
    raw_response: string;
    decision: unknown;
  };

  type DashboardOverview = {
    total_equity: string;
    available_balance: string;
    realized_pnl: string;
    unrealized_pnl: string;
    funding: string;
    total_fee: string;
    net_pnl: string;
    position_count: number;
    closed_trade_count: number;
    last_updated_at?: string;
  };

  type EquityPoint = {
    captured_at: string;
    total_equity: string;
    available_balance: string;
    unrealized_pnl: string;
    position_count: number;
  };

  type PerformanceGroup = {
    key: string;
    total_trades: number;
    winning_trades: number;
    win_rate: number;
    net_pnl: string;
    average_pnl: string;
  };

  type PerformanceSummary = {
    total_trades: number;
    winning_trades: number;
    losing_trades: number;
    break_even_trades: number;
    win_rate: number;
    gross_profit: string;
    gross_loss: string;
    gross_pnl: string;
    total_fee: string;
    funding: string;
    net_pnl: string;
    average_win: string;
    average_loss: string;
    profit_factor: number;
    profit_loss_ratio: number;
    trade_sharpe: number;
    max_drawdown_pct: number;
    sharpe_basis: string;
    drawdown_source: string;
    by_side: PerformanceGroup[];
    by_symbol: PerformanceGroup[];
    by_trader: PerformanceGroup[];
    by_strategy: PerformanceGroup[];
    by_exchange: PerformanceGroup[];
  };

  type PerformanceReportBucket = {
    period: 'day' | 'week' | 'month';
    start_at: string;
    end_at: string;
    summary: PerformanceSummary;
  };

  type AIInvocationMetric = {
    key: string;
    calls: number;
    successes: number;
    failures: number;
    success_rate: number;
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
    estimated_cost_usd: string;
    average_latency_ms: number;
  };

  type AIInvocationSummary = {
    start_at: string;
    end_at: string;
    overall: AIInvocationMetric;
    by_model: AIInvocationMetric[];
    by_trader: AIInvocationMetric[];
    by_strategy: AIInvocationMetric[];
  };

  type ClosedTrade = {
    id: string;
    trader_id: string;
    strategy_id: string;
    strategy_version: number;
    attribution_source: 'system' | 'external';
    exchange_id: string;
    market: string;
    symbol: string;
    side: string;
    entry_quantity: string;
    exit_quantity: string;
    entry_price: string;
    exit_price: string;
    gross_pnl: string;
    fee: string;
    funding: string;
    net_pnl: string;
    opened_at: string;
    closed_at: string;
    duration_seconds: number;
    source_fill_count: number;
  };

  type ScheduledTask = {
    id: string;
    owner_user_id: string;
    name: string;
    handler: "account_sync" | "account_snapshot";
    cron_expression: string;
    time_zone: string;
    status: "active" | "paused" | "disabled";
    timeout_seconds: number;
    max_retries: number;
    retry_interval_seconds: number;
    misfire_policy: "skip" | "fire_once";
    concurrency_policy: "forbid" | "allow";
    next_run_at?: string;
    last_run_at?: string;
    created_at: string;
    updated_at: string;
  };

  type ScheduledTaskForm = Pick<
    ScheduledTask,
    | "name"
    | "handler"
    | "cron_expression"
    | "time_zone"
    | "timeout_seconds"
    | "max_retries"
    | "retry_interval_seconds"
    | "misfire_policy"
    | "concurrency_policy"
  >;

  type ScheduledTaskRun = {
    id: string;
    task_id: string;
    task_name: string;
    handler: string;
    trigger_source: "schedule" | "manual";
    scheduled_at: string;
    started_at?: string;
    finished_at?: string;
    status: "pending" | "running" | "success" | "failed" | "skipped";
    attempts: number;
    duration_ms: number;
    instance_id: string;
    error_message: string;
    created_at: string;
  };

  type ScheduledTaskAudit = {
    id: string;
    task_id: string;
    owner_user_id: string;
    actor_user_id: string;
    task_name: string;
    action:
      | "create"
      | "update"
      | "delete"
      | "trigger"
      | "pause"
      | "resume"
      | "disable";
    before_json: string;
    after_json: string;
    created_at: string;
  };

  type MarketExchange = "binance" | "okx";
  type MarketType = "spot" | "futures";
  type MarketInterval = "1m" | "5m" | "15m" | "1h" | "4h" | "1d" | "1w";

  type MarketInstrument = {
    exchange: MarketExchange;
    market: MarketType;
    symbol: string;
    base_asset: string;
    quote_asset: string;
    display: string;
  };

  type MarketTicker = {
    exchange: MarketExchange;
    market: MarketType;
    symbol: string;
    last_price: string;
    open_price: string;
    high_price: string;
    low_price: string;
    price_change: string;
    price_change_rate: string;
    base_volume: string;
    quote_volume: string;
    timestamp: number;
  };

  type MarketKline = {
    time: number;
    open: string;
    high: string;
    low: string;
    close: string;
    volume: string;
    quote_volume: string;
    closed: boolean;
  };

  type MarketDepthLevel = {
    price: string;
    quantity: string;
  };

  type MarketContext = {
    exchange: MarketExchange;
    market: MarketType;
    symbol: string;
    ticker: MarketTicker;
    order_book: {
      bids: MarketDepthLevel[];
      asks: MarketDepthLevel[];
      spread_bps: number;
      imbalance: number;
      timestamp: number;
    };
    derivatives?: {
      open_interest: string;
      open_interest_value_usdt: string;
      funding_rate: string;
      next_funding_time: number;
      mark_price: string;
      taker_buy_volume: string;
      taker_sell_volume: string;
      taker_buy_sell_ratio: number;
      flow_source: string;
      flow_is_proxy: boolean;
    };
    risk_radar: {
      spread_bps: number;
      book_imbalance: number;
      funding_rate_pct: number;
      liquidity_risk: "low" | "medium" | "high";
      crowding_risk: "low" | "medium" | "high";
      risk_score: number;
    };
    generated_at: number;
  };

  type LiquidationHeatmapCell = {
    time_index: number;
    price_index: number;
    long_notional_usdt: number;
    short_notional_usdt: number;
    event_count: number;
  };

  type LiquidationHeatmap = {
    exchange: "binance";
    market: "futures";
    symbol: string;
    from: string;
    to: string;
    min_price: number;
    max_price: number;
    time_buckets: number;
    price_buckets: number;
    cells: LiquidationHeatmapCell[];
    total_notional_usdt: number;
    event_count: number;
    data_available: boolean;
    source: string;
    coverage_notice: string;
  };

  type UserStreamState = {
    exchange_id: string;
    owner_user_id: string;
    exchange_type: string;
    status: "connecting" | "connected" | "reconnecting" | "stopped" | "error";
    reconnect_count: number;
    connected_at?: string;
    last_event_at?: string;
    last_reconciled_at?: string;
    last_error: string;
    updated_at: string;
  };

  type UserStreamEvent = {
    id: number;
    owner_user_id: string;
    exchange_id: string;
    exchange_type: string;
    event_type: string;
    payload_hash: string;
    event_time: string;
    transaction_time?: string;
    received_at: string;
  };

  type TradingSafetyState = {
    owner_user_id: string;
    live_enabled: boolean;
    version: number;
    enabled_by: string;
    enabled_at?: string;
    disabled_by: string;
    disabled_at?: string;
    updated_at: string;
  };

  type TradeCommand = {
    id: string;
    owner_user_id: string;
    trader_id: string;
    strategy_id?: string;
    strategy_version: number;
    exchange_id: string;
    decision_id?: number;
    idempotency_key: string;
    action: 'open' | 'add' | 'reduce' | 'close' | 'cancel';
    market: MarketType;
    symbol: string;
    side: 'buy' | 'sell' | '';
    position_side: 'long' | 'short' | '';
    order_type: 'market' | 'limit' | '';
    time_in_force: string;
    quantity: string;
    quote_quantity: string;
    price: string;
    stop_loss: string;
    take_profit: string;
    leverage: number;
    reduce_only: boolean;
    target_exchange_order_id: string;
    simulation: boolean;
    testnet: boolean;
    risk_snapshot: string;
    status: 'validated' | 'simulated' | 'submitted' | 'accepted' | 'partially_filled' | 'filled' | 'rejected' | 'cancelled';
    exchange_order_id: string;
    exchange_client_order_id: string;
    executed_quantity: string;
    average_price: string;
    provider_status: string;
    error_code: string;
    error_message: string;
    submitted_at?: string;
    finished_at?: string;
    created_at: string;
    updated_at: string;
  };

  type TradeCommandEvent = {
    id: number;
    command_id: string;
    owner_user_id: string;
    actor_user_id: string;
    event_type: string;
    from_status: string;
    to_status: string;
    detail: string;
    created_at: string;
  };

  type TradeApproval = {
    id: string;
    owner_user_id: string;
    trader_id: string;
    exchange_id: string;
    decision_id: number;
    action_index: number;
    action: 'open' | 'add' | 'reduce' | 'close' | 'cancel';
    market: MarketType;
    symbol: string;
    side: 'buy' | 'sell';
    position_side: 'long' | 'short';
    order_type: 'market' | 'limit';
    quantity: string;
    quote_quantity: string;
    price: string;
    stop_loss: string;
    take_profit: string;
    leverage: number;
    reduce_only: boolean;
    target_exchange_order_id: string;
    reason: string;
    status: 'pending' | 'approved' | 'executing' | 'succeeded' | 'failed' | 'rejected' | 'expired';
    command_id: string;
    failure_code: string;
    failure_message: string;
    requested_at: string;
    expires_at: string;
    acted_by: string;
    acted_at?: string;
    created_at: string;
    updated_at: string;
  };

  type TradeApprovalEvent = {
    id: number;
    approval_id: string;
    owner_user_id: string;
    actor_user_id: string;
    event_type: string;
    from_status: string;
    to_status: string;
    detail: string;
    created_at: string;
  };

  type TradeApprovalActForm = {
    confirmation_phrase?: string;
    comment?: string;
  };

  type SpotCoreSatelliteConfig = {
    exchange: 'binance' | 'okx';
    symbol: string;
    market: 'spot';
    quote_asset: 'USDT';
    execution_timeframe: '4h' | '1d';
    strategy_budget_usdt: number;
    core_budget_ratio: number;
    satellite_budget_ratio: number;
    minimum_cash_ratio: number;
    max_symbol_exposure_ratio: number;
    low_area_lookback_bars: number;
    low_area_drawdown_pct: number;
    trend_filter_period: number;
    stabilization_bars: number;
    accumulation_drawdowns: number[];
    accumulation_weights: number[];
    accumulation_cooldown_bars: number;
    satellite_entry_drawdowns: number[];
    satellite_take_profit_ratio: number;
    satellite_sell_ratio: number;
    resistance_lookback_bars: number;
    breakout_confirm_bars: number;
    breakout_volume_multiplier: number;
    distribution_lookback_bars: number;
    distribution_step_gain_pct: number;
    distribution_step_sell_ratio: number;
    max_core_distribution_ratio: number;
    core_minimum_retention_ratio: number;
    enable_drawdown_risk_control: boolean;
    max_strategy_drawdown_pct: number;
    max_daily_buy_ratio: number;
    fee_rate_bps: number;
    slippage_bps: number;
    stress_slippage_bps: number;
    allow_live_execution: boolean;
  };

  type SpotBacktestTask = {
    id: string;
    strategy_id: string;
    status: 'queued' | 'loading_data' | 'running' | 'calculating' | 'completed' | 'failed' | 'cancelled';
    exchange: string;
    symbol: string;
    interval: string;
    start_time: string;
    end_time: string;
    initial_capital: string;
    progress: number;
    error_code: string;
    error_message: string;
    cancel_requested: boolean;
    created_at: string;
    finished_at?: string;
  };

  type SpotBacktestMetrics = {
    initial_capital: number;
    final_equity: number;
    total_return: number;
    annualized_return: number;
    max_drawdown: number;
    calmar: number;
    profit_factor: number;
    profit_factor_available: boolean;
    realized_pnl: number;
    fees: number;
    turnover: number;
    average_cash_ratio: number;
    minimum_cash_ratio: number;
    maximum_exposure_ratio: number;
    maximum_daily_buy_ratio: number;
    trade_count: number;
    winning_trades: number;
    losing_trades: number;
    core_remaining_quantity: number;
    satellite_remaining_quantity: number;
  };

  type SpotBacktestBenchmark = {
    name: string;
    final_equity: number;
    total_return: number;
    max_drawdown: number;
    calmar: number;
  };

  type SpotBacktestLot = {
    id: string;
    bucket: 'core' | 'satellite';
    layer_no: number;
    open_time: number;
    open_price: number;
    acquired_qty: number;
    remaining_qty: number;
    total_cost: number;
    target_price: number;
    realized_pnl: number;
    closed_at?: number;
  };

  type SpotBacktestEvent = {
    time: number;
    type: string;
    from_state?: string;
    to_state?: string;
    reason: string;
    lot_id?: string;
    quantity?: number;
    price?: number;
  };

  type SpotAcceptanceItem = { key: string; passed: boolean; not_applicable?: boolean; actual: string; expected: string };

  type SpotBacktestResult = {
    task: SpotBacktestTask;
    metrics: SpotBacktestMetrics;
    benchmarks: SpotBacktestBenchmark[];
    equity_curve: Array<{ time: number; equity: number; cash: number; exposure: number; state: string }>;
    lots: SpotBacktestLot[];
    events: SpotBacktestEvent[];
    acceptance: SpotAcceptanceItem[];
  };

  type SpotPositionLot = {
    id: string;
    strategy_id: string;
    cycle_id: string;
    trader_id: string;
    exchange_id: string;
    symbol: string;
    bucket: 'core' | 'satellite';
    layer_no: number;
    acquired_qty: string;
    remaining_qty: string;
    reserved_qty: string;
    total_cost: string;
    unit_cost: string;
    target_net_profit_rate: string;
    target_price: string;
    status: string;
    opened_at: string;
    closed_at?: string;
  };
}
