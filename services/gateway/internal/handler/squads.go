package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// Squad event kinds for the audit trail.
const (
	EventKindSquadCreated   events.EventKind = "squad.created"
	EventKindSquadActivated events.EventKind = "squad.activated"
	EventKindSquadPaused    events.EventKind = "squad.paused"
	EventKindSquadRunStart  events.EventKind = "squad.run_started"
)

// SquadStore defines the interface for squad persistence.
type SquadStore interface {
	List(page, pageSize int) ([]models.Squad, int)
	GetByID(id string) (models.Squad, bool)
	Create(squad models.Squad) (models.Squad, error)
	Activate(id string) (models.Squad, error)
	Pause(id string) (models.Squad, error)
	RecordRun(id string) (models.Squad, error)
}

// InMemorySquadStore is an in-memory implementation of SquadStore
// pre-populated with the five built-in squads.
type InMemorySquadStore struct {
	mu     sync.RWMutex
	squads map[string]models.Squad
	order  []string
}

// NewInMemorySquadStore creates a store pre-loaded with the five built-in
// squad templates instantiated into a default workspace.
func NewInMemorySquadStore() *InMemorySquadStore {
	templates := models.AllSquadTemplates()
	store := &InMemorySquadStore{
		squads: make(map[string]models.Squad, len(templates)),
	}
	for _, tmpl := range templates {
		squad := models.SquadFromTemplate(tmpl, "default")
		store.squads[squad.ID] = squad
		store.order = append(store.order, squad.ID)
	}
	return store
}

// List returns a paginated slice of squads and the total count.
func (s *InMemorySquadStore) List(page, pageSize int) ([]models.Squad, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.order)
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	result := make([]models.Squad, 0, end-start)
	for _, id := range s.order[start:end] {
		result = append(result, s.squads[id])
	}
	return result, total
}

// GetByID returns a single squad by its ID.
func (s *InMemorySquadStore) GetByID(id string) (models.Squad, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sq, ok := s.squads[id]
	return sq, ok
}

// Create adds a new squad to the store.
func (s *InMemorySquadStore) Create(squad models.Squad) (models.Squad, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.squads[squad.ID]; exists {
		return models.Squad{}, errAlreadyExists
	}

	now := time.Now().UTC()
	squad.CreatedAt = now
	squad.UpdatedAt = now
	s.squads[squad.ID] = squad
	s.order = append(s.order, squad.ID)
	return squad, nil
}

// Activate sets a squad's IsActive flag to true.
func (s *InMemorySquadStore) Activate(id string) (models.Squad, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sq, ok := s.squads[id]
	if !ok {
		return models.Squad{}, errNotFound
	}
	sq.IsActive = true
	sq.UpdatedAt = time.Now().UTC()
	s.squads[id] = sq
	return sq, nil
}

// Pause sets a squad's IsActive flag to false.
func (s *InMemorySquadStore) Pause(id string) (models.Squad, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sq, ok := s.squads[id]
	if !ok {
		return models.Squad{}, errNotFound
	}
	sq.IsActive = false
	sq.UpdatedAt = time.Now().UTC()
	s.squads[id] = sq
	return sq, nil
}

// RecordRun updates the LastRunAt timestamp for a squad.
func (s *InMemorySquadStore) RecordRun(id string) (models.Squad, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sq, ok := s.squads[id]
	if !ok {
		return models.Squad{}, errNotFound
	}
	if !sq.IsActive {
		return models.Squad{}, errSquadNotActive
	}
	now := time.Now().UTC()
	sq.LastRunAt = &now
	sq.UpdatedAt = now
	s.squads[id] = sq
	return sq, nil
}

// SQLiteSquadStore persists squads in the gateway's shared SQLite DB.
// On first open it seeds the built-in templates instantiated into the
// default workspace — mirrors the in-memory store's boot behavior.
type SQLiteSquadStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteSquadStore wraps the shared *sql.DB. Seeds built-ins on first run.
func NewSQLiteSquadStore(db *sql.DB, logger *slog.Logger) (*SQLiteSquadStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &SQLiteSquadStore{db: db, logger: logger.With(slog.String("component", "squad_store"))}
	if err := s.seedIfEmpty(); err != nil {
		return nil, fmt.Errorf("squad store seed: %w", err)
	}
	return s, nil
}

