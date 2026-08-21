import {
  AreaChartOutlined,
  BarChartOutlined,
  ClockCircleOutlined,
  DatabaseOutlined,
  DollarOutlined,
  FundOutlined,
  ReloadOutlined,
  RightOutlined,
  RobotOutlined,
  StockOutlined,
  SwapOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Empty,
  Row,
  Segmented,
  Select,
  Skeleton,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import DashboardMarketChart from './components/DashboardMarketChart';
import EquityCurve from './components/EquityCurve';
import { fetchExchanges, fetchTraders } from '@/services/trading/catalog';
import {
  fetchEquityCurve,
  fetchPerformance,
  fetchTradingOverview,
} from '@/services/trading/analytics';
import { fetchDecision, fetchDecisions, fetchPositions } from '@/services/trading/observation';
import { formatDecimal } from '@/utils/decimal';
import styles from './Welcome.less';

type ChartMode = 'equity' | 'market';

const emptyOverview: TradingAPI.DashboardOverview = {
  total_equity: '0',
  available_balance: '0',
  realized_pnl: '0',
  unrealized_pnl: '0',
  funding: '0',
  total_fee: '0',
  net_pnl: '0',
  position_count: 0,
  closed_trade_count: 0,
};

const emptyPerformance: TradingAPI.PerformanceSummary = {
  total_trades: 0,
  winning_trades: 0,
  losing_trades: 0,
  break_even_trades: 0,
  win_rate: 0,
  gross_profit: '0',
  gross_loss: '0',
  gross_pnl: '0',
  total_fee: '0',
  funding: '0',
  net_pnl: '0',
  average_win: '0',
  average_loss: '0',
  profit_factor: 0,
  profit_loss_ratio: 0,
  trade_sharpe: 0,
  max_drawdown_pct: 0,
  sharpe_basis: 'closed_trade_return',
  drawdown_source: 'account_snapshots',
  by_side: [],
  by_symbol: [],
  by_trader: [],
  by_strategy: [],
  by_exchange: [],
};

const asNumber = (value: unknown) => {
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
};

const amount = (value: unknown, available = true) => {
  const number = asNumber(value);
  if (!available || number === undefined) return '--';
  return `${formatDecimal(value, { grouping: true, maximumFractionDigits: 2 })} USDT`;
};

const decimalOrDash = (value: unknown, meaningful = true) => {
  const number = asNumber(value);
  if (!meaningful || number === undefined || number === 0) return '--';
  return formatDecimal(value, { grouping: true, maximumFractionDigits: 8 });
};

const pnlClass = (value: unknown) => (Number(value) >= 0 ? styles.positive : styles.negative);

const toList = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value;
  if (typeof value === 'string') {
    try {
      return toList(JSON.parse(value));
    } catch {
      return value.trim() ? [value] : [];
    }
  }
  if (value && typeof value === 'object') return [value];
  return [];
};

const field = (value: unknown, keys: string[]) => {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  for (const key of keys) {
    if (record[key] !== undefined && record[key] !== null && String(record[key]).trim()) {
      return String(record[key]);
    }
  }
  return undefined;
};

const itemLabel = (value: unknown, fallback: string) => {
  if (typeof value === 'string') return value;
  return field(value, ['symbol', 'coin', 'asset', 'instrument', 'name']) ?? fallback;
};

const actionLabel = (value: unknown) => {
  if (typeof value === 'string') return value;
  const symbol = field(value, ['symbol', 'coin', 'asset', 'instrument']);
  const action = field(value, ['action', 'decision', 'side', 'operation', 'type']);
  return [symbol, action].filter(Boolean).join(' · ') || '结构化动作';
};

const readablePayload = (value: unknown) => {
  if (value === undefined || value === null || value === '') return '--';
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return JSON.stringify(value, null, 2);
};

