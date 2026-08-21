package llm

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) callOpenAICompatible(ctx context.Context, provider, baseURL, modelName, apiKey string, input InvocationRequest) (*Result, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]message, 0, 2)
	if strings.TrimSpace(input.SystemPrompt) != "" {
		messages = append(messages, message{Role: "system", Content: input.SystemPrompt})
	}
	messages = append(messages, message{Role: "user", Content: input.UserPrompt})
	payload := map[string]interface{}{"model": modelName, "messages": messages, "max_tokens": input.MaxTokens, "stream": false}
	if input.Temperature != nil {
		payload["temperature"] = *input.Temperature
	}
	var response struct {
		ID      string `json:"id"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+apiKey)
	meta, err := s.doJSON(ctx, http.MethodPost, endpoint(baseURL, "chat/completions"), headers, payload, &response)
	if err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, unsupportedResponse(provider)
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if err := ensureContent(content); err != nil {
		return nil, err
	}
	meta.Provider, meta.ModelName, meta.Content, meta.FinishReason = provider, modelName, content, response.Choices[0].FinishReason
	meta.InputTokens, meta.OutputTokens, meta.TotalTokens = response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens
	if meta.RequestID == "" {
		meta.RequestID = response.ID
	}
	return meta, nil
}

func (s *Service) callAnthropic(ctx context.Context, baseURL, modelName, apiKey string, input InvocationRequest) (*Result, error) {
	payload := map[string]interface{}{
		"model": modelName, "max_tokens": input.MaxTokens,
		"messages": []map[string]string{{"role": "user", "content": input.UserPrompt}},
	}
	if strings.TrimSpace(input.SystemPrompt) != "" {
		payload["system"] = input.SystemPrompt
	}
	if input.Temperature != nil {
		payload["temperature"] = *input.Temperature
	}
	var response struct {
		ID         string `json:"id"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	headers := make(http.Header)
	headers.Set("x-api-key", apiKey)
	headers.Set("anthropic-version", "2023-06-01")
	meta, err := s.doJSON(ctx, http.MethodPost, endpoint(baseURL, "messages"), headers, payload, &response)
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, len(response.Content))
	for _, block := range response.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	content := combineText(parts)
	if err := ensureContent(content); err != nil {
		return nil, err
	}
	meta.Provider, meta.ModelName, meta.Content, meta.FinishReason = "anthropic", modelName, content, response.StopReason
	meta.InputTokens, meta.OutputTokens = response.Usage.InputTokens, response.Usage.OutputTokens
	meta.TotalTokens = meta.InputTokens + meta.OutputTokens
	if meta.RequestID == "" {
		meta.RequestID = response.ID
	}
	return meta, nil
}

func (s *Service) callGemini(ctx context.Context, baseURL, modelName, apiKey string, input InvocationRequest) (*Result, error) {
	payload := map[string]interface{}{
		"contents":         []map[string]interface{}{{"role": "user", "parts": []map[string]string{{"text": input.UserPrompt}}}},
		"generationConfig": map[string]interface{}{"maxOutputTokens": input.MaxTokens},
	}
	if strings.TrimSpace(input.SystemPrompt) != "" {
		payload["systemInstruction"] = map[string]interface{}{"parts": []map[string]string{{"text": input.SystemPrompt}}}
	}
	if input.Temperature != nil {
		payload["generationConfig"].(map[string]interface{})["temperature"] = *input.Temperature
	}
	var response struct {
		ResponseID string `json:"responseId"`
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	headers := make(http.Header)
	headers.Set("x-goog-api-key", apiKey)
	modelPath := "models/" + url.PathEscape(modelName) + ":generateContent"
	meta, err := s.doJSON(ctx, http.MethodPost, endpoint(baseURL, modelPath), headers, payload, &response)
	if err != nil {
		return nil, err
	}
	if len(response.Candidates) == 0 {
		return nil, unsupportedResponse("gemini")
	}
	parts := make([]string, 0, len(response.Candidates[0].Content.Parts))
	for _, part := range response.Candidates[0].Content.Parts {
		parts = append(parts, part.Text)
	}
	content := combineText(parts)
	if err := ensureContent(content); err != nil {
		return nil, err
	}
	meta.Provider, meta.ModelName, meta.Content = "gemini", modelName, content
	meta.FinishReason = response.Candidates[0].FinishReason
	meta.InputTokens, meta.OutputTokens, meta.TotalTokens = response.Usage.PromptTokenCount, response.Usage.CandidatesTokenCount, response.Usage.TotalTokenCount
	if meta.RequestID == "" {
		meta.RequestID = response.ResponseID
	}
	return meta, nil
}
