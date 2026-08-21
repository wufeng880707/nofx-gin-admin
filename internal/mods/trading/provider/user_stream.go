package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type UserDataEvent struct {
	Type            string
	EventTime       time.Time
	TransactionTime time.Time
	PayloadHash     string
}

type UserDataStream interface {
	Run(context.Context, func(UserDataEvent) error) error
}

type exchangeUserStream struct {
	exchangeType string
	credentials  Credentials
	httpClient   *http.Client
	dialer       *websocket.Dialer
	binanceREST  string
	binanceWSAPI string
	binanceFWS   string
	okxWS        string
}

func (a *Factory) BuildUserDataStream(exchange *schema.Exchange) (UserDataStream, error) {
	if exchange == nil {
		return nil, fmt.Errorf("exchange configuration is required")
	}
	apiKey, err := a.decrypt(exchange.APIKeyCiphertext, exchange, "api_key")
	if err != nil {
		return nil, err
	}
	secretKey, err := a.decrypt(exchange.SecretKeyCiphertext, exchange, "secret_key")
	if err != nil {
		return nil, err
	}
	credentials := Credentials{APIKey: apiKey, SecretKey: secretKey, Testnet: exchange.Testnet,
		AccountType: strings.ToLower(strings.TrimSpace(exchange.AccountType))}
	if credentials.APIKey == "" || credentials.SecretKey == "" {
		return nil, ErrCredentialsIncomplete
	}
	exchangeType := strings.ToLower(strings.TrimSpace(exchange.ExchangeType))
	if exchangeType == "okx" {
		credentials.Passphrase, err = a.decrypt(exchange.PassphraseCiphertext, exchange, "passphrase")
		if err != nil {
			return nil, err
		}
		if credentials.Passphrase == "" {
			return nil, ErrCredentialsIncomplete
		}
	}
	if exchangeType != "binance" && exchangeType != "okx" {
		return nil, fmt.Errorf("%w: %s user stream", ErrUnsupportedExchange, exchangeType)
	}
	if exchangeType == "binance" && credentials.AccountType == BinanceAccountTypeUnified {
		return nil, fmt.Errorf("%w: Binance Portfolio Margin user stream", ErrAccountTypeUnsupported)
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	stream := &exchangeUserStream{exchangeType: exchangeType, credentials: credentials, httpClient: client,
		dialer: websocket.DefaultDialer, binanceREST: "https://fapi.binance.com",
		binanceWSAPI: "wss://ws-api.binance.com:443/ws-api/v3", binanceFWS: "wss://fstream.binance.com/private/ws",
		okxWS: "wss://ws.okx.com:8443/ws/v5/private"}
	if credentials.Testnet {
		stream.binanceREST = "https://testnet.binancefuture.com"
		stream.binanceWSAPI = "wss://ws-api.testnet.binance.vision/ws-api/v3"
		stream.binanceFWS = "wss://stream.binancefuture.com/ws"
		stream.okxWS = "wss://wspap.okx.com:8443/ws/v5/private"
	}
	return stream, nil
}

func (a *exchangeUserStream) Run(ctx context.Context, handle func(UserDataEvent) error) error {
	if handle == nil {
		return fmt.Errorf("user data event handler is required")
	}
	if a.exchangeType == "okx" {
		return a.runOKX(ctx, handle)
	}
	if a.credentials.AccountType == BinanceAccountTypeFutures {
		return a.runBinanceFutures(ctx, handle)
	}
	return a.runBinanceSpot(ctx, handle)
}

func (a *exchangeUserStream) runBinanceSpot(ctx context.Context, handle func(UserDataEvent) error) error {
	connection, _, err := a.dialer.DialContext(ctx, a.binanceWSAPI, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	timestamp := time.Now().UnixMilli()
	params := url.Values{"apiKey": {a.credentials.APIKey}, "timestamp": {strconv.FormatInt(timestamp, 10)}}
	params.Set("signature", hmacHex(a.credentials.SecretKey, params.Encode()))
	request := map[string]interface{}{"id": uuid.NewString(), "method": "userDataStream.subscribe.signature",
		"params": map[string]interface{}{"apiKey": a.credentials.APIKey, "timestamp": timestamp, "signature": params.Get("signature")}}
	if err := connection.WriteJSON(request); err != nil {
		return err
	}
	if err := awaitBinanceWSAPIResponse(connection); err != nil {
		return err
	}
	return readUserEvents(ctx, connection, "binance", handle)
}

func (a *exchangeUserStream) runBinanceFutures(ctx context.Context, handle func(UserDataEvent) error) error {
	listenKey, err := a.createBinanceListenKey(ctx)
	if err != nil {
		return err
	}
	defer a.closeBinanceListenKey(context.Background(), listenKey)
	connection, _, err := a.dialer.DialContext(ctx, strings.TrimRight(a.binanceFWS, "/")+"/"+listenKey, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	keepaliveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go a.keepaliveBinanceListenKey(keepaliveCtx, listenKey)
	return readUserEvents(ctx, connection, "binance", handle)
}

func (a *exchangeUserStream) createBinanceListenKey(ctx context.Context) (string, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.binanceREST+"/fapi/v1/listenKey", nil)
	request.Header.Set("X-MBX-APIKEY", a.credentials.APIKey)
	var response struct {
		ListenKey string `json:"listenKey"`
	}
	if err := doJSON(ctx, a.httpClient, request, &response); err != nil {
		return "", err
	}
	if response.ListenKey == "" {
		return "", fmt.Errorf("Binance returned an empty listen key")
	}
	return response.ListenKey, nil
}

func (a *exchangeUserStream) keepaliveBinanceListenKey(ctx context.Context, listenKey string) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			request, _ := http.NewRequestWithContext(ctx, http.MethodPut, a.binanceREST+"/fapi/v1/listenKey", nil)
			request.Header.Set("X-MBX-APIKEY", a.credentials.APIKey)
			_, _ = a.httpClient.Do(request)
		}
	}
}

