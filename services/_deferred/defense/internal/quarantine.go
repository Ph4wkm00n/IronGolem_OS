// Package internal implements the quarantine subsystem for the IronGolem OS
// Defense service. It manages isolation of suspicious actions, content,
// connectors, and agents with support for release, escalation, and
// automatic expiration.
package internal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// QuarantineType describes what kind of entity is quarantined.
type QuarantineType string

const (
	QuarantineTypeAction    QuarantineType = "action"
	QuarantineTypeContent   QuarantineType = "content"
	QuarantineTypeConnector QuarantineType = "connector"
	QuarantineTypeAgent     QuarantineType = "agent"
)

// QuarantineSeverity indicates how dangerous the quarantined entity is.
type QuarantineSeverity string

const (
	SeverityLow      QuarantineSeverity = "low"
	SeverityMedium   QuarantineSeverity = "medium"
	SeverityHigh     QuarantineSeverity = "high"
	SeverityCritical QuarantineSeverity = "critical"
)

// QuarantineStatus tracks the lifecycle of a quarantined item.
type QuarantineStatus string

const (
	StatusQuarantined QuarantineStatus = "quarantined"
	StatusReleased    QuarantineStatus = "released"
	StatusEscalated   QuarantineStatus = "escalated"
	StatusExpired     QuarantineStatus = "expired"
)

// QuarantineItem represents an entity placed in quarantine for review.
type QuarantineItem struct {
	ID            string             `json:"id"`
	Type          QuarantineType     `json:"type"`
	Target        string             `json:"target"`
	Reason        string             `json:"reason"`
	Severity      QuarantineSeverity `json:"severity"`
	DetectedBy    string             `json:"detected_by"`
	Status        QuarantineStatus   `json:"status"`
	QuarantinedAt time.Time          `json:"quarantined_at"`
	ReleasedAt    *time.Time         `json:"released_at,omitempty"`
	ReviewedBy    string             `json:"reviewed_by,omitempty"`
	Evidence      json.RawMessage    `json:"evidence,omitempty"`
	TenantID      string             `json:"tenant_id,omitempty"`
	ReleaseReason string             `json:"release_reason,omitempty"`
	TTL           time.Duration      `json:"ttl_ns,omitempty"`
}

// QuarantineStore defines persistence operations for quarantine items.
type QuarantineStore interface {
	Save(item QuarantineItem) error
	Get(id string) (QuarantineItem, bool)
	List() []QuarantineItem
	Update(item QuarantineItem) error
	Delete(id string) error
}

// InMemoryQuarantineStore is a thread-safe in-memory QuarantineStore.
type InMemoryQuarantineStore struct {
	mu    sync.RWMutex
	items map[string]QuarantineItem
}

// NewInMemoryQuarantineStore creates a new in-memory quarantine store.
func NewInMemoryQuarantineStore() *InMemoryQuarantineStore {
	return &InMemoryQuarantineStore{
		items: make(map[string]QuarantineItem),
	}
}

// Save persists a quarantine item.
func (s *InMemoryQuarantineStore) Save(item QuarantineItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

// Get retrieves a quarantine item by ID.
func (s *InMemoryQuarantineStore) Get(id string) (QuarantineItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

// List returns all quarantine items.
func (s *InMemoryQuarantineStore) List() []QuarantineItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]QuarantineItem, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result
}

// Update replaces an existing quarantine item.
func (s *InMemoryQuarantineStore) Update(item QuarantineItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[item.ID]; !ok {
		return fmt.Errorf("quarantine item not found: %s", item.ID)
	}
	s.items[item.ID] = item
	return nil
}

