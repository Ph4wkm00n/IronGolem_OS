package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// RecipeStore defines the interface for recipe persistence.
type RecipeStore interface {
	List(page, pageSize int) ([]models.DetailedRecipe, int)
	GetByID(id string) (models.DetailedRecipe, bool)
	Activate(id string) (models.DetailedRecipe, error)
	Deactivate(id string) (models.DetailedRecipe, error)
}

// InMemoryRecipeStore is an in-memory implementation of RecipeStore
// pre-populated with the four built-in recipe templates.
type InMemoryRecipeStore struct {
	mu      sync.RWMutex
	recipes map[string]models.DetailedRecipe
	order   []string // maintains insertion order for listing
}

// NewInMemoryRecipeStore creates a store with the four built-in recipes.
func NewInMemoryRecipeStore() *InMemoryRecipeStore {
	builtins := []models.DetailedRecipe{
		models.EmailTriageRecipe(),
		models.CalendarManagerRecipe(),
		models.ResearchMonitorRecipe(),
		models.FilesystemOrganizerRecipe(),
	}

	store := &InMemoryRecipeStore{
		recipes: make(map[string]models.DetailedRecipe, len(builtins)),
	}
	for _, r := range builtins {
		store.recipes[r.ID] = r
		store.order = append(store.order, r.ID)
	}
	return store
}

// List returns a paginated slice of recipes and the total count.
func (s *InMemoryRecipeStore) List(page, pageSize int) ([]models.DetailedRecipe, int) {
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

	result := make([]models.DetailedRecipe, 0, end-start)
	for _, id := range s.order[start:end] {
		result = append(result, s.recipes[id])
	}
	return result, total
}

// GetByID returns a single recipe by its ID.
func (s *InMemoryRecipeStore) GetByID(id string) (models.DetailedRecipe, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.recipes[id]
	return r, ok
}

// Activate sets a recipe's IsActive flag to true.
func (s *InMemoryRecipeStore) Activate(id string) (models.DetailedRecipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.recipes[id]
	if !ok {
		return models.DetailedRecipe{}, errNotFound
	}
	r.IsActive = true
	r.UpdatedAt = time.Now().UTC()
	s.recipes[id] = r
	return r, nil
}

// Deactivate sets a recipe's IsActive flag to false.
func (s *InMemoryRecipeStore) Deactivate(id string) (models.DetailedRecipe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.recipes[id]
	if !ok {
		return models.DetailedRecipe{}, errNotFound
	}
	r.IsActive = false
	r.UpdatedAt = time.Now().UTC()
	s.recipes[id] = r
	return r, nil
}

// SQLiteRecipeStore persists recipes in the gateway's shared SQLite DB.
// On first open (empty table) it seeds the built-in recipe templates so
// the gallery is non-empty for new deployments — same UX as the in-memory
// store, but durable across restarts.
type SQLiteRecipeStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteRecipeStore returns a recipe store backed by db. Seeds built-ins
// inside a single transaction the first time the table is empty.
func NewSQLiteRecipeStore(db *sql.DB, logger *slog.Logger) (*SQLiteRecipeStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &SQLiteRecipeStore{db: db, logger: logger.With(slog.String("component", "recipe_store"))}
	if err := s.seedIfEmpty(); err != nil {
		return nil, fmt.Errorf("recipe store seed: %w", err)
	}
	return s, nil
}

func (s *SQLiteRecipeStore) seedIfEmpty() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_recipes").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	builtins := []models.DetailedRecipe{
		models.EmailTriageRecipe(),
		models.CalendarManagerRecipe(),
		models.ResearchMonitorRecipe(),
		models.FilesystemOrganizerRecipe(),
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, r := range builtins {
		body, err := json.Marshal(r)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		active := 0
		if r.IsActive {
			active = 1
		}
		if _, err := tx.Exec(
			`INSERT INTO gateway_recipes (id, is_active, created_at, updated_at, body, seq) VALUES (?, ?, ?, ?, ?, ?)`,
			r.ID, active, now, now, string(body), i,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// List returns a page of recipes ordered by insertion sequence.
func (s *SQLiteRecipeStore) List(page, pageSize int) ([]models.DetailedRecipe, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM gateway_recipes").Scan(&total); err != nil {
		s.logger.Warn("recipe count failed", slog.String("error", err.Error()))
		return nil, 0
	}

	offset := (page - 1) * pageSize
	rows, err := s.db.Query(
		"SELECT body FROM gateway_recipes ORDER BY seq ASC LIMIT ? OFFSET ?",
		pageSize, offset,
	)
	if err != nil {
		s.logger.Warn("recipe list failed", slog.String("error", err.Error()))
		return nil, total
	}
	defer rows.Close()

	var out []models.DetailedRecipe
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			s.logger.Warn("recipe scan failed", slog.String("error", err.Error()))
			continue
		}
		var r models.DetailedRecipe
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			s.logger.Warn("recipe unmarshal failed", slog.String("error", err.Error()))
			continue
		}
		out = append(out, r)
	}
	return out, total
}

