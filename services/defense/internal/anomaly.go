// Package internal implements improved anomaly scoring for the IronGolem OS
// Defense service. The AnomalyEngine uses multiple detection strategies to
// identify volume spikes, unusual access patterns, behavioral divergence,
// and cross-tenant access attempts.
package internal

import (
	"fmt"
	"sync"
	"time"
)

// AnomalyType identifies the category of anomaly.
type AnomalyType string

const (
	AnomalyVolume      AnomalyType = "volume"
	AnomalyPattern     AnomalyType = "pattern"
	AnomalyBehavior    AnomalyType = "behavior"
	AnomalyCrossTenant AnomalyType = "cross_tenant"
)

// AnomalyRecommendation indicates the suggested response to an anomaly.
type AnomalyRecommendation string

const (
	RecommendMonitor    AnomalyRecommendation = "monitor"
	RecommendAlert      AnomalyRecommendation = "alert"
	RecommendQuarantine AnomalyRecommendation = "quarantine"
)

// AnomalyFactor describes a single contributing factor to the overall score.
type AnomalyFactor struct {
	Type        AnomalyType `json:"type"`
	Score       float64     `json:"score"`
	Description string      `json:"description"`
}

// AnomalyScore is the aggregated anomaly assessment.
type AnomalyScore struct {
	Score          float64               `json:"score"`
	Contributors   []AnomalyFactor       `json:"contributors"`
	Recommendation AnomalyRecommendation `json:"recommendation"`
}

// AnomalyThresholds configures per-type thresholds for the anomaly engine.
type AnomalyThresholds struct {
	// VolumeMaxPerWindow is the maximum requests in the baseline window
	// before a volume anomaly is raised.
	VolumeMaxPerWindow int

	// PatternTimeStart and PatternTimeEnd define "normal" operating hours
	// (0-23). Access outside this range scores higher.
	PatternTimeStart int
	PatternTimeEnd   int

	// BehaviorDivergenceThreshold is the fraction of novel actions (0-1)
	// above which a behavior anomaly is raised.
	BehaviorDivergenceThreshold float64

	// CrossTenantScore is the fixed score assigned to any cross-tenant
	// access attempt.
	CrossTenantScore float64
}

func (t *AnomalyThresholds) applyDefaults() {
	if t.VolumeMaxPerWindow <= 0 {
		t.VolumeMaxPerWindow = 100
	}
	if t.PatternTimeStart == 0 && t.PatternTimeEnd == 0 {
		t.PatternTimeStart = 6
		t.PatternTimeEnd = 22
	}
	if t.BehaviorDivergenceThreshold <= 0 {
		t.BehaviorDivergenceThreshold = 0.3
	}
	if t.CrossTenantScore <= 0 {
		t.CrossTenantScore = 1.0
	}
}

