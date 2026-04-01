// Package models - preference types for the adaptive intelligence layer.
//
// These types support the preference learning engine (Phase 3), which
// observes user behavior patterns and proposes learned preferences in
// shadow mode before promoting them to active use.
package models

import (
	"encoding/json"
	"time"
)

// PreferenceCategory classifies what kind of preference was learned.
type PreferenceCategory string

const (
	PreferenceCategoryScheduling         PreferenceCategory = "scheduling"
	PreferenceCategoryCommunicationStyle PreferenceCategory = "communication_style"
	PreferenceCategoryContentFormat      PreferenceCategory = "content_format"
	PreferenceCategoryNotification       PreferenceCategory = "notification"
	PreferenceCategoryPrivacy            PreferenceCategory = "privacy"
	PreferenceCategoryToolUsage          PreferenceCategory = "tool_usage"
)

// PreferenceEvidence records a single piece of evidence supporting
// a learned preference. Evidence comes from user actions observed
// by the preference learning engine.
type PreferenceEvidence struct {
	// EventID links back to the event-sourced history.
	EventID string `json:"event_id"`

	// Action describes what the user did: approved, rejected, or edited.
	Action string `json:"action"`

	// Timestamp records when the evidence was observed.
	Timestamp time.Time `json:"timestamp"`

	// Description is a human-readable summary of the evidence.
	Description string `json:"description"`
}

// Preference represents a learned or inferred user preference.
// All preferences start in shadow mode (proposed but not active)
// and must be explicitly promoted by the user or an admin.
type Preference struct {
	// ID is a unique identifier for this preference.
	ID string `json:"id"`

	// WorkspaceID scopes the preference to a workspace.
	WorkspaceID string `json:"workspace_id"`

	// UserID identifies the user this preference belongs to.
	UserID string `json:"user_id"`

	// Category classifies the preference type.
	Category PreferenceCategory `json:"category"`

	// Key is the preference name (e.g. "preferred_meeting_time").
	Key string `json:"key"`

	// Value is the preference value, stored as arbitrary JSON.
	Value json.RawMessage `json:"value"`

	// Confidence is a score from 0.0 to 1.0 indicating how certain
	// the system is about this preference.
	Confidence float64 `json:"confidence"`

	// LearnedFrom describes the signal source (e.g. "approval_pattern").
	LearnedFrom string `json:"learned_from"`

	// Evidence lists the observations that led to this preference.
	Evidence []PreferenceEvidence `json:"evidence"`

	// ShadowMode indicates whether this preference is proposed but
	// not yet active. All learned preferences start in shadow mode.
	ShadowMode bool `json:"shadow_mode"`

	// CreatedAt records when the preference was first detected.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt records the last time the preference was modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// LearningSignal represents a single user behavior observation that
// the preference learning engine can process to detect patterns.
type LearningSignal struct {
	// UserID identifies the user who performed the action.
	UserID string `json:"user_id"`

	// WorkspaceID scopes the signal to a workspace.
	WorkspaceID string `json:"workspace_id"`

	// Action describes what the user did (e.g. "approved", "rejected", "edited").
	Action string `json:"action"`

	// Category helps classify which kind of preference this might relate to.
	Category PreferenceCategory `json:"category,omitempty"`

	// Context contains action-specific metadata (e.g. the edit diff, the
	// approval target, the scheduling details).
	Context map[string]string `json:"context,omitempty"`

	// Timestamp records when the action occurred.
	Timestamp time.Time `json:"timestamp"`

	// EventID links back to the originating event for audit trail.
	EventID string `json:"event_id,omitempty"`
}
