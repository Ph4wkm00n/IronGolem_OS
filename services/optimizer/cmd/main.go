// Package main is the entry point for the IronGolem OS Optimizer service.
//
// The optimizer service implements the adaptive intelligence layer (Phase 3).
// It learns user preferences from behavioral signals, runs shadow-mode
// experiments to test optimizations, manages prompt caching, and benchmarks
// LLM providers. All learned changes start in shadow mode and require
// explicit promotion before affecting production behavior.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/optimizer/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/provider"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("optimizer")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	// Initialize core components.
	prefStore := internal.NewMemoryPreferenceStore()
	learner := internal.NewPreferenceLearner(prefStore, logger)
	shadow := internal.NewShadowController(logger)
	promptOpt := internal.NewPromptOptimizer(logger)
	registry := provider.NewProviderRegistry(logger)
	providerOpt := internal.NewProviderOptimizer(registry, logger)
	depthCtrl := internal.NewReasoningDepthController(logger)
	cache := internal.NewPromptCache(30*time.Minute, logger)

	// Suppress unused variable warnings for components used only by handlers.
	_ = depthCtrl

	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "optimizer",
			"time":    time.Now().UTC(),
		})
	})

	// --- Preference endpoints ---

	// List all learned preferences.
	mux.HandleFunc("GET /api/v1/optimizer/preferences", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.list_preferences")
		defer span.End(logger)

		prefs := prefStore.ListAll()
		writeJSON(w, http.StatusOK, map[string]any{
			"preferences": prefs,
			"count":       len(prefs),
		})
	})

	// Ingest a learning signal.
	mux.HandleFunc("POST /api/v1/optimizer/signals", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.ingest_signal")
		defer span.End(logger)

		var signal models.LearningSignal
		if err := json.NewDecoder(r.Body).Decode(&signal); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if signal.UserID == "" || signal.Action == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "user_id and action are required",
			})
			return
		}

		learner.ProcessSignal(signal)

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "recorded",
		})
	})

	// Promote a preference from shadow mode.
	mux.HandleFunc("POST /api/v1/optimizer/preferences/", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.preference_action")
		defer span.End(logger)

		// Parse /:id/promote or /:id/reject from the path.
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/optimizer/preferences/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "expected /preferences/:id/promote or /preferences/:id/reject",
			})
			return
		}

		id := parts[0]
		action := parts[1]

		pref, ok := prefStore.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "preference not found",
			})
			return
		}

		switch action {
		case "promote":
			pref.ShadowMode = false
			pref.UpdatedAt = time.Now().UTC()
			if err := prefStore.Save(pref); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to promote preference",
				})
				return
			}
			logger.Info("preference promoted from shadow mode",
				slog.String("id", id),
				slog.String("key", pref.Key),
			)
			writeJSON(w, http.StatusOK, map[string]any{
				"status":     "promoted",
				"preference": pref,
			})

		case "reject":
			if err := prefStore.Delete(id); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to reject preference",
				})
				return
			}
			logger.Info("preference rejected and removed",
				slog.String("id", id),
				slog.String("key", pref.Key),
			)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "rejected",
			})

		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "unknown action: " + action + " (expected promote or reject)",
			})
		}
	})

	// --- Experiment endpoints ---

	// List all experiments.
	mux.HandleFunc("GET /api/v1/optimizer/experiments", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.list_experiments")
		defer span.End(logger)

		experiments := shadow.ListExperiments()
		writeJSON(w, http.StatusOK, map[string]any{
			"experiments": experiments,
			"count":       len(experiments),
		})
	})

	// Create a new experiment.
	mux.HandleFunc("POST /api/v1/optimizer/experiments", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.create_experiment")
		defer span.End(logger)

		var req struct {
			Name        string                `json:"name"`
			Description string                `json:"description"`
			Type        internal.ExperimentType `json:"type"`
			Baseline    string                `json:"baseline"`
			Candidate   string                `json:"candidate"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "name is required",
			})
			return
		}

		exp := shadow.StartExperiment(req.Name, req.Description, req.Type, req.Baseline, req.Candidate)
		writeJSON(w, http.StatusCreated, exp)
	})

	// Get experiment by ID, or perform actions (promote/reject/revert).
	mux.HandleFunc("GET /api/v1/optimizer/experiments/", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.get_experiment")
		defer span.End(logger)

		id := strings.TrimPrefix(r.URL.Path, "/api/v1/optimizer/experiments/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "experiment id is required",
			})
			return
		}

		exp, ok := shadow.GetExperiment(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "experiment not found",
			})
			return
		}

		writeJSON(w, http.StatusOK, exp)
	})

	// Experiment actions: promote, reject, revert, stop.
	mux.HandleFunc("POST /api/v1/optimizer/experiments/", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.experiment_action")
		defer span.End(logger)

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/optimizer/experiments/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "expected /experiments/:id/:action",
			})
			return
		}

		id := parts[0]
		action := parts[1]

		var err error
		switch action {
		case "promote":
			err = shadow.PromoteExperiment(id)
		case "reject":
			err = shadow.RejectExperiment(id)
		case "revert":
			err = shadow.RevertExperiment(id)
		case "stop":
			err = shadow.StopExperiment(id)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "unknown action: " + action,
			})
			return
		}

		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		exp, _ := shadow.GetExperiment(id)
		writeJSON(w, http.StatusOK, exp)
	})

	// --- Cache endpoints ---

	// Get cache performance metrics.
	mux.HandleFunc("GET /api/v1/optimizer/cache/metrics", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.cache_metrics")
		defer span.End(logger)

		metrics := cache.Metrics()
		writeJSON(w, http.StatusOK, metrics)
	})

	// --- Benchmark endpoints ---

	// Run a provider benchmark.
	mux.HandleFunc("POST /api/v1/optimizer/benchmark", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "optimizer.run_benchmark")
		defer span.End(logger)

		var req struct {
			Task      string   `json:"task"`
			Providers []string `json:"providers"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if req.Task == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "task is required",
			})
			return
		}

		// If no providers specified, benchmark all registered providers.
		providers := req.Providers
		if len(providers) == 0 {
			providers = registry.List()
		}

		results := providerOpt.BenchmarkProviders(ctx, req.Task, providers)
		writeJSON(w, http.StatusOK, map[string]any{
			"task":       req.Task,
			"benchmarks": results,
			"count":      len(results),
			"summary":    internal.FormatBenchmarkSummary(results),
		})
	})

	// --- Prompt optimization endpoints ---

	// Create a prompt variant.
	mux.HandleFunc("POST /api/v1/optimizer/prompts/variants", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.create_variant")
		defer span.End(logger)

		var req struct {
			BasePrompt   string `json:"base_prompt"`
			Modification string `json:"modification"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if req.BasePrompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "base_prompt is required",
			})
			return
		}

		variant := promptOpt.CreateVariant(req.BasePrompt, req.Modification)
		writeJSON(w, http.StatusCreated, variant)
	})

	// --- Reasoning depth endpoint ---

	// Get recommended reasoning depth for a task.
	mux.HandleFunc("GET /api/v1/optimizer/depth", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "optimizer.reasoning_depth")
		defer span.End(logger)

		taskType := r.URL.Query().Get("task_type")
		if taskType != "" {
			depth := depthCtrl.DepthForTaskType(taskType)
			writeJSON(w, http.StatusOK, map[string]any{
				"task_type":  taskType,
				"depth":      depth,
				"max_tokens": internal.DepthToMaxTokens(depth),
				"temperature": internal.DepthToTemperature(depth),
			})
			return
		}

		// If no task_type, use complexity parameter.
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "task_type query parameter is required",
		})
	})

	addr := envOrDefault("OPTIMIZER_ADDR", ":8086")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start background cache eviction.
	cacheDone := make(chan struct{})
	go cache.RunEvictionLoop(cacheDone, 5*time.Minute)

	go func() {
		logger.Info("optimizer service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("optimizer service shutting down")

	close(cacheDone)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("optimizer service stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
