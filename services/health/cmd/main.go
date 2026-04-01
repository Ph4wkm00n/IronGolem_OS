// Package main is the entry point for the IronGolem OS Health service.
//
// The health service monitors the status of all services and agents via
// heartbeats. It detects missed heartbeats, tracks service health states,
// and triggers self-healing when services degrade.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/health/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("health")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	hbMgr := internal.NewHeartbeatManager(logger, internal.HeartbeatConfig{
		Timeout:       30 * time.Second,
		CheckInterval: 10 * time.Second,
	})

	// Set up the self-healing engine with escalating recovery strategies.
	healer := internal.NewDefaultHealer(logger)
	hbMgr.SetHealer(healer)

	// Set up the system monitor for resource and connector tracking.
	monitor := internal.NewSystemMonitor(logger, hbMgr, internal.MonitorConfig{
		PollInterval:     15 * time.Second,
		ConnectorTimeout: 5 * time.Second,
	})

	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "health",
			"time":    time.Now().UTC(),
		})
	})

	// Overall system health summary.
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "health.system_status")
		defer span.End(logger)

		summary := hbMgr.SystemSummary(ctx)
		writeJSON(w, http.StatusOK, summary)
	})

	// List all heartbeat records.
	mux.HandleFunc("GET /api/v1/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "health.list_heartbeats")
		defer span.End(logger)

		records := hbMgr.ListAll(ctx)
		writeJSON(w, http.StatusOK, map[string]any{
			"heartbeats": records,
			"count":      len(records),
		})
	})

	// Receive a heartbeat check-in from a service or agent.
	mux.HandleFunc("POST /api/v1/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "health.receive_heartbeat")
		defer span.End(logger)

		var payload events.HeartbeatPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			logger.WarnContext(ctx, "invalid heartbeat body",
				slog.String("error", err.Error()),
			)
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if payload.ServiceName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "service_name is required",
			})
			return
		}

		hbMgr.RecordHeartbeat(ctx, payload)

		logger.InfoContext(ctx, "heartbeat received",
			slog.String("service", payload.ServiceName),
			slog.String("status", string(payload.Status)),
		)

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "recorded",
		})
	})

	// Health summary endpoint for the Health Center dashboard.
	mux.HandleFunc("GET /api/v1/health/summary", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "health.summary")
		defer span.End(logger)

		summary := monitor.Summary(ctx)
		writeJSON(w, http.StatusOK, summary)
	})

	// Connector health status.
	mux.Handle("GET /api/v1/health/connectors", monitor.HandleConnectors())

	// Resource usage endpoint.
	mux.Handle("GET /api/v1/health/resources", monitor.HandleResources())

	// Healing log endpoint.
	mux.HandleFunc("GET /api/v1/health/healing", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "health.healing_log")
		defer span.End(logger)

		entries := healer.Log().Entries()
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"count":   len(entries),
		})
	})

	addr := envOrDefault("HEALTH_ADDR", ":8082")
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
		logger.Info("health service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Start background heartbeat monitor and system monitor.
	go hbMgr.Run(ctx)
	go monitor.Run(ctx)

	<-ctx.Done()
	logger.Info("health service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	hbMgr.Stop()
	monitor.Stop()
	logger.Info("health service stopped")
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
