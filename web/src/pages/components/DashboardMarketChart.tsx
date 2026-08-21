import { ReloadOutlined } from '@ant-design/icons';
import {
  CandlestickSeries,
  ColorType,
  HistogramSeries,
  createChart,
  type CandlestickData,
  type HistogramData,
  type IChartApi,
  type ISeriesApi,
  type Time,
} from 'lightweight-charts';
import { Button, Empty, Select, Space, Tag, Tooltip, Typography, theme } from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  fetchMarketContext,
  fetchMarketInstruments,
  fetchMarketKlines,
  fetchMarketTicker,
} from '@/services/trading/market';
import { connectMarketStream, type MarketStreamStatus } from '@/services/trading/marketSocket';
import { formatDecimal } from '@/utils/decimal';
import styles from '../Welcome.less';

type Props = {
  preferredExchange?: TradingAPI.MarketExchange;
};

const intervals: TradingAPI.MarketInterval[] = ['1m', '5m', '15m', '1h', '4h', '1d'];

const streamLabel: Record<MarketStreamStatus, string> = {
  connecting: '连接中',
  connected: '实时',
  reconnecting: '重连中',
  disconnected: '已断开',
  error: '异常',
};

const defaultSymbol = (exchange: TradingAPI.MarketExchange, market: TradingAPI.MarketType) => {
  if (exchange === 'okx') return market === 'spot' ? 'BTC-USDT' : 'BTC-USDT-SWAP';
  return 'BTCUSDT';
};

const compact = (value: unknown) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return '--';
  return new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 2 }).format(
    number,
  );
};

