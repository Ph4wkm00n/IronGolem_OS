// Package internal implements the core scheduling engine for IronGolem OS.
//
// The scheduler supports one-time and recurring jobs (cron expressions and
// fixed intervals). Every job transition produces an event for the audit
// trail, and all operations propagate context for tracing and cancellation.
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// JobState represents the lifecycle state of a scheduled job.
type JobState string

const (
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

// ScheduleKind determines how a job is triggered.
type ScheduleKind string

const (
	ScheduleOnce     ScheduleKind = "once"
	ScheduleCron     ScheduleKind = "cron"
	ScheduleInterval ScheduleKind = "interval"
)

// Job represents a scheduled unit of work.
type Job struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	WorkspaceID  string            `json:"workspace_id,omitempty"`
	Name         string            `json:"name"`
	State        JobState          `json:"state"`
	ScheduleKind ScheduleKind      `json:"schedule_kind"`
	CronExpr     string            `json:"cron_expr,omitempty"`
	Interval     time.Duration     `json:"interval_ns,omitempty"`
	Payload      json.RawMessage   `json:"payload,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	ScheduledAt  time.Time         `json:"scheduled_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	LastRunAt    *time.Time        `json:"last_run_at,omitempty"`
	RunCount     int               `json:"run_count"`
	MaxRetries   int               `json:"max_retries"`
	RetryCount   int               `json:"retry_count"`
	Error        string            `json:"error,omitempty"`
}

// IsTerminal returns true if the job is in a final state.
func (j *Job) IsTerminal() bool {
	return j.State == JobStateCompleted || j.State == JobStateFailed || j.State == JobStateCancelled
}

// JobStore defines the persistence interface for jobs. Implementations must
// be safe for concurrent access.
type JobStore interface {
	// Save persists a job, creating or updating as needed.
	Save(ctx context.Context, job *Job) error

	// Get retrieves a job by ID. Returns an error if not found.
	Get(ctx context.Context, id string) (*Job, error)

	// List returns all jobs, optionally filtered by state.
	List(ctx context.Context, stateFilter *JobState) ([]*Job, error)

	// ListDue returns jobs whose ScheduledAt is at or before the given time
	// and whose state is Pending.
	ListDue(ctx context.Context, now time.Time) ([]*Job, error)

	// Delete removes a job by ID.
	Delete(ctx context.Context, id string) error
}

// MemoryJobStore is an in-memory implementation of JobStore, suitable for
// development and single-instance deployments.
type MemoryJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemoryJobStore creates an empty in-memory job store.
func NewMemoryJobStore() *MemoryJobStore {
	return &MemoryJobStore{
		jobs: make(map[string]*Job),
	}
}

func (s *MemoryJobStore) Save(_ context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external mutation.
	cp := *job
	s.jobs[job.ID] = &cp
	return nil
}

func (s *MemoryJobStore) Get(_ context.Context, id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	cp := *job
	return &cp, nil
}

func (s *MemoryJobStore) List(_ context.Context, stateFilter *JobState) ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if stateFilter != nil && job.State != *stateFilter {
			continue
		}
		cp := *job
		result = append(result, &cp)
	}
	return result, nil
}

func (s *MemoryJobStore) ListDue(_ context.Context, now time.Time) ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Job
	for _, job := range s.jobs {
		if job.State == JobStatePending && !job.ScheduledAt.After(now) {
			cp := *job
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *MemoryJobStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("job %q not found", id)
	}
	delete(s.jobs, id)
	return nil
}

// JobExecutor is called by the scheduler to actually run a job. Implementations
// dispatch work to the appropriate agent, recipe, or maintenance routine.
type JobExecutor interface {
	Execute(ctx context.Context, job *Job) error
}

// Scheduler orchestrates job scheduling, due-job detection, and execution.
type Scheduler struct {
	store    JobStore
	executor JobExecutor
	logger   *slog.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	tick     time.Duration
}

