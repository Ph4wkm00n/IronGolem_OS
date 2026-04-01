package internal

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ExperimentType classifies what kind of optimization is being tested.
type ExperimentType string

const (
	ExperimentTypePreference ExperimentType = "preference"
	ExperimentTypePrompt     ExperimentType = "prompt"
	ExperimentTypeProvider   ExperimentType = "provider"
)

// ExperimentStatus tracks the lifecycle of a shadow experiment.
type ExperimentStatus string

const (
	ExperimentStatusRunning   ExperimentStatus = "running"
	ExperimentStatusCompleted ExperimentStatus = "completed"
	ExperimentStatusPromoted  ExperimentStatus = "promoted"
	ExperimentStatusRejected  ExperimentStatus = "rejected"
	ExperimentStatusReverted  ExperimentStatus = "reverted"
)

// ExperimentMetrics captures performance comparison data between
// baseline and candidate approaches.
type ExperimentMetrics struct {
	// BaselineScore is the aggregate quality score for the baseline.
	BaselineScore float64 `json:"baseline_score"`

	// CandidateScore is the aggregate quality score for the candidate.
	CandidateScore float64 `json:"candidate_score"`

	// SampleCount is the number of comparisons performed.
	SampleCount int `json:"sample_count"`

	// Improvement is the percentage improvement of candidate over baseline.
	Improvement float64 `json:"improvement"`

	// Details holds experiment-type-specific metrics.
	Details map[string]any `json:"details,omitempty"`
}

// ShadowExperiment represents a controlled comparison between a
// baseline approach and a candidate change. Experiments run in
// shadow mode, meaning the candidate's results are recorded but
// not surfaced to users until promoted.
type ShadowExperiment struct {
	// ID is a unique identifier for this experiment.
	ID string `json:"id"`

	// Name is a human-readable label.
	Name string `json:"name"`

	// Description explains what is being tested.
	Description string `json:"description"`

	// Type classifies the experiment (preference, prompt, provider).
	Type ExperimentType `json:"type"`

	// Baseline describes the current approach.
	Baseline string `json:"baseline"`

	// Candidate describes the proposed change.
	Candidate string `json:"candidate"`

	// Metrics holds the comparison results.
	Metrics ExperimentMetrics `json:"metrics"`

	// Status tracks the experiment lifecycle.
	Status ExperimentStatus `json:"status"`

	// CreatedAt records when the experiment was started.
	CreatedAt time.Time `json:"created_at"`

	// CompletedAt records when the experiment finished, if applicable.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// PromotedAt records when the candidate was promoted, if applicable.
	PromotedAt *time.Time `json:"promoted_at,omitempty"`

	// RevertedAt records when a promotion was reverted, if applicable.
	RevertedAt *time.Time `json:"reverted_at,omitempty"`

	// PreviousState stores the baseline state for revert capability.
	PreviousState string `json:"previous_state,omitempty"`
}

// ShadowController manages the lifecycle of shadow-mode experiments.
// It ensures all changes are reversible and that experiments produce
// an audit trail.
type ShadowController struct {
	mu          sync.RWMutex
	experiments map[string]*ShadowExperiment
	logger      *slog.Logger
}

// NewShadowController creates a new controller.
func NewShadowController(logger *slog.Logger) *ShadowController {
	return &ShadowController{
		experiments: make(map[string]*ShadowExperiment),
		logger:      logger,
	}
}

// StartExperiment creates and registers a new shadow experiment.
func (sc *ShadowController) StartExperiment(name, description string, expType ExperimentType, baseline, candidate string) *ShadowExperiment {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp := &ShadowExperiment{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Type:        expType,
		Baseline:    baseline,
		Candidate:   candidate,
		Status:      ExperimentStatusRunning,
		CreatedAt:   time.Now().UTC(),
		Metrics: ExperimentMetrics{
			Details: make(map[string]any),
		},
	}

	sc.experiments[exp.ID] = exp

	sc.logger.Info("shadow experiment started",
		slog.String("id", exp.ID),
		slog.String("name", name),
		slog.String("type", string(expType)),
	)

	return exp
}

