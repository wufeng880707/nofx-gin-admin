package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	BinanceAccountTypeSpot    = "spot"
	BinanceAccountTypeFutures = "futures"
	BinanceAccountTypeUnified = "unified"
)

type Binance struct {
	credentials Credentials
	client      *http.Client
	baseURL     string
	accountType string
	now         func() time.Time
	symbols     map[string]struct{}
}

func NewBinance(credentials Credentials, client *http.Client) *Binance {
	accountType := normalizeBinanceAccountType(credentials.AccountType)
	baseURL := "https://fapi.binance.com"
	switch accountType {
	case BinanceAccountTypeSpot:
		baseURL = "https://api.binance.com"
		if credentials.Testnet {
			baseURL = "https://testnet.binance.vision"
		}
	case BinanceAccountTypeUnified:
		baseURL = "https://papi.binance.com"
	default:
		if credentials.Testnet {
			baseURL = "https://testnet.binancefuture.com"
		}
	}
	return &Binance{
		credentials: credentials,
		client:      client,
		baseURL:     baseURL,
		accountType: accountType,
		now:         time.Now,
		symbols:     make(map[string]struct{}),
	}
}

func (*Binance) Name() string { return "binance" }

func (a *Binance) Health(ctx context.Context) error {
	path := "/fapi/v1/ping"
	switch a.accountType {
	case BinanceAccountTypeSpot:
		path = "/api/v3/ping"
	case BinanceAccountTypeUnified:
		path = "/papi/v1/ping"
	}
	request, _ := http.NewRequest(http.MethodGet, a.baseURL+path, nil)
	var response map[string]interface{}
	return doJSON(ctx, a.client, request, &response)
}

func (a *Binance) ValidateCredentials(ctx context.Context) error {
	path := "/fapi/v2/account"
	query := url.Values(nil)
	switch a.accountType {
	case BinanceAccountTypeSpot:
		path = "/api/v3/account"
		query = url.Values{"omitZeroBalances": {"true"}}
	case BinanceAccountTypeUnified:
		path = "/papi/v1/balance"
	}
	var response map[string]interface{}
	return a.signedGet(ctx, path, query, &response)
}

func (a *Binance) Snapshot(ctx context.Context) (*Account, []*Position, error) {
	switch a.accountType {
	case BinanceAccountTypeSpot:
		return a.snapshotSpot(ctx)
	case BinanceAccountTypeUnified:
		return a.snapshotUnified(ctx)
	default:
		return a.snapshotFutures(ctx)
	}
}

func (a *Binance) snapshotFutures(ctx context.Context) (*Account, []*Position, error) {
	var account struct {
		TotalMarginBalance    string `json:"totalMarginBalance"`
		TotalWalletBalance    string `json:"totalWalletBalance"`
		AvailableBalance      string `json:"availableBalance"`
		TotalUnrealizedProfit string `json:"totalUnrealizedProfit"`
		Assets                []struct {
			Asset            string `json:"asset"`
			WalletBalance    string `json:"walletBalance"`
			UnrealizedProfit string `json:"unrealizedProfit"`
			MarginBalance    string `json:"marginBalance"`
			AvailableBalance string `json:"availableBalance"`
		} `json:"assets"`
	}
	if err := a.signedGet(ctx, "/fapi/v2/account", nil, &account); err != nil {
		return nil, nil, err
	}
	var rawPositions []binancePositionRow
	if err := a.signedGet(ctx, "/fapi/v2/positionRisk", nil, &rawPositions); err != nil {
		return nil, nil, err
	}
	positions, _ := normalizeBinancePositions(rawPositions)
	for _, position := range positions {
		a.rememberSymbol(position.Symbol)
	}
	assets := make([]*Asset, 0, len(account.Assets))
	for _, item := range account.Assets {
		wallet, _ := new(big.Rat).SetString(item.WalletBalance)
		if wallet == nil || wallet.Sign() == 0 {
			continue
		}
		price := "0"
		value := "0"
		if isUSDStableAsset(item.Asset) {
			price, value = "1", formatDecimal(wallet)
		}
		assets = append(assets, &Asset{Asset: strings.ToUpper(item.Asset), Total: zeroDecimal(item.WalletBalance),
			Available: zeroDecimal(item.AvailableBalance), PriceUSDT: price, ValueUSDT: value,
			UnrealizedPnL: zeroDecimal(item.UnrealizedProfit)})
	}
	return &Account{TotalEquity: account.TotalMarginBalance, Balance: account.TotalWalletBalance,
		AvailableBalance: account.AvailableBalance, UnrealizedPnL: account.TotalUnrealizedProfit, Assets: assets}, positions, nil
}

