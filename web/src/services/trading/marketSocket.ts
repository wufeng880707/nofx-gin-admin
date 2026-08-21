export type MarketStreamStatus =
  "connecting" | "connected" | "reconnecting" | "disconnected" | "error";

type MarketStreamParams = {
  exchange: TradingAPI.MarketExchange;
  market: TradingAPI.MarketType;
  symbol: string;
  interval: TradingAPI.MarketInterval;
};

type MarketStreamHandlers = {
  onTicker: (ticker: TradingAPI.MarketTicker) => void;
  onKline: (kline: TradingAPI.MarketKline) => void;
  onStatus: (status: MarketStreamStatus) => void;
};

export type MarketStreamConnection = {
  close: () => void;
};

const okxCandleChannels: Record<TradingAPI.MarketInterval, string> = {
  "1m": "candle1m",
  "5m": "candle5m",
  "15m": "candle15m",
  "1h": "candle1H",
  "4h": "candle4H",
  "1d": "candle1D",
  "1w": "candle1W",
};

const fixedDecimal = (value: number, precision: number) => {
  if (!Number.isFinite(value)) return "0";
  return value.toFixed(precision).replace(/\.?0+$/, "") || "0";
};

const subtractDecimal = (leftText: string, rightText: string) => {
  const left = leftText.match(/^([+-]?)(\d+)(?:\.(\d+))?$/);
  const right = rightText.match(/^([+-]?)(\d+)(?:\.(\d+))?$/);
  if (!left || !right) return "0";

  const scale = Math.max(left[3]?.length ?? 0, right[3]?.length ?? 0);
  const toScaledInteger = (parts: RegExpMatchArray) => {
    const fraction = (parts[3] ?? "").padEnd(scale, "0");
    const value = BigInt(`${parts[2]}${fraction}`);
    return parts[1] === "-" ? -value : value;
  };
  const difference = toScaledInteger(left) - toScaledInteger(right);
  const zero = BigInt(0);
  const sign = difference < zero ? "-" : "";
  const absolute = (difference < zero ? -difference : difference)
    .toString()
    .padStart(scale + 1, "0");
  if (scale === 0) return `${sign}${absolute}`;

  const integer = absolute.slice(0, -scale);
  const fraction = absolute.slice(-scale).replace(/0+$/, "");
  return `${sign}${integer}${fraction ? `.${fraction}` : ""}`;
};

const decimalChange = (lastText: string, openText: string) => {
  const last = Number(lastText);
  const open = Number(openText);
  if (!Number.isFinite(last) || !Number.isFinite(open) || open === 0) {
    return { change: "0", rate: "0" };
  }
  const change = last - open;
  return {
    change: subtractDecimal(lastText, openText),
    rate: fixedDecimal((change / open) * 100, 6),
  };
};

const buildBinanceURL = ({ market, symbol, interval }: MarketStreamParams) => {
  const base =
    market === "spot"
      ? "wss://stream.binance.com:9443"
      : "wss://fstream.binance.com";
  const normalizedSymbol = symbol.replaceAll("-", "").toLowerCase();
  return `${base}/stream?streams=${normalizedSymbol}@ticker/${normalizedSymbol}@kline_${interval}`;
};

const parseBinanceMessage = (
  params: MarketStreamParams,
  raw: string,
  handlers: MarketStreamHandlers,
) => {
  const envelope = JSON.parse(raw) as {
    data?: Record<string, unknown>;
  };
  const data = envelope.data;
  if (!data) return;

  if (data.e === "24hrTicker") {
    handlers.onTicker({
      exchange: "binance",
      market: params.market,
      symbol: String(data.s ?? params.symbol),
      last_price: String(data.c ?? ""),
      open_price: String(data.o ?? ""),
      high_price: String(data.h ?? ""),
      low_price: String(data.l ?? ""),
      price_change: String(data.p ?? ""),
      price_change_rate: String(data.P ?? ""),
      base_volume: String(data.v ?? ""),
      quote_volume: String(data.q ?? ""),
      timestamp: Number(data.E ?? Date.now()),
    });
    return;
  }

  if (data.e === "kline" && data.k && typeof data.k === "object") {
    const kline = data.k as Record<string, unknown>;
    handlers.onKline({
      time: Math.floor(Number(kline.t) / 1000),
      open: String(kline.o ?? ""),
      high: String(kline.h ?? ""),
      low: String(kline.l ?? ""),
      close: String(kline.c ?? ""),
      volume: String(kline.v ?? ""),
      quote_volume: String(kline.q ?? ""),
      closed: Boolean(kline.x),
    });
  }
};

