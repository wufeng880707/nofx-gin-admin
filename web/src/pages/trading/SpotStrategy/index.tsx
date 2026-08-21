import {
  AimOutlined,
  ControlOutlined,
  ExperimentOutlined,
  FundOutlined,
  PieChartOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  Button,
  Col,
  Collapse,
  DatePicker,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Row,
  Select,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import { getDefaultStrategyConfig } from '@/services/trading/catalog';
import {
  cancelSpotBacktest,
  createSpotBacktest,
  fetchSpotBacktests,
  fetchSpotLots,
  getSpotBacktestResult,
  type SpotBacktestCreateForm,
} from '@/services/trading/spotStrategy';
import { formatDecimal } from '@/utils/decimal';
import styles from './index.less';

const { Text, Title } = Typography;
const { RangePicker } = DatePicker;

type FormValue = {
  exchange: 'binance' | 'okx';
  symbol: string;
  interval: '4h' | '1d';
  initialCapital: number;
  range: [dayjs.Dayjs, dayjs.Dayjs];
  coreBudgetRatio: number;
  satelliteBudgetRatio: number;
  minimumCashRatio: number;
  lowAreaDrawdownPct: number;
  satelliteTakeProfitRatio: number;
  maxStrategyDrawdownPct: number;
  maxSymbolExposureRatio: number;
  maxDailyBuyRatio: number;
  lowAreaLookbackBars: number;
  trendFilterPeriod: number;
  stabilizationBars: number;
  accumulationCooldownBars: number;
  resistanceLookbackBars: number;
  breakoutConfirmBars: number;
  breakoutVolumeMultiplier: number;
  satelliteSellRatio: number;
  distributionLookbackBars: number;
  distributionStepGainPct: number;
  distributionStepSellRatio: number;
  maxCoreDistributionRatio: number;
  coreMinimumRetentionRatio: number;
  enableDrawdownRiskControl: boolean;
  feeRateBPS: number;
  slippageBPS: number;
  stressSlippageBPS: number;
};

const percent = (value: number) => `${(value * 100).toFixed(2)}%`;
const decimal = (value: unknown, digits = 4) =>
  formatDecimal(value, { grouping: true, fallback: '0', maximumFractionDigits: digits });

const statusColor: Record<string, string> = {
  queued: 'default', loading_data: 'processing', running: 'processing', calculating: 'processing',
  completed: 'success', failed: 'error', cancelled: 'warning',
};

const acceptanceLabels: Record<string, string> = {
  lot_inventory_non_negative: 'Lot 库存不为负',
  minimum_cash_ratio: '最低现金比例',
  maximum_symbol_exposure: '最大币种成本敞口',
  maximum_daily_buy: '单日最大买入',
  max_drawdown: '最大回撤',
  net_return_positive: '总收益为正',
  profit_factor: 'Profit Factor（已实现）',
  drawdown_vs_buy_hold: '回撤低于买入持有',
};

const acceptanceRatioKeys = new Set([
  'minimum_cash_ratio',
  'maximum_symbol_exposure',
  'maximum_daily_buy',
  'max_drawdown',
  'net_return_positive',
  'drawdown_vs_buy_hold',
]);

const formatAcceptanceValue = (key: string, value: string) => {
  if (key === 'lot_inventory_non_negative') return value === 'true' ? '是' : '否';
  if (!acceptanceRatioKeys.has(key)) return value;
  const match = value.match(/^([<>]=?)?(-?\d+(?:\.\d+)?)$/);
  if (!match) return value;
  return `${match[1] ?? ''}${(Number(match[2]) * 100).toFixed(2)}%`;
};

const SpotStrategyPage: React.FC = () => {
  const [form] = Form.useForm<FormValue>();
  const drawdownRiskControlEnabled = Form.useWatch('enableDrawdownRiskControl', form) ?? true;
  const coreBudgetRatio = Form.useWatch('coreBudgetRatio', form) ?? 0;
  const satelliteBudgetRatio = Form.useWatch('satelliteBudgetRatio', form) ?? 0;
  const minimumCashRatio = Form.useWatch('minimumCashRatio', form) ?? 0;
  const capitalRatioTotal = coreBudgetRatio + satelliteBudgetRatio + minimumCashRatio;
  const capitalRatioValid = Math.abs(capitalRatioTotal - 100) < 0.000001;
  const actionRef = useRef<ActionType>();
  const [config, setConfig] = useState<TradingAPI.SpotCoreSatelliteConfig>();
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<TradingAPI.SpotBacktestResult>();
  const [resultLoading, setResultLoading] = useState(false);
  const [lots, setLots] = useState<TradingAPI.SpotPositionLot[]>([]);

  useEffect(() => {
    void getDefaultStrategyConfig().then((response) => {
      const next = response.data?.spot_core_satellite_config;
      if (!next) return;
      setConfig(next);
      form.setFieldsValue({
        exchange: next.exchange,
        symbol: next.symbol,
        interval: next.execution_timeframe,
        initialCapital: next.strategy_budget_usdt,
        range: [dayjs().subtract(3, 'year').startOf('day'), dayjs().subtract(1, 'day').endOf('day')],
        coreBudgetRatio: next.core_budget_ratio * 100,
        satelliteBudgetRatio: next.satellite_budget_ratio * 100,
        minimumCashRatio: next.minimum_cash_ratio * 100,
        lowAreaDrawdownPct: next.low_area_drawdown_pct * 100,
        satelliteTakeProfitRatio: next.satellite_take_profit_ratio * 100,
        maxStrategyDrawdownPct: next.max_strategy_drawdown_pct * 100,
        maxSymbolExposureRatio: next.max_symbol_exposure_ratio * 100,
        maxDailyBuyRatio: next.max_daily_buy_ratio * 100,
        lowAreaLookbackBars: next.low_area_lookback_bars,
        trendFilterPeriod: next.trend_filter_period,
        stabilizationBars: next.stabilization_bars,
        accumulationCooldownBars: next.accumulation_cooldown_bars,
        resistanceLookbackBars: next.resistance_lookback_bars,
        breakoutConfirmBars: next.breakout_confirm_bars,
        breakoutVolumeMultiplier: next.breakout_volume_multiplier,
        satelliteSellRatio: next.satellite_sell_ratio * 100,
        distributionLookbackBars: next.distribution_lookback_bars,
        distributionStepGainPct: next.distribution_step_gain_pct * 100,
        distributionStepSellRatio: next.distribution_step_sell_ratio * 100,
        maxCoreDistributionRatio: next.max_core_distribution_ratio * 100,
        coreMinimumRetentionRatio: next.core_minimum_retention_ratio * 100,
        enableDrawdownRiskControl: next.enable_drawdown_risk_control,
        feeRateBPS: next.fee_rate_bps,
        slippageBPS: next.slippage_bps,
        stressSlippageBPS: next.stress_slippage_bps,
      });
    });
    void fetchSpotLots({ current: 1, pageSize: 100 }).then((response) => setLots(response.data ?? []));
  }, [form]);

  const runBacktest = async () => {
    const values = await form.validateFields();
    if (!config) return;
    const capital = Number(values.initialCapital);
    const nextConfig: TradingAPI.SpotCoreSatelliteConfig = {
      ...config,
      exchange: values.exchange,
      symbol: values.symbol.trim().toUpperCase().replace('/', ''),
      execution_timeframe: values.interval,
      strategy_budget_usdt: capital,
      core_budget_ratio: values.coreBudgetRatio / 100,
      satellite_budget_ratio: values.satelliteBudgetRatio / 100,
      minimum_cash_ratio: values.minimumCashRatio / 100,
      max_symbol_exposure_ratio: values.maxSymbolExposureRatio / 100,
      low_area_drawdown_pct: values.lowAreaDrawdownPct / 100,
      low_area_lookback_bars: values.lowAreaLookbackBars,
      trend_filter_period: values.trendFilterPeriod,
      stabilization_bars: values.stabilizationBars,
      accumulation_cooldown_bars: values.accumulationCooldownBars,
      satellite_take_profit_ratio: values.satelliteTakeProfitRatio / 100,
      satellite_sell_ratio: values.satelliteSellRatio / 100,
      resistance_lookback_bars: values.resistanceLookbackBars,
      breakout_confirm_bars: values.breakoutConfirmBars,
      breakout_volume_multiplier: values.breakoutVolumeMultiplier,
      distribution_lookback_bars: values.distributionLookbackBars,
      distribution_step_gain_pct: values.distributionStepGainPct / 100,
      distribution_step_sell_ratio: values.distributionStepSellRatio / 100,
      max_core_distribution_ratio: values.maxCoreDistributionRatio / 100,
      core_minimum_retention_ratio: values.coreMinimumRetentionRatio / 100,
      enable_drawdown_risk_control: values.enableDrawdownRiskControl,
      max_strategy_drawdown_pct: values.maxStrategyDrawdownPct / 100,
      max_daily_buy_ratio: values.maxDailyBuyRatio / 100,
      fee_rate_bps: values.feeRateBPS,
      slippage_bps: values.slippageBPS,
      stress_slippage_bps: values.stressSlippageBPS,
      allow_live_execution: false,
    };
    const capitalRatio = nextConfig.core_budget_ratio + nextConfig.satellite_budget_ratio + nextConfig.minimum_cash_ratio;
    if (Math.abs(capitalRatio - 1) > 0.000001) {
      message.warning('底仓、波段仓和现金比例合计必须为 100%');
      return;
    }
    if (nextConfig.max_symbol_exposure_ratio > nextConfig.core_budget_ratio + nextConfig.satellite_budget_ratio) {
      message.warning('最大币种成本敞口不能高于底仓与波段仓预算之和');
      return;
    }
    if (nextConfig.stress_slippage_bps < nextConfig.slippage_bps) {
      message.warning('压力滑点不能低于基础滑点');
      return;
    }
    setSubmitting(true);
    try {
      const payload: SpotBacktestCreateForm = {
        exchange: values.exchange,
        symbol: nextConfig.symbol,
        interval: values.interval,
        start_time: values.range[0].toISOString(),
        end_time: values.range[1].toISOString(),
        initial_capital: String(capital),
        config: nextConfig,
        cost_scenario: { fee_rate_bps: values.feeRateBPS, slippage_bps: values.slippageBPS },
        benchmarks: ['buy_and_hold', 'fixed_dca', 'core_only'],
      };
      await createSpotBacktest(payload);
      message.success('回测任务已创建');
      actionRef.current?.reload();
    } finally {
      setSubmitting(false);
    }
  };

  const openResult = async (task: TradingAPI.SpotBacktestTask) => {
    setResultLoading(true);
    try {
      const response = await getSpotBacktestResult(task.id);
      setResult(response.data);
    } finally {
      setResultLoading(false);
    }
  };

  const columns: ProColumns<TradingAPI.SpotBacktestTask>[] = [
    { title: '交易对', dataIndex: 'symbol', width: 120, render: (_, row) => <Text strong>{row.symbol}</Text> },
    { title: '来源', dataIndex: 'exchange', width: 90, valueEnum: { binance: 'Binance', okx: 'OKX' } },
    { title: '周期', dataIndex: 'interval', width: 70, search: false },
    { title: '状态', dataIndex: 'status', width: 120, render: (_, row) => <Tag color={statusColor[row.status]}>{row.status}</Tag> },
    { title: '进度', dataIndex: 'progress', width: 140, search: false, render: (_, row) => <Progress percent={row.progress} size="small" /> },
    { title: '回测区间', search: false, render: (_, row) => `${dayjs(row.start_time).format('YYYY-MM-DD')} - ${dayjs(row.end_time).format('YYYY-MM-DD')}` },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', search: false, width: 170 },
    {
      title: '操作', valueType: 'option', width: 150, render: (_, row) => [
        row.status === 'completed' ? <Button key="result" type="link" onClick={() => void openResult(row)}>结果</Button> : null,
        !['completed', 'failed', 'cancelled'].includes(row.status) ? (
          <Button key="cancel" type="link" danger onClick={async () => { await cancelSpotBacktest(row.id); actionRef.current?.reload(); }}>取消</Button>
        ) : null,
      ],
    },
  ];

  return (
    <PageContainer title={false} className={styles.page}>
      <header className={styles.header}>
        <div className={styles.heading}>
          <span className={styles.icon}><FundOutlined /></span>
          <div><Title level={3}>现货仓位策略</Title><Text type="secondary">底仓与波段仓独立记账，卖出成交显式关联原始买入 lot</Text></div>
        </div>
        <Tag color="success">仅回测，不提交真实订单</Tag>
      </header>

      <section className={styles.configSection}>
        <div className={styles.configHeader}>
          <div className={styles.sectionTitle}><AimOutlined /><div><Text strong>回测参数</Text><Text type="secondary">市场、资金、信号与退出规则</Text></div></div>
          <Tag color={capitalRatioValid ? 'success' : 'error'}>资金合计 {capitalRatioTotal.toFixed(0)}%</Tag>
        </div>
        <Form form={form} layout="vertical" className={styles.configForm}>
          <div className={styles.parameterBand}>
            <div className={styles.bandTitle}><ControlOutlined /><Text strong>市场与执行</Text></div>
            <div className={styles.parameterGrid}>
              <Form.Item name="exchange" label="行情来源" rules={[{ required: true }]}><Select options={[{ label: 'Binance', value: 'binance' }, { label: 'OKX', value: 'okx' }]} /></Form.Item>
              <Form.Item name="symbol" label="现货交易对" rules={[{ required: true }]}><Input placeholder="BTCUSDT / BTC-USDT" /></Form.Item>
              <Form.Item name="interval" label="执行周期" rules={[{ required: true }]}><Select options={[{ label: '4 小时', value: '4h' }, { label: '1 天', value: '1d' }]} /></Form.Item>
              <Form.Item name="initialCapital" label="策略资金（USDT）" rules={[{ required: true }]}><InputNumber min={100} precision={2} className={styles.full} /></Form.Item>
              <Form.Item name="range" label="回测区间" rules={[{ required: true }]} className={styles.rangeField}><RangePicker className={styles.full} allowClear={false} /></Form.Item>
              <Form.Item name="feeRateBPS" label="单边手续费（BPS）" tooltip="每次买入和卖出分别收取；10 BPS 等于 0.10%。"><InputNumber min={0} max={100} className={styles.full} /></Form.Item>
              <Form.Item name="slippageBPS" label="单边基础滑点（BPS）" tooltip="买入价向上、卖出价向下调整；5 BPS 等于 0.05%。"><InputNumber min={0} max={100} className={styles.full} /></Form.Item>
            </div>
          </div>

          <div className={styles.parameterBand}>
            <div className={styles.bandTitle}><PieChartOutlined /><Text strong>资金与核心规则</Text></div>
            <div className={styles.parameterGrid}>
              <Form.Item name="coreBudgetRatio" label="底仓预算（%）" tooltip="用于长期核心仓的资金上限；波段卖出规则不能动用底仓 Lot。"><InputNumber min={1} max={70} className={styles.full} /></Form.Item>
              <Form.Item name="satelliteBudgetRatio" label="波段仓预算（%）" tooltip="用于震荡交易的独立资金上限；每次买入会生成独立 Lot。"><InputNumber min={1} max={70} className={styles.full} /></Form.Item>
              <Form.Item name="minimumCashRatio" label="最低现金保留（%）" tooltip="任何买入成交后，现金都不得低于策略初始资金的该比例。"><InputNumber min={20} max={90} className={styles.full} /></Form.Item>
              <Form.Item name="maxSymbolExposureRatio" label="最大 Lot 成本敞口（%）" tooltip="所有未核销 Lot 的含费成本占初始策略资金的上限，不按币价自然上涨后的市值计算。"><InputNumber min={1} max={80} className={styles.full} /></Form.Item>
              <Form.Item name="lowAreaDrawdownPct" label="低位区域门槛（%）" tooltip="相对滚动高点的回撤达到该比例，并满足企稳条件后，才允许开始底仓建仓。"><InputNumber min={1} max={90} className={styles.full} /></Form.Item>
              <Form.Item name="satelliteTakeProfitRatio" label="波段 Lot 净收益目标（%）" tooltip="Lot 达到该净收益目标只是可卖条件之一；还必须出现收盘突破和成交量确认。"><InputNumber min={0.1} max={100} step={0.1} className={styles.full} /></Form.Item>
              <Form.Item name="maxDailyBuyRatio" label="单日最大买入（%）" tooltip="按 UTC 自然日累计；4 小时周期不会在每根 K 线重置。"><InputNumber min={1} max={30} className={styles.full} /></Form.Item>
              <Form.Item name="enableDrawdownRiskControl" label="权益回撤风控" valuePropName="checked" tooltip="开启后，策略权益从历史峰值回撤达到停买线时进入 RISK_OFF；关闭后仍统计最大回撤，但不触发停买。" className={styles.switchField}><Switch checkedChildren="启用" unCheckedChildren="停用" /></Form.Item>
              <Form.Item name="maxStrategyDrawdownPct" label="策略回撤停买线（%）" tooltip="仅在权益回撤风控启用时生效；进入 RISK_OFF 后停止新买入，不会自动卖出底仓。"><InputNumber min={1} max={80} disabled={!drawdownRiskControlEnabled} className={styles.full} /></Form.Item>
              <Form.Item name="stressSlippageBPS" label="压力滑点（BPS）" tooltip="用于压力场景参数，必须不低于基础滑点；当前单次基础回测仍使用基础滑点。"><InputNumber min={0} max={500} className={styles.full} /></Form.Item>
            </div>
          </div>

          <Collapse
            className={styles.advanced}
            expandIconPosition="end"
            items={[
              {
                key: 'signal',
                label: <div className={styles.advancedLabel}><ControlOutlined /><span><Text strong>信号与状态参数</Text><Text type="secondary">回看周期、企稳与突破确认</Text></span></div>,
                children: <div className={styles.parameterGrid}>
                  <Form.Item name="lowAreaLookbackBars" label="低位回看 K 线" tooltip="用于计算滚动高点；1 天周期默认 365 根。"><InputNumber min={20} max={2000} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="trendFilterPeriod" label="长期均线周期" tooltip="用于确认长期趋势并决定是否进入核心仓分配状态。"><InputNumber min={20} max={1000} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="stabilizationBars" label="企稳确认 K 线" tooltip="最近若干根 K 线不再持续创新低后，才允许开始建仓。"><InputNumber min={1} max={50} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="accumulationCooldownBars" label="建仓冷却 K 线" tooltip="两次买入之间至少间隔的 K 线数量，避免连续快速补仓。"><InputNumber min={1} max={100} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="resistanceLookbackBars" label="压力位回看 K 线" tooltip="使用此前 K 线高点计算近期压力位。"><InputNumber min={5} max={500} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="breakoutConfirmBars" label="突破确认 K 线" tooltip="需要连续多少根收盘价位于压力位上方。"><InputNumber min={1} max={10} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="breakoutVolumeMultiplier" label="突破成交量倍数" tooltip="最新成交量相对回看期平均成交量的最低倍数。"><InputNumber min={1} max={10} step={0.1} className={styles.full} /></Form.Item>
                </div>,
              },
              {
                key: 'risk',
                label: <div className={styles.advancedLabel}><SafetyCertificateOutlined /><span><Text strong>退出与核心仓分配</Text><Text type="secondary">Lot 止盈和长期趋势减仓</Text></span></div>,
                children: <div className={styles.parameterGrid}>
                  <Form.Item name="satelliteSellRatio" label="达标 Lot 卖出比例（%）" tooltip="突破确认时，每个满足净收益目标的波段 Lot 卖出自身剩余数量的比例。"><InputNumber min={1} max={100} className={styles.full} /></Form.Item>
                  <Form.Item name="distributionLookbackBars" label="长期突破回看 K 线" tooltip="突破该长期窗口高点且长期均线向上时，进入核心仓分配状态。"><InputNumber min={20} max={2000} precision={0} className={styles.full} /></Form.Item>
                  <Form.Item name="distributionStepGainPct" label="核心仓分配涨幅（%）" tooltip="进入分配状态后，每继续上涨一个档位触发一次核心仓减仓。"><InputNumber min={1} max={50} step={0.1} className={styles.full} /></Form.Item>
                  <Form.Item name="distributionStepSellRatio" label="每档核心仓卖出（%）" tooltip="每个分配档位卖出当前核心仓剩余数量的比例。"><InputNumber min={1} max={50} step={0.1} className={styles.full} /></Form.Item>
                  <Form.Item name="maxCoreDistributionRatio" label="周期最大分配（%）" tooltip="一个策略周期内累计最多卖出原始核心仓数量的比例。"><InputNumber min={1} max={100} className={styles.full} /></Form.Item>
                  <Form.Item name="coreMinimumRetentionRatio" label="核心仓最低保留（%）" tooltip="分配后仍需保留的原始核心仓数量比例；设置为 0 表示允许全部分配。"><InputNumber min={0} max={100} className={styles.full} /></Form.Item>
                </div>,
              },
            ]}
          />
          <div className={styles.actionBar}>
            <Text type="secondary">底仓 {coreBudgetRatio.toFixed(0)}% · 波段仓 {satelliteBudgetRatio.toFixed(0)}% · 现金 {minimumCashRatio.toFixed(0)}%</Text>
            <Button type="primary" size="large" icon={<ExperimentOutlined />} loading={submitting} onClick={() => void runBacktest()}>开始回测</Button>
          </div>
        </Form>
      </section>

      <section className={styles.tableSection}>
        <ProTable<TradingAPI.SpotBacktestTask>
          rowKey="id" actionRef={actionRef} columns={columns} headerTitle="回测任务"
          toolBarRender={() => [<Button key="refresh" icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>刷新</Button>]}
          request={async (params) => {
            const response = await fetchSpotBacktests({ current: params.current, pageSize: params.pageSize, status: params.status as string, symbol: params.symbol as string });
            return { data: response.data ?? [], total: response.total ?? 0, success: true };
          }}
        />
      </section>

      <section className={styles.tableSection}>
        <Table<TradingAPI.SpotPositionLot>
          rowKey="id" dataSource={lots} pagination={{ pageSize: 10 }} title={() => <Text strong>实时候选 lot 账本</Text>}
          columns={[
            { title: 'Lot', dataIndex: 'id', width: 120, ellipsis: true },
            { title: '类型', dataIndex: 'bucket', render: (value) => <Tag color={value === 'core' ? 'gold' : 'blue'}>{value === 'core' ? '底仓' : '波段仓'}</Tag> },
            { title: '层级', dataIndex: 'layer_no' },
            { title: '买入均价', dataIndex: 'unit_cost', render: (value) => decimal(value) },
            { title: '原始数量', dataIndex: 'acquired_qty', render: (value) => decimal(value) },
            { title: '剩余数量', dataIndex: 'remaining_qty', render: (value) => decimal(value) },
            { title: '目标价', dataIndex: 'target_price', render: (value) => decimal(value) },
            { title: '状态', dataIndex: 'status' },
          ]}
        />
      </section>

      <Modal open={Boolean(result) || resultLoading} onCancel={() => setResult(undefined)} footer={null} width={1180} title="回测结果" loading={resultLoading}>
        {result && <BacktestResultView result={result} />}
      </Modal>
    </PageContainer>
  );
};

const BacktestResultView: React.FC<{ result: TradingAPI.SpotBacktestResult }> = ({ result }) => {
  const metrics = result.metrics;
  const profitFactorAvailable = metrics.profit_factor_available ?? metrics.trade_count > 0;
  const isNotApplicable = (row: TradingAPI.SpotAcceptanceItem) =>
    row.not_applicable || (row.key === 'profit_factor' && !profitFactorAvailable);
  return <div className={styles.result}>
    <Row gutter={[16, 16]}>
      <Col span={6}><Statistic title="期末权益" value={metrics.final_equity} precision={2} suffix="USDT" /></Col>
      <Col span={6}><Statistic title="总收益率（含未实现）" value={metrics.total_return * 100} precision={2} suffix="%" /></Col>
      <Col span={6}><Statistic title="最大回撤" value={metrics.max_drawdown * 100} precision={2} suffix="%" /></Col>
      <Col span={6}><Statistic title="Profit Factor（已实现）" value={profitFactorAvailable ? metrics.profit_factor : '-'} precision={profitFactorAvailable ? 2 : undefined} /></Col>
    </Row>
    <Tabs items={[
      { key: 'acceptance', label: '验收', children: <Table<TradingAPI.SpotAcceptanceItem> pagination={false} rowKey="key" dataSource={result.acceptance} columns={[
        { title: '指标', dataIndex: 'key', render: (value) => acceptanceLabels[value] ?? value },
        { title: '状态', dataIndex: 'passed', render: (passed, row) => isNotApplicable(row) ? <Tag>不适用</Tag> : passed ? <Tag color="success">通过</Tag> : <Tag color="error">未通过</Tag> },
        { title: '实际值', dataIndex: 'actual', render: (value, row) => isNotApplicable(row) ? 'N/A（没有已实现卖出）' : formatAcceptanceValue(row.key, value) },
        { title: '标准', dataIndex: 'expected', render: (value, row) => formatAcceptanceValue(row.key, value) },
      ]} /> },
      { key: 'benchmarks', label: '基准对比', children: <Table<TradingAPI.SpotBacktestBenchmark> pagination={false} rowKey="name" dataSource={result.benchmarks} columns={[
        { title: '基准', dataIndex: 'name' }, { title: '期末权益', dataIndex: 'final_equity', render: (value) => decimal(value) },
        { title: '收益率', dataIndex: 'total_return', render: (value) => percent(value) }, { title: '最大回撤', dataIndex: 'max_drawdown', render: (value) => percent(value) },
        { title: 'Calmar', dataIndex: 'calmar', render: (value) => decimal(value) },
      ]} /> },
      { key: 'lots', label: `逐笔 Lot（${result.lots.length}）`, children: <Table<TradingAPI.SpotBacktestLot> rowKey="id" dataSource={result.lots} pagination={{ pageSize: 8 }} columns={[
        { title: 'Lot', dataIndex: 'id' }, { title: '类型', dataIndex: 'bucket' }, { title: '层级', dataIndex: 'layer_no' },
        { title: '买入价', dataIndex: 'open_price', render: (value) => decimal(value) }, { title: '原始数量', dataIndex: 'acquired_qty', render: (value) => decimal(value) },
        { title: '剩余数量', dataIndex: 'remaining_qty', render: (value) => decimal(value) }, { title: '目标价', dataIndex: 'target_price', render: (value) => decimal(value) },
        { title: '已实现盈亏', dataIndex: 'realized_pnl', render: (value) => decimal(value) },
      ]} /> },
      { key: 'events', label: `状态事件（${result.events.length}）`, children: <Table<TradingAPI.SpotBacktestEvent> rowKey={(row, index) => `${row.time}-${row.type}-${index}`} dataSource={result.events} pagination={{ pageSize: 8 }} columns={[
        { title: '时间', dataIndex: 'time', render: (value) => dayjs.unix(value).format('YYYY-MM-DD HH:mm') },
        { title: '事件', dataIndex: 'type' }, { title: '状态', render: (_, row) => row.to_state ? `${row.from_state || '-'} → ${row.to_state}` : '-' },
        { title: 'Lot', dataIndex: 'lot_id' }, { title: '原因', dataIndex: 'reason' },
      ]} /> },
      { key: 'details', label: '统计详情', children: <Descriptions column={3} bordered items={[
        { key: 'realized', label: '已实现盈亏', children: `${decimal(metrics.realized_pnl, 2)} USDT` },
        { key: 'fees', label: '手续费', children: `${decimal(metrics.fees, 2)} USDT` },
        { key: 'turnover', label: '成交额', children: `${decimal(metrics.turnover, 2)} USDT` },
        { key: 'cash', label: '平均现金比例', children: percent(metrics.average_cash_ratio) },
        { key: 'min-cash', label: '最低现金比例', children: percent(metrics.minimum_cash_ratio) },
        { key: 'max-exposure', label: '最大币种敞口', children: percent(metrics.maximum_exposure_ratio) },
        { key: 'max-daily-buy', label: '最大单日买入', children: percent(metrics.maximum_daily_buy_ratio) },
        { key: 'trades', label: '卖出分配次数', children: metrics.trade_count },
        { key: 'wins', label: '盈利/亏损', children: `${metrics.winning_trades} / ${metrics.losing_trades}` },
      ]} /> },
    ]} />
  </div>;
};

export default SpotStrategyPage;
