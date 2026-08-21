import {
  AimOutlined,
  BarChartOutlined,
  CheckOutlined,
  CopyOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  EyeOutlined,
  FundOutlined,
  HistoryOutlined,
  PlusOutlined,
  RocketOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Button,
  Checkbox,
  Empty,
  Input,
  InputNumber,
  List,
  message,
  Modal,
  Popconfirm,
  Radio,
  Segmented,
  Select,
  Slider,
  Space,
  Spin,
  Switch,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  theme,
} from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  activateStrategy,
  createStrategy,
  deleteStrategy,
  duplicateStrategy,
  estimateStrategyTokens,
  fetchStrategyVersions,
  fetchModels,
  fetchStrategies,
  getDefaultStrategyConfig,
  getStrategy,
  previewStrategy,
  publishStrategy,
  rollbackStrategy,
  testStrategy,
  updateStrategy,
} from '@/services/trading/catalog';
import styles from './index.less';

const { Text, Title, Paragraph } = Typography;

type Draft = {
  id?: string;
  owner_user_id?: string;
  name: string;
  description: string;
  is_active: boolean;
  is_default: boolean;
  current_version?: number;
  published_version?: number;
  config: TradingAPI.StrategyStudioConfig;
};

const timeframes = ['1m', '3m', '5m', '15m', '30m', '1h', '2h', '4h', '6h', '12h', '1d', '3d', '1w'];
const technicalIndicators = [
  ['enable_ema', 'EMA', '指数移动平均线'],
  ['enable_macd', 'MACD', '趋势与动量交叉'],
  ['enable_rsi', 'RSI', '相对强弱指标'],
  ['enable_atr', 'ATR', '真实波动幅度'],
  ['enable_boll', 'BOLL', '布林带区间'],
] as const;
const marketIndicators = [
  ['enable_volume', '成交量', '分析量价配合'],
  ['enable_oi', '持仓量 OI', '合约持仓变化'],
  ['enable_funding_rate', '资金费率', '永续合约情绪'],
] as const;
const quantIndicators = [
  ['enable_quant_oi', '量化 OI', '单币种 OI 聚合'],
  ['enable_quant_netflow', '资金流 NetFlow', '机构与个人资金流'],
  ['enable_oi_ranking', 'OI 排名', '全市场持仓变化榜'],
  ['enable_netflow_ranking', '资金流排名', '全市场资金流榜'],
  ['enable_price_ranking', '价格涨跌榜', '多周期涨跌排行'],
] as const;

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value));

const mergeConfig = (
  base: TradingAPI.StrategyStudioConfig,
  value?: Record<string, unknown>,
): TradingAPI.StrategyStudioConfig => {
  const source = value as Partial<TradingAPI.StrategyStudioConfig> | undefined;
  const ai = source?.ai_config;
  return {
    ...clone(base),
    ...source,
    strategy_type: ['grid', 'spot_core_satellite'].includes(source?.strategy_type ?? '')
      ? source!.strategy_type!
      : 'ai_trading',
    language: source?.language === 'en' ? 'en' : 'zh',
    grid_config: { ...base.grid_config, ...source?.grid_config },
    spot_core_satellite_config: {
      ...base.spot_core_satellite_config,
      ...source?.spot_core_satellite_config,
    },
    ai_config: {
      ...clone(base.ai_config),
      ...ai,
      coin_source: { ...base.ai_config.coin_source, ...ai?.coin_source },
      indicators: {
        ...base.ai_config.indicators,
        ...ai?.indicators,
        klines: { ...base.ai_config.indicators.klines, ...ai?.indicators?.klines },
      },
      risk_control: { ...base.ai_config.risk_control, ...ai?.risk_control },
      prompt_sections: { ...base.ai_config.prompt_sections, ...ai?.prompt_sections },
      custom_prompt: ai?.custom_prompt ?? '',
    },
  };
};

