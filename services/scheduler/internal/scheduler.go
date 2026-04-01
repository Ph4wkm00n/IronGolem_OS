// Package internal implements the job scheduler for IronGolem OS.
//
// The scheduler supports one-time and recurring (cron-pattern) jobs with
// a state machine: Pending -> Running -> Completed | Failed. Jobs can also
// be cancelled at any point before completion.
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
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

// JobKind differentiates one-time jobs from recurring ones.
type JobKind string

const (
	JobKindOneTime   JobKind = "one_time"
	JobKindRecurring JobKind = "recurring"
)

// Job represents a scheduled unit of work.
type Job struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Name        string         `json:"name"`
	Kind        JobKind        `json:"kind"`
	State       JobState       `json:"state"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	ScheduledAt time.Time      `json:"scheduled_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	Retries     int            `json:"retries"`
	MaxRetries  int            `json:"max_retries"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Scheduler manages the job queue and dispatches jobs when their scheduled
// time arrives.
type Scheduler struct {
	mu     sync.RWMutex
	jobs   map[string]*Job
	logger *slog.Logger
	nextID int64
	stopCh chan struct{}
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		jobs:   make(map[string]*Job),
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// CreateJob adds a new job to the queue.
func (s *Scheduler) CreateJob(job Job) (*Job, error) {
	if job.TenantID == "" {
		return nil, errors.New("tenant_id is required")
	}
	if job.Name == "" {
		return nil, errors.New("name is required")
	}
	if job.Kind == JobKindRecurring && job.CronExpr == "" {
		return nil, errors.New("cron_expr is required for recurring jobs")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	job.ID = fmt.Sprintf("job_%d", s.nextID)
	job.State = JobStatePending
	job.CreatedAt = time.Now().UTC()

	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = job.CreatedAt
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}

	s.jobs[job.ID] = &job

	s.logger.Info("job created",
		slog.String("job_id", job.ID),
		slog.String("name", job.Name),
		slog.String("kind", string(job.Kind)),
		slog.String("tenant_id", job.TenantID),
	)

	return &job, nil
}

// GetJob returns a job by ID.
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *job
	return &cp, true
}

// ListJobs returns all jobs, optionally filtered by tenant and state.
func (s *Scheduler) ListJobs(tenantID string, state JobState) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Job
	for _, j := range s.jobs {
		if tenantID != "" && j.TenantID != tenantID {
			continue
		}
		if state != "" && j.State != state {
			continue
		}
		result = append(result, *j)
	}
	return result
}

// CancelJob transitions a pending or running job to cancelled.
func (s *Scheduler) CancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return errors.New("job not found")
	}

	switch job.State {
	case JobStatePending, JobStateRunning:
		job.State = JobStateCancelled
		now := time.Now().UTC()
		job.CompletedAt = &now
		s.logger.Info("job cancelled", slog.String("job_id", id))
		return nil
	default:
		return fmt.Errorf("cannot cancel job in state %s", job.State)
	}
}

// Run starts the scheduler loop that dispatches jobs when their scheduled
// time arrives. It blocks until the context is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	s.logger.Info("scheduler loop started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.dispatch(now.UTC())
		}
	}
}

// Stop signals the scheduler loop to exit.
func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
		// Already stopped.
	default:
		close(s.stopCh)
	}
}

// dispatch finds pending jobs whose scheduled time has arrived and runs them.
func (s *Scheduler) dispatch(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, job := range s.jobs {
		if job.State != JobStatePending {
			continue
		}
		if now.Before(job.ScheduledAt) {
			continue
		}

		job.State = JobStateRunning
		startedAt := now
		job.StartedAt = &startedAt

		s.logger.Info("job dispatched",
			slog.String("job_id", job.ID),
			slog.String("name", job.Name),
		)

		// Simulate job execution. In production this would delegate to a
		// worker pool that invokes the appropriate agent or recipe.
		go s.executeJob(job.ID)
	}
}

// executeJob simulates running a job to completion. A real implementation
// would invoke the Rust runtime or delegate to an agent squad.
func (s *Scheduler) executeJob(jobID string) {
	// Simulate work duration.
	time.Sleep(100 * time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job.State != JobStateRunning {
		return
	}

	now := time.Now().UTC()
	job.State = JobStateCompleted
	job.CompletedAt = &now
	job.Result = "completed successfully"

	s.logger.Info("job completed",
		slog.String("job_id", jobID),
		slog.String("name", job.Name),
	)

	// If recurring, schedule the next occurrence.
	if job.Kind == JobKindRecurring && job.CronExpr != "" {
		next, err := nextCronTime(job.CronExpr, now)
		if err != nil {
			s.logger.Error("failed to calculate next cron time",
				slog.String("job_id", jobID),
				slog.String("error", err.Error()),
			)
			return
		}

		s.nextID++
		nextJob := &Job{
			ID:          fmt.Sprintf("job_%d", s.nextID),
			TenantID:    job.TenantID,
			WorkspaceID: job.WorkspaceID,
			Name:        job.Name,
			Kind:        JobKindRecurring,
			State:       JobStatePending,
			CronExpr:    job.CronExpr,
			ScheduledAt: next,
			Payload:     job.Payload,
			MaxRetries:  job.MaxRetries,
			CreatedAt:   now,
		}
		s.jobs[nextJob.ID] = nextJob

		s.logger.Info("recurring job rescheduled",
			slog.String("job_id", nextJob.ID),
			slog.String("parent_id", jobID),
			slog.Time("next_at", next),
		)
	}
}

// nextCronTime provides a simplified cron parser that supports basic
// interval patterns like "@every 5m", "@every 1h", "@hourly", "@daily".
// A production implementation would use a full cron expression parser.
func nextCronTime(expr string, from time.Time) (time.Time, error) {
	expr = strings.TrimSpace(expr)

	switch expr {
	case "@hourly":
		return from.Add(1 * time.Hour), nil
	case "@daily":
		return from.Add(24 * time.Hour), nil
	case "@weekly":
		return from.Add(7 * 24 * time.Hour), nil
	}

	if strings.HasPrefix(expr, "@every ") {
		durationStr := strings.TrimPrefix(expr, "@every ")
		d, err := parseDuration(durationStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid @every duration %q: %w", durationStr, err)
		}
		return from.Add(d), nil
	}

	return time.Time{}, fmt.Errorf("unsupported cron expression %q", expr)
}

// parseDuration extends time.ParseDuration to also handle "d" for days.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// --- HTTP Handlers ---

// Handler provides HTTP handlers for the scheduler API.
type Handler struct {
	logger    *slog.Logger
	scheduler *Scheduler
}

// NewHandler creates a new Handler.
func NewHandler(logger *slog.Logger, sched *Scheduler) *Handler {
	return &Handler{logger: logger, scheduler: sched}
}

// HealthCheck responds with the service health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "scheduler",
		"time":    time.Now().UTC(),
	})
}

// CreateJobRequest is the request body for job creation.
type CreateJobRequest struct {
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Name        string         `json:"name"`
	Kind        JobKind        `json:"kind"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	MaxRetries  int            `json:"max_retries,omitempty"`
}

// CreateJob handles POST /api/v1/jobs.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	job := Job{
		TenantID:    req.TenantID,
		WorkspaceID: req.WorkspaceID,
		Name:        req.Name,
		Kind:        req.Kind,
		CronExpr:    req.CronExpr,
		Payload:     req.Payload,
		MaxRetries:  req.MaxRetries,
	}
	if req.ScheduledAt != nil {
		job.ScheduledAt = *req.ScheduledAt
	}

	created, err := h.scheduler.CreateJob(job)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// ListJobs handles GET /api/v1/jobs with optional query params tenant_id and state.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	state := JobState(r.URL.Query().Get("state"))

	jobs := h.scheduler.ListJobs(tenantID, state)
	if jobs == nil {
		jobs = []Job{}
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
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "job id is required",
		})
		return
	}

	job, ok := h.scheduler.GetJob(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "job not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// CancelJob handles POST /api/v1/jobs/{id}/cancel.
func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "job id is required",
		})
		return
	}

	if err := h.scheduler.CancelJob(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"job_id": id,
		"status": "cancelled",
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
