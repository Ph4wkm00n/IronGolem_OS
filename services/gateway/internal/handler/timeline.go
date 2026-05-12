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

// SQLiteEventStore persists the audit-trail event log in the gateway's
// shared SQLite database. Schema is owned by persist.Migrate (see
// services/gateway/internal/persist/db.go).
type SQLiteEventStore struct {
	db *sql.DB
	// errLog is used for best-effort logging on persist failures. The
	// EventStore.Append signature returns no error (it's a fire-and-forget
	// audit write), so we have to surface failures via the logger.
	errLog *slog.Logger
}

// NewSQLiteEventStore wraps the shared *sql.DB. The caller is responsible
// for keeping the connection open for the gateway's lifetime; this store
// does not own the handle.
func NewSQLiteEventStore(db *sql.DB, logger *slog.Logger) *SQLiteEventStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SQLiteEventStore{db: db, errLog: logger.With(slog.String("component", "event_store"))}
}

// Append inserts the event into gateway_events. Best-effort — failures
// are logged so a missing audit row never crashes the inbound flow.
func (s *SQLiteEventStore) Append(evt events.Event) {
	payload := string(evt.Payload)
	metaJSON, _ := json.Marshal(evt.Metadata)
	if _, err := s.db.Exec(
		`INSERT OR REPLACE INTO gateway_events
			(id, kind, tenant_id, workspace_id, source_service,
			 correlation_id, causation_id, ts, payload, metadata, version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evt.ID, string(evt.Kind), evt.TenantID, evt.WorkspaceID, evt.SourceService,
		evt.CorrelationID, evt.CausationID,
		evt.Timestamp.UTC().Format(time.RFC3339Nano),
		payload, string(metaJSON), evt.Version,
	); err != nil {
		s.errLog.Warn("event append failed", slog.String("id", evt.ID), slog.String("error", err.Error()))
	}
}

// List returns a page of events newest-first, with optional workspace +
// kind filters applied at the SQL layer.
func (s *SQLiteEventStore) List(page, pageSize int, workspaceFilter, kindFilter string) ([]events.Event, int) {
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
	if workspaceFilter != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceFilter)
	}
	if kindFilter != "" {
		clauses = append(clauses, "kind = ?")
		args = append(args, kindFilter)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	// Total first so the response carries the unpaginated count.
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_events"+where, args...).Scan(&total); err != nil {
		s.errLog.Warn("event count failed", slog.String("error", err.Error()))
		return nil, 0
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		"SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, ts, payload, metadata, version "+
			"FROM gateway_events"+where+" ORDER BY ts DESC LIMIT ? OFFSET ?",
		append(append([]any{}, args...), pageSize, offset)...,
	)
	if err != nil {
		s.errLog.Warn("event list failed", slog.String("error", err.Error()))
		return nil, total
	}
	defer rows.Close()

	var out []events.Event
	for rows.Next() {
		evt, err := scanEvent(rows)
		if err != nil {
			s.errLog.Warn("event scan failed", slog.String("error", err.Error()))
			continue
		}
		out = append(out, evt)
	}
	return out, total
}

// Get returns a single event by id.
func (s *SQLiteEventStore) Get(id string) (events.Event, bool) {
	row := s.db.QueryRow(
		"SELECT id, kind, tenant_id, workspace_id, source_service, correlation_id, causation_id, ts, payload, metadata, version "+
			"FROM gateway_events WHERE id = ?",
		id,
	)
	evt, err := scanEvent(row)
	if err != nil {
		return events.Event{}, false
	}
	return evt, true
}

// rowScanner is the common interface satisfied by both *sql.Row and
// *sql.Rows. Lets scanEvent serve both Get and List paths.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(r rowScanner) (events.Event, error) {
	var (
		evt        events.Event
		kind       string
		tsString   string
		payloadStr string
		metaStr    string
	)
	if err := r.Scan(
		&evt.ID, &kind, &evt.TenantID, &evt.WorkspaceID, &evt.SourceService,
		&evt.CorrelationID, &evt.CausationID, &tsString, &payloadStr, &metaStr, &evt.Version,
	); err != nil {
		return events.Event{}, err
	}
	evt.Kind = events.EventKind(kind)
	if ts, err := time.Parse(time.RFC3339Nano, tsString); err == nil {
		evt.Timestamp = ts
	}
	if payloadStr != "" {
		evt.Payload = json.RawMessage(payloadStr)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &evt.Metadata)
	}
	return evt, nil
}

// TimelineHandler holds dependencies for timeline/event HTTP handlers.
type TimelineHandler struct {
	logger     *slog.Logger
	eventStore EventStore
}

// NewTimelineHandler creates a TimelineHandler.
func NewTimelineHandler(logger *slog.Logger, eventStore EventStore) *TimelineHandler {
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
