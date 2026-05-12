// Package main is the entry point for the IronGolem OS Research service.
//
// The research service implements Phase 3: Adaptive Intelligence, providing
// topic tracking, source fetching, contradiction detection, and research
// brief generation via the auto-research loop.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
	"github.com/Ph4wkm00n/IronGolem_OS/services/research/internal"
)

func main() {
	cfg := telemetry.DefaultConfig("research")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	// Initialize the in-memory topic store.
	store := internal.NewMemoryTopicStore()
	tracker := internal.NewTopicTracker(store, logger)

	// Initialize the source fetcher with SSRF-safe defaults.
	fetcherCfg := internal.DefaultHTTPFetcherConfig()
	fetcher := internal.NewHTTPFetcher(fetcherCfg, logger)

	// Initialize trust scorer and rate limiter.
	scorer := internal.NewTrustScorer(logger)
	limiter := internal.NewRateLimiter(2*time.Second, logger)

	// The LLM provider is optional; when nil, analysis features are degraded.
	// In production this would be wired via the provider registry.
	var analyzer *internal.ContentAnalyzer
	var detector *internal.ContradictionDetector
	var generator *internal.BriefGenerator

	// Placeholder: in production, resolve provider from registry/config.
	// analyzer = internal.NewContentAnalyzer(llmProvider, modelName, logger)
	// detector = internal.NewContradictionDetector(llmProvider, modelName, logger)
	// generator = internal.NewBriefGenerator(analyzer, detector, scorer, logger)
	_ = scorer // used when provider is available
	_ = analyzer
	_ = detector
	_ = generator

	// Initialize the scheduler.
	schedCfg := internal.DefaultSchedulerConfig()
	scheduler := internal.NewResearchScheduler(
		tracker, fetcher, limiter, nil, // generator is nil until provider is configured
		schedCfg, logger,
	)

	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "research",
			"time":    time.Now().UTC(),
		})
	})

	// GET /api/v1/research/topics - list tracked topics.
	mux.HandleFunc("GET /api/v1/research/topics", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.list_topics")
		defer span.End(logger)

		workspaceID := r.URL.Query().Get("workspace_id")
		topics, err := tracker.ListTopics(ctx, workspaceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list topics",
			})
			return
		}
		if topics == nil {
			topics = []models.TrackedTopic{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"topics": topics,
			"count":  len(topics),
		})
	})

	// POST /api/v1/research/topics - add a tracked topic.
	mux.HandleFunc("POST /api/v1/research/topics", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.add_topic")
		defer span.End(logger)

		var topic models.TrackedTopic
		if err := json.NewDecoder(r.Body).Decode(&topic); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		created, err := tracker.AddTopic(ctx, topic)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		logger.InfoContext(ctx, "topic created",
			slog.String("id", created.ID),
			slog.String("name", created.Name),
		)
		writeJSON(w, http.StatusCreated, created)
	})

	// GET /api/v1/research/topics/{id} - get topic with latest brief.
	mux.HandleFunc("GET /api/v1/research/topics/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.get_topic")
		defer span.End(logger)

		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "topic id is required",
			})
			return
		}

		topic, err := tracker.GetTopic(ctx, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"error": "topic not found",
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to get topic",
			})
			return
		}

		// Fetch the latest brief for this topic.
		briefs, _ := store.ListBriefs(ctx, id, 1)
		var latestBrief *models.ResearchBrief
		if len(briefs) > 0 {
			latestBrief = &briefs[0]
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"topic":        topic,
			"latest_brief": latestBrief,
		})
	})

	// DELETE /api/v1/research/topics/{id} - remove a topic.
	mux.HandleFunc("DELETE /api/v1/research/topics/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.remove_topic")
		defer span.End(logger)

		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "topic id is required",
			})
			return
		}

		if err := tracker.RemoveTopic(ctx, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"error": "topic not found",
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to remove topic",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "removed",
		})
	})

	// POST /api/v1/research/topics/{id}/check - force immediate check.
	mux.HandleFunc("POST /api/v1/research/topics/{id}/check", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.force_check")
		defer span.End(logger)

		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "topic id is required",
			})
			return
		}

		if err := scheduler.CheckTopicNow(ctx, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"error": "topic not found",
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]string{
			"status": "check initiated",
		})
	})

	// GET /api/v1/research/briefs - list recent briefs.
	mux.HandleFunc("GET /api/v1/research/briefs", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.list_briefs")
		defer span.End(logger)

		topicID := r.URL.Query().Get("topic_id")
		briefs, err := store.ListBriefs(ctx, topicID, 50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list briefs",
			})
			return
		}
		if briefs == nil {
			briefs = []models.ResearchBrief{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"briefs": briefs,
			"count":  len(briefs),
		})
	})

	// GET /api/v1/research/briefs/{id} - get brief detail.
	mux.HandleFunc("GET /api/v1/research/briefs/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.get_brief")
		defer span.End(logger)

		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "brief id is required",
			})
			return
		}

		brief, err := store.GetBrief(ctx, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{
					"error": "brief not found",
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to get brief",
			})
			return
		}

		writeJSON(w, http.StatusOK, brief)
	})

	// GET /api/v1/research/contradictions - list all detected contradictions.
	mux.HandleFunc("GET /api/v1/research/contradictions", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "research.list_contradictions")
		defer span.End(logger)

		contradictions, err := store.ListContradictions(ctx, 100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list contradictions",
			})
			return
		}
		if contradictions == nil {
			contradictions = []models.Contradiction{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"contradictions": contradictions,
			"count":          len(contradictions),
		})
	})

	addr := envOrDefault("RESEARCH_ADDR", ":8085")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("research service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Start the background research scheduler.
	go scheduler.Run(ctx)

	<-ctx.Done()
	logger.Info("research service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	scheduler.Stop()
	logger.Info("research service stopped")
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