func (a *Binance) snapshotSpot(ctx context.Context) (*Account, []*Position, error) {
	var account struct {
		Balances []struct {
			Asset  string `json:"asset"`
			Free   string `json:"free"`
			Locked string `json:"locked"`
		} `json:"balances"`
	}
	if err := a.signedGet(ctx, "/api/v3/account", url.Values{"omitZeroBalances": {"true"}}, &account); err != nil {
		return nil, nil, err
	}

	balances := make(map[string]*binanceAssetBalance)
	ldBalances := make(map[string]*big.Rat)
	for _, balance := range account.Balances {
		asset := strings.ToUpper(strings.TrimSpace(balance.Asset))
		free, err := parsePositiveDecimal(balance.Free)
		if err != nil {
			return nil, nil, fmt.Errorf("decode Binance spot free balance for %s: %w", balance.Asset, err)
		}
		locked, err := parsePositiveDecimal(balance.Locked)
		if err != nil {
			return nil, nil, fmt.Errorf("decode Binance spot locked balance for %s: %w", balance.Asset, err)
		}
		amount := new(big.Rat).Add(free, locked)
		if amount.Sign() == 0 {
			continue
		}
		if underlying := binanceSimpleEarnUnderlying(asset); underlying != "" {
			addBinanceAmount(ldBalances, underlying, amount)
			continue
		}
		entry := ensureBinanceAssetBalance(balances, asset)
		entry.total.Add(entry.total, amount)
		entry.available.Add(entry.available, free)
		entry.locked.Add(entry.locked, locked)
	}

	if !a.credentials.Testnet {
		flexible, err := a.simpleEarnFlexibleBalances(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("query Binance Simple Earn flexible positions: %w", err)
		}
		locked, err := a.simpleEarnLockedBalances(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("query Binance Simple Earn locked positions: %w", err)
		}
		funding, err := a.fundingWalletBalances(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("query Binance Funding Wallet: %w", err)
		}
		for asset, amount := range flexible {
			entry := ensureBinanceAssetBalance(balances, asset)
			entry.total.Add(entry.total, amount)
			entry.locked.Add(entry.locked, amount)
			delete(ldBalances, asset)
		}
		for asset, amount := range locked {
			entry := ensureBinanceAssetBalance(balances, asset)
			entry.total.Add(entry.total, amount)
			entry.locked.Add(entry.locked, amount)
		}
		for asset, amount := range funding {
			entry := ensureBinanceAssetBalance(balances, asset)
			entry.total.Add(entry.total, amount.total)
			entry.available.Add(entry.available, amount.available)
			entry.locked.Add(entry.locked, amount.locked)
		}
	}
	// LD assets represent Simple Earn collateral. Keep them only when the
	// authoritative Simple Earn endpoint did not return the underlying asset.
	for asset, amount := range ldBalances {
		entry := ensureBinanceAssetBalance(balances, asset)
		entry.total.Add(entry.total, amount)
		entry.locked.Add(entry.locked, amount)
	}

	var tickers []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	request, err := http.NewRequest(http.MethodGet, a.baseURL+"/api/v3/ticker/price", nil)
	if err != nil {
		return nil, nil, err
	}
	if err := doJSON(ctx, a.client, request, &tickers); err != nil {
		return nil, nil, err
	}
	prices := make(map[string]string, len(tickers))
	for _, ticker := range tickers {
		prices[strings.ToUpper(ticker.Symbol)] = ticker.Price
	}

	total := new(big.Rat)
	available := new(big.Rat)
	assets := make([]*Asset, 0, len(balances))
	for asset, balance := range balances {
		if balance.total.Sign() == 0 {
			continue
		}
		price, ok := binanceUSDTPrice(asset, prices)
		if !ok {
			assets = append(assets, &Asset{Asset: asset, Total: formatDecimal(balance.total),
				Available: formatDecimal(balance.available), Locked: formatDecimal(balance.locked), PriceUSDT: "0", ValueUSDT: "0"})
			continue
		}
		value := new(big.Rat).Mul(balance.total, price)
		total.Add(total, value)
		available.Add(available, new(big.Rat).Mul(balance.available, price))
		assets = append(assets, &Asset{Asset: asset, Total: formatDecimal(balance.total),
			Available: formatDecimal(balance.available), Locked: formatDecimal(balance.locked), PriceUSDT: formatDecimal(price),
			ValueUSDT: formatDecimal(value)})
		if symbol := binanceSpotHistorySymbol(asset, prices); symbol != "" {
			a.rememberSymbol(symbol)
		}
	}
	sort.SliceStable(assets, func(i, j int) bool {
		left, _ := new(big.Rat).SetString(assets[i].ValueUSDT)
		right, _ := new(big.Rat).SetString(assets[j].ValueUSDT)
		if left.Cmp(right) == 0 {
			return assets[i].Asset < assets[j].Asset
		}
		return left.Cmp(right) > 0
	})
	totalValue := formatDecimal(total)
	return &Account{
		TotalEquity:      totalValue,
		Balance:          totalValue,
		AvailableBalance: formatDecimal(available),
		UnrealizedPnL:    "0",
		MarginUsedPct:    "0",
		Assets:           assets,
	}, nil, nil
}

