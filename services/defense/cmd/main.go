// Package main is the entry point for the IronGolem OS Defense service.
//
// The defense service is the self-defending layer of the platform. It checks
// inputs for prompt injection, SSRF attempts, and anomalous behavior patterns.
// Blocked actions and quarantined items are tracked for audit and review.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/defense/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("defense")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	detector := internal.NewThreatDetector(logger, internal.DetectorConfig{
		InjectionThreshold: 0.7,
		AnomalyWindow:     5 * time.Minute,
		AnomalyMaxVolume:  100,
	})

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "defense",
			"time":    time.Now().UTC(),
		})
	})

	// Check input for threats.
	mux.HandleFunc("POST /api/v1/defense/check", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "defense.check_input")
		defer span.End(logger)

		var req internal.CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if req.Input == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "input is required",
			})
			return
		}

		assessment := detector.Assess(ctx, req)

		// If any threats were detected and blocked, emit an event.
		if assessment.Blocked {
			payload, _ := json.Marshal(events.ThreatPayload{
				ThreatType:  assessment.PrimaryThreat,
				Severity:    assessment.Severity,
				Description: assessment.Summary,
				Score:       assessment.Score,
				Blocked:     true,
			})
			evt := events.NewEvent(events.EventKindThreatDetected, req.TenantID, "defense", payload)
			logger.WarnContext(ctx, "threat blocked",
				slog.String("event_id", evt.ID),
				slog.String("threat_type", assessment.PrimaryThreat),
				slog.Float64("score", assessment.Score),
				slog.String("tenant_id", req.TenantID),
			)
		}

		writeJSON(w, http.StatusOK, assessment)
	})

	// List blocked actions.
	mux.HandleFunc("GET /api/v1/defense/blocked", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_blocked")
		defer span.End(logger)

		blocked := detector.ListBlocked()
		writeJSON(w, http.StatusOK, map[string]any{
			"blocked": blocked,
			"count":   len(blocked),
		})
	})

	// List quarantined items.
	mux.HandleFunc("GET /api/v1/defense/quarantine", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_quarantined")
		defer span.End(logger)

		quarantined := detector.ListQuarantined()
		writeJSON(w, http.StatusOK, map[string]any{
			"quarantined": quarantined,
			"count":       len(quarantined),
		})
	})

	addr := envOrDefault("DEFENSE_ADDR", ":8083")
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
		logger.Info("defense service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("defense service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("defense service stopped")
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