// NewScheduler creates a Scheduler with an in-memory store and a default
// no-op executor. Use SetExecutor to provide a real implementation.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:    NewMemoryJobStore(),
		executor: &noopExecutor{},
		logger:   logger,
		stopCh:   make(chan struct{}),
		tick:     5 * time.Second,
	}
}

// SetExecutor replaces the job executor.
func (s *Scheduler) SetExecutor(exec JobExecutor) {
	s.executor = exec
}

// SetStore replaces the job store (useful for testing or swapping backends).
func (s *Scheduler) SetStore(store JobStore) {
	s.store = store
}

// SubmitJob validates and persists a new job, returning its assigned ID.
func (s *Scheduler) SubmitJob(ctx context.Context, job *Job) error {
	if job.Name == "" {
		return errors.New("job name is required")
	}
	if job.TenantID == "" {
		return errors.New("tenant_id is required")
	}
	if job.ScheduleKind == "" {
		job.ScheduleKind = ScheduleOnce
	}
	if job.ScheduleKind == ScheduleInterval && job.Interval <= 0 {
		return errors.New("interval must be positive for interval-scheduled jobs")
	}
	if job.ScheduleKind == ScheduleCron && job.CronExpr == "" {
		return errors.New("cron_expr is required for cron-scheduled jobs")
	}

	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = generateID()
	}
	job.State = JobStatePending
	job.CreatedAt = now
	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = now
	}

	if err := s.store.Save(ctx, job); err != nil {
		return fmt.Errorf("saving job: %w", err)
	}

	// Emit event.
	payload, _ := json.Marshal(map[string]string{
		"job_id":        job.ID,
		"name":          job.Name,
		"schedule_kind": string(job.ScheduleKind),
	})
	evt := events.NewEvent(events.EventKindJobCreated, job.TenantID, "scheduler", payload)
	s.logger.InfoContext(ctx, "job submitted",
		slog.String("event_id", evt.ID),
		slog.String("job_id", job.ID),
		slog.String("name", job.Name),
		slog.String("schedule_kind", string(job.ScheduleKind)),
	)

	return nil
}

// GetJob retrieves a job by ID.
func (s *Scheduler) GetJob(ctx context.Context, id string) (*Job, error) {
	return s.store.Get(ctx, id)
}

// ListJobs returns all jobs, optionally filtered by state.
func (s *Scheduler) ListJobs(ctx context.Context, stateFilter *JobState) ([]*Job, error) {
	return s.store.List(ctx, stateFilter)
}

// CancelJob transitions a pending or running job to cancelled.
func (s *Scheduler) CancelJob(ctx context.Context, id string) error {
	job, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if job.IsTerminal() {
		return fmt.Errorf("job %q is already in terminal state %q", id, job.State)
	}

	job.State = JobStateCancelled
	now := time.Now().UTC()
	job.CompletedAt = &now

	if err := s.store.Save(ctx, job); err != nil {
		return fmt.Errorf("saving cancelled job: %w", err)
	}

	s.logger.InfoContext(ctx, "job cancelled",
		slog.String("job_id", id),
	)
	return nil
}

// Run starts the scheduling loop. It blocks until the context is cancelled
// or Stop is called.
func (s *Scheduler) Run(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	s.logger.Info("scheduler loop started", slog.Duration("tick", s.tick))

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler loop context cancelled")
			return
		case <-s.stopCh:
			s.logger.Info("scheduler loop stop signal received")
			return
		case <-ticker.C:
			s.processDueJobs(ctx)
		}
	}
}

// Stop signals the scheduler loop to exit and waits for it to finish.
func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
		// Already closed.
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}

// processDueJobs finds and executes all jobs that are due.
func (s *Scheduler) processDueJobs(ctx context.Context) {
	now := time.Now().UTC()
	due, err := s.store.ListDue(ctx, now)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list due jobs",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, job := range due {
		s.executeJob(ctx, job)
	}
}