type binanceAssetBalance struct {
	total     *big.Rat
	available *big.Rat
	locked    *big.Rat
}

func ensureBinanceAssetBalance(balances map[string]*binanceAssetBalance, asset string) *binanceAssetBalance {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if balance := balances[asset]; balance != nil {
		return balance
	}
	balance := &binanceAssetBalance{total: new(big.Rat), available: new(big.Rat), locked: new(big.Rat)}
	balances[asset] = balance
	return balance
}

func addBinanceAmount(balances map[string]*big.Rat, asset string, amount *big.Rat) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if balances[asset] == nil {
		balances[asset] = new(big.Rat)
	}
	balances[asset].Add(balances[asset], amount)
}

func (a *Binance) simpleEarnFlexibleBalances(ctx context.Context) (map[string]*big.Rat, error) {
	balances := make(map[string]*big.Rat)
	for current := 1; ; current++ {
		var response struct {
			Rows []struct {
				Asset       string `json:"asset"`
				TotalAmount string `json:"totalAmount"`
			} `json:"rows"`
			Total int `json:"total"`
		}
		query := url.Values{"current": {strconv.Itoa(current)}, "size": {"100"}}
		if err := a.signedGet(ctx, "/sapi/v1/simple-earn/flexible/position", query, &response); err != nil {
			return nil, err
		}
		for _, row := range response.Rows {
			amount, err := parsePositiveDecimal(row.TotalAmount)
			if err != nil {
				return nil, fmt.Errorf("decode flexible position for %s: %w", row.Asset, err)
			}
			addBinanceAmount(balances, row.Asset, amount)
		}
		if len(response.Rows) == 0 || current*100 >= response.Total {
			break
		}
	}
	return balances, nil
}

