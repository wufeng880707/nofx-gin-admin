import {
  BarChartOutlined,
  DownloadOutlined,
  FallOutlined,
  RiseOutlined,
  SearchOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Input,
  InputNumber,
  Row,
  Segmented,
  Select,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { fetchExchanges, fetchStrategies, fetchTraders } from '@/services/trading/catalog';
import {
  downloadPerformanceReport,
  fetchClosedTrades,
  fetchAIInvocationSummary,
  fetchPerformance,
  fetchPerformanceReport,
  type AnalyticsParams,
  type PerformanceReportParams,
} from '@/services/trading/analytics';
import { formatDecimal } from '@/utils/decimal';
import styles from './index.less';

type AnalyticsFilters = {
  exchangeID?: string;
  traderID?: string;
  strategyID?: string;
  strategyVersion?: number;
  market?: string;
  symbol?: string;
  side?: string;
  range?: [Dayjs, Dayjs] | null;
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

const toParams = (filters: AnalyticsFilters): AnalyticsParams => ({
  exchangeID: filters.exchangeID || undefined,
  traderID: filters.traderID || undefined,
  strategyID: filters.strategyID || undefined,
  strategyVersion: filters.strategyVersion || undefined,
  market: filters.market || undefined,
  symbol: filters.symbol?.trim().toUpperCase() || undefined,
  side: filters.side || undefined,
  startAt: filters.range?.[0]?.toISOString(),
  endAt: filters.range?.[1]?.toISOString(),
});

const money = (value: unknown) =>
  `${formatDecimal(value, { grouping: true, fallback: '0', maximumFractionDigits: 2 })} USDT`;

const number = (value: number, digits = 2) =>
  Number.isFinite(value) ? value.toFixed(digits) : '0';

const duration = (seconds: number) => {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)}h`;
  return `${(seconds / 86400).toFixed(1)}d`;
};

const pnlClassName = (value: unknown) => (Number(value) >= 0 ? styles.positive : styles.negative);
const pnlTone = (value: unknown) => (Number(value) >= 0 ? 'success' : 'error');

const TradingAnalytics: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [draft, setDraft] = useState<AnalyticsFilters>({});
  const [filters, setFilters] = useState<AnalyticsFilters>({});
  const [exchanges, setExchanges] = useState<TradingAPI.Exchange[]>([]);
  const [traders, setTraders] = useState<TradingAPI.Trader[]>([]);
  const [strategies, setStrategies] = useState<TradingAPI.Strategy[]>([]);
  const [performance, setPerformance] = useState(emptyPerformance);
  const [reportPeriod, setReportPeriod] = useState<PerformanceReportParams['period']>('day');
  const [report, setReport] = useState<TradingAPI.PerformanceReportBucket[]>([]);
  const [aiSummary, setAISummary] = useState<TradingAPI.AIInvocationSummary>();
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const queryParams = useMemo(() => toParams(filters), [filters]);

  useEffect(() => {
    Promise.all([
      fetchExchanges({ current: 1, pageSize: 100 }),
      fetchTraders({ current: 1, pageSize: 100 }),
      fetchStrategies({ current: 1, pageSize: 100 }),
    ])
      .then(([exchangeResponse, traderResponse, strategyResponse]) => {
        setExchanges(exchangeResponse.data ?? []);
        setTraders(traderResponse.data ?? []);
        setStrategies(strategyResponse.data ?? []);
      })
      .catch(() => {
        setExchanges([]);
        setTraders([]);
        setStrategies([]);
      });
  }, []);

  const loadPerformance = useCallback(async () => {
    setLoading(true);
    try {
      const [performanceResponse, aiResponse, reportResponse] = await Promise.all([
        fetchPerformance(queryParams),
        fetchAIInvocationSummary({
          traderID: queryParams.traderID,
          strategyID: queryParams.strategyID,
          startAt: queryParams.startAt,
          endAt: queryParams.endAt,
        }),
        fetchPerformanceReport({ ...queryParams, period: reportPeriod }),
      ]);
      setPerformance(performanceResponse.data ?? emptyPerformance);
      setAISummary(aiResponse.data);
      setReport(reportResponse.data ?? []);
    } catch {
      setPerformance(emptyPerformance);
      setAISummary(undefined);
      setReport([]);
      message.error('交易分析加载失败');
    } finally {
      setLoading(false);
    }
  }, [queryParams, reportPeriod]);

  useEffect(() => {
    void loadPerformance();
    actionRef.current?.reload();
  }, [loadPerformance]);

  const applyFilters = () => {
    setFilters({ ...draft, symbol: draft.symbol?.trim().toUpperCase() });
  };

  const resetFilters = () => {
    setDraft({});
    setFilters({});
  };

  const exportReport = async () => {
    setExporting(true);
    try {
      const blob = await downloadPerformanceReport({ ...queryParams, period: reportPeriod });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `xtrading-${reportPeriod}-report.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success('报表已导出');
    } catch {
      message.error('报表导出失败');
    } finally {
      setExporting(false);
    }
  };

  const sideSummary = (side: string) =>
    performance.by_side.find((item) => item.key.toLowerCase() === side) ?? {
      key: side,
      total_trades: 0,
      winning_trades: 0,
      win_rate: 0,
      net_pnl: '0',
      average_pnl: '0',
    };

  const longSummary = sideSummary('long');
  const shortSummary = sideSummary('short');
  const traderNames = useMemo(
    () => new Map(traders.map((item) => [item.id, item.name])),
    [traders],
  );
  const strategyNames = useMemo(
    () => new Map(strategies.map((item) => [item.id, item.name])),
    [strategies],
  );
  const exchangeNames = useMemo(
    () => new Map(exchanges.map((item) => [item.id, item.name || item.account_name])),
    [exchanges],
  );

  const attributionLabel = (type: 'symbol' | 'trader' | 'strategy' | 'exchange', key: string) => {
    if (key === 'unassigned') return '未归因';
    if (type === 'trader') return traderNames.get(key) || key;
    if (type === 'exchange') return exchangeNames.get(key) || key;
    if (type === 'strategy') {
      const [strategyID, version] = key.split(':v');
      return `${strategyNames.get(strategyID) || strategyID}${version ? ` v${version}` : ''}`;
    }
    return key;
  };

  const metrics = [
    { title: '完整交易', value: performance.total_trades, suffix: '笔', hint: `盈利 ${performance.winning_trades} / 亏损 ${performance.losing_trades}` },
    { title: '胜率', value: number(performance.win_rate, 1), suffix: '%' },
    { title: '净盈亏', value: money(performance.net_pnl), tone: pnlTone(performance.net_pnl), hint: `手续费 ${money(performance.total_fee)}` },
    { title: 'Profit Factor', value: number(performance.profit_factor), hint: '总盈利 / 总亏损' },
    { title: '盈亏比', value: number(performance.profit_loss_ratio), hint: '平均盈利 / 平均亏损' },
    { title: '交易 Sharpe', value: number(performance.trade_sharpe), hint: '基于每笔完整交易收益率' },
    { title: '最大回撤', value: number(performance.max_drawdown_pct), suffix: '%', hint: '基于 12 小时账户快照' },
    { title: '平均盈利', value: money(performance.average_win), tone: 'success' },
    { title: '平均亏损', value: money(performance.average_loss), tone: 'error' },
    { title: '资金费', value: money(performance.funding), tone: pnlTone(performance.funding) },
  ];

  const performanceColumns = (type: 'symbol' | 'trader' | 'strategy' | 'exchange') => [
    {
      title: type === 'symbol' ? '币种' : type === 'trader' ? 'Trader' : type === 'strategy' ? '策略版本' : '交易所账户',
      dataIndex: 'key',
      key: 'key',
      render: (value: string) => attributionLabel(type, value),
    },
    { title: '交易数', dataIndex: 'total_trades', key: 'total_trades', align: 'right' as const },
    {
      title: '胜率',
      dataIndex: 'win_rate',
      key: 'win_rate',
      align: 'right' as const,
      render: (value: number) => `${number(value, 1)}%`,
    },
    {
      title: '平均盈亏',
      dataIndex: 'average_pnl',
      key: 'average_pnl',
      align: 'right' as const,
      render: (value: string) => <span className={pnlClassName(value)}>{money(value)}</span>,
    },
    {
      title: '净盈亏',
      dataIndex: 'net_pnl',
      key: 'net_pnl',
      align: 'right' as const,
      render: (value: string) => <strong className={pnlClassName(value)}>{money(value)}</strong>,
    },
  ];

  const reportColumns = [
    { title: '周期开始', dataIndex: 'start_at', key: 'start_at', render: (value: string) => new Date(value).toLocaleString() },
    { title: '交易数', dataIndex: ['summary', 'total_trades'], key: 'trades', align: 'right' as const },
    { title: '胜率', dataIndex: ['summary', 'win_rate'], key: 'win_rate', align: 'right' as const, render: (value: number) => `${number(value, 1)}%` },
    { title: '毛盈亏', dataIndex: ['summary', 'gross_pnl'], key: 'gross_pnl', align: 'right' as const, render: (value: string) => <span className={pnlClassName(value)}>{money(value)}</span> },
    { title: '手续费', dataIndex: ['summary', 'total_fee'], key: 'fee', align: 'right' as const, render: (value: string) => money(value) },
    { title: '资金费', dataIndex: ['summary', 'funding'], key: 'funding', align: 'right' as const, render: (value: string) => money(value) },
    { title: '净盈亏', dataIndex: ['summary', 'net_pnl'], key: 'net_pnl', align: 'right' as const, render: (value: string) => <strong className={pnlClassName(value)}>{money(value)}</strong> },
  ];

  const tradeColumns: ProColumns<TradingAPI.ClosedTrade>[] = [
    { title: '交易对', dataIndex: 'symbol', width: 120, fixed: 'left' },
    {
      title: '方向',
      dataIndex: 'side',
      width: 90,
      render: (_, record) => (
        <Tag color={record.side === 'long' ? 'success' : 'error'}>{record.side.toUpperCase()}</Tag>
      ),
    },
    { title: '市场', dataIndex: 'market', width: 90 },
    {
      title: '归因',
      dataIndex: 'attribution_source',
      width: 90,
      render: (_, record) => <Tag color={record.attribution_source === 'system' ? 'processing' : 'default'}>{record.attribution_source === 'system' ? '系统' : '外部'}</Tag>,
    },
    { title: 'Trader', dataIndex: 'trader_id', width: 140, renderText: (value) => traderNames.get(String(value)) || value || '未归因' },
    {
      title: '策略版本',
      dataIndex: 'strategy_id',
      width: 160,
      render: (_, record) => record.strategy_id
        ? `${strategyNames.get(record.strategy_id) || record.strategy_id} v${record.strategy_version}`
        : '未归因',
    },
    { title: '开仓价', dataIndex: 'entry_price', align: 'right', width: 130, renderText: (value) => formatDecimal(value) },
    { title: '平仓价', dataIndex: 'exit_price', align: 'right', width: 130, renderText: (value) => formatDecimal(value) },
    { title: '数量', dataIndex: 'exit_quantity', align: 'right', width: 130, renderText: (value) => formatDecimal(value) },
    {
      title: '毛盈亏',
      dataIndex: 'gross_pnl',
      align: 'right',
      width: 130,
      render: (_, record) => <span className={pnlClassName(record.gross_pnl)}>{money(record.gross_pnl)}</span>,
    },
    { title: '手续费', dataIndex: 'fee', align: 'right', width: 120, renderText: (value) => money(value) },
    {
      title: '净盈亏',
      dataIndex: 'net_pnl',
      align: 'right',
      width: 140,
      render: (_, record) => <strong className={pnlClassName(record.net_pnl)}>{money(record.net_pnl)}</strong>,
    },
    { title: '持仓时长', dataIndex: 'duration_seconds', align: 'right', width: 100, renderText: (value) => duration(Number(value)) },
    { title: '成交数', dataIndex: 'source_fill_count', align: 'right', width: 90 },
    { title: '开仓时间', dataIndex: 'opened_at', valueType: 'dateTime', width: 180 },
    { title: '平仓时间', dataIndex: 'closed_at', valueType: 'dateTime', width: 180 },
  ];

  return (
    <PageContainer
      className={styles.page}
      title="交易分析"
      subTitle="完整开平仓口径，统一按 USDT 计价"
    >
      <Card className={styles.filterPanel}>
        <div className={styles.filterGrid}>
          <Select
            allowClear
            placeholder="全部交易所账户"
            value={draft.exchangeID}
            options={exchanges.map((item) => ({
              label: `${item.name || item.account_name} · ${item.exchange_type.toUpperCase()}`,
              value: item.id,
            }))}
            onChange={(value) => setDraft((current) => ({ ...current, exchangeID: value }))}
          />
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="全部 Trader"
            value={draft.traderID}
            options={traders.map((item) => ({ label: item.name, value: item.id }))}
            onChange={(value) => setDraft((current) => ({ ...current, traderID: value }))}
          />
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="全部策略"
            value={draft.strategyID}
            options={strategies.map((item) => ({ label: item.name, value: item.id }))}
            onChange={(value) => setDraft((current) => ({
              ...current,
              strategyID: value,
              strategyVersion: value ? current.strategyVersion : undefined,
            }))}
          />
          <InputNumber
            min={1}
            precision={0}
            placeholder="策略版本"
            disabled={!draft.strategyID}
            value={draft.strategyVersion}
            onChange={(value) => setDraft((current) => ({ ...current, strategyVersion: value ?? undefined }))}
          />
          <Segmented
            block
            value={draft.market ?? ''}
            options={[{ label: '全部市场', value: '' }, { label: '现货', value: 'spot' }, { label: '合约', value: 'futures' }]}
            onChange={(value) => setDraft((current) => ({ ...current, market: String(value) }))}
          />
          <Input
            allowClear
            placeholder="交易对，如 BTCUSDT"
            value={draft.symbol}
            onChange={(event) => setDraft((current) => ({ ...current, symbol: event.target.value }))}
          />
          <Segmented
            block
            value={draft.side ?? ''}
            options={[{ label: '全部方向', value: '' }, { label: 'Long', value: 'long' }, { label: 'Short', value: 'short' }]}
            onChange={(value) => setDraft((current) => ({ ...current, side: String(value) }))}
          />
          <DatePicker.RangePicker
            showTime
            className={styles.rangePicker}
            value={draft.range}
            onChange={(value) => setDraft((current) => ({ ...current, range: value as [Dayjs, Dayjs] | null }))}
          />
          <Space>
            <Button type="primary" icon={<SearchOutlined />} onClick={applyFilters}>查询</Button>
            <Button onClick={resetFilters}>重置</Button>
          </Space>
        </div>
      </Card>

      <Row gutter={[12, 12]} className={styles.section}>
        {metrics.map((item) => (
          <Col xs={24} sm={12} md={8} xl={6} xxl={4} key={item.title}>
            <Card className={styles.metricCard} loading={loading}>
              <Statistic
                title={item.title}
                value={item.value}
                suffix={item.suffix}
                valueStyle={{ color: item.tone ? `var(--ant-color-${item.tone})` : undefined }}
              />
              {item.hint && <Typography.Text type="secondary" className={styles.hint}>{item.hint}</Typography.Text>}
            </Card>
          </Col>
        ))}
      </Row>

      <Card title="AI 调用观测" className={styles.section} loading={loading}>
        <Row gutter={[12, 12]}>
          <Col xs={12} lg={6}><Statistic title="调用次数" value={aiSummary?.overall.calls ?? 0} /></Col>
          <Col xs={12} lg={6}><Statistic title="成功率" value={number(aiSummary?.overall.success_rate ?? 0, 1)} suffix="%" /></Col>
          <Col xs={12} lg={6}><Statistic title="Token" value={aiSummary?.overall.total_tokens ?? 0} /></Col>
          <Col xs={12} lg={6}><Statistic title="估算费用" value={formatDecimal(aiSummary?.overall.estimated_cost_usd ?? '0', { maximumFractionDigits: 8 })} suffix="USD" /></Col>
          <Col xs={12} lg={6}><Statistic title="平均延迟" value={number(aiSummary?.overall.average_latency_ms ?? 0, 0)} suffix="ms" /></Col>
          <Col xs={12} lg={6}><Statistic title="失败次数" value={aiSummary?.overall.failures ?? 0} /></Col>
        </Row>
      </Card>

      <Row gutter={[12, 12]} className={styles.section}>
        {[
          { title: 'LONG', icon: <RiseOutlined />, data: longSummary, className: styles.longCard },
          { title: 'SHORT', icon: <FallOutlined />, data: shortSummary, className: styles.shortCard },
        ].map((item) => (
          <Col xs={24} lg={12} key={item.title}>
            <Card title={<Space>{item.icon}{item.title}</Space>} className={item.className}>
              <Row gutter={[12, 12]}>
                <Col xs={12} sm={6}><Statistic title="交易数" value={item.data.total_trades} /></Col>
                <Col xs={12} sm={6}><Statistic title="胜率" value={number(item.data.win_rate, 1)} suffix="%" /></Col>
                <Col xs={12} sm={6}><Statistic title="平均盈亏" value={money(item.data.average_pnl)} /></Col>
                <Col xs={12} sm={6}><Statistic title="净盈亏" value={money(item.data.net_pnl)} /></Col>
              </Row>
            </Card>
          </Col>
        ))}
      </Row>

      <Card title={<Space><TrophyOutlined />绩效归因</Space>} className={styles.section}>
        <Tabs
          items={[
            { key: 'symbol', label: '币种', data: performance.by_symbol },
            { key: 'trader', label: 'Trader', data: performance.by_trader },
            { key: 'strategy', label: '策略版本', data: performance.by_strategy },
            { key: 'exchange', label: '交易所账户', data: performance.by_exchange },
          ].map((item) => ({
            key: item.key,
            label: item.label,
            children: (
              <Table
                rowKey="key"
                columns={performanceColumns(item.key as 'symbol' | 'trader' | 'strategy' | 'exchange')}
                dataSource={item.data}
                pagination={false}
                size="middle"
                scroll={{ x: 680 }}
                locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无完整交易统计" /> }}
              />
            ),
          }))}
        />
      </Card>

      <Card
        title="绩效报表"
        className={styles.section}
        extra={(
          <Space>
            <Segmented
              value={reportPeriod}
              options={[
                { label: '日', value: 'day' },
                { label: '周', value: 'week' },
                { label: '月', value: 'month' },
              ]}
              onChange={(value) => setReportPeriod(value as PerformanceReportParams['period'])}
            />
            <Button icon={<DownloadOutlined />} loading={exporting} onClick={() => void exportReport()}>
              导出 CSV
            </Button>
          </Space>
        )}
      >
        <Table
          rowKey="start_at"
          columns={reportColumns}
          dataSource={report}
          pagination={{ pageSize: 12, hideOnSinglePage: true }}
          size="middle"
          scroll={{ x: 980 }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前筛选范围暂无报表数据" /> }}
        />
      </Card>

      <ProTable<TradingAPI.ClosedTrade>
        className={styles.section}
        headerTitle={<Space><BarChartOutlined />完整交易明细</Space>}
        actionRef={actionRef}
        columns={tradeColumns}
        request={(params) => fetchClosedTrades({
          ...queryParams,
          current: params.current,
          pageSize: params.pageSize,
        })}
        rowKey="id"
        search={false}
        pagination={{ pageSize: 10, showSizeChanger: true }}
        scroll={{ x: 2180 }}
        options={{ density: true, reload: true, fullScreen: true }}
      />
    </PageContainer>
  );
};

export default TradingAnalytics;
