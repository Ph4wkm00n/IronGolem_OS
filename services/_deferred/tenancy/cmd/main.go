// Package main is the entry point for the IronGolem OS Tenancy service.
//
// The tenancy service manages the multi-tenant hierarchy: tenants, workspaces,
// and user roles. It enforces isolation boundaries and supports all three
// deployment modes (Solo, Household, Team).
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
	"github.com/Ph4wkm00n/IronGolem_OS/services/tenancy/internal"
)

func main() {
	cfg := telemetry.DefaultConfig("tenancy")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	mgr := internal.NewTenantManager(logger)
	h := &handler{logger: logger, mgr: mgr}

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "tenancy",
			"time":    time.Now().UTC(),
		})
	})

	// Workspace routes.
	mux.HandleFunc("POST /api/v1/workspaces", h.CreateWorkspace)
	mux.HandleFunc("GET /api/v1/workspaces", h.ListWorkspaces)
	mux.HandleFunc("GET /api/v1/workspaces/{id}", h.GetWorkspace)

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

type handler struct {
	logger *slog.Logger
	mgr    *internal.TenantManager
}

// CreateWorkspaceRequest is the JSON body for workspace creation.
type CreateWorkspaceRequest struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
}

func (h *handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "tenancy.create_workspace")
	defer span.End(h.logger)

	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	ws, err := h.mgr.CreateWorkspace(ctx, req.TenantID, req.Name)
	if err != nil {
		h.logger.WarnContext(ctx, "workspace creation failed",
			slog.String("error", err.Error()),
		)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

func (h *handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "tenancy.list_workspaces")
	defer span.End(h.logger)

	tenantID := r.URL.Query().Get("tenant_id")

	workspaces, err := h.mgr.ListWorkspaces(ctx, tenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces": workspaces,
		"count":      len(workspaces),
	})
}

func (h *handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "tenancy.get_workspace")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "workspace id is required",
		})
		return
	}

	// Tenant isolation: require tenant_id header.
	tenantID := r.Header.Get("X-Tenant-ID")

	ws, err := h.mgr.GetWorkspace(ctx, id, tenantID)
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, internal.ErrAccessDenied) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, ws)
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
