package provider

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestParseBinanceUserDataEventStoresMetadataOnly(t *testing.T) {
	payload := []byte(`{"subscriptionId":0,"event":{"e":"executionReport","E":1728972148778,"T":1728972148779,"s":"BTCUSDT","secret":"must-not-persist"}}`)
	event, ok, err := parseUserDataEvent("binance", payload)
	if err != nil || !ok {
		t.Fatalf("parse Binance event: ok=%v err=%v", ok, err)
	}
	if event.Type != "executionReport" || event.EventTime.UnixMilli() != 1728972148778 || event.TransactionTime.UnixMilli() != 1728972148779 {
		t.Fatalf("unexpected Binance event: %#v", event)
	}
	if len(event.PayloadHash) != 64 {
		t.Fatalf("expected SHA-256 payload hash, got %q", event.PayloadHash)
	}
}

func TestParseOKXUserDataEventAndControlFrames(t *testing.T) {
	payload := []byte(`{"arg":{"channel":"orders","instType":"ANY"},"data":[{"ordId":"1","uTime":"1728972148778"}]}`)
	event, ok, err := parseUserDataEvent("okx", payload)
	if err != nil || !ok || event.Type != "orders" || event.EventTime.UnixMilli() != 1728972148778 {
		t.Fatalf("unexpected OKX event: %#v ok=%v err=%v", event, ok, err)
	}
	_, ok, err = parseUserDataEvent("okx", []byte(`{"event":"subscribe","arg":{"channel":"orders"}}`))
	if err != nil || ok {
		t.Fatalf("subscription acknowledgement must not become a data event: ok=%v err=%v", ok, err)
	}
	_, ok, err = parseUserDataEvent("okx", []byte(`{"event":"error","code":"60012","msg":"invalid"}`))
	if err == nil || ok {
		t.Fatalf("OKX error frame must fail the stream: ok=%v err=%v", ok, err)
	}
}

func TestUserStreamSignaturesAreDeterministic(t *testing.T) {
	if got := hmacHex("secret", "apiKey=key&timestamp=1"); got != hmacHex("secret", "apiKey=key&timestamp=1") || len(got) != 64 {
		t.Fatalf("unexpected HMAC hex: %s", got)
	}
	if got := hmacBase64("secret", "1GET/users/self/verify"); got == "" {
		t.Fatal("expected OKX base64 signature")
	}
}

func TestUserStreamSubscriptionAcknowledgements(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.WriteJSON(map[string]interface{}{"status": 200, "result": map[string]interface{}{}})
		_ = connection.WriteJSON(map[string]interface{}{"event": "login", "code": "0"})
		_ = connection.WriteJSON(map[string]interface{}{"event": "subscribe", "code": "0"})
	}))
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := awaitBinanceWSAPIResponse(connection); err != nil {
		t.Fatalf("Binance acknowledgement: %v", err)
	}
	if err := awaitOKXControlEvent(connection, "login"); err != nil {
		t.Fatalf("OKX login acknowledgement: %v", err)
	}
	if err := awaitOKXControlEvent(connection, "subscribe"); err != nil {
		t.Fatalf("OKX subscribe acknowledgement: %v", err)
	}
}
