package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoJSONRetriesReadOnlyRequestAfterRateLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	var result struct {
		OK bool `json:"ok"`
	}
	if err := doJSON(context.Background(), server.Client(), request, &result); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !result.OK {
		t.Fatalf("unexpected retry result: requests=%d result=%#v", requests, result)
	}
}

func TestDoJSONDoesNotRetryNonIdempotentRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodPost, server.URL, nil)
	err := doJSON(context.Background(), server.Client(), request, new(map[string]interface{}))
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("non-idempotent request retried %d times", requests)
	}
}

func TestDoJSONStopsAfterFiniteServerErrorRetries(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	err := doJSON(context.Background(), server.Client(), request, new(map[string]interface{}))
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != providerReadMaxAttempts {
		t.Fatalf("unexpected request attempts: %d", requests)
	}
}

func TestDoJSONDoesNotRetryClientError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_ = doJSON(context.Background(), server.Client(), request, new(map[string]interface{}))
	if requests != 1 {
		t.Fatalf("client error retried %d times", requests)
	}
}

func TestDoJSONStopsRetryWaitWhenContextIsCanceled(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	err := doJSON(ctx, server.Client(), request, new(map[string]interface{}))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("request retried after context cancellation: %d", requests)
	}
}
