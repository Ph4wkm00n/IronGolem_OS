// Package internal implements threat detection for the IronGolem OS Defense
// service. It provides three detection mechanisms:
//
//   - PromptInjectionDetector: pattern-based detection of prompt injection attacks
//   - SSRFChecker: destination allowlist enforcement to prevent SSRF
//   - AnomalyScorer: volume-based anomaly detection for burst/abuse patterns
//
// Each detector produces a score and findings that are aggregated into a
// ThreatAssessment.
package internal

import (
	"context"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DetectorConfig holds tunable parameters for the threat detection engine.
type DetectorConfig struct {
	// InjectionThreshold is the minimum score (0-1) to flag an injection.
	InjectionThreshold float64

	// AnomalyWindow is the time window for volume-based anomaly detection.
	AnomalyWindow time.Duration

	// AnomalyMaxVolume is the maximum number of requests within the window
	// before an anomaly is flagged.
	AnomalyMaxVolume int
}

// CheckRequest is the input to the threat assessment endpoint.
type CheckRequest struct {
	Input       string `json:"input"`
	TenantID    string `json:"tenant_id"`
	UserID      string `json:"user_id,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	Destination string `json:"destination,omitempty"` // URL for SSRF checks
}

// ThreatFinding describes a single threat signal detected in the input.
type ThreatFinding struct {
	Detector    string  `json:"detector"`
	ThreatType  string  `json:"threat_type"`
	Pattern     string  `json:"pattern,omitempty"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// ThreatAssessment is the aggregated result of all detectors.
type ThreatAssessment struct {
	Safe          bool            `json:"safe"`
	Blocked       bool            `json:"blocked"`
	Score         float64         `json:"score"`
	Severity      string          `json:"severity"` // "none", "low", "medium", "high", "critical"
	PrimaryThreat string          `json:"primary_threat,omitempty"`
	Summary       string          `json:"summary"`
	Findings      []ThreatFinding `json:"findings"`
	CheckedAt     time.Time       `json:"checked_at"`
}

// BlockedAction records a request that was blocked by the defense service.
type BlockedAction struct {
	ID         string           `json:"id"`
	TenantID   string           `json:"tenant_id"`
	UserID     string           `json:"user_id,omitempty"`
	Assessment ThreatAssessment `json:"assessment"`
	Input      string           `json:"input_preview"` // truncated for safety
	BlockedAt  time.Time        `json:"blocked_at"`
}

// QuarantinedItem represents an input or entity placed in quarantine for
// review by a human administrator.
type QuarantinedItem struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Reason        string    `json:"reason"`
	SourceType    string    `json:"source_type"` // "user", "agent", "connector"
	SourceID      string    `json:"source_id"`
	QuarantinedAt time.Time `json:"quarantined_at"`
}

// ThreatDetector orchestrates all detection mechanisms and maintains
// blocked/quarantine lists.
type ThreatDetector struct {
	logger    *slog.Logger
	config    DetectorConfig
	injection *PromptInjectionDetector
	ssrf      *SSRFChecker
	anomaly   *AnomalyScorer

	mu          sync.RWMutex
	blocked     []BlockedAction
	quarantined []QuarantinedItem
}

// NewThreatDetector creates a fully initialized ThreatDetector.
func NewThreatDetector(logger *slog.Logger, config DetectorConfig) *ThreatDetector {
	if config.InjectionThreshold <= 0 {
		config.InjectionThreshold = 0.7
	}
	if config.AnomalyWindow <= 0 {
		config.AnomalyWindow = 5 * time.Minute
	}
	if config.AnomalyMaxVolume <= 0 {
		config.AnomalyMaxVolume = 100
	}

	return &ThreatDetector{
		logger:      logger,
		config:      config,
		injection:   NewPromptInjectionDetector(config.InjectionThreshold),
		ssrf:        NewSSRFChecker(),
		anomaly:     NewAnomalyScorer(config.AnomalyWindow, config.AnomalyMaxVolume),
		blocked:     make([]BlockedAction, 0),
		quarantined: make([]QuarantinedItem, 0),
	}
}

// Assess runs all detectors against the input and returns an aggregated result.
func (d *ThreatDetector) Assess(ctx context.Context, req CheckRequest) ThreatAssessment {
	var findings []ThreatFinding
	var maxScore float64

	// 1. Prompt injection detection.
	injFindings := d.injection.Detect(req.Input)
	for _, f := range injFindings {
		findings = append(findings, f)
		if f.Score > maxScore {
			maxScore = f.Score
		}
	}

	// 2. SSRF check (if a destination URL is provided).
	if req.Destination != "" {
		ssrfFindings := d.ssrf.Check(req.Destination)
		for _, f := range ssrfFindings {
			findings = append(findings, f)
			if f.Score > maxScore {
				maxScore = f.Score
			}
		}
	}

	// 3. Anomaly scoring (rate-based).
	key := req.TenantID
	if req.UserID != "" {
		key = req.TenantID + ":" + req.UserID
	}
	anomalyFindings := d.anomaly.Score(key)
	for _, f := range anomalyFindings {
		findings = append(findings, f)
		if f.Score > maxScore {
			maxScore = f.Score
		}
	}

	assessment := ThreatAssessment{
		Safe:      len(findings) == 0,
		Score:     maxScore,
		Severity:  scoreSeverity(maxScore),
		Findings:  findings,
		CheckedAt: time.Now().UTC(),
	}

	if len(findings) == 0 {
		assessment.Summary = "no threats detected"
	} else {
		// Find the highest-scoring finding as the primary.
		best := findings[0]
		for _, f := range findings[1:] {
			if f.Score > best.Score {
				best = f
			}
		}
		assessment.PrimaryThreat = best.ThreatType
		assessment.Summary = best.Description
	}

	// Block if score exceeds threshold.
	if maxScore >= d.config.InjectionThreshold {
		assessment.Blocked = true
		assessment.Safe = false

		d.mu.Lock()
		d.blocked = append(d.blocked, BlockedAction{
			ID:         generateID(),
			TenantID:   req.TenantID,
			UserID:     req.UserID,
			Assessment: assessment,
			Input:      truncate(req.Input, 200),
			BlockedAt:  time.Now().UTC(),
		})

		// Quarantine the source if score is critical.
		if maxScore >= 0.9 && req.UserID != "" {
			d.quarantined = append(d.quarantined, QuarantinedItem{
				ID:            generateID(),
				TenantID:      req.TenantID,
				Reason:        "critical threat score: " + assessment.PrimaryThreat,
				SourceType:    "user",
				SourceID:      req.UserID,
				QuarantinedAt: time.Now().UTC(),
			})
			d.logger.WarnContext(ctx, "source quarantined",
				slog.String("user_id", req.UserID),
				slog.String("threat", assessment.PrimaryThreat),
			)
		}
		d.mu.Unlock()
	}

	return assessment
}

// ListBlocked returns all blocked actions.
func (d *ThreatDetector) ListBlocked() []BlockedAction {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]BlockedAction, len(d.blocked))
	copy(result, d.blocked)
	return result
}