const StrategyStudio: React.FC = () => {
  const { token } = theme.useToken();
  const [strategies, setStrategies] = useState<TradingAPI.Strategy[]>([]);
  const [models, setModels] = useState<TradingAPI.AIModel[]>([]);
  const [defaultConfig, setDefaultConfig] = useState<TradingAPI.StrategyStudioConfig>();
  const defaultConfigRef = useRef<TradingAPI.StrategyStudioConfig>();
  const [draft, setDraft] = useState<Draft>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [testing, setTesting] = useState(false);
  const [promptVariant, setPromptVariant] = useState('balanced');
  const [modelID, setModelID] = useState<string>();
  const [preview, setPreview] = useState<TradingAPI.StrategyPreview>();
  const [tokenEstimate, setTokenEstimate] = useState<TradingAPI.StrategyTokenEstimate>();
  const [preflight, setPreflight] = useState<TradingAPI.StrategyTestResult>();
  const [rightTab, setRightTab] = useState('preview');
  const [versions, setVersions] = useState<TradingAPI.StrategyVersion[]>([]);
  const [versionsOpen, setVersionsOpen] = useState(false);
  const [versionAction, setVersionAction] = useState(false);

  const loadCatalog = useCallback(async () => {
    setLoading(true);
    try {
      const [strategyResult, modelResult, configResult] = await Promise.all([
        fetchStrategies({ current: 1, pageSize: 100 }),
        fetchModels({ current: 1, pageSize: 100 }),
        getDefaultStrategyConfig(),
      ]);
      const nextStrategies = strategyResult.data ?? [];
      const nextModels = (modelResult.data ?? []).filter((item) => item.enabled);
      setStrategies(nextStrategies);
      setModels(nextModels);
      setDefaultConfig(configResult.data);
      defaultConfigRef.current = configResult.data;
      setModelID((current) => current ?? nextModels[0]?.id);
      return { strategies: nextStrategies, config: configResult.data };
    } finally {
      setLoading(false);
    }
  }, []);

  const openStrategy = useCallback(
    async (strategy: TradingAPI.Strategy, base?: TradingAPI.StrategyStudioConfig) => {
      const configBase = base ?? defaultConfigRef.current;
      if (!configBase) return;
      setLoading(true);
      try {
        const [result, versionResult] = await Promise.all([
          getStrategy(strategy.id),
          fetchStrategyVersions(strategy.id),
        ]);
        if (!result.data) return;
        setVersions(versionResult.data ?? []);
        setDraft({
          id: result.data.id,
          owner_user_id: result.data.owner_user_id,
          name: result.data.name,
          description: result.data.description,
          is_active: result.data.is_active,
          is_default: result.data.is_default,
          current_version: result.data.current_version,
          published_version: result.data.published_version,
          config: mergeConfig(configBase, result.data.config),
        });
        setPreview(undefined);
        setTokenEstimate(undefined);
        setPreflight(undefined);
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    void loadCatalog().then(({ strategies: items, config }) => {
      if (items[0] && config) void openStrategy(items[0], config);
      else if (config) {
        setDraft({
          name: '新策略',
          description: '',
          is_active: false,
          is_default: false,
          config: clone(config),
        });
      }
    });
  }, [loadCatalog, openStrategy]);

  const createNew = () => {
    if (!defaultConfig) return;
    setDraft({ name: '新策略', description: '', is_active: false, is_default: false, config: clone(defaultConfig) });
    setPreview(undefined);
    setTokenEstimate(undefined);
    setPreflight(undefined);
    setVersions([]);
  };

  const updateConfig = (mutator: (next: TradingAPI.StrategyStudioConfig) => void) => {
    setDraft((current) => {
      if (!current) return current;
      const next = clone(current);
      mutator(next.config);
      return next;
    });
    setPreview(undefined);
    setTokenEstimate(undefined);
    setPreflight(undefined);
  };

  const save = async () => {
    if (!draft) return;
    if (!draft.name.trim()) {
      message.warning('请输入策略名称');
      return;
    }
    setSaving(true);
    try {
      const payload: TradingAPI.StrategyForm = {
        owner_user_id: draft.owner_user_id ?? '',
        name: draft.name.trim(),
        description: draft.description.trim(),
        strategy_type: draft.config.strategy_type,
        is_active: draft.is_active,
        is_default: draft.is_default,
        is_public: false,
        config_visible: true,
        config: draft.config as unknown as Record<string, unknown>,
      };
      let strategyID = draft.id;
      if (strategyID) await updateStrategy(strategyID, payload);
      else {
        const result = await createStrategy(payload);
        strategyID = result.data?.id;
      }
      message.success('策略已保存');
      const catalog = await loadCatalog();
      const saved = catalog.strategies.find((item) => item.id === strategyID);
      if (saved && catalog.config) await openStrategy(saved, catalog.config);
    } finally {
      setSaving(false);
    }
  };

  const runPreview = async () => {
    if (!draft) return;
    setPreviewing(true);
    try {
      const [previewResult, estimateResult] = await Promise.all([
        previewStrategy(draft.config, promptVariant),
        estimateStrategyTokens(draft.config),
      ]);
      setPreview(previewResult.data);
      setTokenEstimate(estimateResult.data);
      setRightTab('preview');
    } finally {
      setPreviewing(false);
    }
  };

  const runPreflight = async () => {
    if (!draft || !modelID) {
      message.warning('请先选择已启用的 AI 模型');
      return;
    }
    setTesting(true);
    try {
      const result = await testStrategy(draft.config, promptVariant, modelID);
      setPreflight(result.data);
      if (result.data) setPreview(result.data);
      setRightTab('test');
      message.success('策略 AI 干跑完成，未提交交易指令');
    } finally {
      setTesting(false);
    }
  };

  const removeCurrent = async () => {
    if (!draft?.id) return;
    await deleteStrategy(draft.id);
    message.success('策略已删除');
    setDraft(undefined);
    const result = await loadCatalog();
    if (result.strategies[0] && result.config) void openStrategy(result.strategies[0], result.config);
    else createNew();
  };

  const activateCurrent = async () => {
    if (!draft?.id || draft.is_active) return;
    await activateStrategy(draft.id);
    message.success('策略已激活，其他策略已自动停用');
    const result = await loadCatalog();
    const current = result.strategies.find((item) => item.id === draft.id);
    if (current && result.config) await openStrategy(current, result.config);
  };

  const duplicateCurrent = async () => {
    if (!draft?.id) return;
    const result = await duplicateStrategy(draft.id, `${draft.name} 副本`);
    message.success('策略副本已创建');
    const catalog = await loadCatalog();
    const duplicate = result.data && catalog.strategies.find((item) => item.id === result.data?.id);
    if (duplicate && catalog.config) await openStrategy(duplicate, catalog.config);
  };

  const publishCurrent = async () => {
    if (!draft?.id) return;
    setVersionAction(true);
    try {
      await publishStrategy(draft.id);
      message.success(`策略 v${draft.current_version ?? 1} 已发布`);
      const catalog = await loadCatalog();
      const current = catalog.strategies.find((item) => item.id === draft.id);
      if (current && catalog.config) await openStrategy(current, catalog.config);
    } finally {
      setVersionAction(false);
    }
  };

  const rollbackCurrent = async (version: number) => {
    if (!draft?.id) return;
    setVersionAction(true);
    try {
      const result = await rollbackStrategy(draft.id, version);
      message.success(`已基于 v${version} 创建新草稿 v${result.data?.current_version ?? ''}`);
      const catalog = await loadCatalog();
      const current = catalog.strategies.find((item) => item.id === draft.id);
      if (current && catalog.config) await openStrategy(current, catalog.config);
    } finally {
      setVersionAction(false);
    }
  };

  const coinSource = draft?.config.ai_config.coin_source;
  const indicators = draft?.config.ai_config.indicators;
  const risk = draft?.config.ai_config.risk_control;
  const grid = draft?.config.grid_config;
  const spot = draft?.config.spot_core_satellite_config;
  const activeCount = useMemo(() => strategies.filter((item) => item.is_active).length, [strategies]);
  const cssVars = {
    '--studio-bg': token.colorBgLayout,
    '--studio-panel': token.colorBgContainer,
    '--studio-subtle': token.colorFillAlter,
    '--studio-border': token.colorBorderSecondary,
    '--studio-text': token.colorText,
    '--studio-muted': token.colorTextSecondary,
    '--studio-primary': token.colorPrimary,
  } as React.CSSProperties;

  return (
    <PageContainer title={false} className={styles.page} style={cssVars}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <div className={styles.titleIcon}><ThunderboltOutlined /></div>
          <div>
            <Title level={3}>策略工作室</Title>
            <Text type="secondary">配置候选币种、指标、风险边界，并预览最终 Prompt</Text>
          </div>
        </div>
        <Space wrap>
          <Tag color="processing">{strategies.length} 个策略</Tag>
          <Tag color="success">{activeCount} 个已激活</Tag>
          {draft?.id && (
            <>
              <Tag>当前 v{draft.current_version ?? 1}</Tag>
              <Tag color={draft.published_version === draft.current_version ? 'success' : 'warning'}>
                {draft.published_version ? `已发布 v${draft.published_version}` : '未发布'}
              </Tag>
              <Tooltip title="版本历史"><Button icon={<HistoryOutlined />} onClick={() => setVersionsOpen(true)}>版本</Button></Tooltip>
              {draft.published_version !== draft.current_version && (
                <Popconfirm title="发布当前策略版本？" description="新启动的 Trader 将引用此版本。" onConfirm={publishCurrent}>
                  <Button icon={<RocketOutlined />} loading={versionAction}>发布</Button>
                </Popconfirm>
              )}
              <Tooltip title="复制策略"><Button icon={<CopyOutlined />} onClick={duplicateCurrent}>复制</Button></Tooltip>
              {!draft.is_active && (
                <Popconfirm title="确认激活当前策略？" description="同一用户的其他策略会自动停用。" onConfirm={activateCurrent}>
                  <Button icon={<CheckOutlined />}>激活</Button>
                </Popconfirm>
              )}
              <Popconfirm title="确认删除当前策略？" description="已被 Trader 使用的策略不能删除。" onConfirm={removeCurrent}>
                <Button danger icon={<DeleteOutlined />} aria-label="删除策略" />
              </Popconfirm>
            </>
          )}
          <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={save}>保存</Button>
        </Space>
      </header>

      <div className={styles.workspace}>
        <aside className={styles.strategyList}>
          <div className={styles.asideTitle}>
            <Text strong>策略</Text>
            <Tooltip title="新建策略"><Button type="text" icon={<PlusOutlined />} onClick={createNew} /></Tooltip>
          </div>
          {loading && !draft ? <Spin /> : strategies.length ? strategies.map((item) => (
            <button
              type="button"
              key={item.id}
              className={`${styles.strategyItem} ${draft?.id === item.id ? styles.selected : ''}`}
              onClick={() => void openStrategy(item)}
            >
              <span>{item.name}</span>
              {item.is_active && <CheckOutlined />}
            </button>
          )) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无已保存策略" />}
          {!draft?.id && draft && <button type="button" className={`${styles.strategyItem} ${styles.selected}`}><span>未保存策略</span></button>}
        </aside>

        <main className={styles.editor}>
          {!draft || !coinSource || !indicators || !risk ? <Spin /> : (
            <>
              <section className={styles.identity}>
                <div className={styles.identityFields}>
                  <Input value={draft.name} maxLength={128} onChange={(event) => setDraft({ ...draft, name: event.target.value })} className={styles.nameInput} placeholder="策略名称" />
                  <Input value={draft.description} maxLength={1000} onChange={(event) => setDraft({ ...draft, description: event.target.value })} placeholder="策略描述" />
                  <Select
                    value={draft.config.strategy_type}
                    options={[{ label: 'AI 决策策略', value: 'ai_trading' }, { label: '网格策略（仅模拟）', value: 'grid' }, { label: '现货底仓 + 波段仓', value: 'spot_core_satellite' }]}
                    onChange={(value) => updateConfig((next) => { next.strategy_type = value; })}
                  />
                </div>
                <Space>
                  <Text type="secondary">激活</Text>
                  <Switch checked={draft.is_active} onChange={(checked) => setDraft({ ...draft, is_active: checked })} />
                </Space>
              </section>

              {draft.config.strategy_type === 'grid' && grid && (
                <EditorSection icon={<DatabaseOutlined />} title="网格策略" subtitle="独立区间、资金和风险边界；当前仅生成模拟计划，不创建交易命令">
                  <div className={styles.riskGrid}>
                    <Field label="交易对"><Input value={grid.symbol} onChange={(event) => updateConfig((next) => { next.grid_config.symbol = event.target.value.toUpperCase().replace('/', ''); })} /></Field>
                    <Field label="市场"><Select value={grid.market} options={[{ label: '现货', value: 'spot' }, { label: '永续合约', value: 'futures' }]}
                      onChange={(value) => updateConfig((next) => { next.grid_config.market = value; if (value === 'spot') { next.grid_config.direction = 'neutral'; next.grid_config.leverage = 1; } })} /></Field>
                    <Field label="方向"><Select value={grid.direction} options={[{ label: '中性', value: 'neutral' }, { label: '做多', value: 'long' }, ...(grid.market === 'futures' ? [{ label: '做空', value: 'short' as const }] : [])]}
                      onChange={(value) => updateConfig((next) => { next.grid_config.direction = value; })} /></Field>
                    <Field label="网格模式"><Select value={grid.price_mode} options={[{ label: '等差', value: 'arithmetic' }, { label: '等比', value: 'geometric' }]}
                      onChange={(value) => updateConfig((next) => { next.grid_config.price_mode = value; })} /></Field>
                    <NumberField label="区间下限" value={grid.lower_price} min={0.00000001} max={1000000000} onChange={(value) => updateConfig((next) => { next.grid_config.lower_price = value; })} />
                    <NumberField label="区间上限" value={grid.upper_price} min={0.00000001} max={1000000000} onChange={(value) => updateConfig((next) => { next.grid_config.upper_price = value; })} />
                    <NumberField label="网格数量" value={grid.grid_count} min={2} max={200} onChange={(value) => updateConfig((next) => { next.grid_config.grid_count = value; })} />
                    <NumberField label="投入资金" value={grid.investment_usdt} min={10} max={100000000} suffix="USDT" onChange={(value) => updateConfig((next) => { next.grid_config.investment_usdt = value; })} />
                    <NumberField label="最大持仓" value={grid.max_position_usdt} min={10} max={grid.investment_usdt} suffix="USDT" onChange={(value) => updateConfig((next) => { next.grid_config.max_position_usdt = value; })} />
                    <NumberField label="最大挂单数" value={grid.max_open_orders} min={1} max={Math.min(50, grid.grid_count)} onChange={(value) => updateConfig((next) => { next.grid_config.max_open_orders = value; })} />
                    <NumberField label="杠杆" value={grid.leverage} min={1} max={grid.market === 'spot' ? 1 : 20} suffix="x" onChange={(value) => updateConfig((next) => { next.grid_config.leverage = value; })} />
                    <NumberField label="止损比例" value={grid.stop_loss_pct} min={0.1} max={50} step={0.1} suffix="%" onChange={(value) => updateConfig((next) => { next.grid_config.stop_loss_pct = value; })} />
                    <NumberField label="止盈比例" value={grid.take_profit_pct} min={0.1} max={200} step={0.1} suffix="%" onChange={(value) => updateConfig((next) => { next.grid_config.take_profit_pct = value; })} />
                    <NumberField label="最大回撤" value={grid.max_drawdown_pct} min={0.1} max={50} step={0.1} suffix="%" onChange={(value) => updateConfig((next) => { next.grid_config.max_drawdown_pct = value; })} />
                    <Field label={`预留资金 ${Math.round(grid.reserve_ratio * 100)}%`}><Slider min={5} max={90} value={Math.round(grid.reserve_ratio * 100)} onChange={(value) => updateConfig((next) => { next.grid_config.reserve_ratio = value / 100; })} /></Field>
                  </div>
                </EditorSection>
              )}

              {draft.config.strategy_type === 'spot_core_satellite' && spot && (
                <>
                  <EditorSection icon={<FundOutlined />} title="现货底仓 + 波段仓" subtitle="每笔买入独立形成 lot；卖出只核销满足条件且被明确选中的 lot">
                    <div className={styles.riskGrid}>
                      <Field label="行情来源"><Select value={spot.exchange} options={[{ label: 'Binance', value: 'binance' }, { label: 'OKX', value: 'okx' }]} onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.exchange = value; })} /></Field>
                      <Field label="现货交易对"><Input value={spot.symbol} onChange={(event) => updateConfig((next) => { next.spot_core_satellite_config.symbol = event.target.value.toUpperCase().replace('/', ''); })} /></Field>
                      <Field label="执行周期"><Select value={spot.execution_timeframe} options={[{ label: '4 小时', value: '4h' }, { label: '1 天', value: '1d' }]} onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.execution_timeframe = value; })} /></Field>
                      <NumberField label="策略资金" value={spot.strategy_budget_usdt} min={100} max={100000000} suffix="USDT" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.strategy_budget_usdt = value; })} />
                      <NumberField label="底仓比例" value={spot.core_budget_ratio * 100} min={1} max={70} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.core_budget_ratio = value / 100; })} />
                      <NumberField label="波段仓比例" value={spot.satellite_budget_ratio * 100} min={1} max={70} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.satellite_budget_ratio = value / 100; })} />
                      <NumberField label="最低现金比例" value={spot.minimum_cash_ratio * 100} min={20} max={90} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.minimum_cash_ratio = value / 100; })} />
                      <NumberField label="低位区回撤" value={spot.low_area_drawdown_pct * 100} min={1} max={90} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.low_area_drawdown_pct = value / 100; })} />
                      <NumberField label="波段净收益目标" value={spot.satellite_take_profit_ratio * 100} min={0.1} max={100} step={0.1} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.satellite_take_profit_ratio = value / 100; })} />
                      <Field label="权益回撤风控"><Switch checked={spot.enable_drawdown_risk_control} checkedChildren="启用" unCheckedChildren="停用" onChange={(checked) => updateConfig((next) => { next.spot_core_satellite_config.enable_drawdown_risk_control = checked; })} /></Field>
                      <NumberField label="策略最大回撤" value={spot.max_strategy_drawdown_pct * 100} min={1} max={80} suffix="%" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.max_strategy_drawdown_pct = value / 100; })} />
                      <NumberField label="手续费" value={spot.fee_rate_bps} min={0} max={100} suffix="BPS" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.fee_rate_bps = value; })} />
                      <NumberField label="滑点" value={spot.slippage_bps} min={0} max={100} suffix="BPS" onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.slippage_bps = value; })} />
                    </div>
                  </EditorSection>
                  <EditorSection icon={<SafetyCertificateOutlined />} title="分层规则" subtitle="下跌层级决定哪一笔资金建仓，目标价决定哪一笔 lot 可被卖出">
                    <div className={styles.parameterGrid}>
                      <DecimalListField label="底仓回撤层级" value={spot.accumulation_drawdowns} onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.accumulation_drawdowns = value; })} />
                      <DecimalListField label="底仓层级权重" value={spot.accumulation_weights} max={10} onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.accumulation_weights = value; })} />
                      <DecimalListField label="波段回撤层级" value={spot.satellite_entry_drawdowns} onChange={(value) => updateConfig((next) => { next.spot_core_satellite_config.satellite_entry_drawdowns = value; })} />
                    </div>
                    <Space wrap>
                      <Button type="primary" onClick={() => { window.location.href = '/trading/spot-strategy'; }}>进入回测与 Lot 账本</Button>
                      <Tag color="success">实盘默认关闭</Tag>
                    </Space>
                  </EditorSection>
                </>
              )}

              {draft.config.strategy_type === 'ai_trading' && <EditorSection icon={<DatabaseOutlined />} title="币种来源" subtitle="决定每轮分析进入候选池的交易对">
                <Radio.Group
                  value={coinSource.source_type}
                  className={styles.sourceGrid}
                  onChange={(event) => updateConfig((next) => {
                    const source = next.ai_config.coin_source;
                    source.source_type = event.target.value;
                    source.use_ai500 = ['ai500', 'mixed'].includes(event.target.value);
                    source.use_oi_top = ['oi_top', 'mixed'].includes(event.target.value);
                  })}
                >
                  <Radio.Button value="static"><DatabaseOutlined /> 静态列表<small>手动维护交易对</small></Radio.Button>
                  <Radio.Button value="ai500"><AimOutlined /> AI500 币种池<small>智能筛选热门币种</small></Radio.Button>
                  <Radio.Button value="oi_top"><FundOutlined /> OI Top<small>持仓增长排名</small></Radio.Button>
                  <Radio.Button value="mixed"><BarChartOutlined /> 混合模式<small>组合多个数据源</small></Radio.Button>
                </Radio.Group>
                {coinSource.source_type === 'static' && (
                  <Field label="静态交易对">
                    <Select mode="tags" tokenSeparators={[',', ' ']} value={coinSource.static_coins} placeholder="例如 BTCUSDT"
                      onChange={(values) => updateConfig((next) => { next.ai_config.coin_source.static_coins = values.map((value) => value.toUpperCase().replace('/', '')); })} />
                  </Field>
                )}
                {coinSource.use_ai500 && <NumberField label="AI500 数量上限" value={coinSource.ai500_limit} min={1} max={30} onChange={(value) => updateConfig((next) => { next.ai_config.coin_source.ai500_limit = value; })} />}
                {coinSource.use_oi_top && <NumberField label="OI Top 数量上限" value={coinSource.oi_top_limit} min={1} max={30} onChange={(value) => updateConfig((next) => { next.ai_config.coin_source.oi_top_limit = value; })} />}
              </EditorSection>}

              {draft.config.strategy_type === 'ai_trading' && <>
              <EditorSection icon={<BarChartOutlined />} title="K 线与技术指标" subtitle="多周期行情会共同进入策略上下文">
                <Field label="分析周期（最多 6 个）">
                  <Checkbox.Group
                    options={timeframes}
                    value={indicators.klines.selected_timeframes}
                    onChange={(values) => updateConfig((next) => {
                      const selected = values.slice(0, 6) as string[];
                      next.ai_config.indicators.klines.selected_timeframes = selected;
                      next.ai_config.indicators.klines.enable_multi_timeframe = selected.length > 1;
                      if (!selected.includes(next.ai_config.indicators.klines.primary_timeframe)) next.ai_config.indicators.klines.primary_timeframe = selected[0] ?? '15m';
                    })}
                  />
                </Field>
                <div className={styles.inlineFields}>
                  <Field label="主周期">
                    <Select value={indicators.klines.primary_timeframe} options={indicators.klines.selected_timeframes.map((value) => ({ value, label: value }))}
                      onChange={(value) => updateConfig((next) => { next.ai_config.indicators.klines.primary_timeframe = value; })} />
                  </Field>
                  <NumberField label="K 线数量" value={indicators.klines.primary_count} min={20} max={200} onChange={(value) => updateConfig((next) => { next.ai_config.indicators.klines.primary_count = value; })} />
                </div>
                <OptionGrid options={technicalIndicators} values={indicators} onToggle={(key, checked) => updateConfig((next) => { next.ai_config.indicators[key] = checked; })} />
                <div className={styles.parameterGrid}>
                  {indicators.enable_ema && <PeriodField label="EMA 周期" value={indicators.ema_periods} onChange={(value) => updateConfig((next) => { next.ai_config.indicators.ema_periods = value; })} />}
                  {indicators.enable_rsi && <PeriodField label="RSI 周期" value={indicators.rsi_periods} onChange={(value) => updateConfig((next) => { next.ai_config.indicators.rsi_periods = value; })} />}
                  {indicators.enable_atr && <PeriodField label="ATR 周期" value={indicators.atr_periods} onChange={(value) => updateConfig((next) => { next.ai_config.indicators.atr_periods = value; })} />}
                  {indicators.enable_boll && <PeriodField label="BOLL 周期" value={indicators.boll_periods} onChange={(value) => updateConfig((next) => { next.ai_config.indicators.boll_periods = value; })} />}
                </div>
              </EditorSection>

              <EditorSection icon={<FundOutlined />} title="市场情绪" subtitle="成交量、持仓量和资金费率">
                <OptionGrid options={marketIndicators} values={indicators} onToggle={(key, checked) => updateConfig((next) => { next.ai_config.indicators[key] = checked; })} />
              </EditorSection>

              <EditorSection icon={<DatabaseOutlined />} title="量化数据" subtitle="NOFXOS AI500、OI 与资金流增强数据；启用前需配置服务凭证">
                <OptionGrid options={quantIndicators} values={indicators} onToggle={(key, checked) => updateConfig((next) => {
                  next.ai_config.indicators[key] = checked;
                  next.ai_config.indicators.enable_quant_data = quantIndicators.some(([item]) => item === key ? checked : next.ai_config.indicators[item]);
                })} />
                {(indicators.enable_oi_ranking || indicators.enable_netflow_ranking || indicators.enable_price_ranking) && (
                  <div className={styles.rankingGrid}>
                    {indicators.enable_oi_ranking && <RankingField label="OI 排名" duration={indicators.oi_ranking_duration} limit={indicators.oi_ranking_limit}
                      onDuration={(value) => updateConfig((next) => { next.ai_config.indicators.oi_ranking_duration = value; })}
                      onLimit={(value) => updateConfig((next) => { next.ai_config.indicators.oi_ranking_limit = value; })} />}
                    {indicators.enable_netflow_ranking && <RankingField label="资金流排名" duration={indicators.netflow_ranking_duration} limit={indicators.netflow_ranking_limit}
                      onDuration={(value) => updateConfig((next) => { next.ai_config.indicators.netflow_ranking_duration = value; })}
                      onLimit={(value) => updateConfig((next) => { next.ai_config.indicators.netflow_ranking_limit = value; })} />}
                    {indicators.enable_price_ranking && (
                      <div className={styles.rankingField}>
                        <Text strong>价格涨跌榜</Text>
                        <Select mode="multiple" maxCount={3} value={indicators.price_ranking_duration.split(',').filter(Boolean)}
                          options={['1h', '4h', '24h'].map((value) => ({ value, label: value }))}
                          onChange={(values) => updateConfig((next) => { next.ai_config.indicators.price_ranking_duration = values.join(','); })} />
                        <NumberField label="每周期数量" value={indicators.price_ranking_limit} min={1} max={50}
                          onChange={(value) => updateConfig((next) => { next.ai_config.indicators.price_ranking_limit = value; })} />
                      </div>
                    )}
                  </div>
                )}
              </EditorSection>

              <EditorSection icon={<SafetyCertificateOutlined />} title="风险控制" subtitle="这些参数是策略硬边界，不应由模型绕过">
                <div className={styles.riskGrid}>
                  <NumberField label="最大持仓数" value={risk.max_positions} min={1} max={20} onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.max_positions = value; })} />
                  <NumberField label="BTC/ETH 最大杠杆" value={risk.btc_eth_max_leverage} min={1} max={125} suffix="x" onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.btc_eth_max_leverage = value; })} />
                  <NumberField label="山寨币最大杠杆" value={risk.altcoin_max_leverage} min={1} max={125} suffix="x" onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.altcoin_max_leverage = value; })} />
                  <NumberField label="最小仓位" value={risk.min_position_size} min={5} max={1000000} suffix="USDT" onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.min_position_size = value; })} />
                  <NumberField label="最小盈亏比" value={risk.min_risk_reward_ratio} min={0.5} max={20} step={0.1} onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.min_risk_reward_ratio = value; })} />
                  <Field label={`最大保证金占用 ${Math.round(risk.max_margin_usage * 100)}%`}>
                    <Slider min={10} max={100} value={Math.round(risk.max_margin_usage * 100)} onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.max_margin_usage = value / 100; })} />
                  </Field>
                  <Field label={`最低置信度 ${risk.min_confidence}%`}>
                    <Slider min={1} max={100} value={risk.min_confidence} onChange={(value) => updateConfig((next) => { next.ai_config.risk_control.min_confidence = value; })} />
                  </Field>
                </div>
              </EditorSection>

              <EditorSection icon={<RobotOutlined />} title="Prompt 分区" subtitle="按职责组织策略指令，便于预览、审计和后续版本管理">
                <div className={styles.promptSectionGrid}>
                  <PromptField label="角色定义" value={draft.config.ai_config.prompt_sections.role_definition}
                    placeholder="例如：你是遵循趋势和严格风险边界的加密资产分析模型。"
                    onChange={(value) => updateConfig((next) => { next.ai_config.prompt_sections.role_definition = value; })} />
                  <PromptField label="交易频率" value={draft.config.ai_config.prompt_sections.trading_frequency}
                    placeholder="例如：同一标的信号确认前保持观望，避免短时间重复进出。"
                    onChange={(value) => updateConfig((next) => { next.ai_config.prompt_sections.trading_frequency = value; })} />
                  <PromptField label="入场标准" value={draft.config.ai_config.prompt_sections.entry_standards}
                    placeholder="例如：至少两个周期方向一致，且风险收益比满足硬风控。"
                    onChange={(value) => updateConfig((next) => { next.ai_config.prompt_sections.entry_standards = value; })} />
                  <PromptField label="决策流程" value={draft.config.ai_config.prompt_sections.decision_process}
                    placeholder="例如：先检查持仓与风险，再筛选候选币，最后输出结构化决策。"
                    onChange={(value) => updateConfig((next) => { next.ai_config.prompt_sections.decision_process = value; })} />
                </div>
                <Field label="其他自定义约束">
                <Input.TextArea rows={6} maxLength={4000} showCount value={draft.config.ai_config.custom_prompt}
                  onChange={(event) => updateConfig((next) => { next.ai_config.custom_prompt = event.target.value; })}
                  placeholder="例如：信号冲突时保持观望；不得追涨；每次决策给出简短证据。" />
                </Field>
              </EditorSection>
              </>}
            </>
          )}
        </main>

        <aside className={styles.inspector}>
          <Tabs activeKey={rightTab} onChange={setRightTab} items={[
            { key: 'preview', label: <span><EyeOutlined /> Prompt 预览</span>, children: (
              <div className={styles.inspectorBody}>
                <Segmented block value={promptVariant} onChange={(value) => setPromptVariant(String(value))} options={[{ label: '保守', value: 'conservative' }, { label: '均衡', value: 'balanced' }, { label: '积极', value: 'aggressive' }]} />
                <Button block type="primary" ghost icon={<EyeOutlined />} loading={previewing} onClick={runPreview}>{draft?.config.strategy_type === 'grid' ? '生成网格计划摘要' : '生成 Prompt'}</Button>
                {tokenEstimate && <TokenEstimatePanel estimate={tokenEstimate} />}
                {preview ? <pre className={styles.prompt}>{preview.system_prompt}</pre> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="生成后查看最终系统 Prompt 与 Token 估算" />}
              </div>
            )},
            { key: 'test', disabled: draft?.config.strategy_type !== 'ai_trading', label: <span><RobotOutlined /> AI 测试</span>, children: (
              <div className={styles.inspectorBody}>
                <Select value={modelID} onChange={setModelID} placeholder="选择 AI 模型" options={models.map((item) => ({ value: item.id, label: `${item.name} (${item.provider})` }))} />
                <Button block type="primary" icon={<RobotOutlined />} loading={testing} onClick={runPreflight}>运行 AI 干跑测试</Button>
                <Paragraph type="secondary" className={styles.preflightHint}>会使用已保存凭证请求模型正式接口，并使用合成行情验证 Prompt；不会创建交易指令，也不会调用交易所下单接口。</Paragraph>
                {preflight?.ready ? <>
                  <div className={styles.ready}><CheckOutlined /><div><Text strong>AI 干跑完成</Text><Text type="secondary">{preflight.ai_model_name} · {preflight.model_name} · {preflight.latency_ms} ms · {preflight.total_tokens} tokens</Text></div></div>
                  <pre className={styles.prompt}>{preflight.ai_response}</pre>
                </> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="等待运行 AI 干跑" />}
              </div>
            )},
          ]} />
        </aside>
      </div>
      <Modal
        title={`${draft?.name ?? '策略'} · 版本历史`}
        open={versionsOpen}
        onCancel={() => setVersionsOpen(false)}
        footer={null}
        width={680}
      >
        <List
          loading={versionAction}
          dataSource={versions}
          locale={{ emptyText: '暂无版本' }}
          renderItem={(item) => (
            <List.Item
              actions={item.version !== draft?.current_version ? [
                <Popconfirm
                  key="rollback"
                  title={`回滚到 v${item.version}？`}
                  description="系统会创建一个新的草稿版本，不会覆盖历史。"
                  onConfirm={() => rollbackCurrent(item.version)}
                >
                  <Button type="link">回滚到此版本</Button>
                </Popconfirm>,
              ] : undefined}
            >
              <List.Item.Meta
                title={<Space><Text strong>v{item.version}</Text>{item.version === draft?.current_version && <Tag color="processing">当前</Tag>}{item.version === draft?.published_version && <Tag color="success">已发布</Tag>}{item.rollback_from && <Tag>来自 v{item.rollback_from}</Tag>}</Space>}
                description={`${item.name} · ${new Date(item.created_at).toLocaleString()}`}
              />
            </List.Item>
          )}
        />
      </Modal>
    </PageContainer>
  );
};