func (a *Binance) simpleEarnLockedBalances(ctx context.Context) (map[string]*big.Rat, error) {
	balances := make(map[string]*big.Rat)
	for current := 1; ; current++ {
		var response struct {
			Rows []struct {
				Asset  string `json:"asset"`
				Amount string `json:"amount"`
			} `json:"rows"`
			Total int `json:"total"`
		}
		query := url.Values{"current": {strconv.Itoa(current)}, "size": {"100"}}
		if err := a.signedGet(ctx, "/sapi/v1/simple-earn/locked/position", query, &response); err != nil {
			return nil, err
		}
		for _, row := range response.Rows {
			amount, err := parsePositiveDecimal(row.Amount)
			if err != nil {
				return nil, fmt.Errorf("decode locked position for %s: %w", row.Asset, err)
			}
			addBinanceAmount(balances, row.Asset, amount)
		}
		if len(response.Rows) == 0 || current*100 >= response.Total {
			break
		}
	}
	return balances, nil
}

func (a *Binance) fundingWalletBalances(ctx context.Context) (map[string]*binanceAssetBalance, error) {
	var response []struct {
		Asset       string `json:"asset"`
		Free        string `json:"free"`
		Locked      string `json:"locked"`
		Freeze      string `json:"freeze"`
		Withdrawing string `json:"withdrawing"`
	}
	query := url.Values{"needBtcValuation": {"false"}}
	if err := a.signedRequest(ctx, http.MethodPost, "/sapi/v1/asset/get-funding-asset", query, &response); err != nil {
		return nil, err
	}
	balances := make(map[string]*binanceAssetBalance)
	for _, row := range response {
		free, err := parsePositiveDecimal(row.Free)
		if err != nil {
			return nil, fmt.Errorf("decode funding free balance for %s: %w", row.Asset, err)
		}
		locked, err := parsePositiveDecimal(row.Locked)
		if err != nil {
			return nil, fmt.Errorf("decode funding locked balance for %s: %w", row.Asset, err)
		}
		freeze, err := parsePositiveDecimal(row.Freeze)
		if err != nil {
			return nil, fmt.Errorf("decode funding frozen balance for %s: %w", row.Asset, err)
		}
		withdrawing, err := parsePositiveDecimal(row.Withdrawing)
		if err != nil {
			return nil, fmt.Errorf("decode funding withdrawing balance for %s: %w", row.Asset, err)
		}
		unavailable := new(big.Rat).Add(locked, freeze)
		unavailable.Add(unavailable, withdrawing)
		entry := ensureBinanceAssetBalance(balances, row.Asset)
		entry.available.Add(entry.available, free)
		entry.locked.Add(entry.locked, unavailable)
		entry.total.Add(entry.total, free)
		entry.total.Add(entry.total, unavailable)
	}
	return balances, nil
}

func isUSDStableAsset(asset string) bool {
	switch strings.ToUpper(strings.TrimSpace(asset)) {
	case "USDT", "USDC", "FDUSD", "BUSD":
		return true
	default:
		return false
	}
}

func zeroDecimal(value string) string {
	if parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value)); ok {
		return formatDecimal(parsed)
	}
	return "0"
}

