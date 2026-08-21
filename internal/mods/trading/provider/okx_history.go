package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	okxHistoryLimit        = 100
	okxHistoryMaxPages     = 10000
	okxOrderPageInterval   = 110 * time.Millisecond
	okxFillPageInterval    = 210 * time.Millisecond
	okxAccountPageInterval = 410 * time.Millisecond
)

type okxOrderRow struct {
	InstType   string `json:"instType"`
	InstID     string `json:"instId"`
	OrdID      string `json:"ordId"`
	ClOrdID    string `json:"clOrdId"`
	Side       string `json:"side"`
	PosSide    string `json:"posSide"`
	OrdType    string `json:"ordType"`
	State      string `json:"state"`
	Px         string `json:"px"`
	AvgPx      string `json:"avgPx"`
	Sz         string `json:"sz"`
	AccFillSz  string `json:"accFillSz"`
	FillPx     string `json:"fillPx"`
	FillSz     string `json:"fillSz"`
	ReduceOnly string `json:"reduceOnly"`
	CTime      string `json:"cTime"`
	UTime      string `json:"uTime"`
}

type okxFillRow struct {
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	TradeID  string `json:"tradeId"`
	OrdID    string `json:"ordId"`
	Side     string `json:"side"`
	PosSide  string `json:"posSide"`
	FillPx   string `json:"fillPx"`
	FillSz   string `json:"fillSz"`
	FillPnl  string `json:"fillPnl"`
	Fee      string `json:"fee"`
	FeeCcy   string `json:"feeCcy"`
	ExecType string `json:"execType"`
	FillTime string `json:"fillTime"`
	BillID   string `json:"billId"`
}

type okxBillRow struct {
	BillID   string `json:"billId"`
	Type     string `json:"type"`
	SubType  string `json:"subType"`
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	Ccy      string `json:"ccy"`
	BalChg   string `json:"balChg"`
	TS       string `json:"ts"`
}

