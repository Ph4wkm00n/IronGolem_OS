// Package internal implements improved anomaly scoring for the IronGolem OS
// Defense service. The AnomalyEngine applies multiple detection strategies
// (volume, pattern, behavior, cross-tenant) and produces a composite score
// with actionable recommendations.
package internal

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// AnomalyType identifies the kind of anomaly detected.
type AnomalyType string

const (
	AnomalyVolume      AnomalyType = "volume"
	AnomalyPattern     AnomalyType = "pattern"
	AnomalyBehavior    AnomalyType = "behavior"
	AnomalyCrossTenant AnomalyType = "cross_tenant"
)

// AnomalyRecommendation indicates the suggested response.
type AnomalyRecommendation string

const (
	RecommendMonitor    AnomalyRecommendation = "monitor"
	RecommendAlert      AnomalyRecommendation = "alert"
	RecommendQuarantine AnomalyRecommendation = "quarantine"
)

// AnomalyFactor describes a single contributing factor to the anomaly score.
type AnomalyFactor struct {
	Type        AnomalyType `json:"type"`
	Score       float64     `json:"score"`
	Description string      `json:"description"`
}

// AnomalyScore is the composite result of the anomaly engine.
type AnomalyScore struct {
	// Score is the overall anomaly score in the range [0, 1].
	Score float64 `json:"score"`

	// Contributors lists the individual anomaly factors.
	Contributors []AnomalyFactor `json:"contributors"`

	// Recommendation is the suggested action based on the score.
	Recommendation AnomalyRecommendation `json:"recommendation"`

	// EvaluatedAt is when the scoring was performed.
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// AnomalyThresholds configures the scoring thresholds for each anomaly type.
type AnomalyThresholds struct {
	// VolumeMaxPerWindow is the maximum request count within the baseline window.
	VolumeMaxPerWindow int

	// LatencyStdDevMultiplier flags if latency deviates by this many standard deviations.
	LatencyStdDevMultiplier float64

	// UnusualHourStart and UnusualHourEnd define the off-hours window (UTC).
	UnusualHourStart int
	UnusualHourEnd   int

	// BehaviorDeviationThreshold is the maximum allowed deviation from learned norms (0-1).
	BehaviorDeviationThreshold float64

	// AlertThreshold is the score above which an alert is recommended.
	AlertThreshold float64

	// QuarantineThreshold is the score above which quarantine is recommended.
	QuarantineThreshold float64
}

// DefaultAnomalyThresholds returns sensible defaults.
func DefaultAnomalyThresholds() AnomalyThresholds {
	return AnomalyThresholds{
		VolumeMaxPerWindow:         100,
		LatencyStdDevMultiplier:    3.0,
		UnusualHourStart:           0,
		UnusualHourEnd:             6,
		BehaviorDeviationThreshold: 0.5,
		AlertThreshold:             0.6,
		QuarantineThreshold:        0.85,
	}
}

// AnomalyEngineConfig configures the anomaly engine.
type AnomalyEngineConfig struct {
	// BaselineWindow is the duration over which baseline metrics are tracked.
	BaselineWindow time.Duration

	// Thresholds per anomaly type.
	Thresholds AnomalyThresholds
}

// requestRecord tracks a single request for baseline computation.
type requestRecord struct {
	Timestamp time.Time
	SourceIP  string
	Target    string
	TenantID  string
}

// behaviorRecord tracks agent action patterns.
type behaviorRecord struct {
	Action    string
	Timestamp time.Time
}

// AnomalyEngine applies multiple anomaly detection strategies to produce
// a composite score.
type AnomalyEngine struct {
	mu     sync.RWMutex
	config AnomalyEngineConfig

	// Sliding window data keyed by source identifier.
	requestHistory  map[string][]requestRecord
	behaviorHistory map[string][]behaviorRecord

	// Learned baselines: average request count per window per source.
	baselineVolume map[string]float64

	// Learned behavior norms: action frequency distribution per agent.
	behaviorNorms map[string]map[string]float64
}

// NewAnomalyEngine creates an AnomalyEngine with the given configuration.
func NewAnomalyEngine(config AnomalyEngineConfig) *AnomalyEngine {
	if config.BaselineWindow <= 0 {
		config.BaselineWindow = 5 * time.Minute
	}
	if config.Thresholds.VolumeMaxPerWindow <= 0 {
		config.Thresholds = DefaultAnomalyThresholds()
	}
	if config.Thresholds.AlertThreshold <= 0 {
		config.Thresholds.AlertThreshold = 0.6
	}
	if config.Thresholds.QuarantineThreshold <= 0 {
		config.Thresholds.QuarantineThreshold = 0.85
	}

	return &AnomalyEngine{
		config:          config,
		requestHistory:  make(map[string][]requestRecord),
		behaviorHistory: make(map[string][]behaviorRecord),
		baselineVolume:  make(map[string]float64),
		behaviorNorms:   make(map[string]map[string]float64),
	}
}

// AnomalyRequest contains the information needed to score a request.
type AnomalyRequest struct {
	SourceKey        string
	SourceIP         string
	Target           string
	TenantID         string
	AgentID          string
	Action           string
	AccessedTenantID string // non-empty if accessing another tenant's data
}

// RecordAndScore records a request and evaluates all anomaly strategies,
// returning a composite score.
func (e *AnomalyEngine) RecordAndScore(req AnomalyRequest) AnomalyScore {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-e.config.BaselineWindow)

	// Record the request.
	e.requestHistory[req.SourceKey] = append(e.requestHistory[req.SourceKey], requestRecord{
		Timestamp: now,
		SourceIP:  req.SourceIP,
		Target:    req.Target,
		TenantID:  req.TenantID,
	})

	// Record behavior if an agent action is provided.
	if req.AgentID != "" && req.Action != "" {
		e.behaviorHistory[req.AgentID] = append(e.behaviorHistory[req.AgentID], behaviorRecord{
			Action:    req.Action,
			Timestamp: now,
		})
	}

	// Prune old entries.
	e.pruneHistory(req.SourceKey, cutoff)
	if req.AgentID != "" {
		e.pruneBehavior(req.AgentID, cutoff)
	}

	var factors []AnomalyFactor

	// 1. Volume anomaly.
	if f := e.scoreVolume(req.SourceKey); f != nil {
		factors = append(factors, *f)
	}

	// 2. Pattern anomaly (time-of-day, unusual source).
	if f := e.scorePattern(now, req.SourceIP); f != nil {
		factors = append(factors, *f)
	}

	// 3. Behavior anomaly (agent action divergence).
	if req.AgentID != "" && req.Action != "" {
		if f := e.scoreBehavior(req.AgentID, req.Action); f != nil {
			factors = append(factors, *f)
		}
	}

	// 4. Cross-tenant anomaly.
	if req.AccessedTenantID != "" && req.AccessedTenantID != req.TenantID {
		factors = append(factors, AnomalyFactor{
			Type:        AnomalyCrossTenant,
			Score:       1.0,
			Description: fmt.Sprintf("cross-tenant access attempt: %s accessing %s", req.TenantID, req.AccessedTenantID),
		})
	}

	// Compute composite score (max of all factors).
	var maxScore float64
	for _, f := range factors {
		if f.Score > maxScore {
			maxScore = f.Score
		}
	}

	recommendation := RecommendMonitor
	if maxScore >= e.config.Thresholds.QuarantineThreshold {
		recommendation = RecommendQuarantine
	} else if maxScore >= e.config.Thresholds.AlertThreshold {
		recommendation = RecommendAlert
	}

	return AnomalyScore{
		Score:          maxScore,
		Contributors:   factors,
		Recommendation: recommendation,
		EvaluatedAt:    now,
	}
}

