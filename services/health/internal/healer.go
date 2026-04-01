// Package internal implements the self-healing engine for IronGolem OS.
//
// The healer performs escalating recovery when services degrade:
//   1. Retry the failed operation
//   2. Restart the service/connector
//   3. Restore last known good configuration
//   4. Escalate to user via notification
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// HealingStrategy defines how to diagnose and execute a healing action
// for a degraded service.
type HealingStrategy interface {
	// Diagnose inspects the service status and returns the appropriate
	// healing action to take.
	Diagnose(ctx context.Context, status ServiceStatus) HealingAction

	// Execute runs the healing action and returns an error if it fails.
	Execute(ctx context.Context, action HealingAction) error
}

// ServiceStatus captures the current state of a service for diagnosis.
type ServiceStatus struct {
	ServiceName   string         `json:"service_name"`
	State         HeartbeatState `json:"state"`
	MissedBeats   int            `json:"missed_beats"`
	HealingCount  int            `json:"healing_count"`
	LastHeartbeat time.Time      `json:"last_heartbeat"`
	Metrics       map[string]any `json:"metrics,omitempty"`
}

// HealingStrategyKind enumerates the escalating recovery strategies.
type HealingStrategyKind string

const (
	StrategyRetry         HealingStrategyKind = "retry"
	StrategyRestart       HealingStrategyKind = "restart"
	StrategyConfigRestore HealingStrategyKind = "config_restore"
	StrategyRollback      HealingStrategyKind = "rollback"
	StrategyEscalate      HealingStrategyKind = "escalate"
)

// HealingAction describes a single healing step to be executed.
type HealingAction struct {
	// ID is a unique identifier for this action.
	ID string `json:"id"`
	// ServiceID is the target service.
	ServiceID string `json:"service_id"`
	// Strategy is the recovery approach.
	Strategy HealingStrategyKind `json:"strategy"`
	// Description is a human-readable explanation of the action.
	Description string `json:"description"`
	// AttemptNumber is the current attempt (1-indexed).
	AttemptNumber int `json:"attempt_number"`
	// MaxAttempts is the maximum number of escalation steps.
	MaxAttempts int `json:"max_attempts"`
	// CreatedAt is when the action was created.
	CreatedAt time.Time `json:"created_at"`
}

// HealingResult captures the outcome of a healing action.
type HealingResult struct {
	// Success indicates whether the action resolved the issue.
	Success bool `json:"success"`
	// Message provides details about the outcome.
	Message string `json:"message"`
	// Duration is how long the healing action took.
	Duration time.Duration `json:"duration"`
	// NextAction is the follow-up action if this one failed, or nil.
	NextAction *HealingAction `json:"next_action,omitempty"`
}

// HealingLogEntry records a single healing attempt for audit.
type HealingLogEntry struct {
	Action    HealingAction `json:"action"`
	Result    HealingResult `json:"result"`
	Timestamp time.Time     `json:"timestamp"`
}

// HealingLog tracks all healing attempts and their outcomes. It provides
// a complete audit trail of self-healing activity.
type HealingLog struct {
	mu      sync.RWMutex
	entries []HealingLogEntry
}

// NewHealingLog creates an empty healing log.
func NewHealingLog() *HealingLog {
	return &HealingLog{
		entries: make([]HealingLogEntry, 0),
	}
}

// Append adds a healing log entry.
func (l *HealingLog) Append(entry HealingLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// Entries returns a snapshot of all log entries.
func (l *HealingLog) Entries() []HealingLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]HealingLogEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// EntriesForService returns log entries for a specific service.
func (l *HealingLog) EntriesForService(serviceName string) []HealingLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []HealingLogEntry
	for _, e := range l.entries {
		if e.Action.ServiceID == serviceName {
			out = append(out, e)
		}
	}
	return out
}

// DefaultHealer implements the SelfHealingTrigger interface with escalating
// recovery strategies:
//
//  1. First attempt: retry the failed operation
//  2. Second attempt: restart the service/connector
//  3. Third attempt: restore last known good configuration
//  4. Fourth attempt: escalate to user via notification
type DefaultHealer struct {
	logger   *slog.Logger
	log      *HealingLog
	strategy HealingStrategy

	mu             sync.Mutex
	attemptTracker map[string]int // service name -> current attempt number
}

// NewDefaultHealer creates a DefaultHealer with the given logger.
func NewDefaultHealer(logger *slog.Logger) *DefaultHealer {
	h := &DefaultHealer{
		logger:         logger,
		log:            NewHealingLog(),
		attemptTracker: make(map[string]int),
	}
	// Use the built-in escalation strategy by default.
	h.strategy = &escalatingStrategy{healer: h}
	return h
}

// SetStrategy replaces the healing strategy.
func (h *DefaultHealer) SetStrategy(s HealingStrategy) {
	h.strategy = s
}

// Log returns the healing log for inspection.
func (h *DefaultHealer) Log() *HealingLog {
	return h.log
}

// OnServiceDegraded is called when a service transitions to NeedsAttention.
// It diagnoses the issue and executes the appropriate healing action.
func (h *DefaultHealer) OnServiceDegraded(ctx context.Context, rec ServiceRecord) error {
	status := serviceStatusFromRecord(rec)
	return h.heal(ctx, status)
}

// OnServiceQuarantined is called when a service transitions to Quarantined.
// It escalates directly to user notification.
func (h *DefaultHealer) OnServiceQuarantined(ctx context.Context, rec ServiceRecord) error {
	h.mu.Lock()
	// Force escalation for quarantined services.
	h.attemptTracker[rec.ServiceName] = 4
	h.mu.Unlock()

	status := serviceStatusFromRecord(rec)
	return h.heal(ctx, status)
}