// ListQuarantined returns all quarantined items.
func (d *ThreatDetector) ListQuarantined() []QuarantinedItem {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]QuarantinedItem, len(d.quarantined))
	copy(result, d.quarantined)
	return result
}

// --- Prompt Injection Detector ---

// injectionPattern pairs a regex with a threat description and weight.
type injectionPattern struct {
	regex       *regexp.Regexp
	description string
	weight      float64
}

// PromptInjectionDetector scans text for known prompt injection patterns.
type PromptInjectionDetector struct {
	patterns  []injectionPattern
	threshold float64
}

// NewPromptInjectionDetector creates a detector with the built-in pattern set.
func NewPromptInjectionDetector(threshold float64) *PromptInjectionDetector {
	return &PromptInjectionDetector{
		threshold: threshold,
		patterns: []injectionPattern{
			{
				regex:       regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
				description: "attempt to override system prompt",
				weight:      0.95,
			},
			{
				regex:       regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an)\s+`),
				description: "role reassignment attack",
				weight:      0.85,
			},
			{
				regex:       regexp.MustCompile(`(?i)disregard\s+(all\s+)?(your\s+)?(rules|instructions|guidelines)`),
				description: "instruction override attempt",
				weight:      0.9,
			},
			{
				regex:       regexp.MustCompile(`(?i)system\s*:\s*`),
				description: "system prompt injection via role prefix",
				weight:      0.8,
			},
			{
				regex:       regexp.MustCompile(`(?i)\]\]\s*>\s*`),
				description: "XML/template escape attempt",
				weight:      0.75,
			},
			{
				regex:       regexp.MustCompile(`(?i)forget\s+(everything|all|what)`),
				description: "memory wipe attempt",
				weight:      0.85,
			},
			{
				regex:       regexp.MustCompile(`(?i)do\s+not\s+follow\s+(any\s+)?(safety|content)\s+(guidelines|policies|rules)`),
				description: "safety bypass attempt",
				weight:      0.95,
			},
			{
				regex:       regexp.MustCompile(`(?i)pretend\s+(you\s+)?(are|have)\s+no\s+(restrictions|limits|rules)`),
				description: "restriction removal attempt",
				weight:      0.9,
			},
			{
				regex:       regexp.MustCompile(`(?i)<\s*script\s*>`),
				description: "HTML script injection",
				weight:      0.7,
			},
			{
				regex:       regexp.MustCompile(`(?i)\{\{.*\}\}`),
				description: "template injection attempt",
				weight:      0.6,
			},
		},
	}
}

// Detect scans the input text and returns findings for each matched pattern.
func (d *PromptInjectionDetector) Detect(input string) []ThreatFinding {
	var findings []ThreatFinding

	for _, p := range d.patterns {
		if p.regex.MatchString(input) {
			findings = append(findings, ThreatFinding{
				Detector:    "prompt_injection",
				ThreatType:  "prompt_injection",
				Pattern:     p.regex.String(),
				Score:       p.weight,
				Description: p.description,
			})
		}
	}

	return findings
}

// --- SSRF Checker ---

// SSRFChecker validates destination URLs against an allowlist and blocks
// access to internal/private network ranges.
type SSRFChecker struct {
	// allowedHosts is the set of hosts that are permitted as destinations.
	// An empty set means all external hosts are allowed.
	allowedHosts map[string]bool

	// blockedCIDRs are private/internal network ranges that must never be
	// accessed by the system.
	blockedCIDRs []*net.IPNet
}

// NewSSRFChecker creates a checker with the standard blocked CIDR ranges
// for private networks.
func NewSSRFChecker() *SSRFChecker {
	blocked := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	cidrs := make([]*net.IPNet, 0, len(blocked))
	for _, cidr := range blocked {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			cidrs = append(cidrs, ipNet)
		}
	}

	return &SSRFChecker{
		allowedHosts: make(map[string]bool),
		blockedCIDRs: cidrs,
	}
}

// AddAllowedHost adds a host to the allowlist. If any hosts are allowlisted,
// only those hosts are permitted.
func (c *SSRFChecker) AddAllowedHost(host string) {
	c.allowedHosts[strings.ToLower(host)] = true
}

// Check evaluates a destination URL for SSRF risk.
func (c *SSRFChecker) Check(destination string) []ThreatFinding {
	var findings []ThreatFinding

	parsed, err := url.Parse(destination)
	if err != nil {
		findings = append(findings, ThreatFinding{
			Detector:    "ssrf",
			ThreatType:  "ssrf",
			Score:       0.8,
			Description: "malformed destination URL",
		})
		return findings
	}

	hostname := parsed.Hostname()

	// Check scheme.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		findings = append(findings, ThreatFinding{
			Detector:    "ssrf",
			ThreatType:  "ssrf",
			Score:       0.9,
			Description: "non-HTTP scheme: " + parsed.Scheme,
		})
		return findings
	}

	// Check allowlist if configured.
	if len(c.allowedHosts) > 0 && !c.allowedHosts[strings.ToLower(hostname)] {
		findings = append(findings, ThreatFinding{
			Detector:    "ssrf",
			ThreatType:  "ssrf",
			Score:       0.85,
			Description: "destination host not in allowlist: " + hostname,
		})
		return findings
	}

	// Resolve and check against blocked CIDRs.
	ip := net.ParseIP(hostname)
	if ip != nil {
		for _, cidr := range c.blockedCIDRs {
			if cidr.Contains(ip) {
				findings = append(findings, ThreatFinding{
					Detector:    "ssrf",
					ThreatType:  "ssrf",
					Score:       0.95,
					Description: "destination resolves to private/internal network: " + hostname,
				})
				return findings
			}
		}
	}

	// Check for common SSRF bypass patterns (cloud metadata endpoints).
	lower := strings.ToLower(hostname)
	suspiciousPatterns := []string{
		"metadata.google.internal",
		"169.254.169.254",
		"metadata.azure",
		"instance-data",
	}
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lower, pattern) {
			findings = append(findings, ThreatFinding{
				Detector:    "ssrf",
				ThreatType:  "ssrf",
				Score:       0.95,
				Description: "cloud metadata endpoint access attempt: " + hostname,
			})
			return findings
		}
	}

	return findings
}

// --- Anomaly Scorer ---

// AnomalyScorer tracks request volume per source key and flags anomalous
// bursts that may indicate abuse or a compromised agent.
type AnomalyScorer struct {
	mu        sync.Mutex
	window    time.Duration
	maxVolume int
	buckets   map[string][]time.Time
}

// NewAnomalyScorer creates a scorer with the given window and threshold.
func NewAnomalyScorer(window time.Duration, maxVolume int) *AnomalyScorer {
	return &AnomalyScorer{
		window:    window,
		maxVolume: maxVolume,
		buckets:   make(map[string][]time.Time),
	}
}

// Score records a request for the given key and returns findings if the
// volume exceeds the threshold.
func (a *AnomalyScorer) Score(key string) []ThreatFinding {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-a.window)

	// Prune old entries and add current.
	times := a.buckets[key]
	pruned := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	pruned = append(pruned, now)
	a.buckets[key] = pruned

	count := len(pruned)
	if count <= a.maxVolume {
		return nil
	}

	// Calculate how far over the threshold we are.
	ratio := float64(count) / float64(a.maxVolume)
	score := 0.5 + (ratio-1.0)*0.25
	if score > 1.0 {
		score = 1.0
	}

	return []ThreatFinding{
		{
			Detector:    "anomaly",
			ThreatType:  "volume_anomaly",
			Score:       score,
			Description: "request volume exceeds threshold for source key",
		},
	}
}

// --- Helpers ---

func scoreSeverity(score float64) string {
	switch {
	case score >= 0.9:
		return "critical"
	case score >= 0.7:
		return "high"
	case score >= 0.5:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