// GetByID returns a single recipe by id.
func (s *SQLiteRecipeStore) GetByID(id string) (models.DetailedRecipe, bool) {
	var body string
	if err := s.db.QueryRow("SELECT body FROM gateway_recipes WHERE id = ?", id).Scan(&body); err != nil {
		return models.DetailedRecipe{}, false
	}
	var r models.DetailedRecipe
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return models.DetailedRecipe{}, false
	}
	return r, true
}

// Activate flips is_active=1 and refreshes the recipe's UpdatedAt.
func (s *SQLiteRecipeStore) Activate(id string) (models.DetailedRecipe, error) {
	return s.setActive(id, true)
}

// Deactivate flips is_active=0.
func (s *SQLiteRecipeStore) Deactivate(id string) (models.DetailedRecipe, error) {
	return s.setActive(id, false)
}

func (s *SQLiteRecipeStore) setActive(id string, active bool) (models.DetailedRecipe, error) {
	r, ok := s.GetByID(id)
	if !ok {
		return models.DetailedRecipe{}, errNotFound
	}
	r.IsActive = active
	r.UpdatedAt = time.Now().UTC()
	body, err := json.Marshal(r)
	if err != nil {
		return models.DetailedRecipe{}, fmt.Errorf("recipe marshal: %w", err)
	}
	flag := 0
	if active {
		flag = 1
	}
	res, err := s.db.Exec(
		"UPDATE gateway_recipes SET is_active = ?, updated_at = ?, body = ? WHERE id = ?",
		flag, r.UpdatedAt.UTC().Format(time.RFC3339Nano), string(body), id,
	)
	if err != nil {
		return models.DetailedRecipe{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return models.DetailedRecipe{}, errors.New("recipe not updated")
	}
	return r, nil
}

// RecipeHandler holds the dependencies for recipe HTTP handlers.
type RecipeHandler struct {
	logger     *slog.Logger
	store      RecipeStore
	eventStore EventStore
}

// NewRecipeHandler creates a RecipeHandler with the given store and event store.
func NewRecipeHandler(logger *slog.Logger, store RecipeStore, eventStore EventStore) *RecipeHandler {
	return &RecipeHandler{
		logger:     logger,
		store:      store,
		eventStore: eventStore,
	}
}

// ListRecipes handles GET /api/v1/recipes.
func (rh *RecipeHandler) ListRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.list_recipes")
	defer span.End(rh.logger)

	page, pageSize := parsePagination(r)

	recipes, total := rh.store.List(page, pageSize)

	rh.logger.InfoContext(ctx, "recipes listed",
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
		slog.Int("total", total),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"recipes":   recipes,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetRecipe handles GET /api/v1/recipes/{id}.
func (rh *RecipeHandler) GetRecipe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "recipe id is required",
		})
		return
	}

	recipe, ok := rh.store.GetByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "recipe not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, recipe)
}

// ActivateRecipe handles POST /api/v1/recipes/{id}/activate.
func (rh *RecipeHandler) ActivateRecipe(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.activate_recipe")
	defer span.End(rh.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "recipe id is required",
		})
		return
	}

	recipe, err := rh.store.Activate(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "recipe not found",
		})
		return
	}

	// Emit activation event.
	payload, _ := json.Marshal(map[string]string{
		"recipe_id":   recipe.ID,
		"recipe_name": recipe.Name,
	})
	evt := events.NewEvent(events.EventKindRecipeActivated, "", "gateway", payload)
	rh.eventStore.Append(evt)

	rh.logger.InfoContext(ctx, "recipe activated",
		slog.String("recipe_id", recipe.ID),
		slog.String("recipe_name", recipe.Name),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"recipe":   recipe,
		"event_id": evt.ID,
		"status":   "activated",
	})
}

// DeactivateRecipe handles POST /api/v1/recipes/{id}/deactivate.
func (rh *RecipeHandler) DeactivateRecipe(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.deactivate_recipe")
	defer span.End(rh.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "recipe id is required",
		})
		return
	}

	recipe, err := rh.store.Deactivate(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "recipe not found",
		})
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"recipe_id":   recipe.ID,
		"recipe_name": recipe.Name,
	})
	evt := events.NewEvent(events.EventKindRecipeDeactivated, "", "gateway", payload)
	rh.eventStore.Append(evt)

	rh.logger.InfoContext(ctx, "recipe deactivated",
		slog.String("recipe_id", recipe.ID),
		slog.String("recipe_name", recipe.Name),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"recipe":   recipe,
		"event_id": evt.ID,
		"status":   "deactivated",
	})
}

// parsePagination extracts page and pageSize from query params with defaults.
func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}
	return page, pageSize
}