const Welcome: React.FC = () => {
  const [exchangeID, setExchangeID] = useState<string>();
  const [exchanges, setExchanges] = useState<TradingAPI.Exchange[]>([]);
  const [overview, setOverview] = useState(emptyOverview);
  const [performance, setPerformance] = useState(emptyPerformance);
  const [equity, setEquity] = useState<TradingAPI.EquityPoint[]>([]);
  const [positions, setPositions] = useState<TradingAPI.Position[]>([]);
  const [decisions, setDecisions] = useState<TradingAPI.Decision[]>([]);
  const [decisionDetails, setDecisionDetails] = useState<Record<string, TradingAPI.DecisionDetail>>({});
  const [decisionDetailLoading, setDecisionDetailLoading] = useState<Record<string, boolean>>({});
  const [traders, setTraders] = useState<TradingAPI.Trader[]>([]);
  const [chartMode, setChartMode] = useState<ChartMode>('equity');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

  useEffect(() => {
    fetchExchanges({ current: 1, pageSize: 100 })
      .then((response) => setExchanges(response.data ?? []))
      .catch(() => setExchanges([]));
  }, []);

  const loadDashboard = useCallback(
    async (quiet = false) => {
      if (!quiet) setLoading(true);
      const params = exchangeID ? { exchangeID } : {};
      try {
        const [
          overviewResponse,
          performanceResponse,
          equityResponse,
          positionsResponse,
          decisionsResponse,
          tradersResponse,
        ] = await Promise.all([
          fetchTradingOverview(params),
          fetchPerformance(params),
          fetchEquityCurve(params),
          fetchPositions({
            current: 1,
            pageSize: 8,
            status: 'open',
            ...params,
          } as API.PaginationParam),
          fetchDecisions({ current: 1, pageSize: 5, ...params } as API.PaginationParam),
          fetchTraders({ current: 1, pageSize: 100 }),
        ]);
        setOverview(overviewResponse.data ?? emptyOverview);
        setPerformance(performanceResponse.data ?? emptyPerformance);
        setEquity(equityResponse.data ?? []);
        setPositions(positionsResponse.data ?? []);
        setDecisions(decisionsResponse.data ?? []);
        setTraders(
          (tradersResponse.data ?? []).filter(
            (item) => !exchangeID || item.exchange_id === exchangeID,
          ),
        );
        setLoadError('');
      } catch (error) {
        setLoadError(error instanceof Error ? error.message : '交易工作台加载失败');
        if (!quiet) message.error('交易工作台加载失败');
      } finally {
        if (!quiet) setLoading(false);
      }
    },
    [exchangeID],
  );

  useEffect(() => {
    void loadDashboard();
    const timer = window.setInterval(() => void loadDashboard(true), 30000);
    return () => window.clearInterval(timer);
  }, [loadDashboard]);

  const loadDecisionDetail = useCallback(
    async (id: number) => {
      const key = String(id);
      if (decisionDetails[key] || decisionDetailLoading[key]) return;
      setDecisionDetailLoading((current) => ({ ...current, [key]: true }));
      try {
        const response = await fetchDecision(id);
        if (response.data) {
          setDecisionDetails((current) => ({ ...current, [key]: response.data! }));
        }
      } catch (error) {
        message.error(error instanceof Error ? error.message : '决策详情加载失败');
      } finally {
        setDecisionDetailLoading((current) => ({ ...current, [key]: false }));
      }
    },
    [decisionDetailLoading, decisionDetails],
  );

  const exchangeOptions = useMemo(
    () => [
      { label: '全部交易所账户', value: '' },
      ...exchanges.map((item) => ({
        label: `${item.name || item.account_name} · ${item.exchange_type.toUpperCase()} ${item.account_type}`,
        value: item.id,
      })),
    ],
    [exchanges],
  );
  const selectedExchange = exchanges.find((item) => item.id === exchangeID);
  const preferredMarketExchange = ['binance', 'okx'].includes(
    selectedExchange?.exchange_type?.toLowerCase() ?? '',
  )
    ? (selectedExchange?.exchange_type.toLowerCase() as TradingAPI.MarketExchange)
    : undefined;
  const hasAccountData = Boolean(overview.last_updated_at);
  const totalEquity = asNumber(overview.total_equity);
  const availableBalance = asNumber(overview.available_balance);
  const availableRate =
    totalEquity && availableBalance !== undefined && totalEquity > 0
      ? (availableBalance / totalEquity) * 100
      : undefined;
  const hasPerformance = performance.total_trades > 0;
  const runtimeState = (value: string) => ({
    running: { label: '运行中', color: 'success' },
    starting: { label: '启动中', color: 'processing' },
    stopped: { label: '已停止', color: 'default' },
    error: { label: '异常', color: 'error' },
    unavailable: { label: '未运行', color: 'default' },
  }[value] ?? { label: value || '未知', color: 'default' });
  const strategyState = (value: string) => ({
    ready: { label: '策略就绪', color: 'success' },
    waiting: { label: '等待策略', color: 'warning' },
    running: { label: 'AI 分析中', color: 'processing' },
    paused: { label: '策略暂停', color: 'warning' },
    error: { label: '策略异常', color: 'error' },
  }[value] ?? { label: value || '策略未知', color: 'default' });
  const executionMode = (value: TradingAPI.Trader['execution_mode']) => ({
    notify_only: '仅通知',
    approval_required: '审批后执行',
    auto: '自动执行',
  }[value] ?? value);
  const isHeartbeatLate = (item: TradingAPI.Trader) => {
    if (item.runtime_state !== 'running' || !item.last_heartbeat_at) return false;
    const elapsed = Date.now() - new Date(item.last_heartbeat_at).getTime();
    return elapsed > Math.max(item.scan_interval_minutes * 2, 10) * 60_000;
  };

  const positionColumns: ColumnsType<TradingAPI.Position> = [
    {
      title: '交易对',
      dataIndex: 'symbol',
      width: 126,
      fixed: 'left',
      render: (value) => <Typography.Text strong>{value}</Typography.Text>,
    },
    {
      title: '方向',
      dataIndex: 'side',
      width: 86,
      render: (value: string) => (
        <Tag color={value === 'long' ? 'success' : 'error'}>{value.toUpperCase()}</Tag>
      ),
    },
    {
      title: '开仓价',
      dataIndex: 'entry_price',
      align: 'right',
      width: 120,
      render: (value) => decimalOrDash(value),
    },
    {
      title: '标记价',
      dataIndex: 'mark_price',
      align: 'right',
      width: 120,
      render: (value) => decimalOrDash(value),
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      align: 'right',
      width: 120,
      render: (value) => decimalOrDash(value),
    },
    {
      title: '仓位价值',
      align: 'right',
      width: 130,
      render: (_, record) => {
        const value = Math.abs(Number(record.quantity) * Number(record.mark_price));
        return Number.isFinite(value) && value > 0 ? amount(value) : '--';
      },
    },
    {
      title: '杠杆',
      dataIndex: 'leverage',
      align: 'center',
      width: 76,
      render: (value: number) => (value > 1 ? `${value}x` : '--'),
    },
    {
      title: '未实现盈亏',
      dataIndex: 'unrealized_pnl',
      align: 'right',
      width: 130,
      render: (value: string) => (
        <Typography.Text strong className={pnlClass(value)}>
          {amount(value)}
        </Typography.Text>
      ),
    },
    {
      title: '强平价',
      dataIndex: 'liquidation_price',
      align: 'right',
      width: 120,
      render: (value) => decimalOrDash(value),
    },
    {
      title: '更新时间',
      dataIndex: 'last_synced_at',
      width: 170,
      render: (value) => (value ? new Date(value).toLocaleString() : '--'),
    },
  ];

  const metricCards = [
    {
      title: '总权益',
      value: amount(overview.total_equity, hasAccountData),
      icon: <DollarOutlined />,
      hint: hasAccountData
        ? `更新于 ${new Date(overview.last_updated_at!).toLocaleTimeString()}`
        : '等待账户同步',
    },
    {
      title: '可用余额',
      value: amount(overview.available_balance, hasAccountData),
      icon: <DatabaseOutlined />,
      hint: availableRate === undefined ? '可用率 --' : `可用率 ${availableRate.toFixed(1)}%`,
    },
    {
      title: '净盈亏',
      value: amount(overview.net_pnl, hasAccountData),
      icon: <FundOutlined />,
      valueClass: pnlClass(overview.net_pnl),
      hint: hasAccountData
        ? `已实现 ${amount(overview.realized_pnl)} · 未实现 ${amount(overview.unrealized_pnl)}`
        : '等待收益数据',
    },
    {
      title: '当前仓位',
      value: hasAccountData ? `${overview.position_count}` : '--',
      suffix: '个',
      icon: <SwapOutlined />,
      hint: hasAccountData ? `已完成 ${overview.closed_trade_count} 笔完整交易` : '等待仓位同步',
    },
  ];

  const performanceCards = [
    {
      label: '完整交易',
      value: hasPerformance ? `${performance.total_trades}` : '--',
      suffix: '笔',
    },
    { label: '胜率', value: hasPerformance ? performance.win_rate.toFixed(1) : '--', suffix: '%' },
    {
      label: '净盈亏',
      value: amount(performance.net_pnl, hasPerformance),
      className: pnlClass(performance.net_pnl),
    },
    { label: 'Profit Factor', value: hasPerformance ? performance.profit_factor.toFixed(2) : '--' },
    { label: '交易 Sharpe', value: hasPerformance ? performance.trade_sharpe.toFixed(2) : '--' },
    {
      label: '最大回撤',
      value: hasPerformance ? performance.max_drawdown_pct.toFixed(2) : '--',
      suffix: '%',
    },
  ];

  return (
    <PageContainer
      className={styles.page}
      title="交易工作台"
      subTitle="账户、行情、决策与风险状态"
      extra={[
        <Select
          key="exchange"
          className={styles.accountSelect}
          value={exchangeID ?? ''}
          options={exchangeOptions}
          onChange={(value) => setExchangeID(value || undefined)}
        />,
        <Tooltip title="刷新工作台" key="reload">
          <Button
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => void loadDashboard()}
          />
        </Tooltip>,
      ]}
    >
      {loadError && <Alert className={styles.alert} type="error" showIcon message={loadError} />}

      <Row gutter={[12, 12]}>
        {metricCards.map((item) => (
          <Col xs={24} sm={12} xl={6} key={item.title}>
            <Card className={styles.metricCard}>
              <Skeleton active loading={loading} paragraph={false}>
                <div className={styles.metricHeader}>
                  <span className={styles.metricIcon}>{item.icon}</span>
                  <Typography.Text type="secondary">{item.title}</Typography.Text>
                </div>
                <div className={`${styles.metricValue} ${item.valueClass ?? ''}`}>
                  {item.value}
                  {item.suffix && <small>{item.suffix}</small>}
                </div>
                <Typography.Text type="secondary" className={styles.metricHint}>
                  {item.hint}
                </Typography.Text>
              </Skeleton>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[12, 12]} className={styles.sectionRow}>
        <Col xs={24} xxl={14}>
          <Card
            className={styles.panelCard}
            title={
              <Space>
                <AreaChartOutlined />
                账户表现
              </Space>
            }
            extra={
              <Segmented
                size="small"
                value={chartMode}
                options={[
                  { label: '权益曲线', value: 'equity', icon: <AreaChartOutlined /> },
                  { label: '实时行情', value: 'market', icon: <StockOutlined /> },
                ]}
                onChange={(value) => setChartMode(value as ChartMode)}
              />
            }
          >
            <Skeleton active loading={loading && chartMode === 'equity'}>
              {chartMode === 'equity' ? (
                <EquityCurve data={equity} />
              ) : (
                <DashboardMarketChart preferredExchange={preferredMarketExchange} />
              )}
            </Skeleton>
          </Card>
        </Col>

        <Col xs={24} xxl={10}>
          <Card
            className={styles.panelCard}
            title={
              <Space>
                <BarChartOutlined />
                最近决策
              </Space>
            }
            extra={
              <Button type="link" size="small" onClick={() => history.push('/trading/decisions')}>
                全部记录 <RightOutlined />
              </Button>
            }
          >
            <Skeleton active loading={loading}>
              {decisions.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无决策记录" />
              ) : (
                <Collapse
                  ghost
                  className={styles.decisionList}
                  onChange={(activeKeys) => {
                    const keys = Array.isArray(activeKeys) ? activeKeys : [activeKeys];
                    keys.filter(Boolean).forEach((key) => void loadDecisionDetail(Number(key)));
                  }}
                  items={decisions.map((item) => {
                    const candidates = toList(item.candidate_coins);
                    const actions = toList(item.actions);
                    const logs = toList(item.execution_log);
                    const detail = decisionDetails[String(item.id)];
                    const detailLoading = decisionDetailLoading[String(item.id)];
                    return {
                      key: item.id,
                      label: (
                        <div className={styles.decisionLabel}>
                          <div>
                            <Space size={8} wrap>
                              <Typography.Text strong>周期 #{item.cycle_number}</Typography.Text>
                              <Tag color={item.success ? 'success' : 'error'}>
                                {item.success ? '成功' : '失败'}
                              </Tag>
                            </Space>
                            <div className={styles.decisionMeta}>
                              <ClockCircleOutlined /> {new Date(item.decided_at).toLocaleString()} ·
                              AI {item.ai_request_duration_ms} ms
                            </div>
                          </div>
                          <Typography.Text type="secondary">
                            {candidates.length} 候选 · {actions.length} 动作
                          </Typography.Text>
                        </div>
                      ),
                      children: (
                        <div className={styles.decisionDetail}>
                          {item.error_message && (
                            <Alert type="error" showIcon message={item.error_message} />
                          )}
                          <section>
                            <Typography.Text type="secondary">候选币种</Typography.Text>
                            <div className={styles.tagList}>
                              {candidates.length > 0 ? (
                                candidates
                                  .slice(0, 12)
                                  .map((candidate, index) => (
                                    <Tag key={`${item.id}-candidate-${index}`}>
                                      {itemLabel(candidate, `候选 ${index + 1}`)}
                                    </Tag>
                                  ))
                              ) : (
                                <span>--</span>
                              )}
                            </div>
                          </section>
                          <section>
                            <Typography.Text type="secondary">决策动作</Typography.Text>
                            <div className={styles.tagList}>
                              {actions.length > 0 ? (
                                actions.map((action, index) => (
                                  <Tag color="processing" key={`${item.id}-action-${index}`}>
                                    {actionLabel(action)}
                                  </Tag>
                                ))
                              ) : (
                                <span>--</span>
                              )}
                            </div>
                          </section>
                          <section>
                            <Typography.Text type="secondary">执行结果</Typography.Text>
                            <div className={styles.executionLog}>
                              {logs.length > 0
                                ? logs
                                    .slice(0, 6)
                                    .map((log, index) => (
                                      <div key={`${item.id}-log-${index}`}>
                                        {typeof log === 'string' ? log : JSON.stringify(log)}
                                      </div>
                                    ))
                                : '--'}
                            </div>
                          </section>
                          {detailLoading && <Skeleton active paragraph={{ rows: 4 }} title={false} />}
                          {detail && (
                            <>
                              <section>
                                <Typography.Text type="secondary">决策输入</Typography.Text>
                                <pre className={styles.decisionPayload}>{readablePayload(detail.input_prompt)}</pre>
                              </section>
                              <section>
                                <Typography.Text type="secondary">模型原始输出</Typography.Text>
                                <pre className={styles.decisionPayload}>{readablePayload(detail.raw_response)}</pre>
                              </section>
                              <section>
                                <Typography.Text type="secondary">结构化决策</Typography.Text>
                                <pre className={styles.decisionPayload}>{readablePayload(detail.decision)}</pre>
                              </section>
                            </>
                          )}
                        </div>
                      ),
                    };
                  })}
                />
              )}
            </Skeleton>
          </Card>
        </Col>
      </Row>

      <Card
        title={
          <Space>
            <TrophyOutlined />
            交易表现
          </Space>
        }
        className={styles.performanceCard}
        extra={
          <Button type="link" size="small" onClick={() => history.push('/trading-data/analytics')}>
            查看完整分析 <RightOutlined />
          </Button>
        }
      >
        <div className={styles.performanceGrid}>
          {performanceCards.map((item) => (
            <div key={item.label} className={styles.performanceMetric}>
              <Typography.Text type="secondary">{item.label}</Typography.Text>
              <strong className={item.className}>
                {item.value}
                {item.suffix && <small>{item.suffix}</small>}
              </strong>
            </div>
          ))}
        </div>
        {hasPerformance && (
          <div className={styles.performanceFootnote}>
            盈利 {performance.winning_trades} · 亏损 {performance.losing_trades} · 手续费{' '}
            {amount(performance.total_fee)} · 资金费 {amount(performance.funding)}
          </div>
        )}
      </Card>

      <Card
        title={
          <Space>
            <RobotOutlined />
            Trader Runtime
          </Space>
        }
        className={styles.runtimeCard}
        extra={
          <Button type="link" size="small" onClick={() => history.push('/trading/traders')}>
            Trader 管理 <RightOutlined />
          </Button>
        }
      >
        <Skeleton active loading={loading}>
          {traders.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无 Trader 配置" />
          ) : (
            <div className={styles.runtimeList}>
              {traders.slice(0, 8).map((item) => {
                const state = runtimeState(item.runtime_state);
                const strategy = strategyState(item.strategy_state);
                const heartbeatLate = isHeartbeatLate(item);
                const aiConsentActive = Boolean(
                  item.ai_consent_at && item.ai_consent_model_id === item.ai_model_id,
                );
                return (
                  <div className={styles.runtimeItem} key={item.id}>
                    <div className={styles.runtimeIdentity}>
                      <Space size={8} wrap>
                        <Typography.Text strong>{item.name}</Typography.Text>
                        <Tag color={state.color}>{state.label}</Tag>
                        <Tag color={item.is_simulation ? 'warning' : 'blue'}>
                          {item.is_simulation ? '模拟' : '实盘'}
                        </Tag>
                        <Tag color={strategy.color}>{strategy.label}</Tag>
                        <Tag color={item.safe_mode ? 'warning' : 'success'}>
                          {item.safe_mode ? '安全模式' : '正常模式'}
                        </Tag>
                        <Tag color={aiConsentActive ? 'success' : 'default'}>
                          {aiConsentActive ? 'AI 已授权' : 'AI 未授权'}
                        </Tag>
                        {heartbeatLate && <Tag color="error">心跳延迟</Tag>}
                      </Space>
                      {item.last_error_message && (
                        <Typography.Text type="danger" ellipsis={{ tooltip: item.last_error_message }}>
                          {item.last_error_message}
                        </Typography.Text>
                      )}
                      {item.pause_reason && (
                        <Typography.Text type="warning" ellipsis={{ tooltip: item.pause_reason }}>
                          暂停原因：{item.pause_reason}
                        </Typography.Text>
                      )}
                    </div>
                    <div className={styles.runtimeFacts}>
                      <span><small>扫描周期</small><strong>{item.scan_interval_minutes} 分钟</strong></span>
                      <span><small>执行模式</small><strong>{executionMode(item.execution_mode)}</strong></span>
                      <span><small>策略版本</small><strong>{item.strategy_version > 0 ? `v${item.strategy_version}` : '--'}</strong></span>
                      <span><small>决策周期</small><strong>{item.cycle_number > 0 ? `#${item.cycle_number}` : '--'}</strong></span>
                      <span><small>循环耗时</small><strong>{item.last_cycle_duration_ms > 0 ? `${item.last_cycle_duration_ms} ms` : '--'}</strong></span>
                      <span><small>连续失败</small><strong>{item.consecutive_failures || 0}</strong></span>
                      <span><small>同步状态</small><strong>{item.sync_state || '--'}</strong></span>
                      <span><small>最后决策</small><strong>{item.last_decision_at ? new Date(item.last_decision_at).toLocaleString() : '--'}</strong></span>
                      <span><small>最后心跳</small><strong>{item.last_heartbeat_at ? new Date(item.last_heartbeat_at).toLocaleString() : '--'}</strong></span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Skeleton>
      </Card>

      <Card
        title={
          <Space>
            <StockOutlined />
            当前仓位
          </Space>
        }
        className={styles.positionsCard}
        extra={
          <Button type="link" size="small" onClick={() => history.push('/trading-data/positions')}>
            仓位记录 <RightOutlined />
          </Button>
        }
      >
        <Table
          rowKey="id"
          columns={positionColumns}
          dataSource={positions}
          loading={loading}
          pagination={false}
          scroll={{ x: 1260 }}
          size="middle"
          locale={{
            emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有持仓" />,
          }}
        />
      </Card>
    </PageContainer>
  );
};

export default Welcome;