const DashboardMarketChart: React.FC<Props> = ({ preferredExchange }) => {
  const { token } = theme.useToken();
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi>();
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'>>();
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'>>();
  const [exchange, setExchange] = useState<TradingAPI.MarketExchange>(
    preferredExchange ?? 'binance',
  );
  const [market, setMarket] = useState<TradingAPI.MarketType>('spot');
  const [symbol, setSymbol] = useState(defaultSymbol(preferredExchange ?? 'binance', 'spot'));
  const [interval, setIntervalValue] = useState<TradingAPI.MarketInterval>('15m');
  const [instruments, setInstruments] = useState<TradingAPI.MarketInstrument[]>([]);
  const [ticker, setTicker] = useState<TradingAPI.MarketTicker>();
  const [marketContext, setMarketContext] = useState<TradingAPI.MarketContext>();
  const [klines, setKlines] = useState<TradingAPI.MarketKline[]>([]);
  const [status, setStatus] = useState<MarketStreamStatus>('connecting');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!preferredExchange || preferredExchange === exchange) return;
    setExchange(preferredExchange);
    setSymbol(defaultSymbol(preferredExchange, market));
  }, [exchange, market, preferredExchange]);

  useEffect(() => {
    fetchMarketInstruments({ exchange, market })
      .then((response) => {
        const items = response.data ?? [];
        setInstruments(items);
        if (items.length > 0 && !items.some((item) => item.symbol === symbol)) {
          setSymbol(items[0].symbol);
        }
      })
      .catch(() => setInstruments([]));
  }, [exchange, market, symbol]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [tickerResponse, klineResponse, contextResponse] = await Promise.all([
        fetchMarketTicker({ exchange, market, symbol }),
        fetchMarketKlines({ exchange, market, symbol, interval, limit: 160 }),
        fetchMarketContext({ exchange, market, symbol, depth: 10 }),
      ]);
      setTicker(tickerResponse.data);
      setKlines(klineResponse.data ?? []);
      setMarketContext(contextResponse.data);
      setError('');
    } catch {
      setError('行情暂时不可用');
    } finally {
      setLoading(false);
    }
  }, [exchange, interval, market, symbol]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      fetchMarketContext({ exchange, market, symbol, depth: 10 })
        .then((response) => setMarketContext(response.data))
        .catch(() => undefined);
    }, 15_000);
    return () => window.clearInterval(timer);
  }, [exchange, market, symbol]);

  useEffect(() => {
    const connection = connectMarketStream(
      { exchange, market, symbol, interval },
      {
        onTicker: (next) => {
          setTicker(next);
          setError('');
        },
        onKline: (next) => {
          setKlines((current) => {
            const result = current.filter((item) => item.time !== next.time);
            result.push(next);
            result.sort((left, right) => left.time - right.time);
            return result.slice(-160);
          });
          setLoading(false);
          setError('');
        },
        onStatus: setStatus,
      },
    );
    return connection.close;
  }, [exchange, interval, market, symbol]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;
    const chart = createChart(container, {
      width: container.clientWidth,
      height: 286,
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
    const candles = chart.addSeries(CandlestickSeries, {
      upColor: '#0b9b6d',
      downColor: '#d84a5b',
      borderVisible: false,
      wickUpColor: '#0b9b6d',
      wickDownColor: '#d84a5b',
      priceFormat: { type: 'price', precision: 4, minMove: 0.0001 },
    });
    const volume = chart.addSeries(HistogramSeries, {
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume',
    });
    volume.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } });
    chartRef.current = chart;
    candleSeriesRef.current = candles;
    volumeSeriesRef.current = volume;
    const observer = new ResizeObserver(() => chart.applyOptions({ width: container.clientWidth }));
    observer.observe(container);
    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current = undefined;
      candleSeriesRef.current = undefined;
      volumeSeriesRef.current = undefined;
    };
  }, [token]);

  useEffect(() => {
    const candleData: CandlestickData<Time>[] = klines.map((item) => ({
      time: item.time as Time,
      open: Number(item.open),
      high: Number(item.high),
      low: Number(item.low),
      close: Number(item.close),
    }));
    const volumeData: HistogramData<Time>[] = klines.map((item) => ({
      time: item.time as Time,
      value: Number(item.volume),
      color: Number(item.close) >= Number(item.open) ? '#0b9b6d55' : '#d84a5b55',
    }));
    candleSeriesRef.current?.setData(candleData);
    volumeSeriesRef.current?.setData(volumeData);
    if (candleData.length > 0) chartRef.current?.timeScale().fitContent();
  }, [klines]);

  const symbolOptions = useMemo(
    () => instruments.map((item) => ({ label: item.display || item.symbol, value: item.symbol })),
    [instruments],
  );
  const changeRate = Number(ticker?.price_change_rate);
  const positive = Number.isFinite(changeRate) && changeRate >= 0;
  const signalMatrix = useMemo(() => {
    const closes = klines.map((item) => Number(item.close)).filter(Number.isFinite);
    if (closes.length < 20) return [];
    const last = closes.at(-1) ?? 0;
    const average = (values: number[]) => values.reduce((sum, value) => sum + value, 0) / values.length;
    const ema20 = average(closes.slice(-20));
    const previous = closes.at(-6) ?? last;
    const momentum = previous === 0 ? 0 : ((last - previous) / previous) * 100;
    const returns = closes.slice(-20).slice(1).map((value, index) => {
      const base = closes.slice(-20)[index];
      return base === 0 ? 0 : ((value - base) / base) * 100;
    });
    const volatility = Math.sqrt(average(returns.map((value) => value * value)));
    return [
      { label: '趋势', value: last >= ema20 ? '偏多' : '偏空', tone: last >= ema20 ? 'up' : 'down' },
      { label: '动量', value: `${momentum >= 0 ? '+' : ''}${momentum.toFixed(2)}%`, tone: momentum >= 0 ? 'up' : 'down' },
      { label: '波动', value: `${volatility.toFixed(2)}%`, tone: volatility > 1.5 ? 'warn' : 'neutral' },
    ];
  }, [klines]);
  const book = marketContext?.order_book;
  const derivatives = marketContext?.derivatives;
  const radar = marketContext?.risk_radar;
  const riskColor = radar?.risk_score && radar.risk_score >= 60 ? 'error' : radar?.risk_score && radar.risk_score >= 30 ? 'warning' : 'success';

  return (
    <div className={styles.marketPanel}>
      <div className={styles.marketSummary}>
        <div>
          <Space size={8} wrap>
            <Typography.Text strong className={styles.marketSymbol}>
              {symbol}
            </Typography.Text>
            <Tag
              color={
                status === 'connected' ? 'success' : status === 'error' ? 'error' : 'processing'
              }
            >
              WS {streamLabel[status]}
            </Tag>
          </Space>
          <div className={styles.marketPriceRow}>
            <Typography.Text strong className={styles.marketPrice}>
              {ticker?.last_price ? formatDecimal(ticker.last_price, { grouping: true }) : '--'}
            </Typography.Text>
            <Typography.Text className={positive ? styles.positive : styles.negative}>
              {Number.isFinite(changeRate)
                ? `${positive ? '+' : ''}${changeRate.toFixed(2)}%`
                : '--'}
            </Typography.Text>
          </div>
        </div>
        <div className={styles.marketStats}>
          <span>
            <small>24H HIGH</small>
            {ticker?.high_price ? formatDecimal(ticker.high_price) : '--'}
          </span>
          <span>
            <small>24H LOW</small>
            {ticker?.low_price ? formatDecimal(ticker.low_price) : '--'}
          </span>
          <span>
            <small>24H VOLUME</small>
            {ticker?.quote_volume ? compact(ticker.quote_volume) : '--'}
          </span>
        </div>
      </div>

      <div className={styles.marketToolbar}>
        <Select
          size="small"
          value={exchange}
          options={[
            { label: 'Binance', value: 'binance' },
            { label: 'OKX', value: 'okx' },
          ]}
          onChange={(value) => {
            setExchange(value);
            setSymbol(defaultSymbol(value, market));
          }}
        />
        <Select
          size="small"
          value={market}
          options={[
            { label: '现货', value: 'spot' },
            { label: 'USDT 永续', value: 'futures' },
          ]}
          onChange={(value) => {
            setMarket(value);
            setSymbol(defaultSymbol(exchange, value));
          }}
        />
        <Select
          showSearch
          size="small"
          value={symbol}
          options={symbolOptions}
          onChange={setSymbol}
          className={styles.symbolSelect}
        />
        <div className={styles.intervalControl}>
          {intervals.map((item) => (
            <button
              type="button"
              key={item}
              data-active={interval === item}
              onClick={() => setIntervalValue(item)}
            >
              {item}
            </button>
          ))}
        </div>
        <Tooltip title="刷新历史行情">
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => void load()}
          />
        </Tooltip>
      </div>

      <div className={styles.marketIntelligence}>
        <div className={styles.marketSignals}>
          <span className={styles.marketIntelligenceTitle}>信号矩阵 · {interval}</span>
          {signalMatrix.map((item) => (
            <span key={item.label} data-tone={item.tone}>
              <small>{item.label}</small>
              <strong>{item.value}</strong>
            </span>
          ))}
        </div>
        <div className={styles.marketBook}>
          <span className={styles.marketIntelligenceTitle}>Order Book</span>
          <span>
            <small>最佳买价</small>
            <strong className={styles.positive}>{book?.bids?.[0]?.price ? formatDecimal(book.bids[0].price) : '--'}</strong>
          </span>
          <span>
            <small>最佳卖价</small>
            <strong className={styles.negative}>{book?.asks?.[0]?.price ? formatDecimal(book.asks[0].price) : '--'}</strong>
          </span>
          <span>
            <small>价差</small>
            <strong>{Number.isFinite(book?.spread_bps) ? `${book?.spread_bps.toFixed(2)} bps` : '--'}</strong>
          </span>
          <span>
            <small>深度失衡</small>
            <strong>{Number.isFinite(book?.imbalance) ? `${((book?.imbalance ?? 0) * 100).toFixed(1)}%` : '--'}</strong>
          </span>
        </div>
        <div className={styles.marketDerivatives}>
          <span className={styles.marketIntelligenceTitle}>{market === 'futures' ? '衍生品数据' : '主动成交代理'}</span>
          {market === 'futures' && (
            <>
              <span><small>OI</small><strong>{derivatives?.open_interest_value_usdt ? `${compact(derivatives.open_interest_value_usdt)} USDT` : '--'}</strong></span>
              <span><small>资金费率</small><strong>{derivatives?.funding_rate ? `${(Number(derivatives.funding_rate) * 100).toFixed(4)}%` : '--'}</strong></span>
            </>
          )}
          <Tooltip title="由交易所近期公开成交计算，不等同于机构 NetFlow">
            <span><small>主动买/卖比 · 代理</small><strong>{Number.isFinite(derivatives?.taker_buy_sell_ratio) ? derivatives?.taker_buy_sell_ratio.toFixed(2) : '--'}</strong></span>
          </Tooltip>
        </div>
        <div className={styles.marketRisk}>
          <span className={styles.marketIntelligenceTitle}>风险雷达</span>
          <Tag color={riskColor}>{radar ? `${radar.risk_score}/100` : '--'}</Tag>
          <span><small>流动性</small><strong>{radar?.liquidity_risk ?? '--'}</strong></span>
          <span><small>拥挤度</small><strong>{radar?.crowding_risk ?? '--'}</strong></span>
        </div>
      </div>

      {error && klines.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={error}
          className={styles.marketEmpty}
        />
      ) : (
        <div ref={containerRef} className={styles.marketCanvas} />
      )}
    </div>
  );
};

export default DashboardMarketChart;
