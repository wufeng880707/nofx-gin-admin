package llm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
)

func TestOpenAICompatibleInvocationAndUsage(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		decodeJSON(t, r.Body, &received)
		w.Header().Set("x-request-id", "request-openai")
		_, _ = io.WriteString(w, `{"id":"chat-1","choices":[{"finish_reason":"stop","message":{"content":"OK"}}],"usage":{"prompt_tokens":7,"completion_tokens":1,"total_tokens":8}}`)
	}))
	defer server.Close()

	service, model := testServiceAndModel(t, "deepseek", server.URL, "deepseek-test")
	result, err := service.Invoke(context.Background(), model, InvocationRequest{
		Purpose: "connection_test", SystemPrompt: "system", UserPrompt: "user", MaxTokens: 8,
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Content != "OK" || result.InputTokens != 7 || result.OutputTokens != 1 || result.TotalTokens != 8 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if received["model"] != "deepseek-test" || received["stream"] != false {
		t.Fatalf("unexpected request: %#v", received)
	}
}

func TestAnthropicNativeInvocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" || r.Header.Get("x-api-key") != "secret-key" || r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("unexpected Anthropic request: %s %#v", r.URL.Path, r.Header)
		}
		var payload map[string]interface{}
		decodeJSON(t, r.Body, &payload)
		if payload["system"] != "system" || payload["model"] != "claude-test" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		_, _ = io.WriteString(w, `{"id":"msg-1","stop_reason":"end_turn","content":[{"type":"text","text":"OK"}],"usage":{"input_tokens":9,"output_tokens":2}}`)
	}))
	defer server.Close()

	service, model := testServiceAndModel(t, "anthropic", server.URL, "claude-test")
	result, err := service.Invoke(context.Background(), model, InvocationRequest{Purpose: "strategy_test", SystemPrompt: "system", UserPrompt: "user"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Content != "OK" || result.TotalTokens != 11 || result.Provider != "anthropic" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGeminiNativeInvocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-test:generateContent" || r.Header.Get("x-goog-api-key") != "secret-key" {
			t.Fatalf("unexpected Gemini request: %s %#v", r.URL.Path, r.Header)
		}
		var payload map[string]interface{}
		decodeJSON(t, r.Body, &payload)
		if _, ok := payload["systemInstruction"]; !ok {
			t.Fatalf("systemInstruction missing: %#v", payload)
		}
		_, _ = io.WriteString(w, `{"responseId":"gemini-1","candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"OK"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3,"totalTokenCount":13}}`)
	}))
	defer server.Close()

	service, model := testServiceAndModel(t, "gemini", server.URL, "gemini-test")
	result, err := service.Invoke(context.Background(), model, InvocationRequest{Purpose: "strategy_test", SystemPrompt: "system", UserPrompt: "user"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Content != "OK" || result.TotalTokens != 13 || result.RequestID != "gemini-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRetriesFiniteAndNormalizesErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"message":"secret-key must never leak"}}`)
	}))
	defer server.Close()
	service, model := testServiceAndModel(t, "custom", server.URL, "custom-model")
	service.MaxAttempts, service.RetryDelay = 2, time.Millisecond
	_, err := service.Invoke(context.Background(), model, InvocationRequest{Purpose: "connection_test", UserPrompt: "user"})
	if err == nil || calls.Load() != 2 {
		t.Fatalf("expected two failed attempts, calls=%d err=%v", calls.Load(), err)
	}
	if strings.Contains(err.Error(), "secret-key") || err.Error() != "AI provider is temporarily unavailable" {
		t.Fatalf("provider error was not normalized: %v", err)
	}
}

func TestEndpointValidationBlocksPrivateNetworksAndRedirects(t *testing.T) {
	service := NewService(nil, credential.NewService(make([]byte, 32), nil))
	for _, rawURL := range []string{"http://example.com", "https://127.0.0.1", "https://10.0.0.1", "https://[::1]"} {
		if err := service.ValidateEndpoint(context.Background(), rawURL); err == nil {
			t.Fatalf("expected endpoint %s to be rejected", rawURL)
		}
	}

	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://127.0.0.1/internal"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})}
	_, err := service.client().Get("https://93.184.216.34/start")
	if err == nil {
		t.Fatal("expected redirect to private endpoint to be rejected")
	}
}

func TestInvocationCostUSDUsesConfiguredMillionTokenPrices(t *testing.T) {
	if got := invocationCostUSD(1_000_000, 500_000, "0.25", "1.50"); got != "1" {
		t.Fatalf("unexpected invocation cost: %s", got)
	}
	if got := invocationCostUSD(100, 200, "", "invalid"); got != "0" {
		t.Fatalf("invalid prices must not create cost: %s", got)
	}
}

func testServiceAndModel(t *testing.T, provider, baseURL, modelName string) (*Service, *schema.AIModel) {
	t.Helper()
	key := sha256.Sum256([]byte("test-encryption-key"))
	credentials := credential.NewService(key[:], nil)
	model := &schema.AIModel{ID: "model-1", OwnerUserID: "user-1", Provider: provider, Enabled: true, CustomAPIURL: baseURL, CustomModelName: modelName}
	var err error
	model.APIKeyCiphertext, err = credentials.EncryptField("secret-key", model.OwnerUserID, model.ID, "api_key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	service := NewService(nil, credentials)
	service.AllowPrivateEndpoints = true
	service.RetryDelay = time.Millisecond
	return service, model
}

func decodeJSON(t *testing.T, body io.Reader, target interface{}) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