func (a *exchangeUserStream) closeBinanceListenKey(ctx context.Context, listenKey string) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodDelete, a.binanceREST+"/fapi/v1/listenKey", nil)
	request.Header.Set("X-MBX-APIKEY", a.credentials.APIKey)
	_, _ = a.httpClient.Do(request)
}

func (a *exchangeUserStream) runOKX(ctx context.Context, handle func(UserDataEvent) error) error {
	connection, _, err := a.dialer.DialContext(ctx, a.okxWS, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := hmacBase64(a.credentials.SecretKey, timestamp+"GET/users/self/verify")
	if err := connection.WriteJSON(map[string]interface{}{"op": "login", "args": []map[string]string{{
		"apiKey": a.credentials.APIKey, "passphrase": a.credentials.Passphrase, "timestamp": timestamp, "sign": signature,
	}}}); err != nil {
		return err
	}
	if err := awaitOKXControlEvent(connection, "login"); err != nil {
		return err
	}
	if err := connection.WriteJSON(map[string]interface{}{"id": uuid.NewString(), "op": "subscribe", "args": []map[string]string{
		{"channel": "account"}, {"channel": "positions", "instType": "ANY"}, {"channel": "orders", "instType": "ANY"},
	}}); err != nil {
		return err
	}
	if err := awaitOKXControlEvent(connection, "subscribe"); err != nil {
		return err
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_ = connection.WriteMessage(websocket.TextMessage, []byte("ping"))
			}
		}
	}()
	return readUserEvents(ctx, connection, "okx", handle)
}

func awaitBinanceWSAPIResponse(connection *websocket.Conn) error {
	_ = connection.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, payload, err := connection.ReadMessage()
	if err != nil {
		return err
	}
	var response struct {
		Status int `json:"status"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"msg"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("decode Binance user stream subscription: %w", err)
	}
	if response.Error != nil {
		return fmt.Errorf("Binance user stream subscription failed: code=%d message=%s", response.Error.Code, response.Error.Message)
	}
	if response.Status != http.StatusOK {
		return fmt.Errorf("Binance user stream subscription returned status %d", response.Status)
	}
	return nil
}

func awaitOKXControlEvent(connection *websocket.Conn, expected string) error {
	_ = connection.SetReadDeadline(time.Now().Add(15 * time.Second))
	for attempts := 0; attempts < 8; attempts++ {
		_, payload, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		if string(payload) == "pong" {
			continue
		}
		var response struct {
			Event string `json:"event"`
			Code  string `json:"code"`
			Msg   string `json:"msg"`
		}
		if err := json.Unmarshal(payload, &response); err != nil {
			return fmt.Errorf("decode OKX %s response: %w", expected, err)
		}
		if response.Event == "error" || (response.Code != "" && response.Code != "0") {
			return fmt.Errorf("OKX %s failed: code=%s message=%s", expected, response.Code, response.Msg)
		}
		if response.Event == expected {
			return nil
		}
	}
	return fmt.Errorf("OKX %s acknowledgement not received", expected)
}

func readUserEvents(ctx context.Context, connection *websocket.Conn, exchange string, handle func(UserDataEvent) error) error {
	_ = connection.SetReadDeadline(time.Now().Add(70 * time.Second))
	connection.SetPongHandler(func(string) error { return connection.SetReadDeadline(time.Now().Add(70 * time.Second)) })
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, payload, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		if string(payload) == "pong" {
			continue
		}
		event, ok, err := parseUserDataEvent(exchange, payload)
		if err != nil {
			return err
		}
		if ok {
			if err := handle(event); err != nil {
				return err
			}
		}
	}
}

func parseUserDataEvent(exchange string, payload []byte) (UserDataEvent, bool, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return UserDataEvent{}, false, err
	}
	if event, ok := envelope["event"].(map[string]interface{}); ok {
		envelope = event
	}
	eventType := ""
	eventMillis, transactionMillis := int64(0), int64(0)
	if exchange == "okx" {
		if eventName, _ := envelope["event"].(string); eventName != "" {
			if eventName == "error" {
				return UserDataEvent{}, false, fmt.Errorf("OKX user stream error: %v", envelope["msg"])
			}
			return UserDataEvent{}, false, nil
		}
		arg, _ := envelope["arg"].(map[string]interface{})
		eventType, _ = arg["channel"].(string)
		eventMillis = nestedEventMillis(envelope["data"])
	} else {
		eventType, _ = envelope["e"].(string)
		eventMillis = numberMillis(envelope["E"])
		transactionMillis = numberMillis(envelope["T"])
	}
	if eventType == "" {
		return UserDataEvent{}, false, nil
	}
	hash := sha256.Sum256(payload)
	event := UserDataEvent{Type: eventType, PayloadHash: hex.EncodeToString(hash[:])}
	if eventMillis > 0 {
		event.EventTime = time.UnixMilli(eventMillis).UTC()
	} else {
		event.EventTime = time.Now().UTC()
	}
	if transactionMillis > 0 {
		event.TransactionTime = time.UnixMilli(transactionMillis).UTC()
	}
	return event, true, nil
}

func nestedEventMillis(value interface{}) int64 {
	items, _ := value.([]interface{})
	if len(items) == 0 {
		return 0
	}
	item, _ := items[0].(map[string]interface{})
	for _, key := range []string{"pTime", "uTime", "ts"} {
		if parsed := numberMillis(item[key]); parsed > 0 {
			return parsed
		}
	}
	return 0
}

func numberMillis(value interface{}) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

func hmacHex(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func hmacBase64(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
