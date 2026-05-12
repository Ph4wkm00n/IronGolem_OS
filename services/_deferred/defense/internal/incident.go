package internal

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// IncidentSeverity mirrors quarantine severity for incidents.
type IncidentSeverity string

const (
	IncidentSeverityLow      IncidentSeverity = "low"
	IncidentSeverityMedium   IncidentSeverity = "medium"
	IncidentSeverityHigh     IncidentSeverity = "high"
	IncidentSeverityCritical IncidentSeverity = "critical"
)

// IncidentStatus tracks the lifecycle of an incident.
type IncidentStatus string

const (
	IncidentStatusOpen          IncidentStatus = "open"
	IncidentStatusInvestigating IncidentStatus = "investigating"
	IncidentStatusMitigated     IncidentStatus = "mitigated"
	IncidentStatusResolved      IncidentStatus = "resolved"
	IncidentStatusClosed        IncidentStatus = "closed"
)

// IncidentEvent records a single action in the incident timeline.
type IncidentEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Action      string    `json:"action"`
	Actor       string    `json:"actor"`
	Description string    `json:"description"`
}

// Incident represents a security or reliability incident.
type Incident struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Summary          string           `json:"summary"`
	Severity         IncidentSeverity `json:"severity"`
	Status           IncidentStatus   `json:"status"`
	Timeline         []IncidentEvent  `json:"timeline"`
	AffectedServices []string         `json:"affected_services"`
	RootCause        string           `json:"root_cause,omitempty"`
	Resolution       string           `json:"resolution,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	ResolvedAt       *time.Time       `json:"resolved_at,omitempty"`
	TenantID         string           `json:"tenant_id,omitempty"`
}

// IncidentStore defines persistence operations for incidents.
type IncidentStore interface {
	Save(incident Incident) error
	Get(id string) (Incident, bool)
	List() []Incident
	Update(incident Incident) error
}

// InMemoryIncidentStore is a thread-safe in-memory IncidentStore.
type InMemoryIncidentStore struct {
	mu        sync.RWMutex
	incidents map[string]Incident
}

// NewInMemoryIncidentStore creates a new in-memory incident store.
func NewInMemoryIncidentStore() *InMemoryIncidentStore {
	return &InMemoryIncidentStore{
		incidents: make(map[string]Incident),
	}
}

// Save persists an incident.
func (s *InMemoryIncidentStore) Save(incident Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incidents[incident.ID] = incident
	return nil
}

// Get retrieves an incident by ID.
func (s *InMemoryIncidentStore) Get(id string) (Incident, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inc, ok := s.incidents[id]
	return inc, ok
}

// List returns all incidents.
func (s *InMemoryIncidentStore) List() []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		result = append(result, inc)
	}
	return result
}

// Update replaces an existing incident.
func (s *InMemoryIncidentStore) Update(incident Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.incidents[incident.ID]; !ok {
		return fmt.Errorf("incident not found: %s", incident.ID)
	}
	s.incidents[incident.ID] = incident
	return nil
}

// IncidentManager orchestrates incident creation, updates, and resolution.
type IncidentManager struct {
	logger *slog.Logger
	store  IncidentStore

	// autoCreateThreshold is the number of quarantine events within the
	// tracking window before an incident is automatically created.
	autoCreateThreshold int
	trackingWindow      time.Duration

	mu               sync.Mutex
	recentQuarantine []time.Time
}

// NewIncidentManager creates an IncidentManager with the given dependencies.
func NewIncidentManager(logger *slog.Logger, store IncidentStore) *IncidentManager {
	return &IncidentManager{
		logger:              logger,
		store:               store,
		autoCreateThreshold: 3,
		trackingWindow:      15 * time.Minute,
		recentQuarantine:    make([]time.Time, 0),
	}
}

// Create opens a new incident and adds an initial timeline entry.
func (m *IncidentManager) Create(title, summary string, severity IncidentSeverity, affectedServices []string, tenantID string) (Incident, error) {
	now := time.Now().UTC()
	incident := Incident{
		ID:               generateID(),
		Title:            title,
		Summary:          summary,
		Severity:         severity,
		Status:           IncidentStatusOpen,
		AffectedServices: affectedServices,
		CreatedAt:        now,
		UpdatedAt:        now,
		TenantID:         tenantID,
		Timeline: []IncidentEvent{
			{
				Timestamp:   now,
				Action:      "created",
				Actor:       "system",
				Description: fmt.Sprintf("Incident created: %s", title),
			},
		},
	}

	if err := m.store.Save(incident); err != nil {
		return Incident{}, fmt.Errorf("saving incident: %w", err)
	}

	m.logger.Warn("incident created",
		slog.String("id", incident.ID),
		slog.String("title", title),
		slog.String("severity", string(severity)),
	)

	return incident, nil
}

// UpdateStatus transitions an incident to a new status.
func (m *IncidentManager) UpdateStatus(id string, status IncidentStatus, actor, description string) error {
	incident, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	now := time.Now().UTC()
	incident.Status = status
	incident.UpdatedAt = now
	incident.Timeline = append(incident.Timeline, IncidentEvent{
		Timestamp:   now,
		Action:      "status_change",
		Actor:       actor,
		Description: description,
	})

	if err := m.store.Update(incident); err != nil {
		return fmt.Errorf("updating incident: %w", err)
	}

	m.logger.Info("incident status updated",
		slog.String("id", id),
		slog.String("status", string(status)),
		slog.String("actor", actor),
	)

	return nil
}

// AddTimelineEvent appends an event to the incident timeline.
func (m *IncidentManager) AddTimelineEvent(id string, event IncidentEvent) error {
	incident, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	incident.Timeline = append(incident.Timeline, event)
	incident.UpdatedAt = time.Now().UTC()

	if err := m.store.Update(incident); err != nil {
		return fmt.Errorf("updating incident: %w", err)
	}

	return nil
}

// Resolve closes an incident with a root cause and resolution description.
func (m *IncidentManager) Resolve(id, rootCause, resolution, actor string) error {
	incident, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	now := time.Now().UTC()
	incident.Status = IncidentStatusResolved
	incident.RootCause = rootCause
	incident.Resolution = resolution
	incident.ResolvedAt = &now
	incident.UpdatedAt = now
	incident.Timeline = append(incident.Timeline, IncidentEvent{
		Timestamp:   now,
		Action:      "resolved",
		Actor:       actor,
		Description: fmt.Sprintf("Root cause: %s. Resolution: %s", rootCause, resolution),
	})

	if err := m.store.Update(incident); err != nil {
		return fmt.Errorf("updating incident: %w", err)
	}

	m.logger.Info("incident resolved",
		slog.String("id", id),
		slog.String("root_cause", rootCause),
		slog.String("actor", actor),
	)

	return nil
}

// Get returns a single incident by ID.
func (m *IncidentManager) Get(id string) (Incident, bool) {
	return m.store.Get(id)
}

// List returns all incidents.
func (m *IncidentManager) List() []Incident {
	return m.store.List()
}

// CheckAutoCreate tracks quarantine events and auto-creates an incident
// when the threshold is exceeded within the tracking window. It returns
// the created incident and true if one was created, or a zero Incident
// and false otherwise.
func (m *IncidentManager) CheckAutoCreate(affectedService, tenantID string) (Incident, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-m.trackingWindow)

	// Prune old entries.
	pruned := make([]time.Time, 0, len(m.recentQuarantine))
	for _, t := range m.recentQuarantine {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	pruned = append(pruned, now)
	m.recentQuarantine = pruned

	if len(pruned) < m.autoCreateThreshold {
		return Incident{}, false
	}

	// Reset counter to avoid repeated auto-creation.
	m.recentQuarantine = m.recentQuarantine[:0]

	incident, err := m.Create(
		"Auto-created: multiple quarantine events detected",
		fmt.Sprintf("%d quarantine events detected within %s, indicating a possible coordinated threat or systemic issue.", len(pruned), m.trackingWindow),
		IncidentSeverityHigh,
		[]string{affectedService},
		tenantID,
	)
	if err != nil {
		m.logger.Error("failed to auto-create incident",
			slog.String("error", err.Error()),
		)
		return Incident{}, false
	}

	return incident, true
}

// PlainLanguageSummary generates a non-technical summary of an incident
// suitable for display in the UI to non-technical users.
func PlainLanguageSummary(incident Incident) string {
	var sb strings.Builder

	// Status description.
	var statusDesc string
	switch incident.Status {
	case IncidentStatusOpen:
		statusDesc = "We detected a new issue"
	case IncidentStatusInvestigating:
		statusDesc = "We are actively looking into an issue"
	case IncidentStatusMitigated:
		statusDesc = "We have taken steps to reduce the impact of an issue"
	case IncidentStatusResolved:
		statusDesc = "We have resolved an issue"
	case IncidentStatusClosed:
		statusDesc = "An issue has been fully addressed and closed"
	default:
		statusDesc = "There is an issue"
	}

	sb.WriteString(fmt.Sprintf("%s that affects ", statusDesc))

	if len(incident.AffectedServices) == 0 {
		sb.WriteString("the system")
	} else {
		sb.WriteString(strings.Join(incident.AffectedServices, ", "))
	}
	sb.WriteString(". ")

	// Severity.
	switch incident.Severity {
	case IncidentSeverityCritical:
		sb.WriteString("This is a critical issue that requires immediate attention. ")
	case IncidentSeverityHigh:
		sb.WriteString("This is a high-priority issue. ")
	case IncidentSeverityMedium:
		sb.WriteString("This is a moderate issue. ")
	case IncidentSeverityLow:
		sb.WriteString("This is a minor issue with limited impact. ")
	}

	sb.WriteString(incident.Summary)

	if incident.Resolution != "" {
		sb.WriteString(fmt.Sprintf(" The issue was resolved: %s", incident.Resolution))
	}

	return sb.String()
}