func (a *Binance) snapshotUnified(ctx context.Context) (*Account, []*Position, error) {
	var accountInfo struct {
		AccountEquity            string `json:"accountEquity"`
		ActualEquity             string `json:"actualEquity"`
		TotalAvailableBalance    string `json:"totalAvailableBalance"`
		VirtualMaxWithdrawAmount string `json:"virtualMaxWithdrawAmount"`
	}
	if err := a.signedGet(ctx, "/papi/v1/account", nil, &accountInfo); err != nil {
		return nil, nil, err
	}

	var balances []struct {
		Asset               string `json:"asset"`
		TotalWalletBalance  string `json:"totalWalletBalance"`
		CrossMarginBorrowed string `json:"crossMarginBorrowed"`
		CrossMarginFree     string `json:"crossMarginFree"`
		CrossMarginInterest string `json:"crossMarginInterest"`
		CrossMarginLocked   string `json:"crossMarginLocked"`
		UMUnrealizedPnL     string `json:"umUnrealizedPNL"`
		CMUnrealizedPnL     string `json:"cmUnrealizedPNL"`
	}
	if err := a.signedGet(ctx, "/papi/v1/balance", nil, &balances); err != nil {
		return nil, nil, err
	}

	var umAccount struct {
		Positions []binancePositionRow `json:"positions"`
	}
	if err := a.signedGet(ctx, "/papi/v1/um/account", nil, &umAccount); err != nil {
		return nil, nil, err
	}
	positions, unrealizedPnL := normalizeBinancePositions(umAccount.Positions)
	for _, position := range positions {
		a.rememberSymbol(position.Symbol)
	}
	assets := make([]*Asset, 0, len(balances))
	for _, item := range balances {
		total, ok := new(big.Rat).SetString(strings.TrimSpace(item.TotalWalletBalance))
		if !ok || total.Sign() == 0 {
			continue
		}
		unrealized := addDecimals(item.UMUnrealizedPnL, item.CMUnrealizedPnL)
		price, value := "0", "0"
		if isUSDStableAsset(item.Asset) {
			price, value = "1", formatDecimal(total)
		}
		assets = append(assets, &Asset{Asset: strings.ToUpper(item.Asset), Total: formatDecimal(total),
			Available: zeroDecimal(item.CrossMarginFree), Locked: zeroDecimal(item.CrossMarginLocked),
			Borrowed: zeroDecimal(item.CrossMarginBorrowed), Interest: zeroDecimal(item.CrossMarginInterest),
			PriceUSDT: price, ValueUSDT: value, UnrealizedPnL: unrealized})
	}
	availableBalance := accountInfo.TotalAvailableBalance
	if strings.TrimSpace(availableBalance) == "" {
		availableBalance = accountInfo.VirtualMaxWithdrawAmount
	}
	return &Account{
		TotalEquity:      accountInfo.AccountEquity,
		Balance:          accountInfo.ActualEquity,
		AvailableBalance: availableBalance,
		UnrealizedPnL:    unrealizedPnL,
		Assets:           assets,
	}, positions, nil
}

func addDecimals(values ...string) string {
	total := new(big.Rat)
	for _, value := range values {
		if parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value)); ok {
			total.Add(total, parsed)
		}
	}
	return formatDecimal(total)
}

func (a *Binance) rememberSymbol(symbol string) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol != "" {
		a.symbols[symbol] = struct{}{}
	}
}

type binancePositionRow struct {
	Symbol                  string `json:"symbol"`
	PositionSide            string `json:"positionSide"`
	PositionAmt             string `json:"positionAmt"`
	EntryPrice              string `json:"entryPrice"`
	MarkPrice               string `json:"markPrice"`
	UnrealizedProfit        string `json:"unRealizedProfit"`
	UnifiedUnrealizedProfit string `json:"unrealizedProfit"`
	LiquidationPrice        string `json:"liquidationPrice"`
	Leverage                string `json:"leverage"`
}

func normalizeBinancePositions(rows []binancePositionRow) ([]*Position, string) {
	positions := make([]*Position, 0)
	totalPnL := new(big.Rat)
	for _, item := range rows {
		unrealizedProfit := item.UnrealizedProfit
		if strings.TrimSpace(unrealizedProfit) == "" {
			unrealizedProfit = item.UnifiedUnrealizedProfit
		}
		quantity, _ := strconv.ParseFloat(item.PositionAmt, 64)
		if quantity == 0 {
			continue
		}
		side := strings.ToLower(item.PositionSide)
		if side == "both" || side == "" {
			if quantity < 0 {
				side = "short"
			} else {
				side = "long"
			}
		}
		leverage, _ := strconv.Atoi(strings.Split(item.Leverage, ".")[0])
		positions = append(positions, &Position{
			ExchangePositionID: item.Symbol + ":" + side,
			Symbol:             item.Symbol,
			Side:               side,
			Quantity:           strings.TrimPrefix(item.PositionAmt, "-"),
			EntryPrice:         item.EntryPrice,
			MarkPrice:          item.MarkPrice,
			LiquidationPrice:   item.LiquidationPrice,
			UnrealizedPnL:      unrealizedProfit,
			Leverage:           leverage,
		})
		if pnl, ok := new(big.Rat).SetString(unrealizedProfit); ok {
			totalPnL.Add(totalPnL, pnl)
		}
	}
	return positions, formatDecimal(totalPnL)
}

