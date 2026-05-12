// Package internal implements the config rollback center for the IronGolem OS
// Defense service. It provides snapshot-based configuration management with
// diff comparison and automatic before-state capture on config changes.
package internal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// ConfigSnapshot captures a service configuration at a point in time.
type ConfigSnapshot struct {
	ID        string          `json:"id"`
	Service   string          `json:"service"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	Label     string          `json:"label"`
	IsGood    bool            `json:"is_good"`
}

// ConfigDiffEntry describes a single changed field between two snapshots.
type ConfigDiffEntry struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// ConfigDiffResult holds the comparison between two snapshots.
type ConfigDiffResult struct {
	SnapshotA string            `json:"snapshot_a"`
	SnapshotB string            `json:"snapshot_b"`
	Service   string            `json:"service"`
	Changes   []ConfigDiffEntry `json:"changes"`
	Identical bool              `json:"identical"`
}

// RollbackStore defines persistence operations for config snapshots.
type RollbackStore interface {
	Save(snapshot ConfigSnapshot) error
	Get(id string) (ConfigSnapshot, bool)
	List() []ConfigSnapshot
	ListByService(service string) []ConfigSnapshot
	Delete(id string) error
}

// InMemoryRollbackStore is a thread-safe in-memory RollbackStore.
type InMemoryRollbackStore struct {
	mu        sync.RWMutex
	snapshots map[string]ConfigSnapshot
}

// NewInMemoryRollbackStore creates a new in-memory rollback store.
func NewInMemoryRollbackStore() *InMemoryRollbackStore {
	return &InMemoryRollbackStore{
		snapshots: make(map[string]ConfigSnapshot),
	}
}

// Save persists a config snapshot.
func (s *InMemoryRollbackStore) Save(snapshot ConfigSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.ID] = snapshot
	return nil
}

// Get retrieves a config snapshot by ID.
func (s *InMemoryRollbackStore) Get(id string) (ConfigSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[id]
	return snap, ok
}

// List returns all config snapshots sorted by creation time descending.
func (s *InMemoryRollbackStore) List() []ConfigSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ConfigSnapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		result = append(result, snap)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByService returns snapshots for a specific service sorted by creation
// time descending.
func (s *InMemoryRollbackStore) ListByService(service string) []ConfigSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ConfigSnapshot
	for _, snap := range s.snapshots {
		if snap.Service == service {
			result = append(result, snap)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a config snapshot by ID.
func (s *InMemoryRollbackStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, id)
	return nil
}

// RollbackResult describes the outcome of a rollback operation.
type RollbackResult struct {
	Service        string          `json:"service"`
	RestoredConfig json.RawMessage `json:"restored_config"`
	SnapshotID     string          `json:"snapshot_id"`
	Label          string          `json:"label"`
	RestoredAt     time.Time       `json:"restored_at"`
}

// RollbackManager orchestrates config snapshot capture, diff comparison,
// and rollback operations.
type RollbackManager struct {
	logger *slog.Logger
	store  RollbackStore
}

// NewRollbackManager creates a RollbackManager with the given dependencies.
func NewRollbackManager(logger *slog.Logger, store RollbackStore) *RollbackManager {
	return &RollbackManager{
		logger: logger,
		store:  store,
	}
}

// TakeSnapshot captures the current configuration for a service.
func (m *RollbackManager) TakeSnapshot(service string, config json.RawMessage, label string, isGood bool) (ConfigSnapshot, error) {
	snapshot := ConfigSnapshot{
		ID:        generateID(),
		Service:   service,
		Config:    config,
		CreatedAt: time.Now().UTC(),
		Label:     label,
		IsGood:    isGood,
	}

	if err := m.store.Save(snapshot); err != nil {
		return ConfigSnapshot{}, fmt.Errorf("saving snapshot: %w", err)
	}

	m.logger.Info("config snapshot taken",
		slog.String("id", snapshot.ID),
		slog.String("service", service),
		slog.String("label", label),
		slog.Bool("is_good", isGood),
	)

	return snapshot, nil
}

// Rollback restores a service to the configuration captured in the given
// snapshot. It takes a new snapshot of the current config before restoring
// (auto-snapshot of the before-state).
func (m *RollbackManager) Rollback(snapshotID string, currentConfig json.RawMessage) (RollbackResult, error) {
	target, ok := m.store.Get(snapshotID)
	if !ok {
		return RollbackResult{}, fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	// Auto-capture the current config as a pre-rollback snapshot.
	if currentConfig != nil {
		_, err := m.TakeSnapshot(target.Service, currentConfig, "auto: pre-rollback backup", false)
		if err != nil {
			m.logger.Error("failed to take pre-rollback snapshot",
				slog.String("service", target.Service),
				slog.String("error", err.Error()),
			)
		}
	}

	result := RollbackResult{
		Service:        target.Service,
		RestoredConfig: target.Config,
		SnapshotID:     target.ID,
		Label:          target.Label,
		RestoredAt:     time.Now().UTC(),
	}

	m.logger.Warn("config rolled back",
		slog.String("service", target.Service),
		slog.String("snapshot_id", snapshotID),
		slog.String("label", target.Label),
	)

	return result, nil
}

// GetLastGoodSnapshot returns the most recent snapshot marked as "good" for
// a given service.
func (m *RollbackManager) GetLastGoodSnapshot(service string) (ConfigSnapshot, bool) {
	snapshots := m.store.ListByService(service)
	for _, snap := range snapshots {
		if snap.IsGood {
			return snap, true
		}
	}
	return ConfigSnapshot{}, false
}

// ListSnapshots returns all snapshots, optionally filtered by service.
func (m *RollbackManager) ListSnapshots(service string) []ConfigSnapshot {
	if service != "" {
		return m.store.ListByService(service)
	}
	return m.store.List()
}

// Diff compares two snapshots and returns a list of changed fields.
func (m *RollbackManager) Diff(snapshotAID, snapshotBID string) (ConfigDiffResult, error) {
	a, okA := m.store.Get(snapshotAID)
	if !okA {
		return ConfigDiffResult{}, fmt.Errorf("snapshot A not found: %s", snapshotAID)
	}
	b, okB := m.store.Get(snapshotBID)
	if !okB {
		return ConfigDiffResult{}, fmt.Errorf("snapshot B not found: %s", snapshotBID)
	}

	result := ConfigDiffResult{
		SnapshotA: snapshotAID,
		SnapshotB: snapshotBID,
		Service:   a.Service,
	}

	// Unmarshal both configs into maps for comparison.
	var mapA, mapB map[string]any
	if err := json.Unmarshal(a.Config, &mapA); err != nil {
		return result, fmt.Errorf("unmarshalling snapshot A config: %w", err)
	}
	if err := json.Unmarshal(b.Config, &mapB); err != nil {
		return result, fmt.Errorf("unmarshalling snapshot B config: %w", err)
	}

	changes := diffMaps("", mapA, mapB)
	result.Changes = changes
	result.Identical = len(changes) == 0

	return result, nil
}

// AutoSnapshot hooks into a config change event and captures the before-state.
// This should be called before applying a new configuration.
func (m *RollbackManager) AutoSnapshot(service string, currentConfig json.RawMessage) (ConfigSnapshot, error) {
	return m.TakeSnapshot(service, currentConfig, "auto: pre-change snapshot", false)
}

// MarkAsGood marks an existing snapshot as a known-good configuration.
func (m *RollbackManager) MarkAsGood(snapshotID string) error {
	snap, ok := m.store.Get(snapshotID)
	if !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	snap.IsGood = true
	if err := m.store.Save(snap); err != nil {
		return fmt.Errorf("updating snapshot: %w", err)
	}

	m.logger.Info("snapshot marked as good",
		slog.String("id", snapshotID),
		slog.String("service", snap.Service),
	)

	return nil
}

// diffMaps recursively compares two maps and returns the differences.
func diffMaps(prefix string, a, b map[string]any) []ConfigDiffEntry {
	var changes []ConfigDiffEntry

	// Collect all keys from both maps.
	allKeys := make(map[string]bool)
	for k := range a {
		allKeys[k] = true
	}
	for k := range b {
		allKeys[k] = true
	}

	for key := range allKeys {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		valA, inA := a[key]
		valB, inB := b[key]

		if !inA {
			changes = append(changes, ConfigDiffEntry{
				Field:    fullKey,
				OldValue: "<not set>",
				NewValue: fmt.Sprintf("%v", valB),
			})
			continue
		}
		if !inB {
			changes = append(changes, ConfigDiffEntry{
				Field:    fullKey,
				OldValue: fmt.Sprintf("%v", valA),
				NewValue: "<removed>",
			})
			continue
		}

		// If both are maps, recurse.
		mapA, aIsMap := valA.(map[string]any)
		mapB, bIsMap := valB.(map[string]any)
		if aIsMap && bIsMap {
			changes = append(changes, diffMaps(fullKey, mapA, mapB)...)
			continue
		}

		// Compare as strings.
		strA := fmt.Sprintf("%v", valA)
		strB := fmt.Sprintf("%v", valB)
		if strA != strB {
			changes = append(changes, ConfigDiffEntry{
				Field:    fullKey,
				OldValue: strA,
				NewValue: strB,
			})
		}
	}

	return changes
}