type SectionProps = { icon: React.ReactNode; title: string; subtitle: string; children: React.ReactNode };
const EditorSection: React.FC<SectionProps> = ({ icon, title, subtitle, children }) => (
  <section className={styles.section}>
    <div className={styles.sectionTitle}><span>{icon}</span><div><Title level={5}>{title}</Title><Text type="secondary">{subtitle}</Text></div></div>
    <div className={styles.sectionBody}>{children}</div>
  </section>
);

const Field: React.FC<{ label: string; children: React.ReactNode }> = ({ label, children }) => (
  <label className={styles.field}><Text type="secondary">{label}</Text>{children}</label>
);

type NumberFieldProps = { label: string; value: number; min: number; max: number; step?: number; suffix?: string; onChange: (value: number) => void };
const NumberField: React.FC<NumberFieldProps> = ({ label, value, min, max, step, suffix, onChange }) => (
  <Field label={label}><InputNumber value={value} min={min} max={max} step={step} addonAfter={suffix} onChange={(next) => onChange(Number(next ?? min))} /></Field>
);

const PeriodField: React.FC<{ label: string; value: number[]; onChange: (value: number[]) => void }> = ({ label, value, onChange }) => (
  <Field label={`${label}（2-400，最多 6 个）`}>
    <Select mode="tags" tokenSeparators={[',', ' ']} value={value.map(String)}
      onChange={(values) => onChange([...new Set(values.map(Number).filter((item) => Number.isInteger(item) && item >= 2 && item <= 400))].slice(0, 6).sort((a, b) => a - b))} />
  </Field>
);

