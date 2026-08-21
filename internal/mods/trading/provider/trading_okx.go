package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (a *OKX) PlaceOrder(ctx context.Context, input TradingOrderRequest) (*TradingOrderResult, error) {
	payload := map[string]interface{}{"instId": okxTradingInstrument(input.Symbol, input.Market),
		"tdMode": okxTradingMode(input.Market), "clOrdId": input.ClientOrderID,
		"side": strings.ToLower(input.Side), "ordType": strings.ToLower(input.OrderType), "sz": input.Quantity}
	if input.Price != "" && input.Price != "0" {
		payload["px"] = input.Price
	}
	if input.ReduceOnly {
		payload["reduceOnly"] = true
	}
	if side := strings.ToLower(strings.TrimSpace(input.PositionSide)); side == "long" || side == "short" {
		payload["posSide"] = side
	}
	if (input.StopLoss != "" && input.StopLoss != "0") || (input.TakeProfit != "" && input.TakeProfit != "0") {
		attached := map[string]string{}
		if input.StopLoss != "" && input.StopLoss != "0" {
			attached["slTriggerPx"], attached["slOrdPx"] = input.StopLoss, "-1"
		}
		if input.TakeProfit != "" && input.TakeProfit != "0" {
			attached["tpTriggerPx"], attached["tpOrdPx"] = input.TakeProfit, "-1"
		}
		payload["attachAlgoOrds"] = []map[string]string{attached}
	}
	var response okxTradingResponse
	if err := a.signedPost(ctx, "/api/v5/trade/order", payload, &response); err != nil {
		return nil, err
	}
	return response.result()
}

func (a *OKX) CancelOrder(ctx context.Context, input CancelOrderRequest) (*TradingOrderResult, error) {
	payload := map[string]interface{}{"instId": okxTradingInstrument(input.Symbol, input.Market)}
	if input.ExchangeOrderID != "" {
		payload["ordId"] = input.ExchangeOrderID
	} else {
		payload["clOrdId"] = input.ClientOrderID
	}
	var response okxTradingResponse
	if err := a.signedPost(ctx, "/api/v5/trade/cancel-order", payload, &response); err != nil {
		return nil, err
	}
	return response.result()
}

type okxTradingResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		OrderID       string `json:"ordId"`
		ClientID      string `json:"clOrdId"`
		StatusCode    string `json:"sCode"`
		StatusMessage string `json:"sMsg"`
	} `json:"data"`
}

func (a okxTradingResponse) result() (*TradingOrderResult, error) {
	if a.Code != "0" || len(a.Data) == 0 {
		return nil, fmt.Errorf("OKX trading error %s: %s", a.Code, a.Msg)
	}
	item := a.Data[0]
	if item.StatusCode != "0" {
		return nil, fmt.Errorf("OKX order error %s: %s", item.StatusCode, item.StatusMessage)
	}
	return &TradingOrderResult{ExchangeOrderID: item.OrderID, ClientOrderID: item.ClientID, Status: "accepted"}, nil
}

func (a *OKX) signedPost(ctx context.Context, path string, payload interface{}, output interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := a.now().UTC().Format("2006-01-02T15:04:05.000Z")
	mac := hmac.New(sha256.New, []byte(a.credentials.SecretKey))
	_, _ = mac.Write([]byte(timestamp + http.MethodPost + path + string(body)))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("OK-ACCESS-KEY", a.credentials.APIKey)
	request.Header.Set("OK-ACCESS-SIGN", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	request.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	request.Header.Set("OK-ACCESS-PASSPHRASE", a.credentials.Passphrase)
	if a.credentials.Testnet {
		request.Header.Set("x-simulated-trading", "1")
	}
	return doJSON(ctx, a.client, request, output)
}

func okxTradingInstrument(symbol, market string) string {
	value := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(symbol), "/", "-"))
	if strings.Contains(value, "-") {
		if strings.EqualFold(market, "futures") && !strings.HasSuffix(value, "-SWAP") {
			value += "-SWAP"
		}
		return value
	}
	if strings.HasSuffix(value, "USDT") {
		value = strings.TrimSuffix(value, "USDT") + "-USDT"
	}
	if strings.EqualFold(market, "futures") {
		value += "-SWAP"
	}
	return value
}

func okxTradingMode(market string) string {
	if strings.EqualFold(market, "spot") {
		return "cash"
	}
	return "cross"
}
