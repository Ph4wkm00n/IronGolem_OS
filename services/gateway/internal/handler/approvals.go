package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

// ApprovalHandler holds dependencies for approval HTTP handlers.
type ApprovalHandler struct {
	logger     *slog.Logger
	store      ApprovalStore
	eventStore *InMemoryEventStore
}

// NewApprovalHandler creates an ApprovalHandler with the given dependencies.
func NewApprovalHandler(logger *slog.Logger, store ApprovalStore, eventStore *InMemoryEventStore) *ApprovalHandler {
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
		"approval_id": approval.ID,
		"recipe_id":   approval.RecipeID,
		"step_id":     approval.StepID,
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
