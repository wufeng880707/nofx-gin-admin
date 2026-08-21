import {
  ArrowsAltOutlined,
  CompressOutlined,
  LineChartOutlined,
  ReloadOutlined,
  StarFilled,
  StarOutlined,
} from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import { useModel } from "@umijs/max";
import {
  Alert,
  Button,
  Empty,
  Select,
  Spin,
  Tooltip,
  theme as antdTheme,
} from "antd";
import {
  CandlestickSeries,
  ColorType,
  createChart,
  HistogramSeries,
  type CandlestickData,
  type HistogramData,
  type IChartApi,
  type ISeriesApi,
  type Time,
} from "lightweight-charts";
import React, {
  type CSSProperties,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  fetchLiquidationHeatmap,
  fetchMarketInstruments,
  fetchMarketKlines,
  fetchMarketTicker,
} from "@/services/trading/market";
import {
  connectMarketStream,
  type MarketStreamStatus,
} from "@/services/trading/marketSocket";
import { formatDecimal } from "@/utils/decimal";
import styles from "./index.less";

const intervals: TradingAPI.MarketInterval[] = [
  "1m",
  "5m",
  "15m",
  "1h",
  "4h",
  "1d",
  "1w",
];

const streamLabels: Record<MarketStreamStatus, string> = {
  connecting: "WS 连接中",
  connected: "WS 实时",
  reconnecting: "WS 重连中",
  disconnected: "WS 已断开",
  error: "WS 异常",
};

const defaultSymbol = (
  exchange: TradingAPI.MarketExchange,
  market: TradingAPI.MarketType,
) => {
  if (exchange === "okx") {
    return market === "spot" ? "BTC-USDT" : "BTC-USDT-SWAP";
  }
  return "BTCUSDT";
};

const formatCompact = (value: unknown) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return "-";
  return new Intl.NumberFormat("en-US", {
    notation: "compact",
    maximumFractionDigits: 2,
  }).format(number);
};

const upsertKline = (
  items: TradingAPI.MarketKline[],
  next: TradingAPI.MarketKline,
) => {
  const result = items.filter((item) => item.time !== next.time);
  result.push(next);
  result.sort((left, right) => left.time - right.time);
  return result.slice(-300);
};