func (s *SQLiteSquadStore) seedIfEmpty() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_squads").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	templates := models.AllSquadTemplates()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, tmpl := range templates {
		sq := models.SquadFromTemplate(tmpl, "default")
		body, err := json.Marshal(sq)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		active := 0
		if sq.IsActive {
			active = 1
		}
		if _, err := tx.Exec(
			"INSERT INTO gateway_squads (id, is_active, body, created_at, updated_at, seq) VALUES (?, ?, ?, ?, ?, ?)",
			sq.ID, active, string(body), now, now, i,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// List returns a page of squads ordered by seed/insertion sequence.
func (s *SQLiteSquadStore) List(page, pageSize int) ([]models.Squad, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_squads").Scan(&total); err != nil {
		s.logger.Warn("squad count failed", slog.String("error", err.Error()))
		return nil, 0
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query("SELECT body FROM gateway_squads ORDER BY seq ASC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		s.logger.Warn("squad list failed", slog.String("error", err.Error()))
		return nil, total
	}
	defer rows.Close()

	var out []models.Squad
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			continue
		}
		var sq models.Squad
		if err := json.Unmarshal([]byte(body), &sq); err != nil {
			continue
		}
		out = append(out, sq)
	}
	return out, total
}

// GetByID returns a single squad by id.
func (s *SQLiteSquadStore) GetByID(id string) (models.Squad, bool) {
	var body string
	if err := s.db.QueryRow("SELECT body FROM gateway_squads WHERE id = ?", id).Scan(&body); err != nil {
		return models.Squad{}, false
	}
	var sq models.Squad
	if err := json.Unmarshal([]byte(body), &sq); err != nil {
		return models.Squad{}, false
	}
	return sq, true
}

// Create inserts a new squad. Returns errAlreadyExists if id is taken.
func (s *SQLiteSquadStore) Create(squad models.Squad) (models.Squad, error) {
	if _, ok := s.GetByID(squad.ID); ok {
		return models.Squad{}, errAlreadyExists
	}
	now := time.Now().UTC()
	squad.CreatedAt = now
	squad.UpdatedAt = now
	body, err := json.Marshal(squad)
	if err != nil {
		return models.Squad{}, err
	}
	active := 0
	if squad.IsActive {
		active = 1
	}
	if _, err := s.db.Exec(
		`INSERT INTO gateway_squads (id, is_active, body, created_at, updated_at, seq)
		 VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM gateway_squads))`,
		squad.ID, active, string(body), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	); err != nil {
		return models.Squad{}, err
	}
	return squad, nil
}

// Activate flips is_active=1.
func (s *SQLiteSquadStore) Activate(id string) (models.Squad, error) {
	return s.setActive(id, true)
}

// Pause flips is_active=0.
func (s *SQLiteSquadStore) Pause(id string) (models.Squad, error) {
	return s.setActive(id, false)
}

// RecordRun updates the LastRunAt timestamp.
func (s *SQLiteSquadStore) RecordRun(id string) (models.Squad, error) {
	sq, ok := s.GetByID(id)
	if !ok {
		return models.Squad{}, errNotFound
	}
	if !sq.IsActive {
		return models.Squad{}, errSquadNotActive
	}
	now := time.Now().UTC()
	sq.LastRunAt = &now
	sq.UpdatedAt = now
	body, err := json.Marshal(sq)
	if err != nil {
		return models.Squad{}, err
	}
	if _, err := s.db.Exec(
		"UPDATE gateway_squads SET body = ?, updated_at = ? WHERE id = ?",
		string(body), now.Format(time.RFC3339Nano), id,
	); err != nil {
		return models.Squad{}, err
	}
	return sq, nil
}

func (s *SQLiteSquadStore) setActive(id string, active bool) (models.Squad, error) {
	sq, ok := s.GetByID(id)
	if !ok {
		return models.Squad{}, errNotFound
	}
	sq.IsActive = active
	sq.UpdatedAt = time.Now().UTC()
	body, err := json.Marshal(sq)
	if err != nil {
		return models.Squad{}, err
	}
	flag := 0
	if active {
		flag = 1
	}
	if _, err := s.db.Exec(
		"UPDATE gateway_squads SET is_active = ?, body = ?, updated_at = ? WHERE id = ?",
		flag, string(body), sq.UpdatedAt.Format(time.RFC3339Nano), id,
	); err != nil {
		return models.Squad{}, err
	}
	return sq, nil
}

// errAlreadyExists is returned when attempting to create a duplicate resource.
var errAlreadyExists = errorString("already exists")

// errSquadNotActive is returned when a run is triggered on an inactive squad.
var errSquadNotActive = errorString("squad is not active")

// errorString is a simple error type that avoids sentinel error issues.
type errorString string

func (e errorString) Error() string { return string(e) }

// CreateSquadRequest is the request body for creating a squad from a template.
type CreateSquadRequest struct {
	TemplateID  string `json:"template_id"`
	WorkspaceID string `json:"workspace_id"`
}

// SquadHandler holds the dependencies for squad HTTP handlers.
type SquadHandler struct {
	logger     *slog.Logger
	store      SquadStore
	eventStore EventStore
}

