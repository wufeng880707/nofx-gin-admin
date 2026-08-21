package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type OKX struct {
	credentials Credentials
	client      *http.Client
	baseURL     string
	now         func() time.Time
	wait        func(context.Context, time.Duration) error
}

func NewOKX(credentials Credentials, client *http.Client) *OKX {
	return &OKX{credentials: credentials, client: client, baseURL: "https://www.okx.com", now: time.Now, wait: waitContext}
}

func (*OKX) Name() string { return "okx" }

func (a *OKX) Health(ctx context.Context) error {
	request, _ := http.NewRequest(http.MethodGet, a.baseURL+"/api/v5/public/time", nil)
	var response okxResponse
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return err
	}
	if response.Code != "0" {
		return fmt.Errorf("okx error %s: %s", response.Code, response.Msg)
	}
	return nil
}

func (a *OKX) Snapshot(ctx context.Context) (*Account, []*Position, error) {
	var balance okxResponse
	if err := a.signedGet(ctx, "/api/v5/account/balance", nil, &balance); err != nil {
		return nil, nil, err
	}
	if balance.Code != "0" {
		return nil, nil, fmt.Errorf("okx balance error %s: %s", balance.Code, balance.Msg)
	}
	var balanceRows []struct {
		TotalEq string `json:"totalEq"`
		AdjEq   string `json:"adjEq"`
		AvailEq string `json:"availEq"`
		Upl     string `json:"upl"`
		Details []struct {
			Ccy       string `json:"ccy"`
			Eq        string `json:"eq"`
			EqUSD     string `json:"eqUsd"`
			CashBal   string `json:"cashBal"`
			AvailEq   string `json:"availEq"`
			AvailBal  string `json:"availBal"`
			FrozenBal string `json:"frozenBal"`
			Borrowed  string `json:"borrowFroz"`
			Interest  string `json:"interest"`
			Upl       string `json:"upl"`
		} `json:"details"`
	}
	if err := json.Unmarshal(balance.Data, &balanceRows); err != nil || len(balanceRows) == 0 {
		return nil, nil, fmt.Errorf("decode okx balance: %w", err)
	}
	var positionsResponse okxResponse
	if err := a.signedGet(ctx, "/api/v5/account/positions", url.Values{"instType": {"SWAP"}}, &positionsResponse); err != nil {
		return nil, nil, err
	}
	if positionsResponse.Code != "0" {
		return nil, nil, fmt.Errorf("okx positions error %s: %s", positionsResponse.Code, positionsResponse.Msg)
	}
	var rows []struct {
		PosID   string `json:"posId"`
		InstID  string `json:"instId"`
		PosSide string `json:"posSide"`
		Pos     string `json:"pos"`
		AvgPx   string `json:"avgPx"`
		MarkPx  string `json:"markPx"`
		LiqPx   string `json:"liqPx"`
		Upl     string `json:"upl"`
		Lever   string `json:"lever"`
	}
	if err := json.Unmarshal(positionsResponse.Data, &rows); err != nil {
		return nil, nil, err
	}
	positions := make([]*Position, 0)
	for _, item := range rows {
		quantity, _ := strconv.ParseFloat(item.Pos, 64)
		if quantity == 0 {
			continue
		}
		side := strings.ToLower(item.PosSide)
		if side == "net" {
			if quantity < 0 {
				side = "short"
			} else {
				side = "long"
			}
		}
		leverage, _ := strconv.Atoi(strings.Split(item.Lever, ".")[0])
		positionID := item.PosID
		if positionID == "" {
			positionID = item.InstID + ":" + side
		}
		positions = append(positions, &Position{ExchangePositionID: positionID, Symbol: item.InstID,
			Side: side, Quantity: strings.TrimPrefix(item.Pos, "-"), EntryPrice: item.AvgPx,
			MarkPrice: item.MarkPx, LiquidationPrice: item.LiqPx, UnrealizedPnL: item.Upl, Leverage: leverage})
	}
	item := balanceRows[0]
	assets := make([]*Asset, 0, len(item.Details))
	for _, detail := range item.Details {
		total := detail.Eq
		if strings.TrimSpace(total) == "" {
			total = detail.CashBal
		}
		if zeroDecimal(total) == "0" && zeroDecimal(detail.EqUSD) == "0" {
			continue
		}
		available := detail.AvailEq
		if strings.TrimSpace(available) == "" {
			available = detail.AvailBal
		}
		price := "0"
		if quantity, ok := new(big.Rat).SetString(total); ok && quantity.Sign() != 0 {
			if value, valueOK := new(big.Rat).SetString(detail.EqUSD); valueOK {
				price = formatDecimal(new(big.Rat).Quo(value, quantity))
			}
		}
		assets = append(assets, &Asset{Asset: strings.ToUpper(detail.Ccy), Total: zeroDecimal(total),
			Available: zeroDecimal(available), Locked: zeroDecimal(detail.FrozenBal), Borrowed: zeroDecimal(detail.Borrowed),
			Interest: zeroDecimal(detail.Interest), PriceUSDT: price, ValueUSDT: zeroDecimal(detail.EqUSD),
			UnrealizedPnL: zeroDecimal(detail.Upl)})
	}
	return &Account{TotalEquity: item.TotalEq, Balance: item.AdjEq, AvailableBalance: item.AvailEq,
		UnrealizedPnL: item.Upl, Assets: assets}, positions, nil
}

func (a *OKX) signedGet(ctx context.Context, path string, query url.Values, output interface{}) error {
	requestPath := path
	if len(query) > 0 {
		requestPath += "?" + query.Encode()
	}
	timestamp := a.now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload := timestamp + http.MethodGet + requestPath
	mac := hmac.New(sha256.New, []byte(a.credentials.SecretKey))
	_, _ = mac.Write([]byte(payload))
	request, err := http.NewRequest(http.MethodGet, a.baseURL+requestPath, nil)
	if err != nil {
		return err
	}
	request.Header.Set("OK-ACCESS-KEY", a.credentials.APIKey)
	request.Header.Set("OK-ACCESS-SIGN", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	request.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	request.Header.Set("OK-ACCESS-PASSPHRASE", a.credentials.Passphrase)
	if a.credentials.Testnet {
		request.Header.Set("x-simulated-trading", "1")
	}
	return doJSON(ctx, a.client, request, output)
}

type okxResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
