import { AreaSeries, ColorType, createChart, type IChartApi } from 'lightweight-charts';
import { Empty, Segmented, Typography, theme } from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { formatDecimal } from '@/utils/decimal';
import styles from '../Welcome.less';

type Props = {
  data: TradingAPI.EquityPoint[];
};

type DisplayMode = 'amount' | 'percent';

const EquityCurve: React.FC<Props> = ({ data }) => {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi>();
  const [mode, setMode] = useState<DisplayMode>('amount');

  const validData = useMemo(
    () =>
      data.filter(
        (point) => Number.isFinite(Number(point.total_equity)) && Number(point.total_equity) > 0,
      ),
    [data],
  );
  const first = validData[0];
  const last = validData[validData.length - 1];
  const initialEquity = Number(first?.total_equity);
  const currentEquity = Number(last?.total_equity);
  const delta = currentEquity - initialEquity;
  const deltaRate = initialEquity > 0 ? (delta / initialEquity) * 100 : undefined;
  const positive = delta >= 0;

  useEffect(() => {
    const container = containerRef.current;
    if (!container || validData.length === 0) return undefined;

    const chart = createChart(container, {
      width: container.clientWidth,
      height: 268,
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
      timeScale: {
        borderColor: token.colorBorderSecondary,
        timeVisible: true,
        secondsVisible: false,
      },
      crosshair: {
        vertLine: { color: token.colorTextTertiary },
        horzLine: { color: token.colorTextTertiary },
      },
    });
    const lineColor = positive ? token.colorSuccess : token.colorError;
    const series = chart.addSeries(AreaSeries, {
      lineColor,
      topColor: `${lineColor}48`,
      bottomColor: `${lineColor}05`,
      lineWidth: 2,
      priceFormat:
        mode === 'percent'
          ? { type: 'custom', formatter: (value: number) => `${value.toFixed(2)}%`, minMove: 0.01 }
          : { type: 'price', precision: 2, minMove: 0.01 },
    });
    series.setData(
      validData.map((point) => ({
        time: Math.floor(new Date(point.captured_at).getTime() / 1000) as never,
        value:
          mode === 'percent'
            ? ((Number(point.total_equity) - initialEquity) / initialEquity) * 100
            : Number(point.total_equity),
      })),
    );
    series.createPriceLine({
      price: mode === 'percent' ? 0 : initialEquity,
      color: token.colorTextTertiary,
      lineWidth: 1,
      lineStyle: 2,
      axisLabelVisible: false,
      title: 'Initial',
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
  }, [initialEquity, mode, positive, token, validData]);

  if (validData.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="等待账户快照数据" />;
  }

  const rangeText =
    first && last
      ? `${new Date(first.captured_at).toLocaleDateString()} - ${new Date(last.captured_at).toLocaleDateString()}`
      : '--';

  return (
    <div className={styles.equityPanel}>
      <div className={styles.equityHeader}>
        <div>
          <div className={styles.equityValue}>
            {formatDecimal(last.total_equity, { grouping: true, maximumFractionDigits: 2 })}
            <small>USDT</small>
          </div>
          <div className={positive ? styles.positive : styles.negative}>
            {positive ? '+' : ''}
            {formatDecimal(delta, { grouping: true, maximumFractionDigits: 2 })} USDT
            {deltaRate !== undefined ? ` (${positive ? '+' : ''}${deltaRate.toFixed(2)}%)` : ''}
          </div>
        </div>
        <Segmented
          size="small"
          value={mode}
          options={[
            { label: 'USDT', value: 'amount' },
            { label: '%', value: 'percent' },
          ]}
          onChange={(value) => setMode(value as DisplayMode)}
        />
      </div>

      <div ref={containerRef} className={styles.equityCanvas} />

      <div className={styles.equityFacts}>
        <div>
          <Typography.Text type="secondary">初始权益</Typography.Text>
          <strong>
            {formatDecimal(first.total_equity, { grouping: true, maximumFractionDigits: 2 })} USDT
          </strong>
        </div>
        <div>
          <Typography.Text type="secondary">当前权益</Typography.Text>
          <strong>
            {formatDecimal(last.total_equity, { grouping: true, maximumFractionDigits: 2 })} USDT
          </strong>
        </div>
        <div>
          <Typography.Text type="secondary">历史快照</Typography.Text>
          <strong>{validData.length} 条</strong>
        </div>
        <div>
          <Typography.Text type="secondary">展示区间</Typography.Text>
          <strong>{rangeText}</strong>
        </div>
      </div>
    </div>
  );
};

export default EquityCurve;
