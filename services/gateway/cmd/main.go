// Package main is the entry point for the IronGolem OS Gateway service.
//
// The gateway is the front door for all external communication. It handles
// message ingress and egress, connector lifecycle management, recipe gallery
// and activation, approval workflows, event timeline, and applies Layer 1
// (Gateway Identity) of the five-layer security model.
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

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("gateway")
	logger := telemetry.SetupLogger(cfg)

	slog.SetDefault(logger)

	connMgr := connector.NewManager(logger)
	h := handler.New(logger, connMgr)

	// Shared stores for the recipe, approval, and timeline subsystems.
	eventStore := handler.NewInMemoryEventStore()
	recipeStore := handler.NewInMemoryRecipeStore()
	approvalStore := handler.NewInMemoryApprovalStore()

	recipeHandler := handler.NewRecipeHandler(logger, recipeStore, eventStore)
	approvalHandler := handler.NewApprovalHandler(logger, approvalStore, eventStore)
	timelineHandler := handler.NewTimelineHandler(logger, eventStore)

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", h.HealthCheck)

	// Message routes.
	mux.HandleFunc("POST /api/v1/messages/inbound", h.MessageInbound)
	mux.HandleFunc("POST /api/v1/messages/outbound", h.MessageOutbound)

	// Connector routes.
	mux.HandleFunc("GET /api/v1/connectors/{id}/status", h.ConnectorStatus)
	mux.HandleFunc("POST /api/v1/connectors/{id}/connect", h.ConnectorConnect)
	mux.HandleFunc("POST /api/v1/connectors/{id}/disconnect", h.ConnectorDisconnect)
	mux.HandleFunc("POST /api/v1/connectors/{id}/heartbeat", h.ConnectorHeartbeat)

	// Recipe routes.
	mux.HandleFunc("GET /api/v1/recipes", recipeHandler.ListRecipes)
	mux.HandleFunc("GET /api/v1/recipes/{id}", recipeHandler.GetRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/activate", recipeHandler.ActivateRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/deactivate", recipeHandler.DeactivateRecipe)

	// Approval routes.
	mux.HandleFunc("GET /api/v1/approvals", approvalHandler.ListApprovals)
	mux.HandleFunc("GET /api/v1/approvals/{id}", approvalHandler.GetApproval)
	mux.HandleFunc("POST /api/v1/approvals/{id}/approve", approvalHandler.ApproveAction)
	mux.HandleFunc("POST /api/v1/approvals/{id}/deny", approvalHandler.DenyAction)

	// Timeline / event routes.
	mux.HandleFunc("GET /api/v1/events", timelineHandler.ListEvents)
	mux.HandleFunc("GET /api/v1/events/{id}", timelineHandler.GetEvent)

	addr := envOrDefault("GATEWAY_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("gateway starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("gateway shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	connMgr.DisconnectAll()
	logger.Info("gateway stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
