package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	bybitOrderHistoryLimit   = 50
	bybitFillHistoryLimit    = 100
	bybitHistoryMaxPages     = 10000
	bybitHistoryWindow       = 7 * 24 * time.Hour
	bybitHistoryPageInterval = 25 * time.Millisecond
)

type bybitHistoryMarket struct {
	category   string
	settleCoin string
}

type bybitOrderRow struct {
	OrderID       string `json:"orderId"`
	OrderLinkID   string `json:"orderLinkId"`
	Symbol        string `json:"symbol"`
	Price         string `json:"price"`
	Quantity      string `json:"qty"`
	Side          string `json:"side"`
	PositionIdx   int    `json:"positionIdx"`
	OrderStatus   string `json:"orderStatus"`
	AveragePrice  string `json:"avgPrice"`
	ExecutedQty   string `json:"cumExecQty"`
	ExecutedValue string `json:"cumExecValue"`
	OrderType     string `json:"orderType"`
	ReduceOnly    bool   `json:"reduceOnly"`
	CreatedTime   string `json:"createdTime"`
	UpdatedTime   string `json:"updatedTime"`
}

type bybitFillRow struct {
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId"`
	Side        string `json:"side"`
	PositionIdx int    `json:"positionIdx"`
	OrderType   string `json:"orderType"`
	ExecFee     string `json:"execFee"`
	ExecID      string `json:"execId"`
	ExecPrice   string `json:"execPrice"`
	ExecQty     string `json:"execQty"`
	ExecValue   string `json:"execValue"`
	ExecTime    string `json:"execTime"`
	FeeCurrency string `json:"feeCurrency"`
	IsMaker     bool   `json:"isMaker"`
}

type bybitOrderHistoryResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List           []bybitOrderRow `json:"list"`
		NextPageCursor string          `json:"nextPageCursor"`
	} `json:"result"`
}

type bybitFillHistoryResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List           []bybitFillRow `json:"list"`
		NextPageCursor string         `json:"nextPageCursor"`
	} `json:"result"`
}

func (a *Bybit) History(ctx context.Context, request HistoryRequest) (*HistoryBatch, error) {
	until := request.Until.UTC()
	if until.IsZero() {
		until = a.now().UTC()
	}
	since := request.Since.UTC()
	if since.IsZero() || !since.Before(until) {
		since = until.Add(-bybitHistoryWindow)
	}
	if oldest := until.AddDate(-2, 0, 0); since.Before(oldest) {
		since = oldest
	}

	batch := &HistoryBatch{SyncedUntil: until}
	markets := []bybitHistoryMarket{{category: "spot"}, {category: "linear", settleCoin: "USDT"}}
	for start := since; start.Before(until); {
		end := start.Add(bybitHistoryWindow - time.Millisecond)
		if end.After(until) {
			end = until
		}
		for _, market := range markets {
			if err := a.loadBybitOrders(ctx, market, start, end, batch); err != nil {
				return nil, err
			}
			if err := a.loadBybitFills(ctx, market, start, end, batch); err != nil {
				return nil, err
			}
		}
		start = end.Add(time.Millisecond)
	}
	return batch, nil
}

func (a *Bybit) loadBybitOrders(ctx context.Context, market bybitHistoryMarket, since, until time.Time, batch *HistoryBatch) error {
	cursor := ""
	for page := 0; page < bybitHistoryMaxPages; page++ {
		if page > 0 {
			if err := waitContext(ctx, bybitHistoryPageInterval); err != nil {
				return err
			}
		}
		query := bybitHistoryQuery(market, since, until, bybitOrderHistoryLimit, cursor)
		var response bybitOrderHistoryResponse
		if err := a.signedGet(ctx, "/v5/order/history", query, &response); err != nil {
			return fmt.Errorf("load Bybit %s orders: %w", market.category, err)
		}
		if response.RetCode != 0 {
			return fmt.Errorf("bybit %s order history error %d: %s", market.category, response.RetCode, response.RetMsg)
		}
		for _, row := range response.Result.List {
			batch.Orders = append(batch.Orders, normalizeBybitOrder(row, market.category))
		}
		next := strings.TrimSpace(response.Result.NextPageCursor)
		if next == "" {
			return nil
		}
		if next == cursor {
			return fmt.Errorf("Bybit %s order history cursor did not advance", market.category)
		}
		cursor = next
	}
	return fmt.Errorf("Bybit %s order history exceeded %d pages", market.category, bybitHistoryMaxPages)
}

