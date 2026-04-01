// Package audit provides audit trail export and compliance reporting for
// IronGolem OS. All autonomous actions produce events; this package allows
// exporting those events in structured formats for compliance and review.
package audit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"
)

// ExportFormat identifies the output format for audit exports.
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
)

// RiskLevel indicates the risk classification of an audited action.
type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// PolicyDecision records the policy outcome for an action.
type PolicyDecision string

const (
	DecisionAllowed     PolicyDecision = "allowed"
	DecisionBlocked     PolicyDecision = "blocked"
	DecisionApproved    PolicyDecision = "approved"
	DecisionQuarantined PolicyDecision = "quarantined"
	DecisionEscalated   PolicyDecision = "escalated"
)

// AuditEvent represents a single auditable action in the system.
type AuditEvent struct {
	// Timestamp is when the action occurred.
	Timestamp time.Time `json:"timestamp"`

	// Actor is the user, agent, or service that performed the action.
	Actor string `json:"actor"`

	// Action is the operation that was performed.
	Action string `json:"action"`

	// Target is the resource or entity the action was performed on.
	Target string `json:"target"`

	// Result is the outcome of the action (e.g. "success", "failure").
	Result string `json:"result"`

	// RiskLevel is the assessed risk of this action.
	RiskLevel RiskLevel `json:"risk_level"`

	// PolicyDecision is the policy engine's verdict.
	PolicyDecision PolicyDecision `json:"policy_decision"`

	// TenantID scopes the event to a tenant.
	TenantID string `json:"tenant_id,omitempty"`

	// WorkspaceID scopes the event within a tenant.
	WorkspaceID string `json:"workspace_id,omitempty"`

	// Metadata holds additional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AuditFilter specifies criteria for selecting audit events to export.
type AuditFilter struct {
	// TimeRange restricts events to a start/end window.
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`

	// WorkspaceIDs restricts to specific workspaces.
	WorkspaceIDs []string `json:"workspace_ids,omitempty"`

	// EventTypes restricts to specific action types.
	EventTypes []string `json:"event_types,omitempty"`

	// UserIDs restricts to specific actors.
	UserIDs []string `json:"user_ids,omitempty"`

	// Severity restricts to events at or above this risk level.
	Severity RiskLevel `json:"severity,omitempty"`
}

// AuditReport is the exported audit data with metadata.
type AuditReport struct {
	// GeneratedAt is when the report was created.
	GeneratedAt time.Time `json:"generated_at"`

	// Filter is the filter criteria used to produce this report.
	Filter AuditFilter `json:"filter"`

	// EventCount is the total number of events in the report.
	EventCount int `json:"event_count"`

	// Events is the list of audit events matching the filter.
	Events []AuditEvent `json:"events"`
}

// ComplianceReport summarizes audit activity for compliance review.
type ComplianceReport struct {
	// GeneratedAt is when the report was created.
	GeneratedAt time.Time `json:"generated_at"`

	// PeriodFrom is the start of the reporting period.
	PeriodFrom time.Time `json:"period_from"`

	// PeriodTo is the end of the reporting period.
	PeriodTo time.Time `json:"period_to"`

	// TotalActions is the total number of audited actions.
	TotalActions int `json:"total_actions"`

	// Allowed is the number of actions that were allowed by policy.
	Allowed int `json:"allowed"`

	// Blocked is the number of actions that were blocked.
	Blocked int `json:"blocked"`

	// Approved is the number of actions that required and received approval.
	Approved int `json:"approved"`

	// Quarantined is the number of actions that were quarantined.
	Quarantined int `json:"quarantined"`

	// Escalated is the number of actions that were escalated.
	Escalated int `json:"escalated"`

	// ByRiskLevel summarizes counts by risk level.
	ByRiskLevel map[RiskLevel]int `json:"by_risk_level"`

	// TopActors lists the most active actors by action count.
	TopActors []ActorSummary `json:"top_actors"`
}

// ActorSummary records action counts for a single actor.
type ActorSummary struct {
	Actor       string `json:"actor"`
	ActionCount int    `json:"action_count"`
}

// AuditExporter defines the interface for exporting audit data.
type AuditExporter interface {
	// Export retrieves audit events matching the filter and encodes them
	// in the specified format.
	Export(filter AuditFilter, format ExportFormat) ([]byte, error)
}

// InMemoryStore is a simple in-memory audit event store suitable for
// development and testing. Production deployments should use a persistent
// store backed by the event sourcing system.
type InMemoryStore struct {
	events []AuditEvent
}

// NewInMemoryStore creates an empty in-memory audit store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		events: make([]AuditEvent, 0),
	}
}

// Record adds an audit event to the store.
func (s *InMemoryStore) Record(evt AuditEvent) {
	s.events = append(s.events, evt)
}

// Query returns all events matching the filter.
func (s *InMemoryStore) Query(filter AuditFilter) []AuditEvent {
	var result []AuditEvent

	sevOrder := severityOrder(filter.Severity)

	for _, evt := range s.events {
		if !matchesFilter(evt, filter, sevOrder) {
			continue
		}
		result = append(result, evt)
	}
	return result
}

// Export implements AuditExporter by querying the store and encoding
// the results in the requested format.
func (s *InMemoryStore) Export(filter AuditFilter, format ExportFormat) ([]byte, error) {
	evts := s.Query(filter)

	report := AuditReport{
		GeneratedAt: time.Now().UTC(),
		Filter:      filter,
		EventCount:  len(evts),
		Events:      evts,
	}

	switch format {
	case FormatJSON:
		return json.MarshalIndent(report, "", "  ")
	case FormatCSV:
		return encodeCSV(evts)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// ComplianceSummary generates a compliance report for the given time range.
func (s *InMemoryStore) ComplianceSummary(from, to time.Time) ComplianceReport {
	filter := AuditFilter{
		From: &from,
		To:   &to,
	}
	evts := s.Query(filter)

	report := ComplianceReport{
		GeneratedAt:  time.Now().UTC(),
		PeriodFrom:   from,
		PeriodTo:     to,
		TotalActions: len(evts),
		ByRiskLevel:  make(map[RiskLevel]int),
	}

	actorCounts := make(map[string]int)

	for _, evt := range evts {
		switch evt.PolicyDecision {
		case DecisionAllowed:
			report.Allowed++
		case DecisionBlocked:
			report.Blocked++
		case DecisionApproved:
			report.Approved++
		case DecisionQuarantined:
			report.Quarantined++
		case DecisionEscalated:
			report.Escalated++
		}

		report.ByRiskLevel[evt.RiskLevel]++
		actorCounts[evt.Actor]++
	}

	// Build top actors (simple sort by count, top 10).
	for actor, count := range actorCounts {
		report.TopActors = append(report.TopActors, ActorSummary{
			Actor:       actor,
			ActionCount: count,
		})
	}
	// Sort descending by count.
	for i := 0; i < len(report.TopActors); i++ {
		for j := i + 1; j < len(report.TopActors); j++ {
			if report.TopActors[j].ActionCount > report.TopActors[i].ActionCount {
				report.TopActors[i], report.TopActors[j] = report.TopActors[j], report.TopActors[i]
			}
		}
	}
	if len(report.TopActors) > 10 {
		report.TopActors = report.TopActors[:10]
	}

	return report
}

// matchesFilter checks if an event matches the given filter criteria.
func matchesFilter(evt AuditEvent, filter AuditFilter, minSeverity int) bool {
	if filter.From != nil && evt.Timestamp.Before(*filter.From) {
		return false
	}
	if filter.To != nil && evt.Timestamp.After(*filter.To) {
		return false
	}
	if len(filter.WorkspaceIDs) > 0 && !contains(filter.WorkspaceIDs, evt.WorkspaceID) {
		return false
	}
	if len(filter.EventTypes) > 0 && !contains(filter.EventTypes, evt.Action) {
		return false
	}
	if len(filter.UserIDs) > 0 && !contains(filter.UserIDs, evt.Actor) {
		return false
	}
	if minSeverity > 0 && severityOrder(evt.RiskLevel) < minSeverity {
		return false
	}
	return true
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func severityOrder(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 5
	case RiskHigh:
		return 4
	case RiskMedium:
		return 3
	case RiskLow:
		return 2
	case RiskNone:
		return 1
	default:
		return 0
	}
}

func encodeCSV(evts []AuditEvent) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header.
	if err := w.Write([]string{
		"timestamp", "actor", "action", "target", "result",
		"risk_level", "policy_decision", "tenant_id", "workspace_id",
	}); err != nil {
		return nil, fmt.Errorf("csv header write: %w", err)
	}

	for _, evt := range evts {
		if err := w.Write([]string{
			evt.Timestamp.Format(time.RFC3339),
			evt.Actor,
			evt.Action,
			evt.Target,
			evt.Result,
			string(evt.RiskLevel),
			string(evt.PolicyDecision),
			evt.TenantID,
			evt.WorkspaceID,
		}); err != nil {
			return nil, fmt.Errorf("csv row write: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}
	return buf.Bytes(), nil
}
