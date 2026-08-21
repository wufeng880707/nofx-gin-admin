package provider

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (a *Binance) PlaceOrder(ctx context.Context, input TradingOrderRequest) (*TradingOrderResult, error) {
	if a.accountType == BinanceAccountTypeSpot {
		return a.placeSpotOrder(ctx, input)
	}
	if a.accountType != BinanceAccountTypeFutures {
		return nil, fmt.Errorf("Binance account type %q does not support trading", a.accountType)
	}
	query := binanceOrderValues(input)
	var response binanceTradingOrderResponse
	if err := a.signedRequest(ctx, http.MethodPost, "/fapi/v1/order", query, &response); err != nil {
		return nil, err
	}
	if (input.StopLoss != "" && input.StopLoss != "0") || (input.TakeProfit != "" && input.TakeProfit != "0") {
		protectiveSide := oppositeOrderSide(input.Side)
		for kind, price := range map[string]string{"STOP_MARKET": input.StopLoss, "TAKE_PROFIT_MARKET": input.TakeProfit} {
			if price == "" || price == "0" {
				continue
			}
			protection := url.Values{"algoType": {"CONDITIONAL"}, "symbol": {normalizeTradingSymbol(input.Symbol)},
				"side": {protectiveSide}, "type": {kind}, "triggerPrice": {price}, "quantity": {input.Quantity},
				"workingType": {"MARK_PRICE"}, "reduceOnly": {"true"}, "priceProtect": {"true"}}
			if value := binancePositionSide(input.PositionSide); value != "" {
				protection.Set("positionSide", value)
				protection.Del("reduceOnly")
			}
			var protectiveResponse map[string]interface{}
			if err := a.signedRequest(ctx, http.MethodPost, "/fapi/v1/algoOrder", protection, &protectiveResponse); err != nil {
				return nil, fmt.Errorf("primary order %v accepted but %s protection failed: %w", response.OrderID, kind, err)
			}
		}
	}
	return response.result(), nil
}

func (a *Binance) placeSpotOrder(ctx context.Context, input TradingOrderRequest) (*TradingOrderResult, error) {
	isProtectedEntry := !input.ReduceOnly && input.StopLoss != "" && input.StopLoss != "0" &&
		input.TakeProfit != "" && input.TakeProfit != "0"
	if isProtectedEntry {
		if !strings.EqualFold(input.OrderType, "limit") || input.Price == "" || input.Price == "0" {
			return nil, fmt.Errorf("Binance spot protected entry requires an atomic OTOCO limit order")
		}
		pendingSide := oppositeOrderSide(input.Side)
		query := url.Values{"symbol": {normalizeTradingSymbol(input.Symbol)}, "workingType": {"LIMIT"},
			"workingSide": {strings.ToUpper(input.Side)}, "workingPrice": {input.Price}, "workingQuantity": {input.Quantity},
			"workingTimeInForce": {firstTradingValue(input.TimeInForce, "GTC")}, "pendingSide": {pendingSide},
			"pendingQuantity": {input.Quantity}, "pendingAboveType": {"TAKE_PROFIT_LIMIT"},
			"pendingAboveStopPrice": {input.TakeProfit}, "pendingAbovePrice": {input.TakeProfit},
			"pendingAboveTimeInForce": {"GTC"}, "pendingBelowType": {"STOP_LOSS_LIMIT"},
			"pendingBelowStopPrice": {input.StopLoss}, "pendingBelowPrice": {input.StopLoss},
			"pendingBelowTimeInForce": {"GTC"}, "listClientOrderId": {input.ClientOrderID}}
		var response struct {
			OrderListID    interface{} `json:"orderListId"`
			ListStatusType string      `json:"listStatusType"`
			ListClientID   string      `json:"listClientOrderId"`
		}
		if err := a.signedRequest(ctx, http.MethodPost, "/api/v3/orderList/otoco", query, &response); err != nil {
			return nil, err
		}
		return &TradingOrderResult{ExchangeOrderID: "list:" + fmt.Sprint(response.OrderListID),
			ClientOrderID: response.ListClientID, Status: response.ListStatusType}, nil
	}
	query := binanceOrderValues(input)
	query.Del("positionSide")
	query.Del("reduceOnly")
	var response binanceTradingOrderResponse
	if err := a.signedRequest(ctx, http.MethodPost, "/api/v3/order", query, &response); err != nil {
		return nil, err
	}
	return response.result(), nil
}