// OnServiceRecovered is called when a service returns to Healthy.
// It resets the attempt tracker and logs the recovery.
func (h *DefaultHealer) OnServiceRecovered(ctx context.Context, rec ServiceRecord) error {
	h.mu.Lock()
	delete(h.attemptTracker, rec.ServiceName)
	h.mu.Unlock()

	h.logger.InfoContext(ctx, "healing: service recovered, resetting attempt tracker",
		slog.String("service", rec.ServiceName),
	)

	payload, _ := json.Marshal(map[string]string{
		"service": rec.ServiceName,
		"status":  "recovered",
	})
	_ = events.NewEvent(events.EventKindHealingResolved, "", "health", payload)

	return nil
}

func (h *DefaultHealer) heal(ctx context.Context, status ServiceStatus) error {
	action := h.strategy.Diagnose(ctx, status)

	h.logger.InfoContext(ctx, "healing: executing action",
		slog.String("service", status.ServiceName),
		slog.String("strategy", string(action.Strategy)),
		slog.Int("attempt", action.AttemptNumber),
	)

	start := time.Now()
	err := h.strategy.Execute(ctx, action)
	duration := time.Since(start)

	result := HealingResult{
		Success:  err == nil,
		Duration: duration,
	}

	if err != nil {
		result.Message = fmt.Sprintf("healing action failed: %v", err)
		// If not yet at max attempts, prepare the next action.
		if action.AttemptNumber < action.MaxAttempts {
			next := h.strategy.Diagnose(ctx, status)
			result.NextAction = &next
		}
	} else {
		result.Message = fmt.Sprintf("healing action succeeded: %s", action.Strategy)
	}

	h.log.Append(HealingLogEntry{
		Action:    action,
		Result:    result,
		Timestamp: time.Now().UTC(),
	})

	// Emit event for audit trail.
	payload, _ := json.Marshal(map[string]any{
		"service":  action.ServiceID,
		"strategy": action.Strategy,
		"attempt":  action.AttemptNumber,
		"success":  result.Success,
		"message":  result.Message,
		"duration": duration.String(),
	})
	_ = events.NewEvent(events.EventKindHealingTriggered, "", "health", payload)

	if err != nil {
		h.logger.WarnContext(ctx, "healing: action failed",
			slog.String("service", status.ServiceName),
			slog.String("strategy", string(action.Strategy)),
			slog.String("error", err.Error()),
		)
	} else {
		h.logger.InfoContext(ctx, "healing: action succeeded",
			slog.String("service", status.ServiceName),
			slog.String("strategy", string(action.Strategy)),
		)
	}

	return err
}

// escalatingStrategy implements HealingStrategy with the four-step escalation.
type escalatingStrategy struct {
	healer *DefaultHealer
}

func (s *escalatingStrategy) Diagnose(_ context.Context, status ServiceStatus) HealingAction {
	s.healer.mu.Lock()
	attempt := s.healer.attemptTracker[status.ServiceName] + 1
	s.healer.attemptTracker[status.ServiceName] = attempt
	s.healer.mu.Unlock()

	action := HealingAction{
		ID:            generateActionID(),
		ServiceID:     status.ServiceName,
		AttemptNumber: attempt,
		MaxAttempts:   4,
		CreatedAt:     time.Now().UTC(),
	}

	switch {
	case attempt <= 1:
		action.Strategy = StrategyRetry
		action.Description = fmt.Sprintf("Retry failed operation for service %s", status.ServiceName)
	case attempt == 2:
		action.Strategy = StrategyRestart
		action.Description = fmt.Sprintf("Restart service/connector %s", status.ServiceName)
	case attempt == 3:
		action.Strategy = StrategyConfigRestore
		action.Description = fmt.Sprintf("Restore last known good config for %s", status.ServiceName)
	default:
		action.Strategy = StrategyEscalate
		action.Description = fmt.Sprintf("Escalate to user: service %s unrecoverable after %d attempts", status.ServiceName, attempt)
	}

	return action
}

func (s *escalatingStrategy) Execute(ctx context.Context, action HealingAction) error {
	switch action.Strategy {
	case StrategyRetry:
		s.healer.logger.InfoContext(ctx, "healing: retrying failed operation",
			slog.String("service", action.ServiceID),
		)
		// In a real implementation, this would re-invoke the failed operation.
		return nil

	case StrategyRestart:
		s.healer.logger.InfoContext(ctx, "healing: restarting service",
			slog.String("service", action.ServiceID),
		)
		// In a real implementation, this would signal the orchestrator to
		// restart the service process or container.
		return nil

	case StrategyConfigRestore:
		s.healer.logger.InfoContext(ctx, "healing: restoring last known good config",
			slog.String("service", action.ServiceID),
		)
		// In a real implementation, this would restore a checkpointed
		// configuration snapshot.
		return nil

	case StrategyEscalate:
		s.healer.logger.WarnContext(ctx, "healing: escalating to user notification",
			slog.String("service", action.ServiceID),
			slog.Int("attempt", action.AttemptNumber),
		)
		// In a real implementation, this would send a notification through
		// the user's configured channels (email, Slack, etc.).
		return nil

	default:
		return fmt.Errorf("unknown healing strategy: %s", action.Strategy)
	}
}

func serviceStatusFromRecord(rec ServiceRecord) ServiceStatus {
	return ServiceStatus{
		ServiceName:   rec.ServiceName,
		State:         rec.State,
		MissedBeats:   rec.MissedBeats,
		HealingCount:  rec.HealingCount,
		LastHeartbeat: rec.LastHeartbeat,
		Metrics:       rec.Metrics,
	}
}

func generateActionID() string {
	return fmt.Sprintf("heal-%s", time.Now().UTC().Format("20060102150405.000000000"))
}