func normalizeBinanceAccountType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return BinanceAccountTypeFutures
	}
	return value
}

func isBinanceAccountTypeSupported(value string) bool {
	switch normalizeBinanceAccountType(value) {
	case BinanceAccountTypeSpot, BinanceAccountTypeFutures, BinanceAccountTypeUnified:
		return true
	default:
		return false
	}
}

func parsePositiveDecimal(value string) (*big.Rat, error) {
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 {
		return nil, errors.New("invalid non-negative decimal")
	}
	return parsed, nil
}

func binanceUSDTPrice(asset string, prices map[string]string) (*big.Rat, bool) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset == "USDT" {
		return big.NewRat(1, 1), true
	}
	if price, ok := positivePrice(prices[asset+"USDT"]); ok {
		return price, true
	}
	if reverse, ok := positivePrice(prices["USDT"+asset]); ok {
		return new(big.Rat).Inv(reverse), true
	}
	if underlying := binanceSimpleEarnUnderlying(asset); underlying != "" {
		return binanceUSDTPrice(underlying, prices)
	}
	for _, bridge := range []string{"USDC", "FDUSD", "BTC", "ETH", "BNB"} {
		assetBridge, ok := positivePrice(prices[asset+bridge])
		if !ok {
			continue
		}
		bridgeUSDT, ok := positivePrice(prices[bridge+"USDT"])
		if ok {
			return new(big.Rat).Mul(assetBridge, bridgeUSDT), true
		}
	}
	return nil, false
}

func binanceSpotHistorySymbol(asset string, prices map[string]string) string {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	for _, candidate := range []string{asset, binanceSimpleEarnUnderlying(asset)} {
		if candidate == "" || isUSDStableAsset(candidate) {
			continue
		}
		if _, ok := prices[candidate+"USDT"]; ok {
			return candidate + "USDT"
		}
	}
	return ""
}

func binanceSimpleEarnUnderlying(asset string) string {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if strings.HasPrefix(asset, "LD") && len(asset) > 2 {
		return strings.TrimPrefix(asset, "LD")
	}
	return ""
}

func positivePrice(value string) (*big.Rat, bool) {
	price, ok := new(big.Rat).SetString(value)
	return price, ok && price.Sign() > 0
}

func formatDecimal(value *big.Rat) string {
	result := value.FloatString(18)
	result = strings.TrimRight(result, "0")
	result = strings.TrimRight(result, ".")
	if result == "" || result == "-0" {
		return "0"
	}
	return result
}

func (a *Binance) signedGet(ctx context.Context, path string, query url.Values, output interface{}) error {
	return a.signedRequest(ctx, http.MethodGet, path, query, output)
}

func (a *Binance) signedRequest(ctx context.Context, method, path string, query url.Values, output interface{}) error {
	signedQuery := make(url.Values, len(query)+3)
	for key, values := range query {
		signedQuery[key] = append([]string(nil), values...)
	}
	signedQuery.Del("signature")
	signedQuery.Set("timestamp", strconv.FormatInt(a.now().UnixMilli(), 10))
	signedQuery.Set("recvWindow", "5000")
	payload := signedQuery.Encode()
	mac := hmac.New(sha256.New, []byte(a.credentials.SecretKey))
	_, _ = mac.Write([]byte(payload))
	signedQuery.Set("signature", hex.EncodeToString(mac.Sum(nil)))
	request, err := http.NewRequest(method, fmt.Sprintf("%s%s?%s", a.baseURL, path, signedQuery.Encode()), nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-MBX-APIKEY", a.credentials.APIKey)
	return doJSON(ctx, a.client, request, output)
}