func (a *Binance) CancelOrder(ctx context.Context, input CancelOrderRequest) (*TradingOrderResult, error) {
	path := "/fapi/v1/order"
	query := url.Values{"symbol": {normalizeTradingSymbol(input.Symbol)}}
	if a.accountType == BinanceAccountTypeSpot {
		path = "/api/v3/order"
		if strings.HasPrefix(input.ExchangeOrderID, "list:") {
			path = "/api/v3/orderList"
			query.Set("orderListId", strings.TrimPrefix(input.ExchangeOrderID, "list:"))
		} else {
			query.Set("orderId", input.ExchangeOrderID)
		}
	} else {
		query.Set("orderId", input.ExchangeOrderID)
	}
	var response binanceTradingOrderResponse
	if err := a.signedRequest(ctx, http.MethodDelete, path, query, &response); err != nil {
		return nil, err
	}
	result := response.result()
	if path == "/api/v3/orderList" {
		result.ExchangeOrderID, result.Status = input.ExchangeOrderID, "CANCELLED"
	}
	return result, nil
}

type binanceTradingOrderResponse struct {
	OrderID       interface{} `json:"orderId"`
	ClientOrderID string      `json:"clientOrderId"`
	Status        string      `json:"status"`
	ExecutedQty   string      `json:"executedQty"`
	AveragePrice  string      `json:"avgPrice"`
	QuoteQty      string      `json:"cummulativeQuoteQty"`
}

func (a binanceTradingOrderResponse) result() *TradingOrderResult {
	average := a.AveragePrice
	if (average == "" || average == "0") && a.ExecutedQty != "" && a.QuoteQty != "" {
		quantity, quantityOK := new(big.Rat).SetString(a.ExecutedQty)
		quote, quoteOK := new(big.Rat).SetString(a.QuoteQty)
		if quantityOK && quoteOK && quantity.Sign() > 0 {
			average = formatDecimal(new(big.Rat).Quo(quote, quantity))
		}
	}
	return &TradingOrderResult{ExchangeOrderID: fmt.Sprint(a.OrderID), ClientOrderID: a.ClientOrderID,
		Status: a.Status, ExecutedQuantity: zeroDecimal(a.ExecutedQty), AveragePrice: zeroDecimal(average)}
}

func binanceOrderValues(input TradingOrderRequest) url.Values {
	query := url.Values{"symbol": {normalizeTradingSymbol(input.Symbol)}, "side": {strings.ToUpper(input.Side)},
		"type": {strings.ToUpper(input.OrderType)}, "newClientOrderId": {input.ClientOrderID}, "newOrderRespType": {"RESULT"}}
	if input.Quantity != "" && input.Quantity != "0" {
		query.Set("quantity", input.Quantity)
	}
	if input.QuoteQuantity != "" && input.QuoteQuantity != "0" && strings.EqualFold(input.Market, "spot") {
		query.Set("quoteOrderQty", input.QuoteQuantity)
	}
	if input.Price != "" && input.Price != "0" {
		query.Set("price", input.Price)
	}
	if strings.EqualFold(input.OrderType, "limit") {
		query.Set("timeInForce", firstTradingValue(input.TimeInForce, "GTC"))
	}
	if value := binancePositionSide(input.PositionSide); value != "" {
		query.Set("positionSide", value)
	}
	if input.ReduceOnly && query.Get("positionSide") == "" {
		query.Set("reduceOnly", strconv.FormatBool(true))
	}
	return query
}

func binancePositionSide(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "long":
		return "LONG"
	case "short":
		return "SHORT"
	default:
		return ""
	}
}

func normalizeTradingSymbol(value string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", "/", "", "_", "").Replace(strings.TrimSpace(value)))
}

func oppositeOrderSide(value string) string {
	if strings.EqualFold(value, "buy") {
		return "SELL"
	}
	return "BUY"
}

func firstTradingValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.ToUpper(value)
}
