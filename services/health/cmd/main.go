// Package main is the entry point for the IronGolem OS Health service.
//
// The health service monitors all platform services and agents via heartbeats,
// detects degradation, and triggers self-healing actions. It implements the
// "self-healing loop" described in the architecture.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/health/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("health")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	hbMgr := internal.NewHeartbeatManager(logger)
	h := internal.NewHandler(logger, hbMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /api/v1/heartbeats", h.RecordHeartbeat)
	mux.HandleFunc("GET /api/v1/services", h.ListServices)
	mux.HandleFunc("GET /api/v1/services/{name}/status", h.ServiceStatus)
	mux.HandleFunc("POST /api/v1/services/{name}/pause", h.PauseService)
	mux.HandleFunc("POST /api/v1/services/{name}/resume", h.ResumeService)

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

	go hbMgr.Run(ctx)

	go func() {
		logger.Info("health service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("health service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("health service stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