const DecimalListField: React.FC<{ label: string; value: number[]; max?: number; onChange: (value: number[]) => void }> = ({ label, value, max = 1, onChange }) => (
  <Field label={`${label}（逗号分隔，最多 8 个）`}>
    <Select mode="tags" tokenSeparators={[',', ' ']} value={value.map(String)}
      onChange={(values) => onChange(values.map(Number).filter((item) => Number.isFinite(item) && item >= 0 && item <= max).slice(0, 8))} />
  </Field>
);

type RankingFieldProps = {
  label: string;
  duration: '1h' | '4h' | '24h';
  limit: number;
  onDuration: (value: '1h' | '4h' | '24h') => void;
  onLimit: (value: number) => void;
};
const RankingField: React.FC<RankingFieldProps> = ({ label, duration, limit, onDuration, onLimit }) => (
  <div className={styles.rankingField}>
    <Text strong>{label}</Text>
    <Field label="统计周期"><Select value={duration} options={['1h', '4h', '24h'].map((value) => ({ value, label: value }))} onChange={onDuration} /></Field>
    <NumberField label="排名数量" value={limit} min={1} max={50} onChange={onLimit} />
  </div>
);

const PromptField: React.FC<{ label: string; value: string; placeholder: string; onChange: (value: string) => void }> = ({ label, value, placeholder, onChange }) => (
  <Field label={label}><Input.TextArea rows={4} maxLength={4000} showCount value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} /></Field>
);