func (a *OKX) History(ctx context.Context, request HistoryRequest) (*HistoryBatch, error) {
	until := request.Until.UTC()
	if until.IsZero() {
		until = a.now().UTC()
	}
	since := request.Since.UTC()
	if since.IsZero() || !since.Before(until) {
		since = until.Add(-7 * 24 * time.Hour)
	}
	if oldest := until.Add(-90 * 24 * time.Hour); since.Before(oldest) {
		since = oldest
	}
	batch := &HistoryBatch{SyncedUntil: until}
	for _, market := range []string{"SPOT", "SWAP"} {
		if err := a.loadOKXOrders(ctx, market, since, until, batch); err != nil {
			return nil, err
		}
		if err := a.loadOKXFills(ctx, market, since, until, batch); err != nil {
			return nil, err
		}
	}
	if err := a.loadOKXBills(ctx, since, until, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

func (a *OKX) loadOKXOrders(ctx context.Context, market string, since, until time.Time, batch *HistoryBatch) error {
	after := ""
	for page := 0; page < okxHistoryMaxPages; page++ {
		if page > 0 {
			if err := a.wait(ctx, okxOrderPageInterval); err != nil {
				return err
			}
		}
		query := okxHistoryQuery(market, since, until)
		if after != "" {
			query.Set("after", after)
		}
		var response okxResponse
		if err := a.signedGet(ctx, "/api/v5/trade/orders-history-archive", query, &response); err != nil {
			return fmt.Errorf("load OKX %s orders: %w", market, err)
		}
		if response.Code != "0" {
			return fmt.Errorf("okx order history error %s: %s", response.Code, response.Msg)
		}
		var rows []okxOrderRow
		if err := json.Unmarshal(response.Data, &rows); err != nil {
			return fmt.Errorf("decode OKX %s orders: %w", market, err)
		}
		for _, row := range rows {
			ordered := okxTime(row.CTime)
			updated := okxTime(row.UTime)
			if updated.IsZero() {
				updated = ordered
			}
			batch.Orders = append(batch.Orders, &Order{Market: strings.ToLower(row.InstType), ExchangeOrderID: row.OrdID,
				ClientOrderID: row.ClOrdID, Symbol: strings.ToUpper(row.InstID), Side: strings.ToLower(row.Side),
				PositionSide: strings.ToLower(row.PosSide), OrderType: strings.ToLower(row.OrdType), Status: strings.ToLower(row.State),
				Price: zeroDecimal(row.Px), AveragePrice: zeroDecimal(row.AvgPx), Quantity: zeroDecimal(row.Sz),
				ExecutedQuantity: zeroDecimal(row.AccFillSz), QuoteQuantity: decimalProduct(row.AvgPx, row.AccFillSz),
				ReduceOnly: strings.EqualFold(row.ReduceOnly, "true"), OrderedAt: ordered, ExchangeUpdatedAt: updated})
		}
		if len(rows) < okxHistoryLimit {
			return nil
		}
		next := strings.TrimSpace(rows[len(rows)-1].OrdID)
		if next == "" || next == after {
			return fmt.Errorf("OKX %s order history cursor did not advance", market)
		}
		after = next
	}
	return fmt.Errorf("OKX %s order history exceeded %d pages", market, okxHistoryMaxPages)
}

func (a *OKX) loadOKXFills(ctx context.Context, market string, since, until time.Time, batch *HistoryBatch) error {
	after := ""
	for page := 0; page < okxHistoryMaxPages; page++ {
		if page > 0 {
			if err := a.wait(ctx, okxFillPageInterval); err != nil {
				return err
			}
		}
		query := okxHistoryQuery(market, since, until)
		if after != "" {
			query.Set("after", after)
		}
		var response okxResponse
		if err := a.signedGet(ctx, "/api/v5/trade/fills-history", query, &response); err != nil {
			return fmt.Errorf("load OKX %s fills: %w", market, err)
		}
		if response.Code != "0" {
			return fmt.Errorf("okx fill history error %s: %s", response.Code, response.Msg)
		}
		var rows []okxFillRow
		if err := json.Unmarshal(response.Data, &rows); err != nil {
			return fmt.Errorf("decode OKX %s fills: %w", market, err)
		}
		for _, row := range rows {
			fee := decimalAbs(row.Fee)
			feeValue := "0"
			if isUSDStableAsset(row.FeeCcy) {
				feeValue = fee
			}
			liquidity := "taker"
			if strings.EqualFold(row.ExecType, "M") {
				liquidity = "maker"
			}
			batch.Fills = append(batch.Fills, &Fill{Market: strings.ToLower(row.InstType), ExchangeTradeID: row.TradeID,
				ExchangeOrderID: row.OrdID, Symbol: strings.ToUpper(row.InstID), Side: strings.ToLower(row.Side),
				PositionSide: strings.ToLower(row.PosSide), Price: zeroDecimal(row.FillPx), Quantity: zeroDecimal(row.FillSz),
				QuoteQuantity: decimalProduct(row.FillPx, row.FillSz), FeeAsset: strings.ToUpper(row.FeeCcy),
				FeeQuantity: fee, FeeValueUSDT: feeValue, RealizedPnL: zeroDecimal(row.FillPnl), Liquidity: liquidity,
				FilledAt: okxTime(row.FillTime)})
		}
		if len(rows) < okxHistoryLimit {
			return nil
		}
		next := strings.TrimSpace(rows[len(rows)-1].BillID)
		if next == "" || next == after {
			return fmt.Errorf("OKX %s fill history cursor did not advance", market)
		}
		after = next
	}
	return fmt.Errorf("OKX %s fill history exceeded %d pages", market, okxHistoryMaxPages)
}

func (a *OKX) loadOKXBills(ctx context.Context, since, until time.Time, batch *HistoryBatch) error {
	after := ""
	for page := 0; page < okxHistoryMaxPages; page++ {
		if page > 0 {
			if err := a.wait(ctx, okxAccountPageInterval); err != nil {
				return err
			}
		}
		query := url.Values{"begin": {millis(since)}, "end": {millis(until)}, "limit": {strconv.Itoa(okxHistoryLimit)}}
		if after != "" {
			query.Set("after", after)
		}
		var response okxResponse
		if err := a.signedGet(ctx, "/api/v5/account/bills-archive", query, &response); err != nil {
			return fmt.Errorf("load OKX bills: %w", err)
		}
		if response.Code != "0" {
			return fmt.Errorf("okx bill history error %s: %s", response.Code, response.Msg)
		}
		var rows []okxBillRow
		if err := json.Unmarshal(response.Data, &rows); err != nil {
			return fmt.Errorf("decode OKX bills: %w", err)
		}
		for _, row := range rows {
			value := "0"
			if isUSDStableAsset(row.Ccy) {
				value = zeroDecimal(row.BalChg)
			}
			batch.IncomeEvents = append(batch.IncomeEvents, &IncomeEvent{Market: strings.ToLower(row.InstType),
				EventType: "bill_" + row.Type + "_" + row.SubType, ExchangeEventID: row.BillID,
				Symbol: strings.ToUpper(row.InstID), Asset: strings.ToUpper(row.Ccy), Amount: zeroDecimal(row.BalChg),
				PriceUSDT: stablePrice(row.Ccy), ValueUSDT: value, OccurredAt: okxTime(row.TS)})
		}
		if len(rows) < okxHistoryLimit {
			return nil
		}
		next := strings.TrimSpace(rows[len(rows)-1].BillID)
		if next == "" || next == after {
			return fmt.Errorf("OKX bill history cursor did not advance")
		}
		after = next
	}
	return fmt.Errorf("OKX bill history exceeded %d pages", okxHistoryMaxPages)
}

func okxHistoryQuery(market string, since, until time.Time) url.Values {
	return url.Values{"instType": {market}, "begin": {millis(since)}, "end": {millis(until)}, "limit": {strconv.Itoa(okxHistoryLimit)}}
}

func okxTime(value string) time.Time {
	milliseconds, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return timeFromMillis(milliseconds)
}

func decimalProduct(left, right string) string {
	l, lOK := new(big.Rat).SetString(strings.TrimSpace(left))
	r, rOK := new(big.Rat).SetString(strings.TrimSpace(right))
	if !lOK || !rOK {
		return "0"
	}
	return formatDecimal(new(big.Rat).Mul(l, r))
}

func decimalAbs(value string) string {
	parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return "0"
	}
	return formatDecimal(new(big.Rat).Abs(parsed))
}
