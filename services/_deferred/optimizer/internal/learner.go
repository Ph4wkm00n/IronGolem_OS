// Package internal implements the adaptive intelligence engines for the
// IronGolem OS optimizer service, including preference learning, shadow
// mode experiments, prompt optimization, and provider benchmarking.
package internal

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// PreferenceStore is the interface for persisting and querying learned
// preferences. Implementations must be safe for concurrent use.
type PreferenceStore interface {
	// Save persists a preference, creating or updating as needed.
	Save(pref models.Preference) error

	// Get returns a preference by ID, or false if not found.
	Get(id string) (models.Preference, bool)

	// ListByUser returns all preferences for a user in a workspace.
	ListByUser(workspaceID, userID string) []models.Preference

	// ListAll returns every stored preference.
	ListAll() []models.Preference

	// Delete removes a preference by ID.
	Delete(id string) error
}

// MemoryPreferenceStore is an in-memory implementation of PreferenceStore
// suitable for development and testing.
type MemoryPreferenceStore struct {
	mu    sync.RWMutex
	prefs map[string]models.Preference
}

// NewMemoryPreferenceStore creates an empty in-memory preference store.
func NewMemoryPreferenceStore() *MemoryPreferenceStore {
	return &MemoryPreferenceStore{
		prefs: make(map[string]models.Preference),
	}
}

func (s *MemoryPreferenceStore) Save(pref models.Preference) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefs[pref.ID] = pref
	return nil
}

func (s *MemoryPreferenceStore) Get(id string) (models.Preference, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prefs[id]
	return p, ok
}

func (s *MemoryPreferenceStore) ListByUser(workspaceID, userID string) []models.Preference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []models.Preference
	for _, p := range s.prefs {
		if p.WorkspaceID == workspaceID && p.UserID == userID {
			result = append(result, p)
		}
	}
	return result
}

func (s *MemoryPreferenceStore) ListAll() []models.Preference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Preference, 0, len(s.prefs))
	for _, p := range s.prefs {
		result = append(result, p)
	}
	return result
}

func (s *MemoryPreferenceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.prefs, id)
	return nil
}

// patternThreshold is the minimum number of consistent signals needed
// to infer a preference pattern.
const patternThreshold = 3

// PreferenceLearner observes user behavior patterns and infers
// preferences. It watches approval patterns, edit patterns, and
// scheduling patterns to learn what users prefer.
//
// All learned preferences start in shadow mode (proposed but not
// active), following the "trust before power" principle.
type PreferenceLearner struct {
	mu      sync.Mutex
	store   PreferenceStore
	signals map[string][]models.LearningSignal // keyed by userID
	logger  *slog.Logger
}

// NewPreferenceLearner creates a new learner backed by the given store.
func NewPreferenceLearner(store PreferenceStore, logger *slog.Logger) *PreferenceLearner {
	return &PreferenceLearner{
		store:   store,
		signals: make(map[string][]models.LearningSignal),
		logger:  logger,
	}
}

// Store returns the underlying preference store.
func (pl *PreferenceLearner) Store() PreferenceStore {
	return pl.store
}

// ProcessSignal ingests a single behavioral signal and checks whether
// accumulated signals reveal a new preference pattern.
func (pl *PreferenceLearner) ProcessSignal(signal models.LearningSignal) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	pl.signals[signal.UserID] = append(pl.signals[signal.UserID], signal)

	pl.logger.Info("learning signal recorded",
		slog.String("user_id", signal.UserID),
		slog.String("action", signal.Action),
		slog.String("category", string(signal.Category)),
	)

	// After each signal, check for new patterns from this user's history.
	detected := pl.detectPatternsLocked(pl.signals[signal.UserID])
	for _, pref := range detected {
		if err := pl.store.Save(pref); err != nil {
			pl.logger.Error("failed to save learned preference",
				slog.String("error", err.Error()),
				slog.String("pref_id", pref.ID),
			)
		} else {
			pl.logger.Info("preference learned and saved in shadow mode",
				slog.String("pref_id", pref.ID),
				slog.String("key", pref.Key),
				slog.Float64("confidence", pref.Confidence),
			)
		}
	}
}

