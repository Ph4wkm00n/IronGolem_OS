// Package main is the entry point for the IronGolem OS Defense service.
//
// The defense service implements the "self-defending loop" by detecting
// threats such as prompt injection, SSRF attempts, and anomalous behavior.
// It operates as a sidecar to the gateway and other services, evaluating
// requests before they reach the runtime.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/defense/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("defense")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	detector := internal.NewCompositeThreatDetector(logger)
	h := internal.NewHandler(logger, detector)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /api/v1/scan/prompt", h.ScanPrompt)
	mux.HandleFunc("POST /api/v1/scan/url", h.ScanURL)
	mux.HandleFunc("POST /api/v1/scan/request", h.ScanRequest)

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