func (a *Bybit) loadBybitFills(ctx context.Context, market bybitHistoryMarket, since, until time.Time, batch *HistoryBatch) error {
	cursor := ""
	for page := 0; page < bybitHistoryMaxPages; page++ {
		if page > 0 {
			if err := waitContext(ctx, bybitHistoryPageInterval); err != nil {
				return err
			}
		}
		query := bybitHistoryQuery(market, since, until, bybitFillHistoryLimit, cursor)
		var response bybitFillHistoryResponse
		if err := a.signedGet(ctx, "/v5/execution/list", query, &response); err != nil {
			return fmt.Errorf("load Bybit %s fills: %w", market.category, err)
		}
		if response.RetCode != 0 {
			return fmt.Errorf("bybit %s fill history error %d: %s", market.category, response.RetCode, response.RetMsg)
		}
		for _, row := range response.Result.List {
			batch.Fills = append(batch.Fills, normalizeBybitFill(row, market.category, market.settleCoin))
		}
		next := strings.TrimSpace(response.Result.NextPageCursor)
		if next == "" {
			return nil
		}
		if next == cursor {
			return fmt.Errorf("Bybit %s fill history cursor did not advance", market.category)
		}
		cursor = next
	}
	return fmt.Errorf("Bybit %s fill history exceeded %d pages", market.category, bybitHistoryMaxPages)
}

func bybitHistoryQuery(market bybitHistoryMarket, since, until time.Time, limit int, cursor string) url.Values {
	query := url.Values{
		"category":  {market.category},
		"startTime": {millis(since)},
		"endTime":   {millis(until)},
		"limit":     {strconv.Itoa(limit)},
	}
	if market.settleCoin != "" {
		query.Set("settleCoin", market.settleCoin)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	return query
}

func normalizeBybitOrder(row bybitOrderRow, market string) *Order {
	ordered := bybitTime(row.CreatedTime)
	updated := bybitTime(row.UpdatedTime)
	if updated.IsZero() {
		updated = ordered
	}
	return &Order{
		Market: market, ExchangeOrderID: row.OrderID, ClientOrderID: row.OrderLinkID,
		Symbol: strings.ToUpper(row.Symbol), Side: strings.ToLower(row.Side), PositionSide: bybitPositionSide(row.PositionIdx),
		OrderType: strings.ToLower(row.OrderType), Status: strings.ToLower(row.OrderStatus), Price: zeroDecimal(row.Price),
		AveragePrice: zeroDecimal(row.AveragePrice), Quantity: zeroDecimal(row.Quantity), ExecutedQuantity: zeroDecimal(row.ExecutedQty),
		QuoteQuantity: zeroDecimal(row.ExecutedValue), ReduceOnly: row.ReduceOnly, OrderedAt: ordered, ExchangeUpdatedAt: updated,
	}
}

func normalizeBybitFill(row bybitFillRow, market, settleCoin string) *Fill {
	feeAsset := strings.ToUpper(strings.TrimSpace(row.FeeCurrency))
	if feeAsset == "" && market == "linear" {
		feeAsset = strings.ToUpper(settleCoin)
	}
	fee := decimalAbs(row.ExecFee)
	feeValue := "0"
	if isUSDStableAsset(feeAsset) {
		feeValue = fee
	}
	liquidity := "taker"
	if row.IsMaker {
		liquidity = "maker"
	}
	return &Fill{
		Market: market, ExchangeTradeID: row.ExecID, ExchangeOrderID: row.OrderID,
		Symbol: strings.ToUpper(row.Symbol), Side: strings.ToLower(row.Side), PositionSide: bybitPositionSide(row.PositionIdx),
		Price: zeroDecimal(row.ExecPrice), Quantity: zeroDecimal(row.ExecQty), QuoteQuantity: zeroDecimal(row.ExecValue),
		FeeAsset: feeAsset, FeeQuantity: fee, FeeValueUSDT: feeValue, RealizedPnL: "0", Liquidity: liquidity,
		FilledAt: bybitTime(row.ExecTime),
	}
}

func bybitPositionSide(positionIdx int) string {
	switch positionIdx {
	case 1:
		return "long"
	case 2:
		return "short"
	default:
		return "net"
	}
}

func bybitTime(value string) time.Time {
	milliseconds, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return timeFromMillis(milliseconds)
}