// Delete removes a quarantine item by ID.
func (s *InMemoryQuarantineStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

// QuarantinePolicyRule defines when auto-quarantine should trigger.
type QuarantinePolicyRule struct {
	// MinScore is the minimum threat score to trigger this rule.
	MinScore float64 `json:"min_score"`
	// TargetType is the type of entity to quarantine.
	TargetType QuarantineType `json:"target_type"`
	// Severity is the severity to assign to items matching this rule.
	Severity QuarantineSeverity `json:"severity"`
	// TTL is how long the item stays quarantined before auto-expiring.
	TTL time.Duration `json:"ttl_ns"`
}

// QuarantinePolicy holds rules for automatic quarantine decisions.
type QuarantinePolicy struct {
	Rules []QuarantinePolicyRule
}

// DefaultQuarantinePolicy returns a policy with sensible defaults.
func DefaultQuarantinePolicy() *QuarantinePolicy {
	return &QuarantinePolicy{
		Rules: []QuarantinePolicyRule{
			{MinScore: 0.9, TargetType: QuarantineTypeAgent, Severity: SeverityCritical, TTL: 24 * time.Hour},
			{MinScore: 0.9, TargetType: QuarantineTypeConnector, Severity: SeverityCritical, TTL: 24 * time.Hour},
			{MinScore: 0.7, TargetType: QuarantineTypeAction, Severity: SeverityHigh, TTL: 12 * time.Hour},
			{MinScore: 0.5, TargetType: QuarantineTypeContent, Severity: SeverityMedium, TTL: 6 * time.Hour},
		},
	}
}

// Evaluate checks a threat score and returns the matching rule, if any.
func (p *QuarantinePolicy) Evaluate(score float64, targetType QuarantineType) (QuarantinePolicyRule, bool) {
	for _, rule := range p.Rules {
		if score >= rule.MinScore && rule.TargetType == targetType {
			return rule, true
		}
	}
	return QuarantinePolicyRule{}, false
}

// IsolationAction describes a concrete action taken when an entity is quarantined.
type IsolationAction struct {
	Type        string `json:"type"`        // "disable_connector", "block_agent", "hold_message"
	Target      string `json:"target"`      // the ID of the affected entity
	Description string `json:"description"` // human-readable explanation
}

// IsolationActions returns the set of actions to take when quarantining an item.
func IsolationActions(item QuarantineItem) []IsolationAction {
	var actions []IsolationAction

	switch item.Type {
	case QuarantineTypeConnector:
		actions = append(actions, IsolationAction{
			Type:        "disable_connector",
			Target:      item.Target,
			Description: fmt.Sprintf("Disable connector %s due to quarantine: %s", item.Target, item.Reason),
		})
	case QuarantineTypeAgent:
		actions = append(actions, IsolationAction{
			Type:        "block_agent",
			Target:      item.Target,
			Description: fmt.Sprintf("Block agent %s from executing tasks: %s", item.Target, item.Reason),
		})
	case QuarantineTypeContent, QuarantineTypeAction:
		actions = append(actions, IsolationAction{
			Type:        "hold_message",
			Target:      item.Target,
			Description: fmt.Sprintf("Hold message/action %s pending review: %s", item.Target, item.Reason),
		})
	}

	return actions
}

// QuarantineManager orchestrates quarantine operations including creation,
// release, escalation, and automatic expiration.
type QuarantineManager struct {
	logger *slog.Logger
	store  QuarantineStore
	policy *QuarantinePolicy
}

// NewQuarantineManager creates a QuarantineManager with the given dependencies.
func NewQuarantineManager(logger *slog.Logger, store QuarantineStore, policy *QuarantinePolicy) *QuarantineManager {
	if policy == nil {
		policy = DefaultQuarantinePolicy()
	}
	return &QuarantineManager{
		logger: logger,
		store:  store,
		policy: policy,
	}
}

// Quarantine places an entity into quarantine and returns the isolation
// actions that should be taken.
func (m *QuarantineManager) Quarantine(item QuarantineItem) ([]IsolationAction, error) {
	item.Status = StatusQuarantined
	item.QuarantinedAt = time.Now().UTC()
	if item.ID == "" {
		item.ID = generateID()
	}

	if err := m.store.Save(item); err != nil {
		return nil, fmt.Errorf("saving quarantine item: %w", err)
	}

	actions := IsolationActions(item)

	m.logger.Warn("entity quarantined",
		slog.String("id", item.ID),
		slog.String("type", string(item.Type)),
		slog.String("target", item.Target),
		slog.String("severity", string(item.Severity)),
		slog.String("reason", item.Reason),
	)

	return actions, nil
}

// Release removes an entity from quarantine with a reason and reviewer.
func (m *QuarantineManager) Release(id, reviewer, reason string) error {
	item, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("quarantine item not found: %s", id)
	}

	now := time.Now().UTC()
	item.Status = StatusReleased
	item.ReleasedAt = &now
	item.ReviewedBy = reviewer
	item.ReleaseReason = reason

	if err := m.store.Update(item); err != nil {
		return fmt.Errorf("updating quarantine item: %w", err)
	}

	m.logger.Info("quarantine item released",
		slog.String("id", id),
		slog.String("reviewer", reviewer),
		slog.String("reason", reason),
	)

	return nil
}

// Escalate marks a quarantine item as escalated, requiring admin attention.
func (m *QuarantineManager) Escalate(id, reason string) error {
	item, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("quarantine item not found: %s", id)
	}

	item.Status = StatusEscalated

	if err := m.store.Update(item); err != nil {
		return fmt.Errorf("updating quarantine item: %w", err)
	}

	m.logger.Warn("quarantine item escalated to admin",
		slog.String("id", id),
		slog.String("target", item.Target),
		slog.String("reason", reason),
	)

	return nil
}

// ExpireStale checks all quarantined items and expires those past their TTL.
func (m *QuarantineManager) ExpireStale() int {
	now := time.Now().UTC()
	expired := 0

	for _, item := range m.store.List() {
		if item.Status != StatusQuarantined {
			continue
		}
		if item.TTL <= 0 {
			continue
		}
		if now.After(item.QuarantinedAt.Add(item.TTL)) {
			item.Status = StatusExpired
			releasedAt := now
			item.ReleasedAt = &releasedAt
			if err := m.store.Update(item); err != nil {
				m.logger.Error("failed to expire quarantine item",
					slog.String("id", item.ID),
					slog.String("error", err.Error()),
				)
				continue
			}
			expired++
			m.logger.Info("quarantine item expired",
				slog.String("id", item.ID),
				slog.String("target", item.Target),
			)
		}
	}

	return expired
}

// List returns all quarantine items.
func (m *QuarantineManager) List() []QuarantineItem {
	return m.store.List()
}

// Get returns a single quarantine item by ID.
func (m *QuarantineManager) Get(id string) (QuarantineItem, bool) {
	return m.store.Get(id)
}

// AutoQuarantine evaluates a threat assessment and auto-quarantines if the
// policy matches.
func (m *QuarantineManager) AutoQuarantine(target string, targetType QuarantineType, score float64, detectedBy string, evidence json.RawMessage) ([]IsolationAction, bool, error) {
	rule, matched := m.policy.Evaluate(score, targetType)
	if !matched {
		return nil, false, nil
	}

	item := QuarantineItem{
		Type:       rule.TargetType,
		Target:     target,
		Reason:     fmt.Sprintf("auto-quarantine: threat score %.2f exceeds threshold %.2f", score, rule.MinScore),
		Severity:   rule.Severity,
		DetectedBy: detectedBy,
		Evidence:   evidence,
		TTL:        rule.TTL,
	}

	actions, err := m.Quarantine(item)
	if err != nil {
		return nil, false, err
	}

	return actions, true, nil
}
