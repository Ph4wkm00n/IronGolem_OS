// Package provider contains the Anthropic LLM provider implementation.
//
// This provider makes HTTP requests to the Anthropic Messages API
// (https://api.anthropic.com/v1/messages) and supports claude-sonnet-4-20250514
// and claude-haiku-4-20250414 models.
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
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicAPIVersion     = "2023-06-01"
	anthropicEnvKey         = "IRONGOLEM_ANTHROPIC_API_KEY"
)

// AnthropicProvider implements the Provider interface for the Anthropic
// Messages API.
type AnthropicProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	models     []string
}

// AnthropicOption configures the AnthropicProvider.
type AnthropicOption func(*AnthropicProvider)

// WithAnthropicBaseURL overrides the default Anthropic API base URL.
func WithAnthropicBaseURL(url string) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.baseURL = url
	}
}

// WithAnthropicHTTPClient sets a custom HTTP client.
func WithAnthropicHTTPClient(client *http.Client) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.httpClient = client
	}
}

// NewAnthropicProvider creates an Anthropic provider that reads its API key
// from the IRONGOLEM_ANTHROPIC_API_KEY environment variable.
func NewAnthropicProvider(opts ...AnthropicOption) (*AnthropicProvider, error) {
	apiKey := os.Getenv(anthropicEnvKey)
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic: %s environment variable is not set", anthropicEnvKey)
	}

	p := &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: anthropicDefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		models: []string{
			"claude-sonnet-4-20250514",
			"claude-haiku-4-20250414",
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Models returns the supported Anthropic model identifiers.
func (p *AnthropicProvider) Models() []string {
	return p.models
}

// Complete sends a completion request to the Anthropic Messages API and
// returns the response.
func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	anthropicReq := p.buildRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic: failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic: HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic: failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return CompletionResponse{}, p.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var anthropicResp anthropicMessagesResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic: failed to unmarshal response: %w", err)
	}

	return p.convertResponse(anthropicResp), nil
}

// --- Anthropic API request/response types ---

type anthropicMessagesRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessagesResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []anthropicContentBlock  `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        anthropicUsage           `json:"usage"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// buildRequest converts the generic CompletionRequest into an Anthropic
// Messages API request.
func (p *AnthropicProvider) buildRequest(req CompletionRequest) anthropicMessagesRequest {
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		// The Anthropic API handles system prompts separately; skip system
		// messages in the messages array.
		if msg.Role == RoleSystem {
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	model := req.Model
	if model == "" {
		model = p.models[0]
	}

	return anthropicMessagesRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
	}
}

// convertResponse transforms the Anthropic API response into our generic
// CompletionResponse.
func (p *AnthropicProvider) convertResponse(resp anthropicMessagesResponse) CompletionResponse {
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return CompletionResponse{
		Content: content,
		Model:   resp.Model,
		Usage: Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
		FinishReason: resp.StopReason,
	}
}

// handleErrorResponse parses Anthropic API error responses and returns
// descriptive errors for rate limits, auth errors, and generic failures.
func (p *AnthropicProvider) handleErrorResponse(statusCode int, body []byte) error {
	var apiErr anthropicErrorResponse
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("anthropic: HTTP %d: %s", statusCode, string(body))
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("anthropic: authentication failed: %s", apiErr.Error.Message)
	case http.StatusForbidden:
		return fmt.Errorf("anthropic: permission denied: %s", apiErr.Error.Message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("anthropic: rate limited: %s", apiErr.Error.Message)
	case http.StatusBadRequest:
		return fmt.Errorf("anthropic: bad request: %s", apiErr.Error.Message)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return fmt.Errorf("anthropic: server error (HTTP %d): %s", statusCode, apiErr.Error.Message)
	default:
		return fmt.Errorf("anthropic: unexpected error (HTTP %d): %s", statusCode, apiErr.Error.Message)
	}
}