// StopExperiment marks an experiment as completed without promoting
// or rejecting the candidate.
func (sc *ShadowController) StopExperiment(id string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp, ok := sc.experiments[id]
	if !ok {
		return errors.New("experiment not found: " + id)
	}

	if exp.Status != ExperimentStatusRunning {
		return errors.New("experiment is not running: " + id)
	}

	now := time.Now().UTC()
	exp.Status = ExperimentStatusCompleted
	exp.CompletedAt = &now

	sc.logger.Info("shadow experiment stopped",
		slog.String("id", id),
	)

	return nil
}

// PromoteExperiment promotes the candidate, making it the active
// approach. The previous baseline is saved for revert capability.
func (sc *ShadowController) PromoteExperiment(id string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp, ok := sc.experiments[id]
	if !ok {
		return errors.New("experiment not found: " + id)
	}

	if exp.Status != ExperimentStatusRunning && exp.Status != ExperimentStatusCompleted {
		return errors.New("experiment cannot be promoted in status: " + string(exp.Status))
	}

	now := time.Now().UTC()
	exp.PreviousState = exp.Baseline
	exp.Status = ExperimentStatusPromoted
	exp.PromotedAt = &now
	if exp.CompletedAt == nil {
		exp.CompletedAt = &now
	}

	sc.logger.Info("shadow experiment promoted",
		slog.String("id", id),
		slog.String("candidate", exp.Candidate),
	)

	return nil
}

// RejectExperiment rejects the candidate, keeping the baseline.
func (sc *ShadowController) RejectExperiment(id string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp, ok := sc.experiments[id]
	if !ok {
		return errors.New("experiment not found: " + id)
	}

	if exp.Status != ExperimentStatusRunning && exp.Status != ExperimentStatusCompleted {
		return errors.New("experiment cannot be rejected in status: " + string(exp.Status))
	}

	now := time.Now().UTC()
	exp.Status = ExperimentStatusRejected
	if exp.CompletedAt == nil {
		exp.CompletedAt = &now
	}

	sc.logger.Info("shadow experiment rejected",
		slog.String("id", id),
	)

	return nil
}

// RevertExperiment undoes a promotion, restoring the previous baseline.
// This ensures all changes are reversible.
func (sc *ShadowController) RevertExperiment(id string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp, ok := sc.experiments[id]
	if !ok {
		return errors.New("experiment not found: " + id)
	}

	if exp.Status != ExperimentStatusPromoted {
		return errors.New("only promoted experiments can be reverted: " + id)
	}

	now := time.Now().UTC()
	exp.Status = ExperimentStatusReverted
	exp.RevertedAt = &now

	sc.logger.Info("shadow experiment reverted",
		slog.String("id", id),
		slog.String("restored_baseline", exp.PreviousState),
	)

	return nil
}

// CompareResults evaluates baseline vs candidate performance and
// updates the experiment metrics.
func (sc *ShadowController) CompareResults(id string, baselineScore, candidateScore float64) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	exp, ok := sc.experiments[id]
	if !ok {
		return errors.New("experiment not found: " + id)
	}

	exp.Metrics.SampleCount++
	// Running average.
	n := float64(exp.Metrics.SampleCount)
	exp.Metrics.BaselineScore = exp.Metrics.BaselineScore*(n-1)/n + baselineScore/n
	exp.Metrics.CandidateScore = exp.Metrics.CandidateScore*(n-1)/n + candidateScore/n

	if exp.Metrics.BaselineScore > 0 {
		exp.Metrics.Improvement = (exp.Metrics.CandidateScore - exp.Metrics.BaselineScore) / exp.Metrics.BaselineScore * 100
	}

	return nil
}

// GetExperiment returns an experiment by ID.
func (sc *ShadowController) GetExperiment(id string) (*ShadowExperiment, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	exp, ok := sc.experiments[id]
	return exp, ok
}

// ListExperiments returns all experiments.
func (sc *ShadowController) ListExperiments() []*ShadowExperiment {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]*ShadowExperiment, 0, len(sc.experiments))
	for _, exp := range sc.experiments {
		result = append(result, exp)
	}
	return result
}
