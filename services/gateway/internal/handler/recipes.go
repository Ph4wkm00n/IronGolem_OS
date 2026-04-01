package handler

import (
	"encoding/json"
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

// RecipeHandler holds the dependencies for recipe HTTP handlers.
type RecipeHandler struct {
	logger     *slog.Logger
	store      RecipeStore
	eventStore *InMemoryEventStore
}

// NewRecipeHandler creates a RecipeHandler with the given store and event store.
func NewRecipeHandler(logger *slog.Logger, store RecipeStore, eventStore *InMemoryEventStore) *RecipeHandler {
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
