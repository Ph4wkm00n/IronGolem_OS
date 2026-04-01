package handler_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// newRecipeTestServer sets up an httptest.Server with the recipe routes wired
// through the same mux pattern used in cmd/main.go.
func newRecipeTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	eventStore := handler.NewInMemoryEventStore()
	recipeStore := handler.NewInMemoryRecipeStore()
	rh := handler.NewRecipeHandler(logger, recipeStore, eventStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/recipes", rh.ListRecipes)
	mux.HandleFunc("GET /api/v1/recipes/{id}", rh.GetRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/activate", rh.ActivateRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/deactivate", rh.DeactivateRecipe)

	return httptest.NewServer(mux)
}

func TestListRecipes(t *testing.T) {
	srv := newRecipeTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/recipes")
	if err != nil {
		t.Fatalf("GET /api/v1/recipes failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Recipes  []models.DetailedRecipe `json:"recipes"`
		Total    int                     `json:"total"`
		Page     int                     `json:"page"`
		PageSize int                     `json:"page_size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Total != 4 {
		t.Errorf("expected 4 built-in recipes, got %d", body.Total)
	}
	if len(body.Recipes) != 4 {
		t.Errorf("expected 4 recipes in page, got %d", len(body.Recipes))
	}
}

func TestGetRecipe(t *testing.T) {
	srv := newRecipeTestServer(t)
	defer srv.Close()

	recipeID := "recipe-email-triage-001"

	resp, err := http.Get(srv.URL + "/api/v1/recipes/" + recipeID)
	if err != nil {
		t.Fatalf("GET /api/v1/recipes/%s failed: %v", recipeID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var recipe models.DetailedRecipe
	if err := json.NewDecoder(resp.Body).Decode(&recipe); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if recipe.ID != recipeID {
		t.Errorf("expected recipe ID %q, got %q", recipeID, recipe.ID)
	}
	if recipe.Name != "Email Triage Assistant" {
		t.Errorf("expected name %q, got %q", "Email Triage Assistant", recipe.Name)
	}
}

func TestGetRecipeNotFound(t *testing.T) {
	srv := newRecipeTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/recipes/nonexistent")
	if err != nil {
		t.Fatalf("GET /api/v1/recipes/nonexistent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "recipe not found" {
		t.Errorf("expected error %q, got %q", "recipe not found", body["error"])
	}
}

func TestActivateRecipe(t *testing.T) {
	srv := newRecipeTestServer(t)
	defer srv.Close()

	recipeID := "recipe-calendar-mgr-001"

	// Activate
	resp, err := http.Post(srv.URL+"/api/v1/recipes/"+recipeID+"/activate", "application/json", nil)
	if err != nil {
		t.Fatalf("POST activate failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		Recipe  models.DetailedRecipe `json:"recipe"`
		EventID string                `json:"event_id"`
		Status  string                `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !body.Recipe.IsActive {
		t.Error("expected recipe to be active after activation")
	}
	if body.Status != "activated" {
		t.Errorf("expected status %q, got %q", "activated", body.Status)
	}
	if body.EventID == "" {
		t.Error("expected non-empty event_id")
	}

	// Confirm via GET
	getResp, err := http.Get(srv.URL + "/api/v1/recipes/" + recipeID)
	if err != nil {
		t.Fatalf("GET recipe failed: %v", err)
	}
	defer getResp.Body.Close()

	var recipe models.DetailedRecipe
	if err := json.NewDecoder(getResp.Body).Decode(&recipe); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if !recipe.IsActive {
		t.Error("expected recipe to remain active on subsequent GET")
	}
}

func TestDeactivateRecipe(t *testing.T) {
	srv := newRecipeTestServer(t)
	defer srv.Close()

	recipeID := "recipe-research-mon-001"

	// First activate
	activateResp, err := http.Post(srv.URL+"/api/v1/recipes/"+recipeID+"/activate", "application/json", nil)
	if err != nil {
		t.Fatalf("POST activate failed: %v", err)
	}
	activateResp.Body.Close()

	if activateResp.StatusCode != http.StatusOK {
		t.Fatalf("activate expected 200, got %d", activateResp.StatusCode)
	}

	// Then deactivate
	deactivateResp, err := http.Post(srv.URL+"/api/v1/recipes/"+recipeID+"/deactivate", "application/json", nil)
	if err != nil {
		t.Fatalf("POST deactivate failed: %v", err)
	}
	defer deactivateResp.Body.Close()

	if deactivateResp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate expected 200, got %d", deactivateResp.StatusCode)
	}

	var body struct {
		Recipe models.DetailedRecipe `json:"recipe"`
		Status string                `json:"status"`
	}
	if err := json.NewDecoder(deactivateResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Recipe.IsActive {
		t.Error("expected recipe to be inactive after deactivation")
	}
	if body.Status != "deactivated" {
		t.Errorf("expected status %q, got %q", "deactivated", body.Status)
	}
}
