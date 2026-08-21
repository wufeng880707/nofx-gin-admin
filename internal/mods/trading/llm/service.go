package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/credential"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	utilityprom "github.com/LyricTian/gin-admin/v10/internal/utility/prom"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
)

const maxResponseBytes = 2 << 20

type InvocationRequest struct {
	Purpose      string
	TraderID     *string
	StrategyID   *string
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	Temperature  *float64
}

type Result struct {
	Provider     string
	ModelName    string
	RequestID    string
	Content      string
	FinishReason string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	DurationMS   int64
	HTTPStatus   int
}

type ProviderError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "AI provider request failed"
	}
	return e.Message
}

type Service struct {
	DB                    *gorm.DB
	Credentials           *credential.Service
	HTTPClient            *http.Client
	Resolver              *net.Resolver
	AllowPrivateEndpoints bool
	RetryDelay            time.Duration
	MaxAttempts           int
	semaphore             chan struct{}
}

func NewService(db *gorm.DB, credentials *credential.Service) *Service {
	return &Service{
		DB: db, Credentials: credentials, Resolver: net.DefaultResolver,
		RetryDelay: 250 * time.Millisecond, MaxAttempts: 3, semaphore: make(chan struct{}, 8),
	}
}

func (s *Service) Invoke(ctx context.Context, model *schema.AIModel, input InvocationRequest) (result *Result, err error) {
	startedAt := time.Now().UTC()
	if model == nil {
		return nil, &ProviderError{Code: "model_missing", Message: "AI model configuration is missing"}
	}
	if input.MaxTokens <= 0 {
		input.MaxTokens = 512
	}
	if strings.TrimSpace(input.Purpose) == "" {
		input.Purpose = "unspecified"
	}

	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		return nil, &ProviderError{Code: "request_cancelled", Message: "AI request was cancelled"}
	}

	defer func() {
		s.recordInvocation(ctx, model, input, result, err, startedAt)
	}()

	apiKey, decryptErr := s.Credentials.DecryptField(model.APIKeyCiphertext, model.OwnerUserID, model.ID, "api_key")
	if decryptErr != nil {
		return nil, &ProviderError{Code: "credential_decryption_failed", Message: "Unable to decrypt AI model credential"}
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, &ProviderError{Code: "credential_missing", Message: "AI model API key is missing"}
	}

	provider, baseURL, modelName, resolveErr := resolveProvider(model)
	if resolveErr != nil {
		return nil, resolveErr
	}
	if validateErr := s.ValidateEndpoint(ctx, baseURL); validateErr != nil {
		return nil, validateErr
	}

	requestCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	started := time.Now()
	switch provider {
	case "anthropic":
		result, err = s.callAnthropic(requestCtx, baseURL, modelName, apiKey, input)
	case "gemini":
		result, err = s.callGemini(requestCtx, baseURL, modelName, apiKey, input)
	default:
		result, err = s.callOpenAICompatible(requestCtx, provider, baseURL, modelName, apiKey, input)
	}
	if result != nil {
		result.DurationMS = time.Since(started).Milliseconds()
	}
	return result, err
}

func resolveProvider(model *schema.AIModel) (provider, baseURL, modelName string, err error) {
	provider = strings.ToLower(strings.TrimSpace(model.Provider))
	baseURL, modelName = strings.TrimSpace(model.CustomAPIURL), strings.TrimSpace(model.CustomModelName)
	defaults := map[string][2]string{
		"openai":    {"https://api.openai.com/v1", "gpt-5.4"},
		"deepseek":  {"https://api.deepseek.com", "deepseek-v4-flash"},
		"qwen":      {"https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen3-max"},
		"anthropic": {"https://api.anthropic.com/v1", "claude-opus-4-6"},
		"claude":    {"https://api.anthropic.com/v1", "claude-opus-4-6"},
		"gemini":    {"https://generativelanguage.googleapis.com/v1beta", "gemini-3.1-pro"},
	}
	if provider == "claude" {
		provider = "anthropic"
	}
	if provider == "custom" {
		if baseURL == "" || modelName == "" {
			return "", "", "", &ProviderError{Code: "custom_provider_incomplete", Message: "Custom provider requires both API URL and model name"}
		}
		return provider, baseURL, modelName, nil
	}
	value, ok := defaults[provider]
	if !ok {
		return "", "", "", &ProviderError{Code: "provider_unsupported", Message: "Unsupported AI model provider"}
	}
	if baseURL == "" {
		baseURL = value[0]
	}
	if modelName == "" {
		modelName = value[1]
	}
	return provider, baseURL, modelName, nil
}