// executeJob transitions a job to Running, invokes the executor, and handles
// the result including rescheduling for recurring jobs.
func (s *Scheduler) executeJob(ctx context.Context, job *Job) {
	ctx, span := telemetry.NewSpan(ctx, "scheduler.execute_job")
	defer span.End(s.logger)

	// Transition to Running.
	job.State = JobStateRunning
	now := time.Now().UTC()
	job.StartedAt = &now
	if err := s.store.Save(ctx, job); err != nil {
		s.logger.ErrorContext(ctx, "failed to save running state",
			slog.String("job_id", job.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Emit started event.
	startPayload, _ := json.Marshal(map[string]string{"job_id": job.ID, "name": job.Name})
	startEvt := events.NewEvent(events.EventKindJobStarted, job.TenantID, "scheduler", startPayload)
	s.logger.InfoContext(ctx, "job started",
		slog.String("event_id", startEvt.ID),
		slog.String("job_id", job.ID),
	)

	// Execute.
	execErr := s.executor.Execute(ctx, job)

	completedAt := time.Now().UTC()
	job.CompletedAt = &completedAt
	job.LastRunAt = &completedAt
	job.RunCount++

	if execErr != nil {
		job.RetryCount++
		if job.RetryCount <= job.MaxRetries {
			// Schedule retry.
			job.State = JobStatePending
			job.ScheduledAt = completedAt.Add(time.Duration(job.RetryCount) * 10 * time.Second)
			job.Error = execErr.Error()
			s.logger.WarnContext(ctx, "job failed, scheduling retry",
				slog.String("job_id", job.ID),
				slog.Int("retry", job.RetryCount),
				slog.String("error", execErr.Error()),
			)
		} else {
			job.State = JobStateFailed
			job.Error = execErr.Error()

			failPayload, _ := json.Marshal(map[string]string{"job_id": job.ID, "error": execErr.Error()})
			failEvt := events.NewEvent(events.EventKindJobFailed, job.TenantID, "scheduler", failPayload)
			s.logger.ErrorContext(ctx, "job failed permanently",
				slog.String("event_id", failEvt.ID),
				slog.String("job_id", job.ID),
				slog.String("error", execErr.Error()),
			)
		}
	} else {
		// Handle recurring reschedule.
		switch job.ScheduleKind {
		case ScheduleInterval:
			job.State = JobStatePending
			job.ScheduledAt = completedAt.Add(job.Interval)
			job.RetryCount = 0
			s.logger.InfoContext(ctx, "interval job rescheduled",
				slog.String("job_id", job.ID),
				slog.Time("next_run", job.ScheduledAt),
			)
		case ScheduleCron:
			nextRun, parseErr := nextCronTime(job.CronExpr, completedAt)
			if parseErr != nil {
				job.State = JobStateFailed
				job.Error = fmt.Sprintf("invalid cron expression: %v", parseErr)
			} else {
				job.State = JobStatePending
				job.ScheduledAt = nextRun
				job.RetryCount = 0
				s.logger.InfoContext(ctx, "cron job rescheduled",
					slog.String("job_id", job.ID),
					slog.Time("next_run", job.ScheduledAt),
				)
			}
		default:
			job.State = JobStateCompleted
			donePayload, _ := json.Marshal(map[string]string{"job_id": job.ID})
			doneEvt := events.NewEvent(events.EventKindJobCompleted, job.TenantID, "scheduler", donePayload)
			s.logger.InfoContext(ctx, "job completed",
				slog.String("event_id", doneEvt.ID),
				slog.String("job_id", job.ID),
			)
		}
	}

	if err := s.store.Save(ctx, job); err != nil {
		s.logger.ErrorContext(ctx, "failed to save job result",
			slog.String("job_id", job.ID),
			slog.String("error", err.Error()),
		)
	}
}

// nextCronTime is a simplified cron parser that supports basic expressions.
// A production implementation would use a full cron library. This version
// supports: "@every <duration>" and common shortcuts.
func nextCronTime(expr string, after time.Time) (time.Time, error) {
	// Support "@every 5m" style expressions.
	if len(expr) > 7 && expr[:7] == "@every " {
		d, err := time.ParseDuration(expr[7:])
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing @every duration: %w", err)
		}
		return after.Add(d), nil
	}

	// Support "@hourly", "@daily" shortcuts.
	switch expr {
	case "@hourly":
		return after.Add(time.Hour), nil
	case "@daily":
		return after.Add(24 * time.Hour), nil
	case "@weekly":
		return after.Add(7 * 24 * time.Hour), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported cron expression %q; use @every, @hourly, @daily, or @weekly", expr)
	}
}

// generateID produces a simple unique identifier.
func generateID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}

// noopExecutor is the default executor that does nothing. It is replaced
// by a real executor when the scheduler is wired into the full system.
type noopExecutor struct{}

func (e *noopExecutor) Execute(_ context.Context, _ *Job) error {
	return nil
}

// --- HTTP Handler ---

// Handler exposes the scheduler over HTTP.
type Handler struct {
	logger *slog.Logger
	sched  *Scheduler
}

// NewHandler creates a new HTTP handler for the scheduler.
func NewHandler(logger *slog.Logger, sched *Scheduler) *Handler {
	return &Handler{logger: logger, sched: sched}
}

// HealthCheck responds with the scheduler service status.
func (h *Handler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "scheduler",
		"time":    time.Now().UTC(),
	})
}