// SetBaseline manually sets the baseline volume for a source key.
func (e *AnomalyEngine) SetBaseline(sourceKey string, avgVolume float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.baselineVolume[sourceKey] = avgVolume
}

// SetBehaviorNorm sets the learned behavior distribution for an agent.
// The norms map action names to their expected frequency proportion (0-1).
func (e *AnomalyEngine) SetBehaviorNorm(agentID string, norms map[string]float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.behaviorNorms[agentID] = norms
}

// scoreVolume checks if the current request volume exceeds the baseline.
func (e *AnomalyEngine) scoreVolume(sourceKey string) *AnomalyFactor {
	records := e.requestHistory[sourceKey]
	count := len(records)

	baseline := e.baselineVolume[sourceKey]
	if baseline <= 0 {
		baseline = float64(e.config.Thresholds.VolumeMaxPerWindow)
	}

	if float64(count) <= baseline {
		return nil
	}

	ratio := float64(count) / baseline
	score := math.Min(1.0, 0.4+(ratio-1.0)*0.3)

	return &AnomalyFactor{
		Type:        AnomalyVolume,
		Score:       score,
		Description: fmt.Sprintf("request volume %d exceeds baseline %.0f (%.1fx)", count, baseline, ratio),
	}
}

// scorePattern checks for unusual access patterns based on time of day.
func (e *AnomalyEngine) scorePattern(now time.Time, _ string) *AnomalyFactor {
	hour := now.Hour()
	if hour >= e.config.Thresholds.UnusualHourStart && hour < e.config.Thresholds.UnusualHourEnd {
		return &AnomalyFactor{
			Type:        AnomalyPattern,
			Score:       0.5,
			Description: fmt.Sprintf("access during unusual hours (UTC hour %d)", hour),
		}
	}
	return nil
}

// scoreBehavior checks if an agent's current action deviates from learned norms.
func (e *AnomalyEngine) scoreBehavior(agentID, action string) *AnomalyFactor {
	norms, ok := e.behaviorNorms[agentID]
	if !ok || len(norms) == 0 {
		return nil
	}

	expectedFreq, known := norms[action]
	if !known {
		return &AnomalyFactor{
			Type:        AnomalyBehavior,
			Score:       0.7,
			Description: fmt.Sprintf("agent %s performed unknown action %q", agentID, action),
		}
	}

	// If the action has very low expected frequency, flag it.
	if expectedFreq < e.config.Thresholds.BehaviorDeviationThreshold {
		score := 0.3 + (e.config.Thresholds.BehaviorDeviationThreshold-expectedFreq)*0.8
		if score > 1.0 {
			score = 1.0
		}
		return &AnomalyFactor{
			Type:        AnomalyBehavior,
			Score:       score,
			Description: fmt.Sprintf("agent %s action %q has low expected frequency (%.2f)", agentID, action, expectedFreq),
		}
	}

	return nil
}

// pruneHistory removes request records older than the cutoff.
func (e *AnomalyEngine) pruneHistory(key string, cutoff time.Time) {
	records := e.requestHistory[key]
	pruned := make([]requestRecord, 0, len(records))
	for _, r := range records {
		if r.Timestamp.After(cutoff) {
			pruned = append(pruned, r)
		}
	}
	e.requestHistory[key] = pruned
}

// pruneBehavior removes behavior records older than the cutoff.
func (e *AnomalyEngine) pruneBehavior(agentID string, cutoff time.Time) {
	records := e.behaviorHistory[agentID]
	pruned := make([]behaviorRecord, 0, len(records))
	for _, r := range records {
		if r.Timestamp.After(cutoff) {
			pruned = append(pruned, r)
		}
	}
	e.behaviorHistory[agentID] = pruned
}