func (s *Service) ValidateEndpoint(ctx context.Context, rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" || u.Scheme == "" {
		return &ProviderError{Code: "endpoint_invalid", Message: "AI provider endpoint is invalid"}
	}
	if u.Scheme != "https" && !(s.AllowPrivateEndpoints && u.Scheme == "http") {
		return &ProviderError{Code: "endpoint_https_required", Message: "AI provider endpoint must use HTTPS"}
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return &ProviderError{Code: "endpoint_components_forbidden", Message: "AI provider endpoint cannot contain credentials, query parameters, or fragments"}
	}
	if s.AllowPrivateEndpoints {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return &ProviderError{Code: "endpoint_private_forbidden", Message: "Private AI provider endpoints are not allowed"}
	}
	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return &ProviderError{Code: "endpoint_private_forbidden", Message: "Private AI provider endpoints are not allowed"}
		}
		return nil
	}
	resolver := s.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, lookupErr := resolver.LookupIPAddr(ctx, host)
	if lookupErr != nil || len(addresses) == 0 {
		return &ProviderError{Code: "endpoint_dns_failed", Message: "Unable to resolve AI provider endpoint"}
	}
	for _, address := range addresses {
		if isForbiddenIP(address.IP) {
			return &ProviderError{Code: "endpoint_private_forbidden", Message: "AI provider endpoint resolved to a private address"}
		}
	}
	return nil
}

func isForbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}

func (s *Service) client() *http.Client {
	base := s.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: 60 * time.Second}
	}
	clone := *base
	previousRedirect := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return &ProviderError{Code: "redirect_limit", Message: "AI provider returned too many redirects"}
		}
		if err := s.ValidateEndpoint(req.Context(), req.URL.String()); err != nil {
			return err
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		return nil
	}
	return &clone
}

func endpoint(baseURL, suffix string) string {
	u, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	u.Path = path.Join(u.Path, suffix)
	return u.String()
}