// NewSquadHandler creates a SquadHandler with the given store and event store.
func NewSquadHandler(logger *slog.Logger, store SquadStore, eventStore EventStore) *SquadHandler {
	return &SquadHandler{
		logger:     logger,
		store:      store,
		eventStore: eventStore,
	}
}

// ListSquads handles GET /api/v1/squads.
func (sh *SquadHandler) ListSquads(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.list_squads")
	defer span.End(sh.logger)

	page, pageSize := parsePagination(r)

	squads, total := sh.store.List(page, pageSize)

	sh.logger.InfoContext(ctx, "squads listed",
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
		slog.Int("total", total),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"squads":    squads,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetSquad handles GET /api/v1/squads/{id}.
func (sh *SquadHandler) GetSquad(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "squad id is required",
		})
		return
	}

	squad, ok := sh.store.GetByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "squad not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, squad)
}

// CreateSquad handles POST /api/v1/squads.
func (sh *SquadHandler) CreateSquad(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.create_squad")
	defer span.End(sh.logger)

	var req CreateSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.TemplateID == "" || req.WorkspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "template_id and workspace_id are required",
		})
		return
	}

	// Find the matching template.
	var tmpl *models.SquadTemplate
	for _, t := range models.AllSquadTemplates() {
		if t.ID == req.TemplateID {
			t := t // capture loop var
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "unknown template_id",
		})
		return
	}

	squad := models.SquadFromTemplate(*tmpl, req.WorkspaceID)
	// Generate a unique ID for workspace-scoped squads.
	squad.ID = tmpl.ID + "-" + req.WorkspaceID

	created, err := sh.store.Create(squad)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": err.Error(),
		})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"squad_id":   created.ID,
		"squad_name": created.Name,
		"template":   req.TemplateID,
	})
	evt := events.NewEvent(EventKindSquadCreated, "", "gateway", payload)
	sh.eventStore.Append(evt)

	sh.logger.InfoContext(ctx, "squad created",
		slog.String("squad_id", created.ID),
		slog.String("template", req.TemplateID),
		slog.String("workspace_id", req.WorkspaceID),
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"squad":    created,
		"event_id": evt.ID,
		"status":   "created",
	})
}

// ActivateSquad handles POST /api/v1/squads/{id}/activate.
func (sh *SquadHandler) ActivateSquad(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.activate_squad")
	defer span.End(sh.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "squad id is required",
		})
		return
	}

	squad, err := sh.store.Activate(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "squad not found",
		})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"squad_id":   squad.ID,
		"squad_name": squad.Name,
	})
	evt := events.NewEvent(EventKindSquadActivated, "", "gateway", payload)
	sh.eventStore.Append(evt)

	sh.logger.InfoContext(ctx, "squad activated",
		slog.String("squad_id", squad.ID),
		slog.String("squad_name", squad.Name),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"squad":    squad,
		"event_id": evt.ID,
		"status":   "activated",
	})
}

// PauseSquad handles POST /api/v1/squads/{id}/pause.
func (sh *SquadHandler) PauseSquad(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.pause_squad")
	defer span.End(sh.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "squad id is required",
		})
		return
	}

	squad, err := sh.store.Pause(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "squad not found",
		})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"squad_id":   squad.ID,
		"squad_name": squad.Name,
	})
	evt := events.NewEvent(EventKindSquadPaused, "", "gateway", payload)
	sh.eventStore.Append(evt)

	sh.logger.InfoContext(ctx, "squad paused",
		slog.String("squad_id", squad.ID),
		slog.String("squad_name", squad.Name),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"squad":    squad,
		"event_id": evt.ID,
		"status":   "paused",
	})
}

// RunSquad handles POST /api/v1/squads/{id}/run.
func (sh *SquadHandler) RunSquad(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.run_squad")
	defer span.End(sh.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "squad id is required",
		})
		return
	}

	squad, err := sh.store.RecordRun(id)
	if err != nil {
		status := http.StatusNotFound
		msg := "squad not found"
		if err == errSquadNotActive {
			status = http.StatusConflict
			msg = "squad is not active; activate it before running"
		}
		writeJSON(w, status, map[string]string{
			"error": msg,
		})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"squad_id":   squad.ID,
		"squad_name": squad.Name,
	})
	evt := events.NewEvent(EventKindSquadRunStart, "", "gateway", payload)
	sh.eventStore.Append(evt)

	sh.logger.InfoContext(ctx, "squad run triggered",
		slog.String("squad_id", squad.ID),
		slog.String("squad_name", squad.Name),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"squad":    squad,
		"event_id": evt.ID,
		"status":   "run_started",
	})
}
