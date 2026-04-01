// Package main is the entry point for the IronGolem OS Scheduler service.
//
// The scheduler manages one-time and recurring jobs for automation recipes,
// agent tasks, and maintenance operations. It exposes an HTTP API for job
// creation, listing, and status queries.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
	"github.com/Ph4wkm00n/IronGolem_OS/services/scheduler/internal"
)

func main() {
	cfg := telemetry.DefaultConfig("scheduler")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	sched := internal.NewScheduler(logger)
	h := internal.NewHandler(logger, sched)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /api/v1/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/v1/jobs", h.ListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", h.GetJob)
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", h.CancelJob)

	addr := envOrDefault("SCHEDULER_ADDR", ":8081")
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
		logger.Info("scheduler starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Start the scheduler background loop.
	go sched.Run(ctx)

	<-ctx.Done()
	logger.Info("scheduler shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	sched.Stop()
	logger.Info("scheduler stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
