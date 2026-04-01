// Package models - research domain types for the auto-research loop.
//
// These types support Phase 3: Adaptive Intelligence, enabling tracked topics,
// source trust scoring, contradiction detection, and research brief generation.
package models

import "time"

// TopicStatus represents the lifecycle state of a tracked topic.
type TopicStatus string

const (
	TopicStatusActive   TopicStatus = "active"
	TopicStatusPaused   TopicStatus = "paused"
	TopicStatusArchived TopicStatus = "archived"
)

// ContradictionSeverity indicates how serious a contradiction is.
type ContradictionSeverity string

const (
	ContradictionSeverityLow    ContradictionSeverity = "low"
	ContradictionSeverityMedium ContradictionSeverity = "medium"
	ContradictionSeverityHigh   ContradictionSeverity = "high"
)

// TrackedTopic represents a topic that the research service monitors
// on a recurring schedule, fetching from configured sources and generating
// briefs with contradiction analysis.
type TrackedTopic struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	WorkspaceID string        `json:"workspace_id"`
	Sources     []TopicSource `json:"sources"`
	Schedule    string        `json:"schedule"` // cron expression
	LastChecked time.Time     `json:"last_checked"`
	Status      TopicStatus   `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
}

// TopicSource is a single information source attached to a tracked topic.
type TopicSource struct {
	ID             string        `json:"id"`
	URL            string        `json:"url"`
	Name           string        `json:"name"`
	TrustScore     float64       `json:"trust_score"` // 0.0 to 1.0
	LastFetched    time.Time     `json:"last_fetched"`
	FetchFrequency time.Duration `json:"fetch_frequency"`
	IsActive       bool          `json:"is_active"`
}

// ResearchBrief is a synthesized report produced after checking a tracked
// topic's sources. It includes confidence scoring, source evidence, and
// detected contradictions.
type ResearchBrief struct {
	ID                string          `json:"id"`
	TopicID           string          `json:"topic_id"`
	Title             string          `json:"title"`
	Summary           string          `json:"summary"`
	Confidence        float64         `json:"confidence"` // 0.0 to 1.0
	Freshness         time.Duration   `json:"freshness"`
	Sources           []BriefSource   `json:"sources"`
	Contradictions    []Contradiction `json:"contradictions"`
	ActionSuggestions []string        `json:"action_suggestions"`
	CreatedAt         time.Time       `json:"created_at"`
}

// BriefSource links a research brief to a specific piece of evidence
// from a fetched source.
type BriefSource struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Excerpt    string    `json:"excerpt"`
	TrustScore float64   `json:"trust_score"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// Contradiction represents a conflict between claims from different sources.
type Contradiction struct {
	ClaimA   string                `json:"claim_a"`
	ClaimB   string                `json:"claim_b"`
	SourceA  string                `json:"source_a"`
	SourceB  string                `json:"source_b"`
	Severity ContradictionSeverity `json:"severity"`
}

// SourceTrustFactors captures the signals used to compute a source's
// trust score.
type SourceTrustFactors struct {
	Domain           string  `json:"domain"`
	Age              int     `json:"age"` // days since domain first seen
	CitationCount    int     `json:"citation_count"`
	ConsistencyScore float64 `json:"consistency_score"` // 0.0 to 1.0
}
