package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Bybit struct {
	credentials Credentials
	client      *http.Client
	baseURL     string
	now         func() time.Time
}

func NewBybit(credentials Credentials, client *http.Client) *Bybit {
	baseURL := "https://api.bybit.com"
	if credentials.Testnet {
		baseURL = "https://api-testnet.bybit.com"
	}
	return &Bybit{credentials: credentials, client: client, baseURL: baseURL, now: time.Now}
}

func (*Bybit) Name() string { return "bybit" }

func (a *Bybit) Health(ctx context.Context) error {
	request, _ := http.NewRequest(http.MethodGet, a.baseURL+"/v5/market/time", nil)
	var response bybitResponse
	if err := doJSON(ctx, a.client, request, &response); err != nil {
		return err
	}
	if response.RetCode != 0 {
		return fmt.Errorf("bybit error %d: %s", response.RetCode, response.RetMsg)
	}
	return nil
}

func (a *Bybit) Snapshot(ctx context.Context) (*Account, []*Position, error) {
	var wallet struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				TotalEquity           string `json:"totalEquity"`
				TotalWalletBalance    string `json:"totalWalletBalance"`
				TotalAvailableBalance string `json:"totalAvailableBalance"`
				TotalPerpUPL          string `json:"totalPerpUPL"`
			} `json:"list"`
		} `json:"result"`
	}
	query := url.Values{"accountType": {"UNIFIED"}}
	if err := a.signedGet(ctx, "/v5/account/wallet-balance", query, &wallet); err != nil {
		return nil, nil, err
	}
	if wallet.RetCode != 0 || len(wallet.Result.List) == 0 {
		return nil, nil, fmt.Errorf("bybit wallet error %d: %s", wallet.RetCode, wallet.RetMsg)
	}
	rawPositions, err := a.fetchPositions(ctx)
	if err != nil {
		return nil, nil, err
	}
	positions := make([]*Position, 0, len(rawPositions))
	for _, item := range rawPositions {
		quantity, _ := strconv.ParseFloat(item.Size, 64)
		if quantity == 0 {
			continue
		}
		leverage, _ := strconv.Atoi(strings.Split(item.Leverage, ".")[0])
		side := "long"
		if strings.EqualFold(item.Side, "Sell") {
			side = "short"
		}
		positions = append(positions, &Position{ExchangePositionID: fmt.Sprintf("%s:%d", item.Symbol, item.PositionIdx),
			Symbol: item.Symbol, Side: side, Quantity: item.Size, EntryPrice: item.AvgPrice,
			MarkPrice: item.MarkPrice, LiquidationPrice: item.LiqPrice, UnrealizedPnL: item.UnrealisedPnl, Leverage: leverage})
	}
	item := wallet.Result.List[0]
	return &Account{TotalEquity: item.TotalEquity, Balance: item.TotalWalletBalance,
		AvailableBalance: item.TotalAvailableBalance, UnrealizedPnL: item.TotalPerpUPL}, positions, nil
}

type bybitPosition struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Size          string `json:"size"`
	AvgPrice      string `json:"avgPrice"`
	MarkPrice     string `json:"markPrice"`
	LiqPrice      string `json:"liqPrice"`
	UnrealisedPnl string `json:"unrealisedPnl"`
	Leverage      string `json:"leverage"`
	PositionIdx   int    `json:"positionIdx"`
}

func (a *Bybit) fetchPositions(ctx context.Context) ([]bybitPosition, error) {
	positions := make([]bybitPosition, 0)
	cursor := ""
	for page := 0; page < 100; page++ {
		var response struct {
			RetCode int    `json:"retCode"`
			RetMsg  string `json:"retMsg"`
			Result  struct {
				List           []bybitPosition `json:"list"`
				NextPageCursor string          `json:"nextPageCursor"`
			} `json:"result"`
		}
		query := url.Values{"category": {"linear"}, "settleCoin": {"USDT"}, "limit": {"200"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		if err := a.signedGet(ctx, "/v5/position/list", query, &response); err != nil {
			return nil, err
		}
		if response.RetCode != 0 {
			return nil, fmt.Errorf("bybit positions error %d: %s", response.RetCode, response.RetMsg)
		}
		positions = append(positions, response.Result.List...)
		next := response.Result.NextPageCursor
		if next == "" {
			return positions, nil
		}
		if next == cursor {
			return nil, errors.New("bybit positions pagination cursor did not advance")
		}
		cursor = next
	}
	return nil, errors.New("bybit positions exceeded pagination limit")
}

func (a *Bybit) signedGet(ctx context.Context, path string, query url.Values, output interface{}) error {
	queryString := query.Encode()
	timestamp := strconv.FormatInt(a.now().UnixMilli(), 10)
	recvWindow := "5000"
	payload := timestamp + a.credentials.APIKey + recvWindow + queryString
	mac := hmac.New(sha256.New, []byte(a.credentials.SecretKey))
	_, _ = mac.Write([]byte(payload))
	request, err := http.NewRequest(http.MethodGet, a.baseURL+path+"?"+queryString, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-BAPI-API-KEY", a.credentials.APIKey)
	request.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	request.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	request.Header.Set("X-BAPI-SIGN-TYPE", "2")
	request.Header.Set("X-BAPI-SIGN", hex.EncodeToString(mac.Sum(nil)))
	return doJSON(ctx, a.client, request, output)
}

type bybitResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
}