const TokenEstimatePanel: React.FC<{ estimate: TradingAPI.StrategyTokenEstimate }> = ({ estimate }) => {
  const strictest = [...estimate.model_limits].sort((a, b) => a.context_limit - b.context_limit)[0];
  return (
    <div className={styles.tokenEstimate}>
      <div><Text type="secondary">预计上下文</Text><strong>{estimate.total.toLocaleString()} tokens</strong></div>
      <div><Text type="secondary">行情数据</Text><strong>{estimate.breakdown.market_data.toLocaleString()}</strong></div>
      <div><Text type="secondary">系统 Prompt</Text><strong>{estimate.breakdown.system_prompt.toLocaleString()}</strong></div>
      {strictest && <Tag color={strictest.level === 'danger' ? 'error' : strictest.level === 'warning' ? 'warning' : 'success'}>{strictest.name} {strictest.usage_pct}%</Tag>}
      {estimate.suggestions.map((item) => <Text key={item} type="warning">{item}</Text>)}
    </div>
  );
};

type IndicatorKey =
  | 'enable_ema'
  | 'enable_macd'
  | 'enable_rsi'
  | 'enable_atr'
  | 'enable_boll'
  | 'enable_volume'
  | 'enable_oi'
  | 'enable_funding_rate'
  | 'enable_quant_oi'
  | 'enable_quant_netflow'
  | 'enable_oi_ranking'
  | 'enable_netflow_ranking'
  | 'enable_price_ranking';
type OptionGridProps = {
  options: readonly (readonly [IndicatorKey, string, string])[];
  values: TradingAPI.StrategyStudioConfig['ai_config']['indicators'];
  onToggle: (key: IndicatorKey, checked: boolean) => void;
};
const OptionGrid: React.FC<OptionGridProps> = ({ options, values, onToggle }) => (
  <div className={styles.optionGrid}>{options.map(([key, title, description]) => (
    <label key={key} className={`${styles.option} ${values[key] ? styles.optionChecked : ''}`}>
      <span><Text strong>{title}</Text><Text type="secondary">{description}</Text></span>
      <Checkbox checked={Boolean(values[key])} onChange={(event) => onToggle(key, event.target.checked)} />
    </label>
  ))}</div>
);

export default StrategyStudio;