func (s *Service) doJSON(ctx context.Context, method, endpointURL string, headers http.Header, payload interface{}, output interface{}) (*Result, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &ProviderError{Code: "request_encoding_failed", Message: "Unable to encode AI provider request"}
	}
	attempts := s.MaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	delay := s.RetryDelay
	if delay <= 0 {
		delay = 250 * time.Millisecond
	}
	var lastStatus int
	for attempt := 1; attempt <= attempts; attempt++ {
		req, requestErr := http.NewRequestWithContext(ctx, method, endpointURL, bytes.NewReader(body))
		if requestErr != nil {
			return nil, &ProviderError{Code: "request_build_failed", Message: "Unable to build AI provider request"}
		}
		req.Header = headers.Clone()
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-XTrading-Request-ID", util.NewXID())
		resp, requestErr := s.client().Do(req)
		if requestErr != nil {
			if ctx.Err() != nil {
				return nil, &ProviderError{Code: "request_timeout", Message: "AI provider request timed out"}
			}
			if attempt < attempts {
				if err := sleepContext(ctx, delay*time.Duration(attempt)); err != nil {
					return nil, &ProviderError{Code: "request_cancelled", Message: "AI provider request was cancelled"}
				}
				continue
			}
			return nil, &ProviderError{Code: "network_error", Message: "Unable to reach AI provider"}
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
		lastStatus = resp.StatusCode
		if readErr != nil {
			return nil, &ProviderError{Code: "response_read_failed", Message: "Unable to read AI provider response", HTTPStatus: resp.StatusCode}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.Unmarshal(responseBody, output); err != nil {
				return nil, &ProviderError{Code: "response_invalid", Message: "AI provider returned an invalid response", HTTPStatus: resp.StatusCode}
			}
			return &Result{HTTPStatus: resp.StatusCode, RequestID: firstHeader(resp.Header, "x-request-id", "request-id")}, nil
		}
		if retryableStatus(resp.StatusCode) && attempt < attempts {
			if err := sleepContext(ctx, delay*time.Duration(attempt)); err != nil {
				return nil, &ProviderError{Code: "request_cancelled", Message: "AI provider request was cancelled", HTTPStatus: resp.StatusCode}
			}
			continue
		}
		return nil, normalizeHTTPError(resp.StatusCode)
	}
	return nil, &ProviderError{Code: "provider_unavailable", Message: "AI provider is temporarily unavailable", HTTPStatus: lastStatus}
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func normalizeHTTPError(status int) error {
	code, message := "provider_error", "AI provider rejected the request"
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		code, message = "authentication_failed", "AI provider authentication failed"
	case http.StatusTooManyRequests:
		code, message = "rate_limited", "AI provider rate limit was reached"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code, message = "request_rejected", "AI provider rejected the model or request parameters"
	default:
		if status >= 500 {
			code, message = "provider_unavailable", "AI provider is temporarily unavailable"
		}
	}
	return &ProviderError{Code: code, Message: message, HTTPStatus: status}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) recordInvocation(ctx context.Context, model *schema.AIModel, input InvocationRequest, result *Result, invokeErr error, startedAt time.Time) {
	if s.DB == nil || model == nil {
		return
	}
	finishedAt := time.Now().UTC()
	item := &schema.AIInvocation{
		ID: util.NewXID(), OwnerUserID: model.OwnerUserID, AIModelID: model.ID,
		TraderID: input.TraderID, StrategyID: input.StrategyID, Purpose: input.Purpose,
		Provider: model.Provider, ModelName: model.CustomModelName, StartedAt: startedAt,
		FinishedAt: &finishedAt, DurationMS: finishedAt.Sub(startedAt).Milliseconds(), CreatedAt: finishedAt,
	}
	if _, _, modelName, resolveErr := resolveProvider(model); resolveErr == nil {
		item.ModelName = modelName
	}
	if result != nil {
		item.Provider, item.ModelName, item.RequestID = result.Provider, result.ModelName, result.RequestID
		item.InputTokens, item.OutputTokens, item.TotalTokens = result.InputTokens, result.OutputTokens, result.TotalTokens
		item.DurationMS, item.HTTPStatus = result.DurationMS, result.HTTPStatus
		item.EstimatedCostUSD = invocationCostUSD(result.InputTokens, result.OutputTokens,
			model.InputPricePerMTokens, model.OutputPricePerMTokens)
	}
	item.Success = invokeErr == nil
	if providerErr, ok := invokeErr.(*ProviderError); ok {
		item.ErrorCode, item.ErrorMessage, item.HTTPStatus = providerErr.Code, providerErr.Message, providerErr.HTTPStatus
	} else if invokeErr != nil {
		item.ErrorCode, item.ErrorMessage = "internal_error", "AI provider invocation failed"
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_ = s.DB.WithContext(writeCtx).Create(item).Error
	if utilityprom.Ins != nil {
		code := "success"
		if !item.Success {
			code = "error"
		}
		utilityprom.Ins.RequestLog("trading_ai", item.Provider, item.Purpose, code)
		utilityprom.Ins.SummaryLatencyLog("trading_ai", item.Provider, item.Purpose, float64(item.DurationMS)/1000)
		if !item.Success {
			utilityprom.Ins.ExceptionLog("trading_ai", item.ErrorCode)
		}
	}
}

func invocationCostUSD(inputTokens, outputTokens int, inputPrice, outputPrice string) string {
	total := new(big.Rat)
	for _, item := range []struct {
		tokens int
		price  string
	}{{inputTokens, inputPrice}, {outputTokens, outputPrice}} {
		price, ok := new(big.Rat).SetString(strings.TrimSpace(item.price))
		if !ok || price.Sign() <= 0 || item.tokens <= 0 {
			continue
		}
		total.Add(total, new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(int64(item.tokens), 1), price), big.NewRat(1_000_000, 1)))
	}
	if total.Sign() == 0 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(total.FloatString(8), "0"), ".")
}

func providerErrorf(code, message string) error {
	return &ProviderError{Code: code, Message: message}
}

func ensureContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return providerErrorf("response_empty", "AI provider returned an empty response")
	}
	return nil
}

func combineText(parts []string) string {
	return strings.TrimSpace(strings.Join(parts, ""))
}

func unsupportedResponse(provider string) error {
	return providerErrorf("response_invalid", fmt.Sprintf("%s returned an invalid response", provider))
}
