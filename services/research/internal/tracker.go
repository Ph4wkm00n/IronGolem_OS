// Package internal implements the research service's core logic:
// topic tracking, source fetching, content analysis, and scheduling.
package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// TopicStore is the persistence interface for tracked topics and briefs.
// Implementations may use in-memory storage, SQLite, or PostgreSQL
// depending on the deployment mode.
type TopicStore interface {
	// AddTopic persists a new tracked topic.
	AddTopic(ctx context.Context, topic models.TrackedTopic) error
	// GetTopic retrieves a topic by ID.
	GetTopic(ctx context.Context, id string) (models.TrackedTopic, error)
	// ListTopics returns all topics for a workspace. Pass empty string for all.
	ListTopics(ctx context.Context, workspaceID string) ([]models.TrackedTopic, error)
	// UpdateTopic replaces a topic's data.
	UpdateTopic(ctx context.Context, topic models.TrackedTopic) error
	// RemoveTopic deletes a topic by ID.
	RemoveTopic(ctx context.Context, id string) error

	// AddBrief persists a research brief.
	AddBrief(ctx context.Context, brief models.ResearchBrief) error
	// GetBrief retrieves a brief by ID.
	GetBrief(ctx context.Context, id string) (models.ResearchBrief, error)
	// ListBriefs returns recent briefs, optionally filtered by topic.
	ListBriefs(ctx context.Context, topicID string, limit int) ([]models.ResearchBrief, error)
	// ListContradictions returns all contradictions across recent briefs.
	ListContradictions(ctx context.Context, limit int) ([]models.Contradiction, error)
}

// TopicTracker manages tracked topics per workspace, providing CRUD
// operations and schedule-based update checking.
type TopicTracker struct {
	store  TopicStore
	logger *slog.Logger
}

// NewTopicTracker creates a tracker backed by the given store.
func NewTopicTracker(store TopicStore, logger *slog.Logger) *TopicTracker {
	return &TopicTracker{
		store:  store,
		logger: logger,
	}
}

// AddTopic registers a new tracked topic. It assigns an ID, sets
// defaults, and persists it to the store.
func (t *TopicTracker) AddTopic(ctx context.Context, topic models.TrackedTopic) (models.TrackedTopic, error) {
	if topic.Name == "" {
		return models.TrackedTopic{}, fmt.Errorf("topic name is required")
	}
	if topic.ID == "" {
		topic.ID = generateID()
	}
	if topic.Status == "" {
		topic.Status = models.TopicStatusActive
	}
	if topic.Schedule == "" {
		topic.Schedule = "0 */6 * * *" // default: every 6 hours
	}
	topic.CreatedAt = time.Now().UTC()

	if err := t.store.AddTopic(ctx, topic); err != nil {
		return models.TrackedTopic{}, fmt.Errorf("storing topic: %w", err)
	}

	t.logger.InfoContext(ctx, "topic added",
		slog.String("id", topic.ID),
		slog.String("name", topic.Name),
	)
	return topic, nil
}

// RemoveTopic deletes a tracked topic by ID.
func (t *TopicTracker) RemoveTopic(ctx context.Context, id string) error {
	if err := t.store.RemoveTopic(ctx, id); err != nil {
		return fmt.Errorf("removing topic %s: %w", id, err)
	}
	t.logger.InfoContext(ctx, "topic removed", slog.String("id", id))
	return nil
}

// PauseTopic sets a topic's status to paused.
func (t *TopicTracker) PauseTopic(ctx context.Context, id string) error {
	topic, err := t.store.GetTopic(ctx, id)
	if err != nil {
		return fmt.Errorf("getting topic %s: %w", id, err)
	}
	topic.Status = models.TopicStatusPaused
	if err := t.store.UpdateTopic(ctx, topic); err != nil {
		return fmt.Errorf("updating topic %s: %w", id, err)
	}
	t.logger.InfoContext(ctx, "topic paused", slog.String("id", id))
	return nil
}

// ListTopics returns all topics for a workspace.
func (t *TopicTracker) ListTopics(ctx context.Context, workspaceID string) ([]models.TrackedTopic, error) {
	return t.store.ListTopics(ctx, workspaceID)
}

// GetTopic retrieves a single topic by ID.
func (t *TopicTracker) GetTopic(ctx context.Context, id string) (models.TrackedTopic, error) {
	return t.store.GetTopic(ctx, id)
}

// CheckForUpdates returns all active topics whose schedule indicates they
// are due for a check (last checked time + schedule interval has elapsed).
func (t *TopicTracker) CheckForUpdates(ctx context.Context) ([]models.TrackedTopic, error) {
	topics, err := t.store.ListTopics(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}

	var due []models.TrackedTopic
	now := time.Now().UTC()

	for _, topic := range topics {
		if topic.Status != models.TopicStatusActive {
			continue
		}
		interval := scheduleToInterval(topic.Schedule)
		if now.Sub(topic.LastChecked) >= interval {
			due = append(due, topic)
		}
	}

	return due, nil
}