const LiquidationHeatmap: React.FC<{
  data?: TradingAPI.LiquidationHeatmap;
  loading: boolean;
  hours: number;
  onHoursChange: (value: number) => void;
}> = ({ data, loading, hours, onHoursChange }) => {
  const cells = useMemo(() => {
    const map = new Map<string, TradingAPI.LiquidationHeatmapCell>();
    for (const item of data?.cells ?? []) {
      map.set(`${item.time_index}:${item.price_index}`, item);
    }
    return map;
  }, [data]);
  const maxNotional = useMemo(
    () =>
      Math.max(
        1,
        ...(data?.cells ?? []).map(
          (item) => item.long_notional_usdt + item.short_notional_usdt,
        ),
      ),
    [data],
  );
  const timeBuckets = data?.time_buckets ?? 24;
  const priceBuckets = data?.price_buckets ?? 20;

  return (
    <section className={styles.heatmapSection} aria-label="Binance 实时清算分布">
      <div className={styles.heatmapHeader}>
        <div>
          <strong>Binance 实时清算分布</strong>
          <span>公共 USD-M 强平快照，仅包含采集器启用后的数据</span>
        </div>
        <div className={styles.heatmapSummary}>
          <span>{data?.event_count ?? 0} 笔</span>
          <span>{formatCompact(data?.total_notional_usdt ?? 0)} USDT</span>
          <Select
            aria-label="清算统计周期"
            value={hours}
            size="small"
            className={styles.heatmapRange}
            options={[
              { label: "24 小时", value: 24 },
              { label: "3 天", value: 72 },
              { label: "7 天", value: 168 },
            ]}
            onChange={onHoursChange}
          />
        </div>
      </div>
      <Spin spinning={loading}>
        {!data?.data_available ? (
          <Empty
            className={styles.heatmapEmpty}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={data?.coverage_notice ?? "尚未采集到公开强平数据"}
          />
        ) : (
          <div className={styles.heatmapViewport}>
            <div className={styles.priceAxis}>
              <span>{formatDecimal(data.max_price, { grouping: true })}</span>
              <span>{formatDecimal(data.min_price, { grouping: true })}</span>
            </div>
            <div
              className={styles.heatmapGrid}
              style={{
                gridTemplateColumns: `repeat(${timeBuckets}, minmax(16px, 1fr))`,
                gridTemplateRows: `repeat(${priceBuckets}, 16px)`,
              }}
            >
              {Array.from({ length: priceBuckets }).flatMap((_, displayRow) => {
                const priceIndex = priceBuckets - displayRow - 1;
                return Array.from({ length: timeBuckets }).map((__, timeIndex) => {
                  const item = cells.get(`${timeIndex}:${priceIndex}`);
                  const long = item?.long_notional_usdt ?? 0;
                  const short = item?.short_notional_usdt ?? 0;
                  const longAlpha = Math.min(0.9, 0.12 + (long / maxNotional) * 0.78);
                  const shortAlpha = Math.min(0.9, 0.12 + (short / maxNotional) * 0.78);
                  const background =
                    long > 0 && short > 0
                      ? `linear-gradient(135deg, rgba(234,57,67,${longAlpha}) 0 50%, rgba(22,199,132,${shortAlpha}) 50% 100%)`
                      : long > 0
                        ? `rgba(234,57,67,${longAlpha})`
                        : short > 0
                          ? `rgba(22,199,132,${shortAlpha})`
                          : "transparent";
                  return (
                    <Tooltip
                      key={`${timeIndex}:${priceIndex}`}
                      title={
                        item
                          ? `多头强平 ${formatCompact(long)} USDT · 空头强平 ${formatCompact(short)} USDT · ${item.event_count} 笔`
                          : "无记录"
                      }
                    >
                      <span className={styles.heatmapCell} style={{ background }} />
                    </Tooltip>
                  );
                });
              })}
            </div>
          </div>
        )}
      </Spin>
      <div className={styles.heatmapLegend}>
        <span><i className={styles.longLegend} />多头强平</span>
        <span><i className={styles.shortLegend} />空头强平</span>
        <small>{data?.coverage_notice}</small>
      </div>
    </section>
  );
};

