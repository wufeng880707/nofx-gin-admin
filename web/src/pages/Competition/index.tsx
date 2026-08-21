import {
  ArrowLeftOutlined,
  LineChartOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import { AreaSeries, ColorType, createChart } from 'lightweight-charts';
import type { IChartApi } from 'lightweight-charts';
import {
  Alert,
  Button,
  Drawer,
  Empty,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  message,
  theme,
} from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  fetchCompetitionEquity,
  fetchCompetitionLeaderboard,
} from '@/services/trading/competition';
import styles from './index.less';

const percent = (value: number) => `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;

const CompetitionEquityChart: React.FC<{ points: TradingAPI.CompetitionEquityPoint[] }> = ({ points }) => {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi>();
  const data = useMemo(() => {
    const byTime = new Map<number, number>();
    points.forEach((point) => {
      byTime.set(Math.floor(new Date(point.captured_at).getTime() / 1000), point.equity_index);
    });
    return [...byTime.entries()].sort((left, right) => left[0] - right[0]);
  }, [points]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || data.length === 0) return undefined;
    const chart = createChart(container, {
      width: container.clientWidth,
      height: 320,
      layout: {
        background: { type: ColorType.Solid, color: token.colorBgContainer },
        textColor: token.colorTextSecondary,
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: token.colorBorderSecondary },
        horzLines: { color: token.colorBorderSecondary },
      },
      rightPriceScale: { borderColor: token.colorBorderSecondary },
      timeScale: { borderColor: token.colorBorderSecondary, timeVisible: true },
    });
    const last = data[data.length - 1]?.[1] ?? 100;
    const color = last >= 100 ? token.colorSuccess : token.colorError;
    const series = chart.addSeries(AreaSeries, {
      lineColor: color,
      topColor: `${color}3d`,
      bottomColor: `${color}05`,
      lineWidth: 2,
      priceFormat: { type: 'custom', minMove: 0.01, formatter: (value: number) => value.toFixed(2) },
    });
    series.setData(data.map(([time, value]) => ({ time: time as never, value })));
    series.createPriceLine({
      price: 100,
      color: token.colorTextTertiary,
      lineWidth: 1,
      lineStyle: 2,
      axisLabelVisible: false,
      title: 'Base 100',
    });
    chart.timeScale().fitContent();
    chartRef.current = chart;
    const observer = new ResizeObserver(() => chart.applyOptions({ width: container.clientWidth }));
    observer.observe(container);
    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current = undefined;
    };
  }, [data, token]);

  if (data.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无完整交易权益数据" />;
  }
  return <div ref={containerRef} className={styles.chart} />;
};

const Competition: React.FC = () => {
  const [days, setDays] = useState(90);
  const [current, setCurrent] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loading, setLoading] = useState(true);
  const [leaderboard, setLeaderboard] = useState<TradingAPI.CompetitionLeaderboard>();
  const [detail, setDetail] = useState<TradingAPI.CompetitionEquityHistory>();
  const [detailLoading, setDetailLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetchCompetitionLeaderboard({ current, pageSize, days });
      setLeaderboard(response.data);
    } catch {
      setLeaderboard(undefined);
      message.error('排行榜加载失败');
    } finally {
      setLoading(false);
    }
  }, [current, days, pageSize]);

  useEffect(() => {
    void load();
  }, [load]);

  const openDetail = async (item: TradingAPI.CompetitionTrader) => {
    setDetailLoading(true);
    try {
      const response = await fetchCompetitionEquity(item.public_id, days);
      setDetail(response.data);
    } catch {
      message.error('权益历史加载失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const columns: ColumnsType<TradingAPI.CompetitionTrader> = [
    { title: '排名', dataIndex: 'rank', width: 72, fixed: 'left', render: (value) => <strong>#{value}</strong> },
    { title: 'Trader', dataIndex: 'public_alias', width: 180, fixed: 'left', render: (value) => <Typography.Text strong>{value}</Typography.Text> },
    { title: '模式', dataIndex: 'is_simulation', width: 90, render: (value) => <Tag color={value ? 'warning' : 'blue'}>{value ? '模拟' : '实盘'}</Tag> },
    { title: '收益率', dataIndex: 'return_pct', align: 'right', width: 120, render: (value: number) => <strong className={value >= 0 ? styles.positive : styles.negative}>{percent(value)}</strong> },
    { title: '最大回撤', dataIndex: 'max_drawdown_pct', align: 'right', width: 120, render: (value: number) => `${value.toFixed(2)}%` },
    { title: '胜率', dataIndex: 'win_rate', align: 'right', width: 100, render: (value: number) => `${value.toFixed(1)}%` },
    { title: '完整交易', dataIndex: 'total_trades', align: 'right', width: 110 },
    { title: '权益指数', dataIndex: 'equity_index', align: 'right', width: 110, render: (value: number) => value.toFixed(2) },
    { title: '更新时间', dataIndex: 'last_updated_at', width: 180, render: (value) => value ? new Date(value).toLocaleString() : '--' },
    { title: '', key: 'detail', fixed: 'right', width: 56, render: (_, record) => <Button type="text" icon={<LineChartOutlined />} title="查看权益曲线" onClick={() => void openDetail(record)} /> },
  ];

  const onTableChange = (pagination: TablePaginationConfig) => {
    setCurrent(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.brand}><span className={styles.mark}>X</span><strong>XTrading</strong></div>
        <Button icon={<ArrowLeftOutlined />} onClick={() => window.location.assign('/user/login')}>管理后台</Button>
      </header>
      <section className={styles.content}>
        <div className={styles.titleRow}>
          <div>
            <Typography.Title level={2}><TrophyOutlined /> 交易竞赛</Typography.Title>
            <Typography.Text type="secondary">按归一化收益率排名，基准权益统一为 100</Typography.Text>
          </div>
          <Space wrap>
            <Segmented
              value={days}
              options={[{ label: '30 天', value: 30 }, { label: '90 天', value: 90 }, { label: '180 天', value: 180 }, { label: '365 天', value: 365 }]}
              onChange={(value) => { setDays(Number(value)); setCurrent(1); }}
            />
            <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()} />
          </Space>
        </div>
        <Alert
          className={styles.notice}
          type="info"
          showIcon
          icon={<SafetyCertificateOutlined />}
          message="隐私保护"
          description="仅展示主动参加者的匿名别名与归一化绩效；不公开用户、账户、绝对资产、内部配置或凭证。模拟与实盘分开展示标识，排名不构成投资建议。"
        />
        <Table
          rowKey="public_id"
          columns={columns}
          dataSource={leaderboard?.items ?? []}
          loading={loading}
          pagination={{
            current,
            pageSize,
            total: leaderboard?.page.total ?? 0,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50],
          }}
          onChange={onTableChange}
          scroll={{ x: 1160 }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无主动参赛的 Trader" /> }}
        />
      </section>
      <Drawer
        title={detail ? `${detail.trader.public_alias} · 权益历史` : '权益历史'}
        open={Boolean(detail) || detailLoading}
        loading={detailLoading}
        width={720}
        onClose={() => setDetail(undefined)}
      >
        {detail && (
          <>
            <div className={styles.detailMetrics}>
              <div><Typography.Text type="secondary">收益率</Typography.Text><strong className={detail.trader.return_pct >= 0 ? styles.positive : styles.negative}>{percent(detail.trader.return_pct)}</strong></div>
              <div><Typography.Text type="secondary">最大回撤</Typography.Text><strong>{detail.trader.max_drawdown_pct.toFixed(2)}%</strong></div>
              <div><Typography.Text type="secondary">胜率</Typography.Text><strong>{detail.trader.win_rate.toFixed(1)}%</strong></div>
              <div><Typography.Text type="secondary">交易数</Typography.Text><strong>{detail.trader.total_trades}</strong></div>
            </div>
            <CompetitionEquityChart points={detail.points} />
          </>
        )}
      </Drawer>
    </main>
  );
};

export default Competition;
