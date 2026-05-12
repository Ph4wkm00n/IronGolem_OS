// Package main is the entry point for the IronGolem OS Fleet Management service.
//
// The fleet service manages multiple IronGolem OS instances, receiving health
// reports from each and providing aggregated fleet-wide status for the admin
// dashboard. Default port: 8087.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/fleet/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("fleet")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	mgr := internal.NewFleetManager(logger)

	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "fleet",
			"time":    time.Now().UTC(),
		})
	})

	// List all managed instances.
	mux.HandleFunc("GET /api/v1/fleet/instances", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "fleet.list_instances")
		defer span.End(logger)

		instances := mgr.List()
		writeJSON(w, http.StatusOK, map[string]any{
			"instances": instances,
			"count":     len(instances),
		})
	})

	// Register a new instance.
	mux.HandleFunc("POST /api/v1/fleet/instances", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "fleet.register_instance")
		defer span.End(logger)

		var inst internal.Instance
		if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
			logger.WarnContext(ctx, "invalid instance body",
				slog.String("error", err.Error()),
			)
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if inst.ID == "" || inst.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "id and name are required",
			})
			return
		}

		registered := mgr.Register(inst)
		writeJSON(w, http.StatusCreated, registered)
	})

	// Get instance detail.
	mux.HandleFunc("GET /api/v1/fleet/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "fleet.get_instance")
		defer span.End(logger)

		id := r.PathValue("id")
		inst := mgr.Get(id)
		if inst == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "instance not found",
			})
			return
		}

		writeJSON(w, http.StatusOK, inst)
	})

	// Receive a health report from an instance.
	mux.HandleFunc("POST /api/v1/fleet/instances/{id}/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "fleet.receive_health")
		defer span.End(logger)

		id := r.PathValue("id")

		var report internal.HealthReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			logger.WarnContext(ctx, "invalid health report body",
				slog.String("error", err.Error()),
			)
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		report.InstanceID = id

		if ok := mgr.RecordHealth(report); !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "instance not found",
			})
			return
		}

		logger.InfoContext(ctx, "health report received",
			slog.String("instance_id", id),
		)

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "recorded",
		})
	})

	// Fleet-wide dashboard overview.
	mux.HandleFunc("GET /api/v1/fleet/overview", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "fleet.overview")
		defer span.End(logger)

		overview := mgr.Overview()
		writeJSON(w, http.StatusOK, overview)
	})

	addr := envOrDefault("FLEET_ADDR", ":8087")
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
		logger.Info("fleet service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("fleet service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("fleet service stopped")
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