// AnomalyRequest contains the data to evaluate for anomalies.
type AnomalyRequest struct {
	TenantID       string    `json:"tenant_id"`
	UserID         string    `json:"user_id"`
	SourceIP       string    `json:"source_ip,omitempty"`
	Action         string    `json:"action"`
	TargetResource string    `json:"target_resource,omitempty"`
	TargetTenantID string    `json:"target_tenant_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// AnomalyEngine evaluates requests for anomalous behavior using multiple
// detection strategies and a sliding window baseline.
type AnomalyEngine struct {
	mu         sync.Mutex
	thresholds AnomalyThresholds
	window     time.Duration

	// Sliding window data for volume tracking (key -> timestamps).
	volumeBuckets map[string][]time.Time

	// Known actions per user for behavior baseline.
	knownActions map[string]map[string]bool
}

// NewAnomalyEngine creates an AnomalyEngine with the given window and
// thresholds.
func NewAnomalyEngine(window time.Duration, thresholds AnomalyThresholds) *AnomalyEngine {
	thresholds.applyDefaults()
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &AnomalyEngine{
		thresholds:    thresholds,
		window:        window,
		volumeBuckets: make(map[string][]time.Time),
		knownActions:  make(map[string]map[string]bool),
	}
}

// Evaluate runs all anomaly detection strategies on the request and returns
// an aggregated score.
func (e *AnomalyEngine) Evaluate(req AnomalyRequest) AnomalyScore {
	var factors []AnomalyFactor

	if f, ok := e.checkVolume(req); ok {
		factors = append(factors, f)
	}
	if f, ok := e.checkPattern(req); ok {
		factors = append(factors, f)
	}
	if f, ok := e.checkBehavior(req); ok {
		factors = append(factors, f)
	}
	if f, ok := e.checkCrossTenant(req); ok {
		factors = append(factors, f)
	}

	// Aggregate: take the max score from all factors.
	var maxScore float64
	for _, f := range factors {
		if f.Score > maxScore {
			maxScore = f.Score
		}
	}

	rec := RecommendMonitor
	switch {
	case maxScore >= 0.8:
		rec = RecommendQuarantine
	case maxScore >= 0.5:
		rec = RecommendAlert
	}

	return AnomalyScore{
		Score:          maxScore,
		Contributors:   factors,
		Recommendation: rec,
	}
}

// checkVolume detects request rate anomalies using a sliding window.
func (e *AnomalyEngine) checkVolume(req AnomalyRequest) (AnomalyFactor, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := req.TenantID + ":" + req.UserID
	now := req.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.Add(-e.window)

	// Prune old entries and add current.
	times := e.volumeBuckets[key]
	pruned := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	pruned = append(pruned, now)
	e.volumeBuckets[key] = pruned

	count := len(pruned)
	if count <= e.thresholds.VolumeMaxPerWindow {
		return AnomalyFactor{}, false
	}

	ratio := float64(count) / float64(e.thresholds.VolumeMaxPerWindow)
	score := 0.5 + (ratio-1.0)*0.25
	if score > 1.0 {
		score = 1.0
	}

	return AnomalyFactor{
		Type:        AnomalyVolume,
		Score:       score,
		Description: fmt.Sprintf("request volume %d exceeds baseline %d in window", count, e.thresholds.VolumeMaxPerWindow),
	}, true
}

// checkPattern detects unusual access patterns based on time of day.
func (e *AnomalyEngine) checkPattern(req AnomalyRequest) (AnomalyFactor, bool) {
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	hour := ts.Hour()
	start := e.thresholds.PatternTimeStart
	end := e.thresholds.PatternTimeEnd

	if hour >= start && hour < end {
		return AnomalyFactor{}, false
	}

	// Outside normal hours: score based on distance from boundaries.
	var distance int
	if hour < start {
		distance = start - hour
	} else {
		distance = hour - end
	}
	score := 0.3 + float64(distance)*0.1
	if score > 0.8 {
		score = 0.8
	}

	return AnomalyFactor{
		Type:        AnomalyPattern,
		Score:       score,
		Description: fmt.Sprintf("access at unusual hour %d (normal: %d-%d)", hour, start, end),
	}, true
}

// checkBehavior detects actions that diverge from the user's learned norms.
func (e *AnomalyEngine) checkBehavior(req AnomalyRequest) (AnomalyFactor, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := req.TenantID + ":" + req.UserID
	known, exists := e.knownActions[key]

	if !exists {
		// First request from this user -- learn the action.
		e.knownActions[key] = map[string]bool{req.Action: true}
		return AnomalyFactor{}, false
	}

	if known[req.Action] {
		return AnomalyFactor{}, false
	}

	// Novel action -- calculate divergence.
	novelCount := 1
	totalActions := len(known) + novelCount
	divergence := float64(novelCount) / float64(totalActions)

	// Learn the action for future reference.
	known[req.Action] = true

	if divergence < e.thresholds.BehaviorDivergenceThreshold {
		return AnomalyFactor{}, false
	}

	score := 0.4 + divergence*0.5
	if score > 0.9 {
		score = 0.9
	}

	return AnomalyFactor{
		Type:        AnomalyBehavior,
		Score:       score,
		Description: fmt.Sprintf("novel action %q diverges from learned norms (divergence: %.2f)", req.Action, divergence),
	}, true
}

// checkCrossTenant detects any attempt to access resources in a different
// tenant's scope.
func (e *AnomalyEngine) checkCrossTenant(req AnomalyRequest) (AnomalyFactor, bool) {
	if req.TargetTenantID == "" || req.TargetTenantID == req.TenantID {
		return AnomalyFactor{}, false
	}

	return AnomalyFactor{
		Type:        AnomalyCrossTenant,
		Score:       e.thresholds.CrossTenantScore,
		Description: fmt.Sprintf("cross-tenant access attempt: %s -> %s", req.TenantID, req.TargetTenantID),
	}, true
}

// ResetBaseline clears all sliding window and behavior data.
func (e *AnomalyEngine) ResetBaseline() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.volumeBuckets = make(map[string][]time.Time)
	e.knownActions = make(map[string]map[string]bool)
}