// DetectPatterns analyzes a batch of signals and returns any newly
// detected preferences. Useful for batch processing historical data.
func (pl *PreferenceLearner) DetectPatterns(signals []models.LearningSignal) []models.Preference {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.detectPatternsLocked(signals)
}

// detectPatternsLocked runs pattern detection without holding the lock
// externally (caller must hold pl.mu).
func (pl *PreferenceLearner) detectPatternsLocked(signals []models.LearningSignal) []models.Preference {
	var detected []models.Preference

	// Group signals by category and action.
	type groupKey struct {
		category models.PreferenceCategory
		action   string
	}
	groups := make(map[groupKey][]models.LearningSignal)

	for _, s := range signals {
		key := groupKey{category: s.Category, action: s.Action}
		groups[key] = append(groups[key], s)
	}

	for gk, sigs := range groups {
		if len(sigs) < patternThreshold {
			continue
		}

		pref := pl.inferPreference(gk.category, gk.action, sigs)
		if pref != nil {
			// Check if we already have this preference.
			existing := pl.store.ListByUser(pref.WorkspaceID, pref.UserID)
			alreadyExists := false
			for _, e := range existing {
				if e.Key == pref.Key {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				detected = append(detected, *pref)
			}
		}
	}

	return detected
}

// inferPreference builds a Preference from a set of consistent signals.
func (pl *PreferenceLearner) inferPreference(
	category models.PreferenceCategory,
	action string,
	signals []models.LearningSignal,
) *models.Preference {
	if len(signals) == 0 {
		return nil
	}

	first := signals[0]

	// Build evidence from the contributing signals.
	evidence := make([]models.PreferenceEvidence, 0, len(signals))
	for _, s := range signals {
		evidence = append(evidence, models.PreferenceEvidence{
			EventID:     s.EventID,
			Action:      s.Action,
			Timestamp:   s.Timestamp,
			Description: describeSignal(s),
		})
	}

	// Confidence grows with the number of supporting signals, capped at 0.95.
	confidence := float64(len(signals)) / float64(len(signals)+patternThreshold)
	if confidence > 0.95 {
		confidence = 0.95
	}

	// Derive a preference key from the category and context.
	key := deriveKey(category, action, signals)

	// Build a value summarizing the pattern.
	value := deriveValue(action, signals)

	now := time.Now().UTC()
	return &models.Preference{
		ID:          generateID(),
		WorkspaceID: first.WorkspaceID,
		UserID:      first.UserID,
		Category:    category,
		Key:         key,
		Value:       value,
		Confidence:  confidence,
		LearnedFrom: action + "_pattern",
		Evidence:    evidence,
		ShadowMode:  true, // Always starts in shadow mode.
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// deriveKey produces a human-readable preference key from patterns.
func deriveKey(category models.PreferenceCategory, action string, signals []models.LearningSignal) string {
	base := string(category) + "." + action

	// If signals share a common context key, include it.
	if len(signals) > 0 {
		for k := range signals[0].Context {
			allMatch := true
			for _, s := range signals[1:] {
				if s.Context[k] != signals[0].Context[k] {
					allMatch = false
					break
				}
			}
			if allMatch {
				return base + "." + k + "=" + signals[0].Context[k]
			}
		}
	}

	return base
}

// deriveValue builds a JSON value summarizing the learned pattern.
func deriveValue(action string, signals []models.LearningSignal) json.RawMessage {
	summary := map[string]any{
		"action":       action,
		"signal_count": len(signals),
		"first_seen":   signals[0].Timestamp,
		"last_seen":    signals[len(signals)-1].Timestamp,
	}

	// Collect common context values.
	commonCtx := make(map[string]string)
	for k, v := range signals[0].Context {
		commonCtx[k] = v
	}
	for _, s := range signals[1:] {
		for k, v := range commonCtx {
			if s.Context[k] != v {
				delete(commonCtx, k)
			}
		}
	}
	if len(commonCtx) > 0 {
		summary["common_context"] = commonCtx
	}

	data, _ := json.Marshal(summary)
	return data
}

// describeSignal produces a human-readable description of a signal.
func describeSignal(s models.LearningSignal) string {
	desc := "User " + s.Action
	if s.Category != "" {
		desc += " (" + string(s.Category) + ")"
	}
	return desc
}

// generateID produces a timestamp-based unique identifier.
func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
