package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// ApprovalStore defines the storage interface for approval requests.
type ApprovalStore interface {
	List(page, pageSize int, statusFilter string) ([]models.ApprovalRequest, int)
	Get(id string) (models.ApprovalRequest, bool)
	Create(req models.ApprovalRequest) models.ApprovalRequest
	Approve(id, respondedBy string) (models.ApprovalRequest, bool)
	Deny(id, respondedBy, reason string) (models.ApprovalRequest, bool)
}

// InMemoryApprovalStore is a thread-safe in-memory approval store.
type InMemoryApprovalStore struct {
	mu        sync.RWMutex
	approvals map[string]models.ApprovalRequest
	order     []string
}

// NewInMemoryApprovalStore creates an empty approval store.
func NewInMemoryApprovalStore() *InMemoryApprovalStore {
	return &InMemoryApprovalStore{
		approvals: make(map[string]models.ApprovalRequest),
	}
}

// List returns a page of approvals, optionally filtered by status.
func (s *InMemoryApprovalStore) List(page, pageSize int, statusFilter string) ([]models.ApprovalRequest, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect matching items.
	var filtered []models.ApprovalRequest
	for _, id := range s.order {
		a := s.approvals[id]
		if statusFilter != "" && string(a.Status) != statusFilter {
			continue
		}
		filtered = append(filtered, a)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// Get returns an approval by ID.
func (s *InMemoryApprovalStore) Get(id string) (models.ApprovalRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.approvals[id]
	return a, ok
}

// Create adds a new approval request.
func (s *InMemoryApprovalStore) Create(req models.ApprovalRequest) models.ApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals[req.ID] = req
	s.order = append(s.order, req.ID)
	return req
}

// Approve marks an approval as approved.
func (s *InMemoryApprovalStore) Approve(id, respondedBy string) (models.ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok {
		return a, false
	}
	if a.Status != models.ApprovalStatusPending {
		return a, false
	}
	now := time.Now().UTC()
	a.Status = models.ApprovalStatusApproved
	a.RespondedAt = &now
	a.RespondedBy = respondedBy
	s.approvals[id] = a
	return a, true
}

// Deny marks an approval as denied with a reason.
func (s *InMemoryApprovalStore) Deny(id, respondedBy, reason string) (models.ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok {
		return a, false
	}
	if a.Status != models.ApprovalStatusPending {
		return a, false
	}
	now := time.Now().UTC()
	a.Status = models.ApprovalStatusDenied
	a.RespondedAt = &now
	a.RespondedBy = respondedBy
	a.Reason = reason
	s.approvals[id] = a
	return a, true
}

// SQLiteApprovalStore persists approval requests in the gateway's shared
// SQLite database. Body is stored as JSON; status is replicated to its
// own column for index-friendly filtering.
type SQLiteApprovalStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteApprovalStore wraps the shared *sql.DB. No seeding — approvals
// only exist after Create is called.
func NewSQLiteApprovalStore(db *sql.DB, logger *slog.Logger) *SQLiteApprovalStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SQLiteApprovalStore{db: db, logger: logger.With(slog.String("component", "approval_store"))}
}

// List returns a page of approvals newest-first, optionally filtered by status.
func (s *SQLiteApprovalStore) List(page, pageSize int, statusFilter string) ([]models.ApprovalRequest, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	var (
		clauses []string
		args    []any
	)
	if statusFilter != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, statusFilter)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_approvals"+where, args...).Scan(&total); err != nil {
		s.logger.Warn("approval count failed", slog.String("error", err.Error()))
		return nil, 0
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		"SELECT body FROM gateway_approvals"+where+" ORDER BY seq DESC LIMIT ? OFFSET ?",
		append(append([]any{}, args...), pageSize, offset)...,
	)
	if err != nil {
		s.logger.Warn("approval list failed", slog.String("error", err.Error()))
		return nil, total
	}
	defer rows.Close()

	var out []models.ApprovalRequest
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			continue
		}
		var a models.ApprovalRequest
		if err := json.Unmarshal([]byte(body), &a); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, total
}

// Get returns an approval by id.
func (s *SQLiteApprovalStore) Get(id string) (models.ApprovalRequest, bool) {
	var body string
	if err := s.db.QueryRow("SELECT body FROM gateway_approvals WHERE id = ?", id).Scan(&body); err != nil {
		return models.ApprovalRequest{}, false
	}
	var a models.ApprovalRequest
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		return models.ApprovalRequest{}, false
	}
	return a, true
}

// Create inserts a new approval request. The caller is expected to set
// the ID and RequestedAt fields before calling.
func (s *SQLiteApprovalStore) Create(req models.ApprovalRequest) models.ApprovalRequest {
	body, err := json.Marshal(req)
	if err != nil {
		s.logger.Warn("approval marshal failed", slog.String("error", err.Error()))
		return req
	}
	// seq monotonically increases via the row count; the index on
	// (status, seq) keeps listing performant.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(
		`INSERT OR REPLACE INTO gateway_approvals (id, status, body, created_at, updated_at, seq)
		 VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM gateway_approvals))`,
		req.ID, string(req.Status), string(body), now, now,
	); err != nil {
		s.logger.Warn("approval insert failed", slog.String("error", err.Error()))
	}
	return req
}

// Approve transitions a pending request to approved.
func (s *SQLiteApprovalStore) Approve(id, respondedBy string) (models.ApprovalRequest, bool) {
	return s.transition(id, models.ApprovalStatusApproved, respondedBy, "")
}