const MarketChart: React.FC = () => {
  const { initialState } = useModel("@@initialState");
  const { token } = antdTheme.useToken();
  const isDark = initialState?.settings?.navTheme === "realDark";
  const terminalRef = useRef<HTMLDivElement>(null);
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const lastFitScopeRef = useRef("");
  const hasMarketDataRef = useRef(false);
  const restLoadingScopeRef = useRef("");
  const lastStreamPriceRef = useRef("");

  const [exchange, setExchange] =
    useState<TradingAPI.MarketExchange>("binance");
  const [market, setMarket] = useState<TradingAPI.MarketType>("spot");
  const [symbol, setSymbol] = useState("BTCUSDT");
  const [interval, setIntervalValue] =
    useState<TradingAPI.MarketInterval>("15m");
  const [instruments, setInstruments] = useState<TradingAPI.MarketInstrument[]>(
    [],
  );
  const [ticker, setTicker] = useState<TradingAPI.MarketTicker>();
  const [klines, setKlines] = useState<TradingAPI.MarketKline[]>([]);
  const [streamStatus, setStreamStatus] =
    useState<MarketStreamStatus>("connecting");
  const [priceDirection, setPriceDirection] = useState<"up" | "down" | "flat">(
    "flat",
  );
  const [lastUpdatedAt, setLastUpdatedAt] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [favorite, setFavorite] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [heatmapHours, setHeatmapHours] = useState(24);
  const [heatmap, setHeatmap] = useState<TradingAPI.LiquidationHeatmap>();
  const [heatmapLoading, setHeatmapLoading] = useState(false);

  const marketTheme = useMemo(
    () => ({
      accent:
        initialState?.settings?.colorPrimary ||
        (isDark ? "#f0b90b" : token.colorPrimary),
      pageBackground: isDark ? "#090d0f" : token.colorBgLayout,
      surface: isDark ? "#171d22" : token.colorBgContainer,
      raisedSurface: isDark ? "#1d242a" : token.colorFillAlter,
      chartBackground: isDark ? "#090d0f" : token.colorBgContainer,
      toolbarBackground: isDark ? "#101518" : token.colorFillQuaternary,
      border: isDark ? "#2a333a" : token.colorBorderSecondary,
      text: isDark ? "#e8edf2" : token.colorText,
      secondaryText: isDark ? "#8b96a1" : token.colorTextSecondary,
      mutedText: isDark ? "#66717c" : token.colorTextTertiary,
      hover: isDark ? "#252d34" : token.colorFillSecondary,
      grid: isDark ? "#20272d" : token.colorSplit,
      crosshair: isDark ? "#59636e" : token.colorTextQuaternary,
      crosshairLabel: isDark ? "#303941" : token.colorTextSecondary,
    }),
    [
      initialState?.settings?.colorPrimary,
      isDark,
      token.colorBgContainer,
      token.colorBgLayout,
      token.colorBorderSecondary,
      token.colorFillAlter,
      token.colorFillQuaternary,
      token.colorFillSecondary,
      token.colorPrimary,
      token.colorSplit,
      token.colorText,
      token.colorTextQuaternary,
      token.colorTextSecondary,
      token.colorTextTertiary,
    ],
  );

  const marketThemeStyle = useMemo(
    () =>
      ({
        "--market-accent": marketTheme.accent,
        "--market-page-bg": marketTheme.pageBackground,
        "--market-surface": marketTheme.surface,
        "--market-surface-raised": marketTheme.raisedSurface,
        "--market-chart-bg": marketTheme.chartBackground,
        "--market-toolbar-bg": marketTheme.toolbarBackground,
        "--market-border": marketTheme.border,
        "--market-text": marketTheme.text,
        "--market-text-secondary": marketTheme.secondaryText,
        "--market-text-muted": marketTheme.mutedText,
        "--market-hover": marketTheme.hover,
      }) as CSSProperties,
    [marketTheme],
  );

  const scopeKey = `${exchange}:${market}:${symbol}:${interval}`;
  const activeScopeRef = useRef(scopeKey);
  activeScopeRef.current = scopeKey;

  const applyTicker = useCallback((next: TradingAPI.MarketTicker) => {
    const previousPrice = Number(lastStreamPriceRef.current);
    const nextPrice = Number(next.last_price);
    if (
      Number.isFinite(previousPrice) &&
      previousPrice > 0 &&
      Number.isFinite(nextPrice)
    ) {
      setPriceDirection(
        nextPrice > previousPrice
          ? "up"
          : nextPrice < previousPrice
            ? "down"
            : "flat",
      );
    }
    lastStreamPriceRef.current = next.last_price;
    setTicker(next);
    setLastUpdatedAt(Date.now());
  }, []);

  useEffect(() => {
    let active = true;
    setInstruments([]);
    const preferredSymbol = defaultSymbol(exchange, market);
    setSymbol(preferredSymbol);
    fetchMarketInstruments({ exchange, market })
      .then((response) => {
        if (!active) return;
        const items = response.data ?? [];
        setInstruments(items);
        if (
          !items.some((item) => item.symbol === preferredSymbol) &&
          items[0]
        ) {
          setSymbol(items[0].symbol);
        }
      })
      .catch(() => {
        if (active && !hasMarketDataRef.current) {
          setError("币对列表加载失败，请检查交易所网络连接。");
        }
      });
    return () => {
      active = false;
    };
  }, [exchange, market]);

  const loadMarketData = useCallback(
    async (showLoading = false) => {
      if (!symbol) return;
      const requestedScope = `${exchange}:${market}:${symbol}:${interval}`;
      if (restLoadingScopeRef.current === requestedScope) return;
      restLoadingScopeRef.current = requestedScope;
      if (showLoading) setLoading(true);
      try {
        const [tickerResponse, klineResponse] = await Promise.all([
          fetchMarketTicker({ exchange, market, symbol }),
          fetchMarketKlines({ exchange, market, symbol, interval, limit: 300 }),
        ]);
        if (activeScopeRef.current !== requestedScope) return;
        if (tickerResponse.data) applyTicker(tickerResponse.data);
        setKlines((current) =>
          current.reduce(
            (items, item) => upsertKline(items, item),
            klineResponse.data ?? [],
          ),
        );
        hasMarketDataRef.current = true;
        setError("");
      } catch {
        if (
          activeScopeRef.current === requestedScope &&
          !hasMarketDataRef.current
        ) {
          setError("行情加载失败，正在等待 WebSocket 或下一次重试。");
        }
      } finally {
        if (restLoadingScopeRef.current === requestedScope) {
          restLoadingScopeRef.current = "";
        }
        if (showLoading && activeScopeRef.current === requestedScope) {
          setLoading(false);
        }
      }
    },
    [applyTicker, exchange, interval, market, symbol],
  );

  useEffect(() => {
    hasMarketDataRef.current = false;
    lastStreamPriceRef.current = "";
    setTicker(undefined);
    setKlines([]);
    setPriceDirection("flat");
    setStreamStatus("connecting");
    void loadMarketData(true);
  }, [loadMarketData]);

  useEffect(() => {
    const requestedScope = scopeKey;
    const connection = connectMarketStream(
      { exchange, market, symbol, interval },
      {
        onTicker: (next) => {
          if (activeScopeRef.current !== requestedScope) return;
          applyTicker(next);
          hasMarketDataRef.current = true;
          setError("");
        },
        onKline: (next) => {
          if (activeScopeRef.current !== requestedScope) return;
          setKlines((current) => upsertKline(current, next));
          setLastUpdatedAt(Date.now());
          setLoading(false);
          hasMarketDataRef.current = true;
          setError("");
        },
        onStatus: (status) => {
          if (activeScopeRef.current === requestedScope)
            setStreamStatus(status);
        },
      },
    );
    return connection.close;
  }, [applyTicker, exchange, interval, market, scopeKey, symbol]);

  useEffect(() => {
    if (streamStatus === "connected" || streamStatus === "connecting") return;
    const timer = window.setInterval(() => void loadMarketData(false), 15000);
    return () => window.clearInterval(timer);
  }, [loadMarketData, streamStatus]);

  useEffect(() => {
    let active = true;
    if (exchange !== "binance" || market !== "futures" || !symbol) {
      setHeatmap(undefined);
      setHeatmapLoading(false);
      return () => {
        active = false;
      };
    }
    const load = async () => {
      setHeatmapLoading(true);
      try {
        const response = await fetchLiquidationHeatmap({
          symbol,
          hours: heatmapHours,
          timeBuckets: 24,
          priceBuckets: 20,
        });
        if (active) setHeatmap(response.data);
      } catch {
        if (active) setHeatmap(undefined);
      } finally {
        if (active) setHeatmapLoading(false);
      }
    };
    void load();
    const timer = window.setInterval(() => void load(), 60_000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [exchange, heatmapHours, market, symbol]);

  useEffect(() => {
    const container = chartContainerRef.current;
    if (!container) return;

    const chart = createChart(container, {
      width: container.clientWidth,
      height: Math.max(container.clientHeight, 460),
      layout: {
        background: {
          type: ColorType.Solid,
          color: marketTheme.chartBackground,
        },
        textColor: marketTheme.secondaryText,
        attributionLogo: true,
      },
      grid: {
        vertLines: { color: marketTheme.grid },
        horzLines: { color: marketTheme.grid },
      },
      rightPriceScale: {
        borderColor: marketTheme.border,
        scaleMargins: { top: 0.08, bottom: 0.22 },
      },
      timeScale: {
        borderColor: marketTheme.border,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 5,
      },
      crosshair: {
        vertLine: {
          color: marketTheme.crosshair,
          labelBackgroundColor: marketTheme.crosshairLabel,
        },
        horzLine: {
          color: marketTheme.crosshair,
          labelBackgroundColor: marketTheme.crosshairLabel,
        },
      },
      localization: { locale: "zh-CN" },
    });
    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: "#16c784",
      downColor: "#ea3943",
      borderUpColor: "#16c784",
      borderDownColor: "#ea3943",
      wickUpColor: "#16c784",
      wickDownColor: "#ea3943",
    });
    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "",
    });
    volumeSeries
      .priceScale()
      .applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } });

    chartRef.current = chart;
    candleSeriesRef.current = candleSeries;
    volumeSeriesRef.current = volumeSeries;

    const observer = new ResizeObserver(([entry]) => {
      chart.applyOptions({
        width: entry.contentRect.width,
        height: Math.max(entry.contentRect.height, 420),
      });
    });
    observer.observe(container);
    return () => {
      observer.disconnect();
      chart.remove();
      chartRef.current = null;
      candleSeriesRef.current = null;
      volumeSeriesRef.current = null;
    };
  }, [marketTheme]);

  useEffect(() => {
    const chart = chartRef.current;
    const candleSeries = candleSeriesRef.current;
    const volumeSeries = volumeSeriesRef.current;
    if (!chart || !candleSeries || !volumeSeries) return;

    const candles: CandlestickData<Time>[] = klines.map((item) => ({
      time: item.time as Time,
      open: Number(item.open),
      high: Number(item.high),
      low: Number(item.low),
      close: Number(item.close),
    }));
    const volumes: HistogramData<Time>[] = klines.map((item) => ({
      time: item.time as Time,
      value: Number(item.volume),
      color:
        Number(item.close) >= Number(item.open) ? "#16c78455" : "#ea394355",
    }));
    candleSeries.setData(candles);
    volumeSeries.setData(volumes);
    if (klines.length > 0 && lastFitScopeRef.current !== scopeKey) {
      chart.timeScale().fitContent();
      lastFitScopeRef.current = scopeKey;
    }
  }, [klines, marketTheme, scopeKey]);

  useEffect(() => {
    const handleFullscreen = () =>
      setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", handleFullscreen);
    return () =>
      document.removeEventListener("fullscreenchange", handleFullscreen);
  }, []);

  const symbolOptions = useMemo(
    () =>
      instruments.map((item) => ({
        value: item.symbol,
        label: item.display,
      })),
    [instruments],
  );
  const selectedInstrument = useMemo(
    () => instruments.find((item) => item.symbol === symbol),
    [instruments, symbol],
  );
  const baseAsset = selectedInstrument?.base_asset ?? symbol.split("-")[0];
  const quoteAsset = selectedInstrument?.quote_asset ?? "USDT";
  const changeRate = Number(ticker?.price_change_rate ?? 0);
  const changeTone = changeRate > 0 ? "up" : changeRate < 0 ? "down" : "flat";
  const volume = `${formatCompact(ticker?.base_volume)} ${baseAsset}`;
  const turnover = ticker?.quote_volume
    ? `${formatCompact(ticker.quote_volume)} ${quoteAsset}`
    : "-";

  const toggleFullscreen = async () => {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
    } else {
      await terminalRef.current?.requestFullscreen();
    }
  };

  return (
    <PageContainer
      title={false}
      className={styles.pageContainer}
      style={marketThemeStyle}
    >
      <div
        ref={terminalRef}
        className={`${styles.terminal} ${
          isDark ? styles.darkTheme : styles.lightTheme
        }`}
      >
        <header className={styles.quoteBar}>
          <div className={styles.symbolBlock}>
            <Tooltip title={favorite ? "取消收藏" : "收藏币对"}>
              <button
                type="button"
                className={styles.iconButton}
                aria-label={favorite ? "取消收藏" : "收藏币对"}
                aria-pressed={favorite}
                onClick={() => setFavorite((value) => !value)}
              >
                {favorite ? <StarFilled /> : <StarOutlined />}
              </button>
            </Tooltip>
            <span className={styles.coinBadge}>
              {baseAsset === "BTC" ? "₿" : baseAsset.slice(0, 1)}
            </span>
            <Select
              showSearch
              aria-label="选择币对"
              value={symbol}
              options={symbolOptions}
              loading={instruments.length === 0}
              optionFilterProp="label"
              popupMatchSelectWidth={320}
              popupClassName={styles.marketDropdown}
              variant="borderless"
              className={styles.symbolSelect}
              onChange={setSymbol}
            />
          </div>

          <div className={styles.priceCell}>
            <strong className={styles[priceDirection]}>
              {formatDecimal(ticker?.last_price, { grouping: true })}
            </strong>
            <span>
              {lastUpdatedAt
                ? new Date(lastUpdatedAt).toLocaleTimeString("zh-CN", {
                    hour12: false,
                  })
                : "--:--:--"}
            </span>
          </div>

          <div className={styles.metric}>
            <span>Change</span>
            <strong className={styles[changeTone]}>
              {formatDecimal(ticker?.price_change, { grouping: true })}
              <small>{formatDecimal(ticker?.price_change_rate)}%</small>
            </strong>
          </div>
          <div className={styles.metric}>
            <span>High</span>
            <strong>
              {formatDecimal(ticker?.high_price, { grouping: true })}
            </strong>
          </div>
          <div className={styles.metric}>
            <span>Low</span>
            <strong>
              {formatDecimal(ticker?.low_price, { grouping: true })}
            </strong>
          </div>
          <div className={styles.metric}>
            <span>24H Volume</span>
            <strong>{volume}</strong>
          </div>
          <div className={styles.metric}>
            <span>24H Turnover</span>
            <strong>{turnover}</strong>
          </div>
        </header>

        <nav className={styles.chartToolbar} aria-label="图表工具栏">
          <div className={styles.intervalGroup}>
            {intervals.map((item) => (
              <button
                type="button"
                key={item}
                className={
                  item === interval ? styles.activeInterval : undefined
                }
                aria-pressed={item === interval}
                onClick={() => setIntervalValue(item)}
              >
                {item}
              </button>
            ))}
          </div>
          <span className={styles.toolbarDivider} />
          <LineChartOutlined
            className={styles.chartTypeIcon}
            aria-label="K线图"
          />
          <div className={styles.toolbarSpacer} />
          <Select
            aria-label="选择交易所"
            value={exchange}
            variant="borderless"
            popupClassName={styles.marketDropdown}
            className={styles.scopeSelect}
            options={[
              { label: "Binance", value: "binance" },
              { label: "OKX", value: "okx" },
            ]}
            onChange={(value) => {
              setExchange(value);
              setSymbol(defaultSymbol(value, market));
            }}
          />
          <Select
            aria-label="选择市场"
            value={market}
            variant="borderless"
            popupClassName={styles.marketDropdown}
            className={styles.marketSelect}
            options={[
              { label: "现货", value: "spot" },
              { label: "USDT 永续", value: "futures" },
            ]}
            onChange={(value) => {
              setMarket(value);
              setSymbol(defaultSymbol(exchange, value));
            }}
          />
          <span className={`${styles.streamStatus} ${styles[streamStatus]}`}>
            <i />
            {streamLabels[streamStatus]}
          </span>
          <span className={styles.tradingViewLabel}>TradingView</span>
          <Tooltip title="重新加载历史行情">
            <Button
              type="text"
              className={styles.toolbarButton}
              aria-label="重新加载历史行情"
              icon={<ReloadOutlined />}
              onClick={() => void loadMarketData(true)}
            />
          </Tooltip>
          <Tooltip title={fullscreen ? "退出全屏" : "全屏"}>
            <Button
              type="text"
              className={styles.toolbarButton}
              aria-label={fullscreen ? "退出全屏" : "全屏"}
              icon={fullscreen ? <CompressOutlined /> : <ArrowsAltOutlined />}
              onClick={() => void toggleFullscreen()}
            />
          </Tooltip>
        </nav>

        {error && (
          <Alert
            className={styles.alert}
            type="warning"
            showIcon
            message={error}
          />
        )}

        <Spin
          spinning={loading}
          tip="正在加载行情"
          wrapperClassName={styles.chartSpin}
        >
          <div ref={chartContainerRef} className={styles.chart} />
        </Spin>

        {exchange === "binance" && market === "futures" ? (
          <LiquidationHeatmap
            data={heatmap}
            loading={heatmapLoading}
            hours={heatmapHours}
            onHoursChange={setHeatmapHours}
          />
        ) : (
          <section className={styles.heatmapUnsupported}>
            清算热力图当前仅使用 Binance USD-M
            公共强平流；OKX 已下线公开全市场强平接口，现货市场不提供强平数据。
          </section>
        )}
      </div>
    </PageContainer>
  );
};

export default MarketChart;