// CreateJobRequest is the request body for creating a new job.
type CreateJobRequest struct {
	Name         string            `json:"name"`
	TenantID     string            `json:"tenant_id"`
	WorkspaceID  string            `json:"workspace_id,omitempty"`
	ScheduleKind ScheduleKind      `json:"schedule_kind"`
	CronExpr     string            `json:"cron_expr,omitempty"`
	IntervalSecs int               `json:"interval_secs,omitempty"`
	ScheduleAt   *time.Time        `json:"schedule_at,omitempty"`
	Payload      json.RawMessage   `json:"payload,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	MaxRetries   int               `json:"max_retries,omitempty"`
}

// CreateJob handles POST /api/v1/jobs.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "scheduler.handler.create_job")
	defer span.End(h.logger)

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	job := &Job{
		Name:         req.Name,
		TenantID:     req.TenantID,
		WorkspaceID:  req.WorkspaceID,
		ScheduleKind: req.ScheduleKind,
		CronExpr:     req.CronExpr,
		Interval:     time.Duration(req.IntervalSecs) * time.Second,
		Payload:      req.Payload,
		Metadata:     req.Metadata,
		MaxRetries:   req.MaxRetries,
	}
	if req.ScheduleAt != nil {
		job.ScheduledAt = *req.ScheduleAt
	}

	if err := h.sched.SubmitJob(ctx, job); err != nil {
		h.logger.WarnContext(ctx, "job submission failed",
			slog.String("error", err.Error()),
		)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"job_id": job.ID,
		"state":  job.State,
	})
}

// ListJobs handles GET /api/v1/jobs.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var stateFilter *JobState
	if s := r.URL.Query().Get("state"); s != "" {
		st := JobState(s)
		stateFilter = &st
	}

	jobs, err := h.sched.ListJobs(ctx, stateFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// GetJob handles GET /api/v1/jobs/{id}.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job id is required"})
		return
	}

	job, err := h.sched.GetJob(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// CancelJob handles POST /api/v1/jobs/{id}/cancel.
func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "scheduler.handler.cancel_job")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job id is required"})
		return
	}

	if err := h.sched.CancelJob(ctx, id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"job_id": id,
		"state":  string(JobStateCancelled),
	})
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
