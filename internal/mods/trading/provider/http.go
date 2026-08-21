package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	utilityprom "github.com/LyricTian/gin-admin/v10/internal/utility/prom"
)

const (
	providerReadMaxAttempts = 3
	providerMaxRetryDelay   = 2 * time.Second
	providerErrorBodyLimit  = 2048
)

var providerRequestSlots = make(chan struct{}, 8)

type HTTPError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (a *HTTPError) Error() string {
	return fmt.Sprintf("provider request failed: status=%d body=%s", a.StatusCode, a.Body)
}

func doJSON(ctx context.Context, client *http.Client, request *http.Request, output interface{}) error {
	if client == nil {
		client = http.DefaultClient
	}
	attempts := 1
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		attempts = providerReadMaxAttempts
	}
	providerName := providerNameFromRequest(request)
	breaker := providerCircuits.get(providerName)
	startedAt := time.Now()
	if !breaker.allow(startedAt) {
		providerMetric(providerName, request.Method, "circuit_open", startedAt, ErrProviderCircuitOpen)
		return ErrProviderCircuitOpen
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := acquireProviderRequestSlot(ctx); err != nil {
			breaker.cancelProbe()
			return err
		}
		response, err := client.Do(request.Clone(ctx))
		<-providerRequestSlots
		if err != nil {
			if attempt == attempts || ctx.Err() != nil {
				if ctx.Err() == nil {
					breaker.failure(time.Now())
				} else {
					breaker.cancelProbe()
				}
				providerMetric(providerName, request.Method, "network_error", startedAt, err)
				return err
			}
			if err := waitContext(ctx, providerRetryDelay(attempt, "", time.Now())); err != nil {
				breaker.cancelProbe()
				return err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		closeErr := response.Body.Close()
		if readErr != nil {
			breaker.failure(time.Now())
			providerMetric(providerName, request.Method, "read_error", startedAt, readErr)
			return readErr
		}
		if closeErr != nil {
			breaker.failure(time.Now())
			providerMetric(providerName, request.Method, "close_error", startedAt, closeErr)
			return closeErr
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if err := json.Unmarshal(body, output); err != nil {
				breaker.failure(time.Now())
				providerMetric(providerName, request.Method, "decode_error", startedAt, err)
				return fmt.Errorf("decode provider response: %w", err)
			}
			breaker.success()
			providerMetric(providerName, request.Method, strconv.Itoa(response.StatusCode), startedAt, nil)
			return nil
		}
		retryAfter := providerRetryDelay(attempt, response.Header.Get("Retry-After"), time.Now())
		httpErr := &HTTPError{StatusCode: response.StatusCode, Body: limitProviderErrorBody(body), RetryAfter: retryAfter}
		if attempt == attempts || !providerStatusRetryable(response.StatusCode) {
			if providerStatusRetryable(response.StatusCode) {
				breaker.failure(time.Now())
			} else {
				breaker.success()
			}
			providerMetric(providerName, request.Method, strconv.Itoa(response.StatusCode), startedAt, httpErr)
			return httpErr
		}
		if err := waitContext(ctx, retryAfter); err != nil {
			breaker.cancelProbe()
			return err
		}
	}
	return fmt.Errorf("provider request attempts exhausted")
}

func providerNameFromRequest(request *http.Request) string {
	if request == nil || request.URL == nil || request.URL.Hostname() == "" {
		return "unknown"
	}
	host := strings.ToLower(request.URL.Hostname())
	switch {
	case strings.Contains(host, "binance"):
		return "binance"
	case strings.Contains(host, "okx"):
		return "okx"
	case strings.Contains(host, "bybit"):
		return "bybit"
	default:
		return host
	}
}

func providerMetric(providerName, method, code string, startedAt time.Time, err error) {
	if utilityprom.Ins == nil {
		return
	}
	utilityprom.Ins.RequestLog("trading_provider", providerName, method, code)
	utilityprom.Ins.SummaryLatencyLog("trading_provider", providerName, method, time.Since(startedAt).Seconds())
	if err != nil {
		utilityprom.Ins.ExceptionLog("trading_provider", code)
	}
	breaker := providerCircuits.get(providerName)
	if breaker.isOpen(time.Now()) {
		utilityprom.Ins.StateLog("trading_provider", providerName+"_circuit_open", 1)
	} else {
		utilityprom.Ins.StateLog("trading_provider", providerName+"_circuit_open", 0)
	}
}

func acquireProviderRequestSlot(ctx context.Context) error {
	select {
	case providerRequestSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func providerStatusRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError ||
		status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func providerRetryDelay(attempt int, retryAfter string, now time.Time) time.Duration {
	retryAfter = strings.TrimSpace(retryAfter)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return min(time.Duration(seconds)*time.Second, providerMaxRetryDelay)
	}
	if retryAt, err := http.ParseTime(retryAfter); err == nil {
		return min(max(retryAt.Sub(now), 0), providerMaxRetryDelay)
	}
	delay := time.Duration(1<<(attempt-1)) * 100 * time.Millisecond
	return min(delay, providerMaxRetryDelay)
}

func limitProviderErrorBody(body []byte) string {
	value := strings.TrimSpace(string(body))
	if len(value) > providerErrorBodyLimit {
		return value[:providerErrorBodyLimit] + "..."
	}
	return value
}