// Deny transitions a pending request to denied with a reason.
func (s *SQLiteApprovalStore) Deny(id, respondedBy, reason string) (models.ApprovalRequest, bool) {
	return s.transition(id, models.ApprovalStatusDenied, respondedBy, reason)
}

func (s *SQLiteApprovalStore) transition(id string, target models.ApprovalStatus, respondedBy, reason string) (models.ApprovalRequest, bool) {
	a, ok := s.Get(id)
	if !ok {
		return models.ApprovalRequest{}, false
	}
	if a.Status != models.ApprovalStatusPending {
		return a, false
	}
	now := time.Now().UTC()
	a.Status = target
	a.RespondedAt = &now
	a.RespondedBy = respondedBy
	if target == models.ApprovalStatusDenied {
		a.Reason = reason
	}
	body, err := json.Marshal(a)
	if err != nil {
		s.logger.Warn("approval marshal failed", slog.String("error", err.Error()))
		return a, false
	}
	if _, err := s.db.Exec(
		"UPDATE gateway_approvals SET status = ?, body = ?, updated_at = ? WHERE id = ?",
		string(target), string(body), now.UTC().Format(time.RFC3339Nano), id,
	); err != nil {
		s.logger.Warn("approval update failed", slog.String("error", err.Error()))
		return a, false
	}
	return a, true
}

// ApprovalHandler holds dependencies for approval HTTP handlers.
type ApprovalHandler struct {
	logger     *slog.Logger
	store      ApprovalStore
	eventStore EventStore
}

// NewApprovalHandler creates an ApprovalHandler with the given dependencies.
func NewApprovalHandler(logger *slog.Logger, store ApprovalStore, eventStore EventStore) *ApprovalHandler {
	return &ApprovalHandler{
		logger:     logger,
		store:      store,
		eventStore: eventStore,
	}
}

// ListApprovals handles GET /api/v1/approvals.
func (h *ApprovalHandler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.approvals.list")
	defer span.End(h.logger)

	page, pageSize := parsePagination(r)
	statusFilter := r.URL.Query().Get("status")

	approvals, total := h.store.List(page, pageSize, statusFilter)

	h.logger.InfoContext(ctx, "approvals listed",
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
		slog.String("status_filter", statusFilter),
		slog.Int("total", total),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"approvals": approvals,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetApproval handles GET /api/v1/approvals/{id}.
func (h *ApprovalHandler) GetApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "approval id is required",
		})
		return
	}

	approval, ok := h.store.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "approval not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, approval)
}

// approveRequest is the optional body for an approve action.
type approveRequest struct {
	RespondedBy string `json:"responded_by"`
}

// ApproveAction handles POST /api/v1/approvals/{id}/approve.
func (h *ApprovalHandler) ApproveAction(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.approvals.approve")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "approval id is required",
		})
		return
	}

	var req approveRequest
	// Body is optional; ignore decode errors for empty bodies.
	_ = json.NewDecoder(r.Body).Decode(&req)

	respondedBy := req.RespondedBy
	if respondedBy == "" {
		respondedBy = "anonymous"
	}

	approval, ok := h.store.Approve(id, respondedBy)
	if !ok {
		// Distinguish between not-found and already-responded.
		if _, exists := h.store.Get(id); !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "approval not found",
			})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "approval is not in pending state",
			})
		}
		return
	}

	// Emit approval event.
	payload, _ := json.Marshal(map[string]string{
		"approval_id":  approval.ID,
		"recipe_id":    approval.RecipeID,
		"step_id":      approval.StepID,
		"responded_by": respondedBy,
	})
	evt := events.NewEvent(events.EventKindApprovalApproved, "system", "gateway", payload)
	h.eventStore.Append(evt)

	h.logger.InfoContext(ctx, "approval approved",
		slog.String("approval_id", approval.ID),
		slog.String("responded_by", respondedBy),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"approval": approval,
		"status":   "approved",
	})
}

// denyRequest is the body for a deny action.
type denyRequest struct {
	RespondedBy string `json:"responded_by"`
	Reason      string `json:"reason"`
}

// DenyAction handles POST /api/v1/approvals/{id}/deny.
func (h *ApprovalHandler) DenyAction(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.approvals.deny")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "approval id is required",
		})
		return
	}

	var req denyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "reason is required when denying an approval",
		})
		return
	}

	respondedBy := req.RespondedBy
	if respondedBy == "" {
		respondedBy = "anonymous"
	}

	approval, ok := h.store.Deny(id, respondedBy, req.Reason)
	if !ok {
		if _, exists := h.store.Get(id); !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "approval not found",
			})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "approval is not in pending state",
			})
		}
		return
	}

	// Emit denial event.
	payload, _ := json.Marshal(map[string]string{
		"approval_id":  approval.ID,
		"recipe_id":    approval.RecipeID,
		"step_id":      approval.StepID,
		"responded_by": respondedBy,
		"reason":       req.Reason,
	})
	evt := events.NewEvent(events.EventKindApprovalDenied, "system", "gateway", payload)
	h.eventStore.Append(evt)

	h.logger.InfoContext(ctx, "approval denied",
		slog.String("approval_id", approval.ID),
		slog.String("responded_by", respondedBy),
		slog.String("reason", req.Reason),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"approval": approval,
		"status":   "denied",
	})
}
