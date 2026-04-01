package handler

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// InMemoryEventStore accumulates events from all service actions, providing
// the timeline view for the audit trail.
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events []events.Event
}

// NewInMemoryEventStore creates an empty event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{}
}

// Append adds an event to the store.
func (s *InMemoryEventStore) Append(evt events.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, evt)
}

// List returns a page of events, optionally filtered by workspace and kind.
func (s *InMemoryEventStore) List(page, pageSize int, workspaceFilter, kindFilter string) ([]events.Event, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []events.Event
	for i := len(s.events) - 1; i >= 0; i-- {
		evt := s.events[i]
		if workspaceFilter != "" && evt.WorkspaceID != workspaceFilter {
			continue
		}
		if kindFilter != "" && string(evt.Kind) != kindFilter {
			continue
		}
		filtered = append(filtered, evt)
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

// Get returns an event by ID.
func (s *InMemoryEventStore) Get(id string) (events.Event, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, evt := range s.events {
		if evt.ID == id {
			return evt, true
		}
	}
	return events.Event{}, false
}

// TimelineHandler holds dependencies for timeline/event HTTP handlers.
type TimelineHandler struct {
	logger     *slog.Logger
	eventStore *InMemoryEventStore
}

// NewTimelineHandler creates a TimelineHandler.
func NewTimelineHandler(logger *slog.Logger, eventStore *InMemoryEventStore) *TimelineHandler {
	return &TimelineHandler{
		logger:     logger,
		eventStore: eventStore,
	}
}

// ListEvents handles GET /api/v1/events.
func (h *TimelineHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.events.list")
	defer span.End(h.logger)

	page, pageSize := parsePagination(r)
	workspaceFilter := r.URL.Query().Get("workspace")
	kindFilter := r.URL.Query().Get("kind")

	evts, total := h.eventStore.List(page, pageSize, workspaceFilter, kindFilter)

	h.logger.InfoContext(ctx, "events listed",
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
		slog.String("workspace_filter", workspaceFilter),
		slog.String("kind_filter", kindFilter),
		slog.Int("total", total),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"events":    evts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetEvent handles GET /api/v1/events/{id}.
func (h *TimelineHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "event id is required",
		})
		return
	}

	evt, ok := h.eventStore.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "event not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, evt)
}
