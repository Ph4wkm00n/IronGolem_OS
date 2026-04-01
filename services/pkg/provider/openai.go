// Package provider contains the OpenAI-compatible LLM provider implementation.
//
// This provider makes HTTP requests to the OpenAI Chat Completions API and
// supports a configurable base URL for compatibility with local models and
// third-party OpenAI-compatible endpoints.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	openaiDefaultBaseURL = "https://api.openai.com"
	openaiEnvKey         = "IRONGOLEM_OPENAI_API_KEY"
)

// OpenAIProvider implements the Provider interface for the OpenAI Chat
// Completions API and compatible endpoints.
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	models     []string
	name       string
}

// OpenAIOption configures the OpenAIProvider.
type OpenAIOption func(*OpenAIProvider)

// WithOpenAIBaseURL overrides the default OpenAI API base URL. This allows
// using OpenAI-compatible APIs such as local model servers.
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.baseURL = url
	}
}

// WithOpenAIHTTPClient sets a custom HTTP client.
func WithOpenAIHTTPClient(client *http.Client) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.httpClient = client
	}
}

// WithOpenAIModels overrides the default model list.
func WithOpenAIModels(models []string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.models = models
	}
}

// WithOpenAIName overrides the provider name (useful for distinguishing
// multiple OpenAI-compatible providers).
func WithOpenAIName(name string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.name = name
	}
}

// NewOpenAIProvider creates an OpenAI-compatible provider that reads its API
// key from the IRONGOLEM_OPENAI_API_KEY environment variable.
func NewOpenAIProvider(opts ...OpenAIOption) (*OpenAIProvider, error) {
	apiKey := os.Getenv(openaiEnvKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai: %s environment variable is not set", openaiEnvKey)
	}

	p := &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: openaiDefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		models: []string{
			"gpt-4o",
			"gpt-4o-mini",
		},
		name: "openai",
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string {
	return p.name
}

// Models returns the supported model identifiers.
func (p *OpenAIProvider) Models() []string {
	return p.models
}

// Complete sends a chat completion request to the OpenAI-compatible API
// and returns the response.
func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	openaiReq := p.buildRequest(req)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai: failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai: HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return CompletionResponse{}, p.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var openaiResp openaiChatCompletionResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("openai: failed to unmarshal response: %w", err)
	}

	return p.convertResponse(openaiResp), nil
}

// --- OpenAI API request/response types ---

type openaiChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatCompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// buildRequest converts the generic CompletionRequest into an OpenAI Chat
// Completions API request.
func (p *OpenAIProvider) buildRequest(req CompletionRequest) openaiChatCompletionRequest {
	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	// Add system prompt as a system message if provided.
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		// Skip system messages since we handle SystemPrompt separately.
		if msg.Role == RoleSystem {
			continue
		}
		messages = append(messages, openaiMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = p.models[0]
	}

	openaiReq := openaiChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		temp := req.Temperature
		openaiReq.Temperature = &temp
	}

	return openaiReq
}

// convertResponse transforms the OpenAI API response into our generic
// CompletionResponse.
func (p *OpenAIProvider) convertResponse(resp openaiChatCompletionResponse) CompletionResponse {
	var content string
	var finishReason string

	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		finishReason = resp.Choices[0].FinishReason
	}

	return CompletionResponse{
		Content: content,
		Model:   resp.Model,
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
		FinishReason: finishReason,
	}
}

// handleErrorResponse parses OpenAI API error responses and returns
// descriptive errors.
func (p *OpenAIProvider) handleErrorResponse(statusCode int, body []byte) error {
	var apiErr openaiErrorResponse
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("openai: HTTP %d: %s", statusCode, string(body))
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("openai: authentication failed: %s", apiErr.Error.Message)
	case http.StatusForbidden:
		return fmt.Errorf("openai: permission denied: %s", apiErr.Error.Message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("openai: rate limited: %s", apiErr.Error.Message)
	case http.StatusBadRequest:
		return fmt.Errorf("openai: bad request: %s", apiErr.Error.Message)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("openai: server error (HTTP %d): %s", statusCode, apiErr.Error.Message)
	default:
		return fmt.Errorf("openai: unexpected error (HTTP %d): %s", statusCode, apiErr.Error.Message)
	}
}