const parseOKXMessage = (
  params: MarketStreamParams,
  raw: string,
  handlers: MarketStreamHandlers,
) => {
  if (raw === "pong") return;
  const message = JSON.parse(raw) as {
    arg?: { channel?: string; instId?: string };
    data?: unknown[];
  };
  const channel = message.arg?.channel ?? "";
  const first = message.data?.[0];
  if (!first) return;

  if (channel === "tickers" && typeof first === "object") {
    const ticker = first as Record<string, unknown>;
    const lastPrice = String(ticker.last ?? "");
    const openPrice = String(ticker.open24h ?? "");
    const { change, rate } = decimalChange(lastPrice, openPrice);
    const isFutures = params.market === "futures";
    handlers.onTicker({
      exchange: "okx",
      market: params.market,
      symbol: String(ticker.instId ?? params.symbol),
      last_price: lastPrice,
      open_price: openPrice,
      high_price: String(ticker.high24h ?? ""),
      low_price: String(ticker.low24h ?? ""),
      price_change: change,
      price_change_rate: rate,
      base_volume: String(
        isFutures ? (ticker.volCcy24h ?? "") : (ticker.vol24h ?? ""),
      ),
      quote_volume: String(
        isFutures ? (ticker.volCcyQuote24h ?? "") : (ticker.volCcy24h ?? ""),
      ),
      timestamp: Number(ticker.ts ?? Date.now()),
    });
    return;
  }

  if (
    channel.startsWith("candle") &&
    Array.isArray(first) &&
    first.length >= 9
  ) {
    handlers.onKline({
      time: Math.floor(Number(first[0]) / 1000),
      open: String(first[1] ?? ""),
      high: String(first[2] ?? ""),
      low: String(first[3] ?? ""),
      close: String(first[4] ?? ""),
      volume: String(first[5] ?? ""),
      quote_volume: String(first[7] ?? ""),
      closed: String(first[8]) === "1",
    });
  }
};

export const connectMarketStream = (
  params: MarketStreamParams,
  handlers: MarketStreamHandlers,
): MarketStreamConnection => {
  let socket: WebSocket | undefined;
  let reconnectTimer: number | undefined;
  let pingTimer: number | undefined;
  let closed = false;
  let retries = 0;

  const clearTimers = () => {
    if (reconnectTimer) window.clearTimeout(reconnectTimer);
    if (pingTimer) window.clearInterval(pingTimer);
    reconnectTimer = undefined;
    pingTimer = undefined;
  };

  const scheduleReconnect = () => {
    if (closed || reconnectTimer) return;
    handlers.onStatus("reconnecting");
    const delay = Math.min(1000 * 2 ** retries, 15000);
    retries += 1;
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = undefined;
      connect();
    }, delay);
  };

  const connect = () => {
    if (closed) return;
    handlers.onStatus(retries > 0 ? "reconnecting" : "connecting");
    const url =
      params.exchange === "binance"
        ? buildBinanceURL(params)
        : "wss://ws.okx.com:8443/ws/v5/public";
    const currentSocket = new WebSocket(url);
    socket = currentSocket;

    currentSocket.onopen = () => {
      if (closed || socket !== currentSocket) return;
      retries = 0;
      handlers.onStatus("connected");
      if (params.exchange === "okx") {
        currentSocket.send(
          JSON.stringify({
            op: "subscribe",
            args: [
              { channel: "tickers", instId: params.symbol },
              {
                channel: okxCandleChannels[params.interval],
                instId: params.symbol,
              },
            ],
          }),
        );
        pingTimer = window.setInterval(() => {
          if (currentSocket.readyState === WebSocket.OPEN)
            currentSocket.send("ping");
        }, 20000);
      }
    };

    currentSocket.onmessage = (event) => {
      if (closed || socket !== currentSocket || typeof event.data !== "string")
        return;
      try {
        if (params.exchange === "binance") {
          parseBinanceMessage(params, event.data, handlers);
        } else {
          parseOKXMessage(params, event.data, handlers);
        }
      } catch {
        // Ignore malformed third-party payloads; the next exchange push remains usable.
      }
    };

    currentSocket.onerror = () => {
      if (closed || socket !== currentSocket) return;
      handlers.onStatus("error");
      currentSocket.close();
    };

    currentSocket.onclose = () => {
      if (socket === currentSocket) socket = undefined;
      if (pingTimer) window.clearInterval(pingTimer);
      pingTimer = undefined;
      scheduleReconnect();
    };
  };

  connect();
  return {
    close: () => {
      closed = true;
      clearTimers();
      if (socket && socket.readyState < WebSocket.CLOSING) {
        socket.close(1000, "market scope changed");
      }
      socket = undefined;
    },
  };
};
