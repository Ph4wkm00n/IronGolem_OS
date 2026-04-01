// Package main is the entry point for the IronGolem OS Tenancy service.
//
// The tenancy service manages multi-tenant isolation including workspace
// creation, tenant lifecycle, and role-based access control. It supports
// all three deployment modes: Solo, Household, and Team.
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
	"github.com/Ph4wkm00n/IronGolem_OS/services/tenancy/internal"
)

func main() {
	cfg := telemetry.DefaultConfig("tenancy")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	mgr := internal.NewTenancyManager(logger)
	h := internal.NewHandler(logger, mgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.HealthCheck)

	// Tenant routes.
	mux.HandleFunc("POST /api/v1/tenants", h.CreateTenant)
	mux.HandleFunc("GET /api/v1/tenants", h.ListTenants)
	mux.HandleFunc("GET /api/v1/tenants/{id}", h.GetTenant)

	// Workspace routes.
	mux.HandleFunc("POST /api/v1/tenants/{tenant_id}/workspaces", h.CreateWorkspace)
	mux.HandleFunc("GET /api/v1/tenants/{tenant_id}/workspaces", h.ListWorkspaces)

	// User routes.
	mux.HandleFunc("POST /api/v1/tenants/{tenant_id}/workspaces/{workspace_id}/users", h.AddUser)
	mux.HandleFunc("GET /api/v1/tenants/{tenant_id}/workspaces/{workspace_id}/users", h.ListUsers)

	addr := envOrDefault("TENANCY_ADDR", ":8084")
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
		logger.Info("tenancy service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("tenancy service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("tenancy service stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
