// Package provider defines the LLM provider abstraction for IronGolem OS.
//
// It allows the system to work with multiple LLM providers (Anthropic, OpenAI,
// local models) through a unified interface, with usage telemetry tracking.
package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Role represents the role of a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest holds the parameters for an LLM completion.
type CompletionRequest struct {
	// Model is the model identifier (e.g. "claude-sonnet-4-20250514").
	Model string `json:"model"`
	// Messages is the conversation history.
	Messages []Message `json:"messages"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// Temperature controls randomness (0.0 to 1.0).
	Temperature float64 `json:"temperature"`
	// SystemPrompt is the system-level instruction.
	SystemPrompt string `json:"system_prompt,omitempty"`
}

// Usage tracks token consumption for a completion.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CompletionResponse holds the result of an LLM completion.
type CompletionResponse struct {
	// Content is the generated text.
	Content string `json:"content"`
	// Model is the model that was used.
	Model string `json:"model"`
	// Usage tracks token consumption.
	Usage Usage `json:"usage"`
	// FinishReason explains why generation stopped.
	FinishReason string `json:"finish_reason"`
}

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	// Name returns the provider's identifier (e.g. "anthropic", "openai").
	Name() string
	// Models returns the list of supported model identifiers.
	Models() []string
}

// UsageRecord captures telemetry for a single completion call.
type UsageRecord struct {
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Latency      time.Duration `json:"latency_ns"`
	Cost         float64       `json:"cost"`
	Timestamp    time.Time     `json:"timestamp"`
	Success      bool          `json:"success"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// UsageTelemetry tracks token usage, latency, and cost per provider.
type UsageTelemetry struct {
	mu      sync.RWMutex
	records []UsageRecord
	logger  *slog.Logger
}

// NewUsageTelemetry creates a new telemetry tracker.
func NewUsageTelemetry(logger *slog.Logger) *UsageTelemetry {
	return &UsageTelemetry{
		records: make([]UsageRecord, 0),
		logger:  logger,
	}
}

// Record adds a usage record.
func (t *UsageTelemetry) Record(rec UsageRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, rec)
	t.logger.Info("provider usage recorded",
		slog.String("provider", rec.Provider),
		slog.String("model", rec.Model),
		slog.Int("input_tokens", rec.InputTokens),
		slog.Int("output_tokens", rec.OutputTokens),
		slog.Duration("latency", rec.Latency),
		slog.Bool("success", rec.Success),
	)
}

// Records returns a snapshot of all usage records.
func (t *UsageTelemetry) Records() []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]UsageRecord, len(t.records))
	copy(out, t.records)
	return out
}

// TotalByProvider returns aggregate token counts per provider.
func (t *UsageTelemetry) TotalByProvider() map[string]Usage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	totals := make(map[string]Usage)
	for _, r := range t.records {
		u := totals[r.Provider]
		u.InputTokens += r.InputTokens
		u.OutputTokens += r.OutputTokens
		totals[r.Provider] = u
	}
	return totals
}

// ProviderRegistry manages available LLM providers, allowing selection
// by name and health checking.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	telemetry *UsageTelemetry
	logger    *slog.Logger
}

// NewProviderRegistry creates an empty registry.
func NewProviderRegistry(logger *slog.Logger) *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
		telemetry: NewUsageTelemetry(logger),
		logger:    logger,
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
	r.logger.Info("provider registered",
		slog.String("provider", p.Name()),
		slog.Any("models", p.Models()),
	)
}

// Get returns a provider by name, or an error if not found.
func (r *ProviderRegistry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}

// List returns the names of all registered providers.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Telemetry returns the usage telemetry tracker.
func (r *ProviderRegistry) Telemetry() *UsageTelemetry {
	return r.telemetry
}

// HealthCheck pings all registered providers with a minimal request
// and returns their status.
func (r *ProviderRegistry) HealthCheck(ctx context.Context) map[string]bool {
	r.mu.RLock()
	providers := make(map[string]Provider, len(r.providers))
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	results := make(map[string]bool, len(providers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, p := range providers {
		wg.Add(1)
		go func(name string, p Provider) {
			defer wg.Done()
			_, err := p.Complete(ctx, CompletionRequest{
				Model:     p.Models()[0],
				Messages:  []Message{{Role: RoleUser, Content: "ping"}},
				MaxTokens: 1,
			})
			mu.Lock()
			results[name] = err == nil
			mu.Unlock()
		}(name, p)
	}

	wg.Wait()
	return results
}

// Complete is a convenience method that selects a provider by name,
// sends the request, and records usage telemetry.
func (r *ProviderRegistry) Complete(ctx context.Context, providerName string, req CompletionRequest) (CompletionResponse, error) {
	p, err := r.Get(providerName)
	if err != nil {
		return CompletionResponse{}, err
	}

	start := time.Now()
	resp, err := p.Complete(ctx, req)
	latency := time.Since(start)

	rec := UsageRecord{
		Provider:  providerName,
		Model:     req.Model,
		Latency:   latency,
		Timestamp: time.Now().UTC(),
		Success:   err == nil,
	}

	if err != nil {
		rec.ErrorMessage = err.Error()
	} else {
		rec.InputTokens = resp.Usage.InputTokens
		rec.OutputTokens = resp.Usage.OutputTokens
	}

	r.telemetry.Record(rec)

	return resp, err
}
