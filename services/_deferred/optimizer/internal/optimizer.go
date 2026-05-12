package internal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/provider"
)

// ReasoningDepth controls how much computation an agent invests in a task.
type ReasoningDepth string

const (
	DepthMinimal    ReasoningDepth = "minimal"
	DepthStandard   ReasoningDepth = "standard"
	DepthDeep       ReasoningDepth = "deep"
	DepthExhaustive ReasoningDepth = "exhaustive"
)

// PromptVariant represents an alternative version of a prompt being tested.
type PromptVariant struct {
	ID           string `json:"id"`
	BasePrompt   string `json:"base_prompt"`
	Modification string `json:"modification"`
	FullPrompt   string `json:"full_prompt"`
	CreatedAt    time.Time `json:"created_at"`
}

// ExperimentResult captures the outcome of an A/B prompt experiment.
type ExperimentResult struct {
	VariantID    string  `json:"variant_id"`
	ApprovalRate float64 `json:"approval_rate"`
	EditDistance  float64 `json:"edit_distance"`
	Latency      time.Duration `json:"latency_ns"`
	Cost         float64 `json:"cost"`
	QualityScore float64 `json:"quality_score"`
	SampleCount  int     `json:"sample_count"`
}

// ProviderBenchmark captures performance data for a single LLM provider
// on a specific task type.
type ProviderBenchmark struct {
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	Latency      time.Duration `json:"latency_ns"`
	Cost         float64       `json:"cost"`
	QualityScore float64       `json:"quality_score"`
	CacheHitRate float64       `json:"cache_hit_rate"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
}

// PromptOptimizer experiments with prompt variations to find the most
// effective phrasing for different task types. Experiments run in shadow
// mode so production traffic is never affected.
type PromptOptimizer struct {
	mu       sync.RWMutex
	variants map[string]PromptVariant
	results  map[string]ExperimentResult
	logger   *slog.Logger
}

// NewPromptOptimizer creates a new prompt optimizer.
func NewPromptOptimizer(logger *slog.Logger) *PromptOptimizer {
	return &PromptOptimizer{
		variants: make(map[string]PromptVariant),
		results:  make(map[string]ExperimentResult),
		logger:   logger,
	}
}

// CreateVariant creates a new prompt variant by applying a modification
// to a base prompt. The variant is stored for later experimentation.
func (po *PromptOptimizer) CreateVariant(basePrompt, modification string) PromptVariant {
	po.mu.Lock()
	defer po.mu.Unlock()

	variant := PromptVariant{
		ID:           generateID(),
		BasePrompt:   basePrompt,
		Modification: modification,
		FullPrompt:   basePrompt + "\n\n" + modification,
		CreatedAt:    time.Now().UTC(),
	}

	po.variants[variant.ID] = variant

	po.logger.Info("prompt variant created",
		slog.String("id", variant.ID),
		slog.String("modification", modification),
	)

	return variant
}

// RunExperiment compares a baseline prompt against a variant using the
// provided test cases. Results are recorded but not applied until a
// shadow experiment is promoted.
func (po *PromptOptimizer) RunExperiment(baseline, variant PromptVariant, testCases []string) ExperimentResult {
	po.mu.Lock()
	defer po.mu.Unlock()

	// Simulate experiment scoring based on variant characteristics.
	// In production this would run both prompts against real test cases
	// and compare output quality.
	result := ExperimentResult{
		VariantID:   variant.ID,
		SampleCount: len(testCases),
	}

	if len(testCases) == 0 {
		return result
	}

	// Compute a deterministic quality delta from the prompt hash so
	// experiments produce consistent results for the same inputs.
	h := sha256.Sum256([]byte(variant.FullPrompt))
	qualityDelta := float64(h[0]%20-10) / 100.0

	result.ApprovalRate = 0.75 + qualityDelta
	result.EditDistance = 0.15 - qualityDelta/2
	result.Latency = 500 * time.Millisecond
	result.Cost = 0.002 * float64(len(testCases))
	result.QualityScore = 0.80 + qualityDelta

	po.results[variant.ID] = result

	po.logger.Info("prompt experiment completed",
		slog.String("variant_id", variant.ID),
		slog.Float64("approval_rate", result.ApprovalRate),
		slog.Float64("quality_score", result.QualityScore),
		slog.Int("samples", result.SampleCount),
	)

	return result
}

// ListVariants returns all stored prompt variants.
func (po *PromptOptimizer) ListVariants() []PromptVariant {
	po.mu.RLock()
	defer po.mu.RUnlock()
	out := make([]PromptVariant, 0, len(po.variants))
	for _, v := range po.variants {
		out = append(out, v)
	}
	return out
}

// GetResult returns the experiment result for a variant, if available.
func (po *PromptOptimizer) GetResult(variantID string) (ExperimentResult, bool) {
	po.mu.RLock()
	defer po.mu.RUnlock()
	r, ok := po.results[variantID]
	return r, ok
}

// ProviderOptimizer compares LLM provider performance across task types
// to help the system select the best provider for each situation.
type ProviderOptimizer struct {
	registry   *provider.ProviderRegistry
	benchmarks map[string][]ProviderBenchmark // keyed by task description
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewProviderOptimizer creates a new provider optimizer.
func NewProviderOptimizer(registry *provider.ProviderRegistry, logger *slog.Logger) *ProviderOptimizer {
	return &ProviderOptimizer{
		registry:   registry,
		benchmarks: make(map[string][]ProviderBenchmark),
		logger:     logger,
	}
}

// BenchmarkProviders runs a task against multiple providers and returns
// comparative performance data. Each provider receives the same request
// and results are measured for latency, cost, and quality.
func (po *ProviderOptimizer) BenchmarkProviders(ctx context.Context, task string, providerNames []string) []ProviderBenchmark {
	po.mu.Lock()
	defer po.mu.Unlock()

	var results []ProviderBenchmark

	for _, name := range providerNames {
		p, err := po.registry.Get(name)
		if err != nil {
			po.logger.Warn("provider not found for benchmark",
				slog.String("provider", name),
				slog.String("error", err.Error()),
			)
			continue
		}

		models := p.Models()
		model := ""
		if len(models) > 0 {
			model = models[0]
		}

		req := provider.CompletionRequest{
			Model: model,
			Messages: []provider.Message{
				{Role: provider.RoleUser, Content: task},
			},
			MaxTokens:   256,
			Temperature: 0.7,
		}

		start := time.Now()
		resp, err := p.Complete(ctx, req)
		latency := time.Since(start)

		bench := ProviderBenchmark{
			Provider: name,
			Model:    model,
			Latency:  latency,
		}

		if err != nil {
			po.logger.Warn("benchmark call failed",
				slog.String("provider", name),
				slog.String("error", err.Error()),
			)
			bench.QualityScore = 0
		} else {
			bench.InputTokens = resp.Usage.InputTokens
			bench.OutputTokens = resp.Usage.OutputTokens
			// Estimate cost: $0.01 per 1K input tokens, $0.03 per 1K output tokens.
			bench.Cost = float64(resp.Usage.InputTokens)*0.00001 + float64(resp.Usage.OutputTokens)*0.00003
			// Quality heuristic: longer responses with lower latency score higher.
			if latency > 0 {
				bench.QualityScore = float64(len(resp.Content)) / float64(latency.Milliseconds()+1)
			}
		}

		results = append(results, bench)
	}

	if len(results) > 0 {
		po.benchmarks[task] = results
	}

	po.logger.Info("provider benchmark completed",
		slog.String("task", task),
		slog.Int("providers_tested", len(results)),
	)

	return results
}

// ListBenchmarks returns all stored benchmark results.
func (po *ProviderOptimizer) ListBenchmarks() map[string][]ProviderBenchmark {
	po.mu.RLock()
	defer po.mu.RUnlock()
	out := make(map[string][]ProviderBenchmark, len(po.benchmarks))
	for k, v := range po.benchmarks {
		cp := make([]ProviderBenchmark, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// ReasoningDepthController adjusts the reasoning depth per task type,
// balancing quality against latency and cost.
type ReasoningDepthController struct {
	mu       sync.RWMutex
	defaults map[string]ReasoningDepth // keyed by task type
	logger   *slog.Logger
}

// NewReasoningDepthController creates a controller with sensible defaults.
func NewReasoningDepthController(logger *slog.Logger) *ReasoningDepthController {
	return &ReasoningDepthController{
		defaults: map[string]ReasoningDepth{
			"simple_query":   DepthMinimal,
			"draft_email":    DepthStandard,
			"research":       DepthDeep,
			"code_review":    DepthDeep,
			"planning":       DepthExhaustive,
			"security_check": DepthExhaustive,
		},
		logger: logger,
	}
}

// AutoSelectDepth chooses an appropriate reasoning depth based on the
// estimated complexity of a task. Complexity is a value from 0.0 (trivial)
// to 1.0 (extremely complex).
func (rdc *ReasoningDepthController) AutoSelectDepth(taskComplexity float64) ReasoningDepth {
	switch {
	case taskComplexity < 0.25:
		return DepthMinimal
	case taskComplexity < 0.50:
		return DepthStandard
	case taskComplexity < 0.75:
		return DepthDeep
	default:
		return DepthExhaustive
	}
}

// DepthForTaskType returns the configured depth for a known task type,
// falling back to standard if unknown.
func (rdc *ReasoningDepthController) DepthForTaskType(taskType string) ReasoningDepth {
	rdc.mu.RLock()
	defer rdc.mu.RUnlock()

	if d, ok := rdc.defaults[taskType]; ok {
		return d
	}
	return DepthStandard
}

// SetDepthForTaskType overrides the default depth for a task type.
func (rdc *ReasoningDepthController) SetDepthForTaskType(taskType string, depth ReasoningDepth) {
	rdc.mu.Lock()
	defer rdc.mu.Unlock()
	rdc.defaults[taskType] = depth

	rdc.logger.Info("reasoning depth updated",
		slog.String("task_type", taskType),
		slog.String("depth", string(depth)),
	)
}

// DepthToMaxTokens maps a reasoning depth to a suggested max token limit,
// useful for configuring provider requests.
func DepthToMaxTokens(depth ReasoningDepth) int {
	switch depth {
	case DepthMinimal:
		return 256
	case DepthStandard:
		return 1024
	case DepthDeep:
		return 4096
	case DepthExhaustive:
		return 8192
	default:
		return 1024
	}
}

// DepthToTemperature maps a reasoning depth to a suggested temperature
// setting. Deeper reasoning uses lower temperature for more focused output.
func DepthToTemperature(depth ReasoningDepth) float64 {
	switch depth {
	case DepthMinimal:
		return 0.8
	case DepthStandard:
		return 0.7
	case DepthDeep:
		return 0.5
	case DepthExhaustive:
		return 0.3
	default:
		return 0.7
	}
}

// complexityDescription returns a human-readable label for a complexity score.
func complexityDescription(c float64) string {
	switch {
	case c < 0.25:
		return "trivial"
	case c < 0.50:
		return "moderate"
	case c < 0.75:
		return "complex"
	default:
		return "highly complex"
	}
}

// FormatBenchmarkSummary produces a human-readable summary of benchmark results.
func FormatBenchmarkSummary(benchmarks []ProviderBenchmark) string {
	if len(benchmarks) == 0 {
		return "no benchmarks available"
	}

	best := benchmarks[0]
	for _, b := range benchmarks[1:] {
		if b.QualityScore > best.QualityScore {
			best = b
		}
	}

	return fmt.Sprintf("best provider: %s (model: %s, quality: %.2f, latency: %s)",
		best.Provider, best.Model, best.QualityScore, best.Latency)
}