// MarkChecked updates a topic's LastChecked timestamp.
func (t *TopicTracker) MarkChecked(ctx context.Context, id string) error {
	topic, err := t.store.GetTopic(ctx, id)
	if err != nil {
		return err
	}
	topic.LastChecked = time.Now().UTC()
	return t.store.UpdateTopic(ctx, topic)
}

// Store returns the underlying TopicStore.
func (t *TopicTracker) Store() TopicStore {
	return t.store
}

// scheduleToInterval converts a cron expression to a rough duration.
// This is a simplified parser; a full cron library would be used in production.
func scheduleToInterval(schedule string) time.Duration {
	// Common patterns: every N hours, every N minutes.
	// Default fallback is 6 hours.
	switch schedule {
	case "* * * * *":
		return 1 * time.Minute
	case "*/5 * * * *":
		return 5 * time.Minute
	case "*/15 * * * *":
		return 15 * time.Minute
	case "*/30 * * * *":
		return 30 * time.Minute
	case "0 * * * *":
		return 1 * time.Hour
	case "0 */2 * * *":
		return 2 * time.Hour
	case "0 */6 * * *":
		return 6 * time.Hour
	case "0 */12 * * *":
		return 12 * time.Hour
	case "0 0 * * *":
		return 24 * time.Hour
	default:
		return 6 * time.Hour
	}
}

// generateID produces a simple unique identifier.
func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}

// MemoryTopicStore is an in-memory implementation of TopicStore for
// development and testing.
type MemoryTopicStore struct {
	mu     sync.RWMutex
	topics map[string]models.TrackedTopic
	briefs map[string]models.ResearchBrief
}

// NewMemoryTopicStore creates an empty in-memory store.
func NewMemoryTopicStore() *MemoryTopicStore {
	return &MemoryTopicStore{
		topics: make(map[string]models.TrackedTopic),
		briefs: make(map[string]models.ResearchBrief),
	}
}

// AddTopic stores a topic in memory.
func (m *MemoryTopicStore) AddTopic(_ context.Context, topic models.TrackedTopic) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.topics[topic.ID]; exists {
		return fmt.Errorf("topic %s already exists", topic.ID)
	}
	m.topics[topic.ID] = topic
	return nil
}

// GetTopic retrieves a topic by ID.
func (m *MemoryTopicStore) GetTopic(_ context.Context, id string) (models.TrackedTopic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	topic, ok := m.topics[id]
	if !ok {
		return models.TrackedTopic{}, fmt.Errorf("topic not found: %s", id)
	}
	return topic, nil
}

// ListTopics returns all topics, optionally filtered by workspace.
func (m *MemoryTopicStore) ListTopics(_ context.Context, workspaceID string) ([]models.TrackedTopic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []models.TrackedTopic
	for _, t := range m.topics {
		if workspaceID == "" || t.WorkspaceID == workspaceID {
			result = append(result, t)
		}
	}
	return result, nil
}

// UpdateTopic replaces a topic.
func (m *MemoryTopicStore) UpdateTopic(_ context.Context, topic models.TrackedTopic) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.topics[topic.ID]; !ok {
		return fmt.Errorf("topic not found: %s", topic.ID)
	}
	m.topics[topic.ID] = topic
	return nil
}

// RemoveTopic deletes a topic.
func (m *MemoryTopicStore) RemoveTopic(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.topics[id]; !ok {
		return fmt.Errorf("topic not found: %s", id)
	}
	delete(m.topics, id)
	return nil
}

// AddBrief stores a brief in memory.
func (m *MemoryTopicStore) AddBrief(_ context.Context, brief models.ResearchBrief) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.briefs[brief.ID] = brief
	return nil
}

// GetBrief retrieves a brief by ID.
func (m *MemoryTopicStore) GetBrief(_ context.Context, id string) (models.ResearchBrief, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	brief, ok := m.briefs[id]
	if !ok {
		return models.ResearchBrief{}, fmt.Errorf("brief not found: %s", id)
	}
	return brief, nil
}

// ListBriefs returns recent briefs, optionally filtered by topic.
func (m *MemoryTopicStore) ListBriefs(_ context.Context, topicID string, limit int) ([]models.ResearchBrief, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []models.ResearchBrief
	for _, b := range m.briefs {
		if topicID == "" || b.TopicID == topicID {
			result = append(result, b)
		}
	}
	// Sort by creation time descending (newest first).
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// ListContradictions returns all contradictions across recent briefs.
func (m *MemoryTopicStore) ListContradictions(_ context.Context, limit int) ([]models.Contradiction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []models.Contradiction
	for _, b := range m.briefs {
		result = append(result, b.Contradictions...)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
